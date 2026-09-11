// Package grading holds the core academic logic of the CGPA system — the
// single source of truth used by the web handlers, the batch report and
// the original console mode.
//
// ── The academic model ────────────────────────────────────────────────
// A good CGPA calculator cannot just average raw percentages: every
// course carries Credit Units (CU) that weight its influence, and a
// cumulative CGPA must be derived from running totals — never by
// averaging semester GPAs.
//
// For every course the calculator accepts: a course code/name, the
// credit units, and the raw score (0–100). It then computes:
//
//	GradePoint  — from the score via the standard 5.0-scale mapping
//	QualityPts  = CreditUnits × GradePoint          (per course)
//	SemesterGPA = Σ QualityPoints / Σ CreditUnits   (per semester)
//	Cumulative  = (priorTQP + Σ QualityPoints) / (priorTCU + Σ CreditUnits)
//
// Key academic rules encoded here:
//   - Weighted: an A in a 5-unit course moves the CGPA five times more
//     than an A in a 1-unit course.
//   - Fails still count: an F contributes 0 quality points but its
//     credit units still enter the denominator — this is what drags a
//     GPA down.
//   - Cumulative, not averaged: CGPA is always TQP/TCU over ALL attempts,
//     never (GPA1 + GPA2) / 2.
//   - Repeat/carryover policy: both attempts stay on the record by
//     default (the common Nigerian public-university rule). Passing a
//     retake therefore does not erase the earlier F; callers may pass
//     prior totals that already include the failed attempt and simply
//     append the new passing attempt. A best-so-far "effective CGPA"
//     (as-if the best attempt per course replaced the F) is also
//     provided for institutions with a replacement policy.
package grading

import (
	"fmt"
	"math"
	"strings"
)

const (
	// coursework cap: the system processes at most 100 students
	MaxStudents = 100
	// a student takes at most 5 courses per submission
	MaxCourses = 5
	// credit units bounds per course (typical Nigerian university range)
	MinCreditUnits = 1
	MaxCreditUnits = 6
)

// GradePoint maps each letter grade to its grade point on the 5.0 scale.
var GradePoint = map[string]float64{
	"A": 5, "B": 4, "C": 3, "D": 2, "E": 1, "F": 0,
}

// BandFor maps a percentage score to the standard 100-point breakdown and
// returns the letter grade, its grade point and its remark label.
func BandFor(score float64) (letter string, point float64, label string) {
	switch {
	case score >= 70:
		return "A", 5, "Excellent"
	case score >= 60:
		return "B", 4, "Very Good"
	case score >= 50:
		return "C", 3, "Credit"
	case score >= 45:
		return "D", 2, "Pass"
	case score >= 40:
		return "E", 1, "Low Pass"
	default:
		return "F", 0, "Fail"
	}
}

// CourseInput is the raw per-course data a user supplies. A calculator
// must collect all three fields — score alone is not enough for a
// weighted CGPA.
type CourseInput struct {
	Code        string  `json:"code"`        // course code or name, e.g. "CSC301"
	CreditUnits int     `json:"creditUnits"` // weight of the course, 1–6
	Score       float64 `json:"score"`       // raw percentage, 0–100
}

// CourseGrade is the computed verdict for one course.
type CourseGrade struct {
	Code        string  `json:"Code"`
	CreditUnits int     `json:"CreditUnits"`
	Score       float64 `json:"Score"`
	Letter      string  `json:"Letter"`
	GradePoint  float64 `json:"GradePoint"`
	QualityPts  float64 `json:"QualityPts"` // CreditUnits × GradePoint
}

// PriorRecord carries the running totals from earlier semesters so a
// cumulative CGPA can be computed. CGPA is a ratio (TQP/TCU), so merging
// a new semester requires BOTH the numerator and the denominator — a
// previous CGPA alone cannot be combined. Callers may supply either the
// exact totals (TotalQualityPoints + TotalCreditUnits, preferred) or, as
// a convenience, the previous CGPA together with TotalCreditUnits, from
// which TQP is reconstructed as CGPA × TCU. Leaving everything zero
// treats this submission as the student's first semester.
type PriorRecord struct {
	TotalCreditUnits   int     `json:"previousCreditUnits"`   // Σ CU from prior semesters
	TotalQualityPoints float64 `json:"previousQualityPoints"` // Σ QP from prior semesters
	PreviousCGPA       float64 `json:"previousCGPA"`          // optional convenience input
}

// inputError is a lightweight error type for user-input validation
// failures; its message is safe to show to the browser.
type inputError struct{ msg string }

func (e inputError) Error() string { return e.msg }

