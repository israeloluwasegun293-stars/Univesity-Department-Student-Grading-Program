# ClassEdge — University CGPA Grading System (Web Edition)

A university department grading system that takes student information, computes totals, averages, letter grades and GPA on the **5.0 scale**, and presents everything on a live, animated dashboard. The backend is written in **Go**; the frontend is vanilla HTML/CSS/JS served by the same binary — no framework, no build step.

> Originally a terminal-only program (`go run . -console` still runs it). Now it has a full web UI.

## Features

- **Grade a student**: name, matric number and up to 5 course scores (0–100)
- **Live projected grade preview** as you type, before you even submit
- **Result reveal panel**: animated grade medal, GPA ring gauge and a verdict line — with confetti for A grades
- **Class metrics**: students graded, class average GPA, highest/lowest averages
- **Grade distribution chart** (A–F) and a **leaderboard** table
- **Roster cap of 100 students** (oldest entries roll off, per the coursework brief)

## Grading scale (5.0 system)

| Average  | Grade | GPA |
|----------|-------|-----|
| ≥ 70     | A     | 5.0 |
| ≥ 60     | B     | 4.0 |
| ≥ 50     | C     | 3.0 |
| ≥ 45     | D     | 2.0 |
| ≥ 40     | E     | 1.0 |
| < 40     | F     | 0.0 |

## How to run

1. Install Go: https://golang.org/dl/
2. Clone the repository and navigate into the project directory.
3. Start the web server:

   ```bash
   go run .
   ```

4. Open **http://localhost:8080** in your browser.

Options:

```bash
go run . -addr :3000    # listen on a different port
go run . -console       # run the original terminal-based program
```

## API endpoints

| Endpoint       | Method | Purpose                                  |
|----------------|--------|------------------------------------------|
| `/`            | GET    | The dashboard frontend                   |
| `/api/grade`   | POST   | Grade + store one student (JSON)         |
| `/api/results` | GET    | Current roster + class metrics           |
| `/api/reset`   | POST   | Clear the roster                         |
| `/api/health`  | GET    | Liveness probe (returns Go version)      |

Example:

```bash
curl -X POST localhost:8080/api/grade \
  -H 'Content-Type: application/json' \
  -d '{"name":"Ada Lovelace","matrikNo":"CSC/2026/0142","marks":[85,72,90]}'
```

## Security notes

The server deliberately builds a **dedicated mux**:

```go
mux := http.NewServeMux()   // NOT the package-level http.HandleFunc
mux.HandleFunc("/api/grade", handleGrade)
```

Registering on an explicit `ServeMux` instead of the `DefaultServeMux` global means third-party packages imported anywhere in the binary cannot silently register extra routes on our server — the routing surface stays small and auditable. Additional hardening:

- Explicit method checks with `Allow` headers on every API route
- Request bodies capped at 1 MiB via `http.MaxBytesReader` (`413` on overflow)
- `DisallowUnknownFields` so unexpected JSON fields are rejected
- Input length limits and 0–100 score validation server-side
- Security headers on every response: `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy`, `Permissions-Policy`
- Server timeouts (`ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, `IdleTimeout`) against slow-loris style attacks
- The frontend HTML-escapes all server-provided strings before rendering (XSS defence in depth)

## Project structure

```
main.go      entrypoint: flags, hardened http.Server, global middleware
server.go    the dedicated mux + API handlers + security headers
grades.go    core grading logic (single source of truth for both modes)
payload.go   JSON request/response contracts
console.go   the original terminal experience (-console flag)
static/      index.html, style.css, app.js — the frontend
```

## Biggest technical challenge

Implementing the GPA calculation logic: mapping the university's letter-grade bands to grade points, then computing GPA from the course averages. Researching the standard university 5.0 grading scale and encoding it as one shared `gradeAndGPA()` function fixed it — and now the console and web modes cannot drift apart, because both call the same code.

*Program was written by [Okunade Israel Oluwasegun]*
