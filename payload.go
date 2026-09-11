package main

// payload.go holds the request/response contracts shared between the Go
// handlers and the browser frontend.

// GradeRequest is the JSON body POSTed by the frontend to /api/grade.
type GradeRequest struct {
	Name     string    `json:"name"`
	MatrikNo string    `json:"matrikNo"`
	Marks    []float64 `json:"marks"`
}

// DashboardResponse is everything the dashboard needs in one round trip.
type DashboardResponse struct {
	Students   []StudentRecord `json:"students"`
	Summary    ClassMetrics    `json:"summary"`
	TotalCount int             `json:"totalCount"`
	ServerTime string          `json:"serverTime"`
	GoVersion  string          `json:"goVersion"`
}

const staticDir = "./static"
