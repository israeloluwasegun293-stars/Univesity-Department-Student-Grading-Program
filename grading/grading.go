// Package grading holds the core academic logic of the CGPA system — the
// single source of truth used by the web handlers, the batch report and
// the original console mode.
//
// Grading follows the standard 100-point breakdown on a 5.0 scale:
//
//	70–100%  A  Excellent
//	60–69%   B  Very Good
//	50–59%   C  Credit
//	45–49%   D  Pass
//	40–44%   E  Low Pass
//	0–39%    F  Fail
//
// Accuracy note: every course score is graded INDIVIDUALLY and the
// student's GPA is the mean of the per-course grade points (standard CGPA
// practice). Grading one letter from the overall average and using its
// point value inflates results whenever scores span multiple bands —
// e.g. 85 + 65 must average to (5+4)/2 = 4.50, not 5.00.
package grading

import (
	"fmt"
	"math"
	"strings"
)

const (
	// coursework cap: the system processes at most 100 students
	MaxStudents = 100
	// a student takes at most 5 courses
	MaxCourses = 5
)

// GradePoint maps each letter grade to its grade point on the 5.0 scale.
var GradePoint = map[string]float64{
	"A": 5, "B": 4, "C": 3, "D": 2, "E": 1, "F": 0,
}

// CourseGrade is the individual verdict for one course score.
type CourseGrade struct {
	Course string  `json:"course"` // "C1", "C2", ...
	Score  float64 `json:"score"`
	Letter string  `json:"letter"`
	Point  float64 `json:"point"`
}

// StudentRecord carries everything computed for one student.
type StudentRecord struct {
	Name         string        `json:"Name"`
	MatrikNo     string        `json:"MatrikNo"`
	Marks        []float64     `json:"Marks"`
	CourseGrades []CourseGrade `json:"CourseGrades"`
	TotalMarks   float64       `json:"TotalMarks"`
	AverageMark  float64       `json:"AverageMark"`
	GradeLeter   string        `json:"GradeLeter"` // overall grade, from the average %
	GradeLabel   string        `json:"GradeLabel"` // Excellent / Very Good / Credit / Pass / Low Pass / Fail
	GPA          float64       `json:"GPA"`        // mean of the per-course grade points
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

// inputError is a lightweight error type for user-input validation
// failures; its message is safe to show to the browser.
type inputError struct{ msg string }

func (e inputError) Error() string { return e.msg }

// ProcessStudent validates the raw input and computes the complete
// academic record for one student.
func ProcessStudent(name, matric string, marks []float64) (StudentRecord, error) {
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

	if len(marks) == 0 || len(marks) > MaxCourses {
		return StudentRecord{}, inputError{"Error: A student must have between 1 and 5 courses."}
	}

	rec := StudentRecord{Name: name, MatrikNo: matric, Marks: make([]float64, len(marks))}

	var sum, points float64
	for j, score := range marks {
		// INVALID INPUT! Score must be a valid number between 0 and 100.
		if score < 0 || score > 100 {
			return StudentRecord{}, inputError{"INVALID INPUT! Score must be a valid number between 0 and 100."}
		}
		letter, point, _ := BandFor(score)
		rec.Marks[j] = score
		rec.CourseGrades = append(rec.CourseGrades, CourseGrade{
			Course: fmt.Sprintf("C%d", j+1),
			Score:  score,
			Letter: letter,
			Point:  point,
		})
		sum += score
		points += point
	}

	rec.TotalMarks = round2(sum)
	rec.AverageMark = round2(sum / float64(len(marks)))
	rec.GPA = round2(points / float64(len(marks)))
	rec.GradeLeter, _, rec.GradeLabel = BandFor(rec.AverageMark)
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
	if s.GPA > m.HighestGPA {
		m.HighestGPA = s.GPA
	}
	if s.GPA < m.LowestGPA {
		m.LowestGPA = s.GPA
	}
	m.GradeCounts[s.GradeLeter]++
	m.gpaSum += s.GPA
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