// StudentRecord carries everything computed for one student.
type StudentRecord struct {
	Name          string        `json:"Name"`
	MatrikNo      string        `json:"MatrikNo"`
	Courses       []CourseGrade `json:"Courses"`
	TotalMarks    float64       `json:"TotalMarks"`    // Σ raw scores (informational)
	AverageMark   float64       `json:"AverageMark"`   // mean raw score (informational)
	TotalCU       int           `json:"TotalCU"`       // Σ credit units this submission
	TotalQP       float64       `json:"TotalQP"`       // Σ quality points this submission
	SemesterGPA   float64       `json:"SemesterGPA"`   // ΣQP/ΣCU for this submission
	GradeLeter    string        `json:"GradeLeter"`    // overall letter, from the average %
	GradeLabel    string        `json:"GradeLabel"`    // Excellent / Very Good / ...
	CGPA          float64       `json:"CGPA"`          // cumulative, incl. prior record
	EffectiveCGPA float64       `json:"EffectiveCGPA"` // best-attempt variant (replacement policy)
	Attempts      int           `json:"Attempts"`      // 1 = fresh, 2 = carryover student
}

// validateCU checks a credit-unit value against the allowed range.
func validateCU(cu int) error {
	if cu < MinCreditUnits || cu > MaxCreditUnits {
		return inputError{fmt.Sprintf(
			"INVALID INPUT! Credit units must be a whole number between %d and %d.",
			MinCreditUnits, MaxCreditUnits)}
	}
	return nil
}

// normalizePrior validates the prior record and resolves the CGPA
// convenience input into exact quality points. It must be called before
// the cumulative computation.
func normalizePrior(p PriorRecord) (PriorRecord, error) {
	if p.TotalCreditUnits < 0 {
		return p, inputError{"INVALID INPUT! Previous credit units cannot be negative."}
	}
	if p.TotalQualityPoints < 0 {
		return p, inputError{"INVALID INPUT! Previous quality points cannot be negative."}
	}
	if p.PreviousCGPA < 0 {
		return p, inputError{"INVALID INPUT! Previous CGPA cannot be negative."}
	}
	if p.PreviousCGPA > 5.0 {
		return p, inputError{"INVALID INPUT! Previous CGPA cannot exceed 5.0."}
	}
	if p.PreviousCGPA > 0 {
		if p.TotalQualityPoints > 0 {
			return p, inputError{"INVALID INPUT! Provide either previous quality points or previous CGPA, not both."}
		}
		if p.TotalCreditUnits == 0 {
			return p, inputError{"INVALID INPUT! Previous CGPA requires previous credit units to reconstruct quality points."}
		}
		// Reconstruct the exact numerator: TQP = CGPA × TCU.
		p.TotalQualityPoints = round2(p.PreviousCGPA * float64(p.TotalCreditUnits))
	}
	if p.TotalCreditUnits == 0 && p.TotalQualityPoints > 0 {
		return p, inputError{"INVALID INPUT! Previous quality points supplied without credit units."}
	}
	if p.TotalCreditUnits > 0 {
		// ΣQP/ΣCU can never exceed 5.0 on this scale.
		if p.TotalQualityPoints > float64(p.TotalCreditUnits)*5.0 {
			return p, inputError{"INVALID INPUT! Previous quality points exceed 5.0 × credit units."}
		}
	}
	return p, nil
}

