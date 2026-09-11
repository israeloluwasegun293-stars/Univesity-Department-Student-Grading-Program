package handlers

// pages.go serves the HTML pages (landing + dashboard) and the embedded
// static assets. Every page is rendered from templates/ which is compiled
// into the binary via embed.

import (
	"embed"
	"html/template"
	"net/http"
)

// pageRenderer parses the embedded templates once at startup.
type pageRenderer struct {
	landing *template.Template
	app     *template.Template
}

func newPageRenderer(content embed.FS) (*pageRenderer, error) {
	landing, err := template.ParseFS(content, "templates/landing.html")
	if err != nil {
		return nil, err
	}
	app, err := template.ParseFS(content, "templates/app.html")
	if err != nil {
		return nil, err
	}
	return &pageRenderer{landing: landing, app: app}, nil
}

// handleLanding serves the marketing/overview page at "/".
func (p *pageRenderer) handleLanding(w http.ResponseWriter, r *http.Request) {
	// The mux registers "/" as a subtree; anything that is not exactly the
	// root path falls through to the embedded asset server instead.
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := p.landing.Execute(w, nil); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

// handleApp serves the dashboard page at "/app".
func (p *pageRenderer) handleApp(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/app" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := p.app.Execute(w, nil); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}
