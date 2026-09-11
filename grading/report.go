package grading

import (
	"fmt"
	"strings"
)

// report.go renders the classic department-style results sheet. It is
// shared by the web UI (shown in the "Formatted Class Report" card) and
// the console mode, so both produce identical output.

const reportWidth = 112

// FormatClassReport renders every student plus the class metrics summary
// as an aligned monospace table. Columns show the weighted CGPA inputs —
// credit units and quality points — not just raw scores.
func FormatClassReport(records []StudentRecord, m ClassMetrics) string {
	var b strings.Builder

	section := func(title string) {
		b.WriteString(strings.Repeat("=", reportWidth) + "\n")
		b.WriteString(center(title, reportWidth) + "\n")
		b.WriteString(strings.Repeat("=", reportWidth) + "\n")
	}

	section("FINAL ACADEMIC REPORT")

	// Columns: name(22) matric(15) TCU(4) TQP(8) avg%(9) grade(6) CGPA(6)
	b.WriteString(fmt.Sprintf("%-22s %-15s %4s %8s %9s %-6s %s\n",
		"STUDENT NAME", "MATRIC NO", "TCU", "TQP", "AVG %", "GRADE", "CGPA"))
	b.WriteString(strings.Repeat("-", reportWidth) + "\n")

	for _, s := range records {
		b.WriteString(fmt.Sprintf("%-22s %-15s %4d %8.2f %9.2f %-6s %.2f\n",
			trunc(s.Name, 22), trunc(s.MatrikNo, 15),
			s.TotalCU, s.TotalQP, s.AverageMark, s.GradeLeter, s.CGPA))
	}

	b.WriteString(strings.Repeat("=", reportWidth) + "\n\n")

	section("CLASS METRICS SUMMARY")
	b.WriteString(fmt.Sprintf("  Students processed:      %6d\n", m.Count))
	b.WriteString(fmt.Sprintf("  Highest Average Score:   %6.2f\n", m.HighestAvg))
	b.WriteString(fmt.Sprintf("  Lowest Average Score:    %6.2f\n", m.LowestAvg))
	b.WriteString(fmt.Sprintf("  Highest CGPA:            %6.2f  (5.0 Max Scale)\n", m.HighestGPA))
	b.WriteString(fmt.Sprintf("  Lowest CGPA:             %6.2f\n", m.LowestGPA))
	b.WriteString(fmt.Sprintf("  Class Average CGPA:      %6.2f\n", m.AverageGPA))

	// Grade distribution row: A:2  B:0  C:1 ...
	if m.Count > 0 {
		order := []string{"A", "B", "C", "D", "E", "F"}
		parts := make([]string, 0, len(order))
		for _, g := range order {
			parts = append(parts, fmt.Sprintf("%s:%d", g, m.GradeCounts[g]))
		}
		b.WriteString("  Grade Distribution:      " + strings.Join(parts, "  ") + "\n")
	}
	b.WriteString(strings.Repeat("=", reportWidth) + "\n")

	return b.String()
}

// FormatCourseBreakdown renders the per-course quality-point working for
// a single student — Σ(CU × GP) / ΣCU — useful for transcripts and for
// teaching the weighted model.
func FormatCourseBreakdown(s StudentRecord) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%s (%s) — per-course working\n", s.Name, s.MatrikNo))
	b.WriteString(fmt.Sprintf("  %-12s %4s %8s %5s %8s\n", "COURSE", "CU", "SCORE", "GP", "QP=CU×GP"))
	b.WriteString("  " + strings.Repeat("-", 44) + "\n")
	for _, c := range s.Courses {
		b.WriteString(fmt.Sprintf("  %-12s %4d %8.1f %5s %8.2f\n",
			trunc(c.Code, 12), c.CreditUnits, c.Score, c.Letter, c.QualityPts))
	}
	b.WriteString("  " + strings.Repeat("-", 44) + "\n")
	b.WriteString(fmt.Sprintf("  %-12s %4d %8s %5s %8.2f   ← Total Quality Points\n",
		"TOTALS", s.TotalCU, "", "", s.TotalQP))
	b.WriteString(fmt.Sprintf("  Semester GPA:  %.2f / %.2f = %.2f\n", s.TotalQP, float64(s.TotalCU), s.SemesterGPA))
	if s.Attempts > 1 {
		b.WriteString(fmt.Sprintf("  Cumulative:    %.2f CGPA incl. prior record (effective %.2f)\n", s.CGPA, s.EffectiveCGPA))
	} else {
		b.WriteString(fmt.Sprintf("  Cumulative:    %.2f CGPA\n", s.CGPA))
	}
	return b.String()
}

// center pads s with spaces so it appears centered in a line of width w.
func center(s string, w int) string {
	if len(s) >= w {
		return s
	}
	left := (w - len(s)) / 2
	return strings.Repeat(" ", left) + s
}

// trunc shortens s to at most w runes so the table never breaks alignment.
func trunc(s string, w int) string {
	r := []rune(s)
	if len(r) <= w {
		return s
	}
	return string(r[:w-1]) + "…"
}
