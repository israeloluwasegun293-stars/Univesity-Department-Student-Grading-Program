package grading

import (
	"fmt"
	"strings"
)

// report.go renders the classic department-style results sheet. It is
// shared by the web UI (shown in the "Formatted Class Report" card) and
// the console mode, so both produce identical output.

const reportWidth = 100

// FormatClassReport renders every student plus the class metrics summary
// as an aligned monospace table.
func FormatClassReport(records []StudentRecord, m ClassMetrics) string {
	var b strings.Builder

	section := func(title string) {
		b.WriteString(strings.Repeat("=", reportWidth) + "\n")
		b.WriteString(center(title, reportWidth) + "\n")
		b.WriteString(strings.Repeat("=", reportWidth) + "\n")
	}

	section("FINAL ACADEMIC REPORT")

	// Header row: name(24) matric(16) total(12) avg(14) grade(7) gpa(6)
	b.WriteString(fmt.Sprintf("%-24s %-16s %-12s %-14s %-7s %s\n",
		"STUDENT NAME", "MATRIC NO", "TOTAL SCORE", "AVERAGE SCORE", "GRADE", "CGPA"))
	b.WriteString(strings.Repeat("-", reportWidth) + "\n")

	for _, s := range records {
		b.WriteString(fmt.Sprintf("%-24s %-16s %-12.2f %-14.2f %-7s %.2f\n",
			trunc(s.Name, 24), trunc(s.MatrikNo, 16),
			s.TotalMarks, s.AverageMark, s.GradeLeter, s.GPA))
	}

	b.WriteString(strings.Repeat("=", reportWidth) + "\n\n")

	section("CLASS METRICS SUMMARY")
	b.WriteString(fmt.Sprintf("  Students processed:      %6d\n", m.Count))
	b.WriteString(fmt.Sprintf("  Highest Average Score:   %6.2f\n", m.HighestAvg))
	b.WriteString(fmt.Sprintf("  Lowest Average Score:    %6.2f\n", m.LowestAvg))
	b.WriteString(fmt.Sprintf("  Highest Achieved CGPA:   %6.2f  (5.0 Max Scale)\n", m.HighestGPA))
	b.WriteString(fmt.Sprintf("  Lowest Achieved CGPA:    %6.2f\n", m.LowestGPA))
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
