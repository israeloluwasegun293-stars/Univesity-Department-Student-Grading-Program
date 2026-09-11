package main

// server.go wires the HTTP layer: one dedicated, explicit ServeMux — the
// DefaultServeMux global (and its http.HandleFunc convenience function) is
// deliberately NOT used, so third-party libraries imported for other
// purposes cannot silently register routes on our server. That keeps the
// routing surface small and auditable.

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"runtime"
	"sync"
	"time"
)

// =========================================================================
// SECURE IN-MEMORY DATA STORE
// A small TEMPLATED store guarded by a mutex. We do not use maps as sets
// of user-supplied keys without bounding them, and we cap the roster at
// the 100 students our coursework defines, so a runaway client cannot
// exhaust server memory.
// =========================================================================

const maxRosterSize = totalStudents // 100, same cap as the coursework brief

type resultStore struct {
	mu      sync.Mutex
	records []StudentRecord
	metrics ClassMetrics
}

var store = &resultStore{metrics: newClassMetrics()}

func (s *resultStore) add(rec StudentRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = append(s.records, rec)
	s.metrics.add(rec)
	s.metrics.finalize()
	if len(s.records) > maxRosterSize {
		// Keep only the most recent maxRosterSize records.
		s.records = s.records[len(s.records)-maxRosterSize:]
	}
}

func (s *resultStore) snapshot() ([]StudentRecord, ClassMetrics) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]StudentRecord, len(s.records))
	copy(out, s.records)
	return out, s.metrics
}

func (s *resultStore) reset() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records = nil
	s.metrics = newClassMetrics()
}

// =========================================================================
// RESPONSE HELPERS
// =========================================================================

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

// =========================================================================
// HANDLERS
// =========================================================================

// handleGrade is the main API endpoint. It accepts one student record,
// validates and grades it, stores it, and returns the full class
// dashboard payload.
func handleGrade(w http.ResponseWriter, r *http.Request) {
	// Security: this mux has exactly one prefix-registered subtree ("/"),
	// so an old Go < 1.22 pattern could match other methods. We enforce
	// the contract explicitly.
	if r.URL.Path != "/api/grade" {
		writeJSON(w, http.StatusNotFound, apiError{"Route not found"})
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		writeJSON(w, http.StatusMethodNotAllowed, apiError{"Method not allowed. Use POST."})
		return
	}

	// Security: cap request body size so a hostile client cannot ship us
	// gigabytes of JSON. 1 MiB is far beyond what a single student record
	// needs.
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	var req GradeRequest
	if err := dec.Decode(&req); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeJSON(w, http.StatusRequestEntityTooLarge, apiError{"Request body too large (1 MiB max)."})
			return
		}
		writeJSON(w, http.StatusBadRequest, apiError{"Malformed JSON body: " + err.Error()})
		return
	}

	rec, err := processStudent(req.Name, req.MatrikNo, req.Marks)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{err.Error()})
		return
	}

	store.add(rec)
	respondDashboard(w)
}

// handleResults returns the current class dashboard without grading a new
// student. Handy for refreshing after a page reload.
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

// handleReset clears the roster. Bound to POST only so it cannot be
// triggered by link prefetching or CSRF-via-GET.
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

// respondDashboard renders the shared payload used by both /api/grade and
// /api/results.
func respondDashboard(w http.ResponseWriter) {
	records, metrics := store.snapshot()
	writeJSON(w, http.StatusOK, DashboardResponse{
		Students:   records,
		Summary:    metrics,
		TotalCount: len(records),
		ServerTime: time.Now().Format(time.RFC1123),
		GoVersion:  runtime.Version(),
	})
}

// handleHealth gives load balancers and the curious a cheap liveness probe.
func handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/api/health" {
		writeJSON(w, http.StatusNotFound, apiError{"Route not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "go": runtime.Version()})
}

// newMux builds the application's dedicated ServeMux. Every route the
// server serves is registered HERE and only here — grep for
// "http.HandleFunc" and you will find nothing on the DefaultServeMux.
func newMux() *http.ServeMux {
	mux := http.NewServeMux()

	// API routes (registered as exact patterns).
	mux.HandleFunc("/api/grade", handleGrade)
	mux.HandleFunc("/api/results", handleResults)
	mux.HandleFunc("/api/reset", handleReset)
	mux.HandleFunc("/api/health", handleHealth)

	// Static frontend. We register the pattern on our mux and rely on
	// http.FileServer's built-in protections: it refuses paths containing
	// ".." and never serves files outside the tree root.
	fileServer := http.FileServer(http.Dir(staticDir))
	mux.Handle("/", secureStatic(fileServer))

	return mux
}

// secureStatic wraps the file server with browser-facing security headers.
func secureStatic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		next.ServeHTTP(w, r)
	})
}
