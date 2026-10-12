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

**4. Carryovers / repeats.** Both attempts stay on the record by default: pass the retake and the old F still counts. A replacement-policy CGPA cannot be calculated from aggregate prior ΣQP/ΣCU alone; it requires course-level attempt history. The API omits `EffectiveCGPA` when that history is unavailable (it is present for a first-semester record, where it equals CGPA).

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
- **File import**: import CSV, Excel `.xlsx` workbooks, or text-based PDFs. Imported rows become the same Go request structs and use the same validation and grading algorithm.
- **Download results** as CSV, JSON or plain text.
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

Open **http://localhost:8080/login** to log in or create an account with a Gmail address. After login, users land on `/home`, where they can choose the grading dashboard; administrators also get a link to `/admin`. The first account is an administrator; later self-registrations are standard users. Admins can monitor account and roster counts, create accounts, change roles, enable/disable accounts, reset passwords and delete accounts. Passwords must be at least 8 characters and use 3 of lowercase, uppercase, numbers and symbols; passphrases of 12 or more characters are also accepted. The user database stores bcrypt password hashes in `classedge-users.json`; set `CLASSEDGE_USER_DB` to use another file location. Keep this file private and backed up. Login sessions expire after 24 hours and are cleared when the server restarts.

- Landing page: **http://localhost:8080/**
- Dashboard: **http://localhost:8080/app**

## API

| Endpoint          | Method | Purpose                                      |
|-------------------|--------|----------------------------------------------|
| `/`               | GET    | Landing page                                 |
| `/login`          | GET    | Account creation (first run) or login        |
| `/home`           | GET    | Authenticated post-login landing page        |
| `/app`            | GET    | Authenticated dashboard                      |
| `/admin`          | GET    | Administrator panel                          |
| `/api/auth/register` | POST | Create an account with a Gmail address       |
| `/api/auth/login`  | POST   | Start a login session                        |
| `/api/auth/logout` | POST   | End the current session                      |
| `/api/grade`      | POST   | Grade + store one student                    |
| `/api/grade/batch`| POST   | Grade + store 1–100 students in one call     |
| `/api/import`     | POST   | Import and grade CSV, XLSX or text PDF        |
| `/api/export?format=csv\|json\|txt` | GET | Download current results                   |
| `/api/export/template.csv` | GET | Download a ready-to-fill import template    |
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

`prior` is optional — omit it (or zero both fields) for a first-semester student. Each response record includes `Courses[]` with per-course `gradePoint` and `qualityPoints`, plus `TotalCU`, `TotalQP`, `SemesterGPA` and cumulative `CGPA`. `EffectiveCGPA` is present for first-semester records only; calculating a replacement-policy CGPA for carryovers needs course-level prior history.

### File import layout

Use one row per course with these headers: `name, matricNo, courseCode, creditUnits, score, previousCreditUnits, previousQualityPoints, previousCGPA`. Repeat each student's name, matric number and prior totals on each of their course rows. Leave prior totals blank for first-semester students. The server groups rows by matric number and validates the entire batch before saving it. XLSX imports read the first worksheet. Header capitalization and spaces are ignored; common labels such as `Student Name`, `Matric No`, `Course`, `CU` and `Score` are accepted.

PDF files must contain selectable text and a matching header row. Pipe-separated and spreadsheet-style aligned columns are supported. Scanned/image PDFs need OCR before upload. Imports accept `.csv`, `.xlsx` or `.pdf`, with a 10 MiB file cap, 1,000 row cap and 100 student cap. Legacy `.xls` workbooks are not supported.

## Security notes

- Dedicated `mux := http.NewServeMux()` — the package-level `http.HandleFunc` global is never used anywhere, so imported packages cannot silently register routes.
- 1 MiB JSON body cap and 10 MiB file cap (`http.MaxBytesReader`), bounded spreadsheet expansion, explicit method checks with `Allow` headers.
- Bounded, mutex-guarded in-memory store (max 100 students).
- Gmail account authentication with bcrypt password hashes, protected grading APIs and HttpOnly, SameSite session cookies.
- Security headers on every response; slow-loris server timeouts.
- Frontend HTML-escapes all server-provided strings (XSS defence in depth).

## Biggest technical challenge

Modelling the academic rules, not just the math: the weighted QP = CU × GP foundation, keeping failed units in the denominator, computing cumulative CGPA from running TQP/TCU totals (never averaging GPAs), and preserving repeat attempts by default — all encoded once in `grading/` and shared by the web UI, batch report and console mode.

*Program was written by [Okunade Israel Oluwasegun]*
