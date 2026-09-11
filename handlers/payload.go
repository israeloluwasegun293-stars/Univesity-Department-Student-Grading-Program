package handlers

// payload.go holds the JSON contracts shared between the Go handlers and
// the browser frontend.

import (
	"github.com/israeloluwasegun293-stars/classedge/grading"
)

// GradeRequest is the JSON body POSTed to /api/grade.
type GradeRequest struct {
	Name     string    `json:"name"`
	MatrikNo string    `json:"matrikNo"`
	Marks    []float64 `json:"marks"`
}

// BatchRequest is the JSON body POSTed to /api/grade/batch: one entry per
// student in the class (1–100 students).
type BatchRequest struct {
	Students []GradeRequest `json:"students"`
}

// DashboardResponse is everything the dashboard needs in one round trip.
type DashboardResponse struct {
	Students   []grading.StudentRecord `json:"students"`
	Summary    grading.ClassMetrics    `json:"summary"`
	Report     string                  `json:"report"`
	TotalCount int                     `json:"totalCount"`
	GoVersion  string                  `json:"goVersion"`
}

const maxBodyBytes = 1 << 20 // 1 MiB cap for request bodies
