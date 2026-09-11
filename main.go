package main

// main.go — entrypoint. Run the web server with:
//
//	go run .            (web mode, default, http://localhost:8080)
//	go run . -console   (the original terminal-only experience)
//
// The grading logic itself lives in grades.go; the HTTP layer in server.go.

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"
)

func main() {
	console := flag.Bool("console", false, "run the original terminal-based grading program")
	addr := flag.String("addr", ":8080", "address for the web server to listen on")
	flag.Parse()

	if *console {
		runConsole()
		return
	}

	// newMux() returns our dedicated ServeMux (see server.go) — we never
	// touch http.HandleFunc or the DefaultServeMux global.
	mux := newMux()

	srv := &http.Server{
		Addr:              *addr,
		Handler:           withSecurityHeaders(mux),
		ReadHeaderTimeout: 5 * time.Second, // slowloris protection
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	fmt.Println("================================================================")
	fmt.Println("        CGPA GRADING SYSTEM — WEB EDITION IS LIVE!              ")
	fmt.Println("================================================================")
	fmt.Printf("  Serving the frontend at:  http://localhost%s\n", *addr)
	fmt.Println("  API endpoints:            /api/grade  /api/results  /api/reset")
	fmt.Println("  Press Ctrl+C to stop.")
	fmt.Println("================================================================")

	log.Fatal(srv.ListenAndServe())
}

// withSecurityHeaders adds strict transport-adjacent headers to every
// response, including the API routes.
func withSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}