// ProcessStudent validates the raw input and computes the complete
// academic record for one student, including the cumulative CGPA when a
// prior record is supplied.
func ProcessStudent(name, matric string, courses []CourseInput, prior PriorRecord) (StudentRecord, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return StudentRecord{}, inputError{"Error: Please provide an input, student name cannot be blank."}
	}
	if len(name) > 120 {
		return StudentRecord{}, inputError{"Error: Student name is too long."}
	}

	matric = strings.TrimSpace(matric)
	if matric == "" {
		return StudentRecord{}, inputError{"Error: Matric number cannot be blank."}
	}
	if len(matric) > 40 {
		return StudentRecord{}, inputError{"Error: Matric number is too long."}
	}

	if len(courses) == 0 || len(courses) > MaxCourses {
		return StudentRecord{}, inputError{"Error: A student must have between 1 and 5 courses."}
	}

	prior, err := normalizePrior(prior)
	if err != nil {
		return StudentRecord{}, err
	}

	rec := StudentRecord{
		Name:     name,
		MatrikNo: matric,
		Courses:  make([]CourseGrade, 0, len(courses)),
		Attempts: 1,
	}
	if prior.TotalCreditUnits > 0 {
		rec.Attempts = 2 // carryover student presenting prior semesters
	}

	var sumQP, sumScores float64
	sumCU := 0
	for j, in := range courses {
		code := strings.TrimSpace(in.Code)
		if code == "" {
			return StudentRecord{}, inputError{fmt.Sprintf(
				"Course %d: course code/name cannot be blank.", j+1)}
		}
		if len(code) > 20 {
			return StudentRecord{}, inputError{fmt.Sprintf(
				"Course %d: course code is too long.", j+1)}
		}
		if err := validateCU(in.CreditUnits); err != nil {
			return StudentRecord{}, inputError{fmt.Sprintf("Course %d (%s): %s", j+1, code, err.Error())}
		}
		// INVALID INPUT! Score must be a valid number between 0 and 100.
		if in.Score < 0 || in.Score > 100 {
			return StudentRecord{}, inputError{fmt.Sprintf(
				"Course %d (%s): INVALID INPUT! Score must be a valid number between 0 and 100.", j+1, code)}
		}

		letter, point, _ := BandFor(in.Score)
		qp := float64(in.CreditUnits) * point

		rec.Courses = append(rec.Courses, CourseGrade{
			Code:        code,
			CreditUnits: in.CreditUnits,
			Score:       in.Score,
			Letter:      letter,
			GradePoint:  point,
			QualityPts:  qp,
		})
		sumQP += qp
		sumCU += in.CreditUnits
		sumScores += in.Score
	}

	rec.TotalCU = sumCU
	rec.TotalQP = round2(sumQP)
	rec.SemesterGPA = round2(sumQP / float64(sumCU))
	rec.TotalMarks = round2(sumScores)
	rec.AverageMark = round2(sumScores / float64(len(courses)))
	rec.GradeLeter, _, rec.GradeLabel = BandFor(rec.AverageMark)

	// Cumulative CGPA: ΣTQP / ΣTCU across ALL semesters — never the
	// average of semester GPAs. Fails (F = 0 QP) still contribute their
	// credit units to the denominator.
	cuAll := prior.TotalCreditUnits + sumCU
	qpAll := prior.TotalQualityPoints + sumQP
	if cuAll > 0 {
		rec.CGPA = round2(qpAll / float64(cuAll))
	}

	// Effective CGPA under a replacement policy: the stronger of the two
	// totals (used when an institution replaces a failed attempt with a
	// passing retake). With no prior record the two agree.
	if prior.TotalCreditUnits > 0 {
		bestQP := math.Max(prior.TotalQualityPoints, sumQP)
		bestCU := math.Max(float64(prior.TotalCreditUnits), float64(sumCU))
		rec.EffectiveCGPA = round2(bestQP / bestCU)
	} else {
		rec.EffectiveCGPA = rec.CGPA
	}

	return rec, nil
}

// ClassMetrics tracks the performance of the whole roster — the web
// "Class Metrics" card and the console "CLASS METRICS SUMMARY".
type ClassMetrics struct {
	Count       int            `json:"count"`
	HighestAvg  float64        `json:"highestAvg"`
	LowestAvg   float64        `json:"lowestAvg"`
	HighestGPA  float64        `json:"highestGPA"`
	LowestGPA   float64        `json:"lowestGPA"`
	AverageGPA  float64        `json:"averageGPA"`
	GradeCounts map[string]int `json:"gradeCounts"`

	gpaSum float64 // accumulator for the class mean GPA
}

func NewClassMetrics() ClassMetrics {
	return ClassMetrics{
		HighestAvg:  -1.0,
		LowestAvg:   101.0,
		HighestGPA:  -1.0,
		LowestGPA:   6.0,
		GradeCounts: map[string]int{},
	}
}

func (m *ClassMetrics) Add(s StudentRecord) {
	m.Count++
	if s.AverageMark > m.HighestAvg {
		m.HighestAvg = s.AverageMark
	}
	if s.AverageMark < m.LowestAvg {
		m.LowestAvg = s.AverageMark
	}
	if s.CGPA > m.HighestGPA {
		m.HighestGPA = s.CGPA
	}
	if s.CGPA < m.LowestGPA {
		m.LowestGPA = s.CGPA
	}
	m.GradeCounts[s.GradeLeter]++
	m.gpaSum += s.CGPA
}

// Finalize computes derived values; call it once after the last Add.
func (m *ClassMetrics) Finalize() {
	if m.Count == 0 {
		m.HighestAvg, m.LowestAvg = 0, 0
		m.HighestGPA, m.LowestGPA = 0, 0
		return
	}
	m.AverageGPA = round2(m.gpaSum / float64(m.Count))
}

func round2(v float64) float64 {
	return math.Round(v*100) / 100
}
