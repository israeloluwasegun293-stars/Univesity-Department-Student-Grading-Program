# ClassEdge — University CGPA Grading System (Web Edition)

A university department grading system that computes a **true, weighted, cumulative CGPA** — every course carries its **credit units**, every course is graded on the standard 100-point breakdown, and the cumulative CGPA is derived from running **Total Quality Points ÷ Total Credit Units** across semesters. It handles one student or a whole class, shows a nicely formatted printable report, and ships as a single self-contained Go binary with an animated dashboard.

## Why this is a *real* CGPA calculator

A CGPA calculator cannot just average raw percentages. ClassEdge implements the academic rules:

**1. Weighted calculations (Quality Points).** Each course collects **code, credit units (1–6) and score (0–100)**. Per course:

```
GradePoint  = f(score)        // standard 100-point breakdown
QualityPts  = CreditUnits × GradePoint
```

An A in a 4-unit course (20 QP) carries double the weight of an A in a 2-unit course (10 QP).

**2. Semester GPA vs Cumulative CGPA.**

```
Semester GPA = Σ QualityPoints / Σ CreditUnits          (this submission)
CGPA         = Σ (ALL Quality Points) / Σ (ALL Credit Units)   (cumulative)
```

The cumulative CGPA is **never** the average of semester GPAs. Example: 20 prior CU with TQP 100 (GPA 5.00), then a failed 2-unit course (0 QP): CGPA = 100/22 = **4.55** — averaging GPAs would wrongly give 2.50.

**3. Fails still count.** An F contributes 0 quality points but its credit units stay in the denominator — that is what drags a GPA down.

**4. Carryovers / repeats.** Both attempts stay on the record by default (the common public-university rule): pass the retake and the old F still counts. The API also exposes an `EffectiveCGPA` best-attempt variant for institutions with a replacement policy.

**5. Input validation.** Scores outside 0–100, credit units outside 1–6, negative/blank fields, inconsistent prior totals (QP without CU, QP > 5.0 × CU) are all rejected server-side with precise messages.

### Grading scale (standard 100-point breakdown, 5.0 system)

| Score range | Letter | Grade point | Remark    |
|-------------|--------|-------------|-----------|
| 70 – 100%   | A      | 5.0         | Excellent |
| 60 – 69%    | B      | 4.0         | Very Good |
| 50 – 59%    | C      | 3.0         | Credit    |
| 45 – 49%    | D      | 2.0         | Pass      |
| 40 – 44%    | E      | 1.0         | Low Pass  |
| 0 – 39%     | F      | 0.0         | Fail      |

**Worked example** — 85 in a 4-unit course, 65 in a 2-unit course:
85 → A (5.0) × 4 = 20 QP · 65 → B (4.0) × 2 = 8 QP → CGPA = 28 ÷ 6 = **4.67** ✅ (a flat grade-point average would say 4.50, ignoring that the A came in a course worth double the units).

## Features

- **Landing page** (`/`) explaining the system: features, the grading scale with a worked weighted example, and the 3-step workflow.
- **Batch wizard** (`/app`): choose how many students (1–100), a form per student with **per-course rows** (code + credit units + score), a **carry-over toggle** for previous ΣQP/ΣCU, and **live weighted preview chips** (`A·B·A → GPA 4.67`).
- **Formatted class report** (TCU / TQP / AVG% / GRADE / CGPA columns) with Copy and Print buttons — shared with console mode so both output identically.
- **Per-course working** renderer (`FormatCourseBreakdown`) showing each course's CU × GP = QP.
- **Class metrics**: count, highest/lowest averages, highest/lowest CGPA, class mean CGPA, A–F distribution, leaderboard.
- **Confetti** for A students. Console mode preserved: `go run . -console`.

## Project structure

```
main.go               entrypoint: flags, hardened http.Server, go:embed
console.go            terminal experience (-console): full weighted + cumulative flow
grading/              core academic logic (single source of truth)
  grading.go            CourseInput{Code,CU,Score}, QP=CU×GP, cumulative CGPA,
                        prior-record validation, carryover policy, class metrics
  grading_test.go       tests proving weighted + cumulative + validation behaviour
  report.go             formatted class report + per-course breakdown
handlers/             HTTP layer
  server.go             dedicated ServeMux + API handlers (single + batch)
  pages.go              landing + dashboard rendering, static assets
  payload.go            JSON contracts (courses[], prior{})
  store.go              mutex-guarded roster store (cap 100)
templates/            landing.html, app.html
static/               css/style.css, js/app.js, js/landing.js
```

## How to run

```bash
go test ./...           # run the grading engine tests
go run .                # web mode → http://localhost:8080
go run . -addr :3000    # different port
go run . -console       # terminal mode (weighted + cumulative too)
```

- Landing page: **http://localhost:8080/**
- Dashboard: **http://localhost:8080/app**

## API

| Endpoint          | Method | Purpose                                      |
|-------------------|--------|----------------------------------------------|
| `/`               | GET    | Landing page                                 |
| `/app`            | GET    | Dashboard                                    |
| `/api/grade`      | POST   | Grade + store one student                    |
| `/api/grade/batch`| POST   | Grade + store 1–100 students in one call     |
| `/api/results`    | GET    | Current roster + class metrics + report text |
| `/api/reset`      | POST   | Clear the roster                             |
| `/api/health`     | GET    | Liveness probe                               |

### Payload shape

```json
{
  "name": "Ada Lovelace",
  "matrikNo": "CSC/2026/0142",
  "courses": [
    { "code": "CSC301", "creditUnits": 4, "score": 85 },
    { "code": "MTH102", "creditUnits": 2, "score": 65 }
  ],
  "prior": { "previousCreditUnits": 36, "previousQualityPoints": 142 }
}
```

`prior` is optional — omit it (or zero both fields) for a first-semester student. Each response record includes `Courses[]` with per-course `gradePoint` and `qualityPoints`, plus `TotalCU`, `TotalQP`, `SemesterGPA`, cumulative `CGPA` and `EffectiveCGPA`.

## Security notes

- Dedicated `mux := http.NewServeMux()` — the package-level `http.HandleFunc` global is never used anywhere, so imported packages cannot silently register routes.
- 1 MiB body cap (`http.MaxBytesReader`, `413`), `DisallowUnknownFields`, explicit method checks with `Allow` headers.
- Bounded, mutex-guarded in-memory store (max 100 students).
- Security headers on every response; slow-loris server timeouts.
- Frontend HTML-escapes all server-provided strings (XSS defence in depth).

## Biggest technical challenge

Modelling the academic rules, not just the math: the weighted QP = CU × GP foundation, keeping failed units in the denominator, computing cumulative CGPA from running TQP/TCU totals (never averaging GPAs), and the both-attempts vs replacement carryover policies — all encoded once in `grading/` with unit tests locking the behaviour in, and shared verbatim by the web UI, the batch report and the console mode.

*Program was written by [Okunade Israel Oluwasegun]*
