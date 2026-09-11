# ClassEdge — University CGPA Grading System (Web Edition)

A university department grading system that computes **accurate, per-course CGPA** on the standard 100-point breakdown / 5.0 scale, for **one student or a whole class**, and presents everything on an animated dashboard with a nicely formatted, printable class report. Backend in **Go** (stdlib only), frontend in vanilla HTML/CSS/JS — both compiled into a single self-contained binary.

## Grading scale — standard 100-point breakdown

| Score range | Letter | Grade point | Remark    |
|-------------|--------|-------------|-----------|
| 70 – 100%   | A      | 5.0         | Excellent |
| 60 – 69%    | B      | 4.0         | Very Good |
| 50 – 59%    | C      | 3.0         | Credit    |
| 45 – 49%    | D      | 2.0         | Pass      |
| 40 – 44%    | E      | 1.0         | Low Pass  |
| 0 – 39%     | F      | 0.0         | Fail      |

**Why the GPA calculation is accurate:** every course score is graded *individually* and the student's CGPA is the **mean of the per-course grade points** — the standard CGPA method. Example: scores 85 (→A, 5 pts) and 65 (→B, 4 pts) give CGPA = (5+4)/2 = **4.50**. Grading the 75% average alone would give A → 5.00, which is inflated and wrong.

## Features

- **Landing page** (`/`) explaining what the system is for: features, the full grading scale with a worked example, and how the 3-step workflow runs.
- **Batch grading** (`/app`): choose how many students you're calculating for (1–100), get a form per student with **live per-course grade chips** as you type, then reveal everything at once.
- **Formatted class report**: a monospace, department-results-sheet style table for all students + class metrics — with **Copy** and **Print** buttons.
- **Per-course honesty**: each course shows its own letter grade and grade point, and the CGPA is derived from them.
- **Class metrics**: students graded, class average CGPA, highest/lowest averages, A–F distribution chart, leaderboard.
- **Confetti** when the batch contains an A student. Results should be fun.
- Original console mode preserved: `go run . -console` (also asks for the student count now).

## Project structure

```
main.go               entrypoint: flags, hardened http.Server, embed + global headers
console.go            original terminal experience (-console flag)
grading/              core academic logic — the single source of truth
  grading.go            per-course grade points, CGPA, band labels, class metrics
  report.go             formatted class report renderer (shared web + console)
handlers/             HTTP layer
  server.go             dedicated ServeMux + all API handlers
  pages.go              landing + dashboard page rendering, static assets
  payload.go            JSON request/response contracts
  store.go              mutex-guarded in-memory roster (cap 100)
templates/            HTML pages
  landing.html          marketing / overview page
  app.html              dashboard with batch wizard
static/               frontend assets
  css/style.css         unified stylesheet (landing + dashboard)
  js/app.js             dashboard logic (batch wizard, report, metrics)
  js/landing.js         landing page interactions
```

Templates and static assets are **embedded into the binary** (`go:embed`), so `go build` produces one portable executable.

## How to run

```bash
go run .                # web mode → http://localhost:8080
go run . -addr :3000    # listen on a different port
go run . -console       # original terminal program (now with batch support)
```

- Landing page: **http://localhost:8080/**
- Dashboard: **http://localhost:8080/app**

## API endpoints

| Endpoint          | Method | Purpose                                        |
|-------------------|--------|------------------------------------------------|
| `/`               | GET    | Landing page                                   |
| `/app`            | GET    | Dashboard                                      |
| `/api/grade`      | POST   | Grade + store **one** student                  |
| `/api/grade/batch`| POST   | Grade + store **1–100 students** in one call   |
| `/api/results`    | GET    | Current roster + class metrics + report text   |
| `/api/reset`      | POST   | Clear the roster                               |
| `/api/health`     | GET    | Liveness probe (returns Go version)            |

Examples:

```bash
# single student
curl -X POST localhost:8080/api/grade \
  -H 'Content-Type: application/json' \
  -d '{"name":"Ada Lovelace","matrikNo":"CSC/2026/0142","marks":[85,65,72]}'

# whole class in one call
curl -X POST localhost:8080/api/grade/batch \
  -H 'Content-Type: application/json' \
  -d '{"students":[
        {"name":"Ada Lovelace","matrikNo":"CSC/001","marks":[85,65]},
        {"name":"Alan Turing","matrikNo":"CSC/002","marks":[91,78,64]}
      ]}'
```

Batch validation is all-or-nothing: if any student entry is invalid, nothing is stored and the error names the offending student number.

## Security notes

The server deliberately builds a **dedicated mux** — the package-level `http.HandleFunc` global is never used anywhere:

```go
mux := http.NewServeMux()
mux.HandleFunc("/api/grade/batch", handleGradeBatch)
```

Imported packages therefore cannot silently register routes on our server; the routing surface stays small and auditable. Additional hardening:

- Request bodies capped at 1 MiB via `http.MaxBytesReader` (`413` on overflow)
- `DisallowUnknownFields` so unexpected JSON fields are rejected
- Explicit method checks with `Allow` headers on every API route
- Bounded, mutex-guarded in-memory store (max 100 students)
- Security headers on every response: `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy`, `Permissions-Policy`
- Server timeouts (`ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, `IdleTimeout`) against slow-loris attacks
- Frontend HTML-escapes all server-provided strings before rendering (XSS defence in depth)

## Biggest technical challenge

Getting the GPA semantics right. The naive approach — convert the overall average to one letter and use its point — inflates results whenever scores span multiple bands. The fix was to grade **each course individually** against the standard 100-point breakdown and average the grade points, encoded once in `grading.BandFor()` and reused by the web UI, the batch report and the console mode, so no mode can drift from the others.

*Program was written by [Okunade Israel Oluwasegun]*
