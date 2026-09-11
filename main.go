package main

// main.go — entrypoint. Run the web server with:
//
//	go run .            (web mode, default, http://localhost:8080)
//	go run . -console   (the original terminal-only experience)
//
// Project layout: handlers/ (HTTP layer), templates/ (HTML pages),
// static/ (CSS + JS assets), grading/ (core academic logic).
// Templates and static assets are embedded into the binary, so the
// compiled program is fully self-contained.

import (
	"embed"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/israeloluwasegun293-stars/classedge/handlers"
)

//go:embed templates static
var content embed.FS

func main() {
	console := flag.Bool("console", false, "run the original terminal-based grading program")
	addr := flag.String("addr", ":8080", "address for the web server to listen on")
	flag.Parse()

	if *console {
		runConsole()
		return
	}

	// Build our dedicated ServeMux inside handlers — the package-level
	// http.HandleFunc global is never used anywhere in this codebase.
	mux := handlers.NewMux(content)

	srv := &http.Server{
		Addr:              *addr,
		Handler:           withSecurityHeaders(mux),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}

	fmt.Println("================================================================")
	fmt.Println("        CLASSEDGE — CGPA GRADING SYSTEM IS LIVE!                ")
	fmt.Println("================================================================")
	fmt.Printf("  Landing page:   http://localhost%s/\n", *addr)
	fmt.Printf("  Dashboard:      http://localhost%s/app\n", *addr)
	fmt.Println("  API endpoints:  /api/grade  /api/grade/batch  /api/results  /api/reset")
	fmt.Println("  Press Ctrl+C to stop.")
	fmt.Println("================================================================")

	log.Fatal(srv.ListenAndServe())
}

// withSecurityHeaders adds protective headers to every response.
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
