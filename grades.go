package main

// grades.go holds the core academic logic extracted from the original
// console program so that both the console mode and the web handlers
// share one single source of truth.

// We define our struct for taking informations of the students
type StudentRecord struct {
	Name        string
	MatrikNo    string
	Marks       [5]float64
	TotalMarks  float64
	AverageMark float64
	GradeLeter  string
	GPA         float64
}

const (
	// course work requires 100 student inputs so we have a constant defined for 100 students
	totalStudents = 100
	// we only store marks for 5 courses per student (array is sized [5])
	maxCourses = 5
)

// inputError is a lightweight error type for user-input validation
// failures; its message is safe to show to the browser.
type inputError struct{ msg string }

func (e inputError) Error() string { return e.msg }

// gradeAndGPA converts an average mark to a letter grade and a GPA
// using the university's official 5.0 grading scale.
func gradeAndGPA(avg float64) (string, float64) {
	switch {
	case avg >= 70:
		return "A", 5.0
	case avg >= 60:
		return "B", 4.0
	case avg >= 50:
		return "C", 3.0
	case avg >= 45:
		return "D", 2.0
	case avg >= 40:
		return "E", 1.0
	default:
		return "F", 0.0
	}
}

// processStudent validates the raw input and computes the academic
// record for one student. It returns an error for any invalid field
// instead of printing to the terminal, so the web layer can report it
// safely back to the browser.
func processStudent(name, matric string, marks []float64) (StudentRecord, error) {
	// We capture the student full name safely handling spacess
	name = trimSpace(name)
	if name == "" {
		return StudentRecord{}, inputError{"Error: Please provide an input, student name cannot be blank."}
	}
	if len(name) > 120 {
		return StudentRecord{}, inputError{"Error: Student name is too long."}
	}

	// we capture the Matriculation numnber
	matric = trimSpace(matric)
	if matric == "" {
		return StudentRecord{}, inputError{"Error: Matric number cannot be blank."}
	}
	if len(matric) > 40 {
		return StudentRecord{}, inputError{"Error: Matric number is too long."}
	}

	if len(marks) == 0 || len(marks) > maxCourses {
		return StudentRecord{}, inputError{"Error: A student must have between 1 and 5 courses."}
	}

	var currentStudent StudentRecord
	currentStudent.Name = name
	currentStudent.MatrikNo = matric

	// we cature valid course scores only
	var sum float64 = 0
	for j, score := range marks {
		// INVALID INPUT! Score must be a valid number between 0 and 100.
		if score < 0 || score > 100 {
			return StudentRecord{}, inputError{"INVALID INPUT! Score must be a valid number between 0 and 100."}
		}
		currentStudent.Marks[j] = score
		sum += score
	}

	// the academic calculation logic
	currentStudent.TotalMarks = sum
	currentStudent.AverageMark = sum / float64(len(marks))

	currentStudent.GradeLeter, currentStudent.GPA = gradeAndGPA(currentStudent.AverageMark)
	return currentStudent, nil
}

// ClassMetrics tracks the performance of the whole roster, mirroring the
// "CLASS METRICS SUMMARY" section of the console program.
type ClassMetrics struct {
	Count      int     `json:"count"`
	HighestAvg float64 `json:"highestAvg"`
	LowestAvg  float64 `json:"lowestAvg"`
	HighestGPA float64 `json:"highestGPA"`
	LowestGPA  float64 `json:"lowestGPA"`
	//	AverageGPA is the class mean, computed in finalize().
	AverageGPA  float64        `json:"averageGPA"`
	GPAsum      float64        `json:"-"`
	GradeCounts map[string]int `json:"gradeCounts"`
}

func newClassMetrics() ClassMetrics {
	return ClassMetrics{
		HighestAvg:  -1.0,
		LowestAvg:   101.0,
		HighestGPA:  -1.0,
		LowestGPA:   6.0,
		GradeCounts: map[string]int{},
	}
}

func (m *ClassMetrics) add(s StudentRecord) {
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
	m.GPAsum += s.GPA
}

func (m *ClassMetrics) finalize() {
	if m.Count == 0 {
		m.HighestAvg, m.LowestAvg = 0, 0
		m.HighestGPA, m.LowestGPA = 0, 0
		return
	}
	m.AverageGPA = round2(m.GPAsum / float64(m.Count))
}

// trimSpace is a small helper so the grading logic does not depend on
// the strings package being imported everywhere.
func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}
