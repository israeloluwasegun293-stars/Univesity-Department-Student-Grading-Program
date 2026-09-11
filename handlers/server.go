package handlers

// server.go wires the HTTP layer: one dedicated, explicit ServeMux — the
// DefaultServeMux global (and its http.HandleFunc convenience function) is
// deliberately NOT used, so third-party libraries imported for other
// purposes cannot silently register routes on our server.

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"runtime"

	"github.com/israeloluwasegun293-stars/classedge/grading"
)

// NewMux builds the application's dedicated ServeMux. Every route the
// server serves is registered HERE and only here.
func NewMux(content embed.FS) *http.ServeMux {
	pages, err := newPageRenderer(content)
	if err != nil {
		log.Fatalf("parse templates: %v", err)
	}

	// Serve embedded static assets from the filesystem root so template
	// references like /static/css/style.css resolve.
	assets := content

	mux := http.NewServeMux()

	// Pages.
	mux.HandleFunc("/", pages.handleLanding)
	mux.HandleFunc("/app", pages.handleApp)

	// API.
	mux.HandleFunc("/api/grade", handleGrade)
	mux.HandleFunc("/api/grade/batch", handleGradeBatch)
	mux.HandleFunc("/api/results", handleResults)
	mux.HandleFunc("/api/reset", handleReset)
	mux.HandleFunc("/api/health", handleHealth)

	// Embedded static assets.
	mux.Handle("/static/", secureStatic(http.FileServer(http.FS(assets))))

	return mux
}

var store = newResultStore()

// respondDashboard renders the shared payload used after grading.
func respondDashboard(w http.ResponseWriter) {
	records, metrics := store.snapshot()
	writeJSON(w, http.StatusOK, DashboardResponse{
		Students:   records,
		Summary:    metrics,
		Report:     grading.FormatClassReport(records, metrics),
		TotalCount: len(records),
		GoVersion:  runtime.Version(),
	})
}

// handleGrade accepts one student record, validates and grades it, stores
// it, and returns the full dashboard payload.
func handleGrade(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/grade" {
		writeJSON(w, http.StatusNotFound, apiError{"Route not found"})
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		writeJSON(w, http.StatusMethodNotAllowed, apiError{"Method not allowed. Use POST."})
		return
	}

	var req GradeRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	rec, err := grading.ProcessStudent(req.Name, req.MatrikNo, req.Courses, req.Prior)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{err.Error()})
		return
	}

	store.add(rec)
	respondDashboard(w)
}

// handleGradeBatch accepts a whole class at once (1–100 students). Every
// entry is validated and graded; on any invalid entry the whole batch is
// rejected with the index of the offending student so the user can fix it.
func handleGradeBatch(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/grade/batch" {
		writeJSON(w, http.StatusNotFound, apiError{"Route not found"})
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		writeJSON(w, http.StatusMethodNotAllowed, apiError{"Method not allowed. Use POST."})
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	var req BatchRequest
	if err := dec.Decode(&req); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeJSON(w, http.StatusRequestEntityTooLarge, apiError{"Request body too large (1 MiB max)."})
			return
		}
		writeJSON(w, http.StatusBadRequest, apiError{"Malformed JSON body: " + err.Error()})
		return
	}

	if len(req.Students) < 1 {
		writeJSON(w, http.StatusBadRequest, apiError{"Batch must contain at least 1 student."})
		return
	}
	if len(req.Students) > grading.MaxStudents {
		writeJSON(w, http.StatusBadRequest, apiError{"Batch is capped at 100 students."})
		return
	}

	records := make([]grading.StudentRecord, 0, len(req.Students))
	for i, s := range req.Students {
		rec, err := grading.ProcessStudent(s.Name, s.MatrikNo, s.Courses, s.Prior)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, apiError{fmt.Sprintf("Student #%d: %s", i+1, err.Error())})
			return
		}
		records = append(records, rec)
	}

	for _, rec := range records {
		store.add(rec)
	}
	respondDashboard(w)
}

// handleResults returns the current dashboard without grading anyone.
func handleResults(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/results" {
		writeJSON(w, http.StatusNotFound, apiError{"Route not found"})
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		writeJSON(w, http.StatusMethodNotAllowed, apiError{"Method not allowed. Use GET."})
		return
	}
	respondDashboard(w)
}

// handleReset clears the roster (POST only, so it cannot be triggered by
// link prefetching or CSRF-via-GET).
func handleReset(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/reset" {
		writeJSON(w, http.StatusNotFound, apiError{"Route not found"})
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		writeJSON(w, http.StatusMethodNotAllowed, apiError{"Method not allowed. Use POST."})
		return
	}
	store.reset()
	writeJSON(w, http.StatusOK, map[string]string{"status": "roster cleared"})
}

// handleHealth gives load balancers and the curious a cheap liveness probe.
func handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/health" {
		writeJSON(w, http.StatusNotFound, apiError{"Route not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "go": runtime.Version()})
}

// decodeJSON is the shared strict JSON body reader for single-student
// endpoints: 1 MiB cap + unknown-field rejection.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeJSON(w, http.StatusRequestEntityTooLarge, apiError{"Request body too large (1 MiB max)."})
			return false
		}
		writeJSON(w, http.StatusBadRequest, apiError{"Malformed JSON body: " + err.Error()})
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		log.Printf("json encode: %v", err)
	}
}

type apiError struct {
	Error string `json:"error"`
}

// secureStatic wraps the asset server with browser-facing security headers.
func secureStatic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		next.ServeHTTP(w, r)
	})
}
