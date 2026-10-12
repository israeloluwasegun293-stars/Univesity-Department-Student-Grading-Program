package handlers

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"runtime"
	"strconv"
	"strings"

	"github.com/israeloluwasegun293-stars/classedge/grading"
)

func handleImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		writeJSON(w, http.StatusMethodNotAllowed, apiError{"Method not allowed. Use POST."})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes+(1<<20))
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeJSON(w, http.StatusRequestEntityTooLarge, apiError{"Upload request exceeds the allowed size."})
			return
		}
		writeJSON(w, http.StatusBadRequest, apiError{"Upload must be a multipart form with a file field."})
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{"Choose a CSV, XLSX or PDF file to import."})
		return
	}
	defer file.Close()
	requests, err := parseImportFile(file, header.Filename)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{err.Error()})
		return
	}
	records := make([]grading.StudentRecord, 0, len(requests))
	for i, req := range requests {
		rec, err := grading.ProcessStudent(req.Name, req.MatrikNo, req.Courses, req.Prior)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, apiError{fmt.Sprintf("Imported student #%d (%s): %s", i+1, req.MatrikNo, err.Error())})
			return
		}
		records = append(records, rec)
	}
	store.addBatch(records)
	respondDashboard(w)
}

func handleExportTemplate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		writeJSON(w, http.StatusMethodNotAllowed, apiError{"Method not allowed. Use GET."})
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="classedge-import-template.csv"`)
	w.Write([]byte("name,matricNo,courseCode,creditUnits,score,previousCreditUnits,previousQualityPoints,previousCGPA\n" +
		"Ada Lovelace,CSC/2026/0142,CSC301,4,85,36,142,\n" +
		"Ada Lovelace,CSC/2026/0142,MTH102,2,65,36,142,\n"))
}

func handleExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		writeJSON(w, http.StatusMethodNotAllowed, apiError{"Method not allowed. Use GET."})
		return
	}
	records, metrics := store.snapshot()
	switch r.URL.Query().Get("format") {
	case "json":
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="classedge-results.json"`)
		_ = json.NewEncoder(w).Encode(DashboardResponse{
			Students: records, Summary: metrics,
			Report: grading.FormatClassReport(records, metrics), TotalCount: len(records), GoVersion: runtime.Version(),
		})
	case "csv":
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="classedge-results.csv"`)
		writer := csv.NewWriter(w)
		_ = writer.Write([]string{"name", "matricNo", "courseCode", "creditUnits", "score", "letter", "gradePoint", "qualityPoints", "totalCreditUnits", "totalQualityPoints", "averageMark", "semesterGPA", "CGPA"})
		for _, student := range records {
			for _, course := range student.Courses {
				_ = writer.Write([]string{safeSpreadsheetCell(student.Name), safeSpreadsheetCell(student.MatrikNo), safeSpreadsheetCell(course.Code),
					strconv.Itoa(course.CreditUnits), fmt.Sprintf("%.2f", course.Score), course.Letter,
					fmt.Sprintf("%.2f", course.GradePoint), fmt.Sprintf("%.2f", course.QualityPts),
					strconv.Itoa(student.TotalCU), fmt.Sprintf("%.2f", student.TotalQP),
					fmt.Sprintf("%.2f", student.AverageMark), fmt.Sprintf("%.2f", student.SemesterGPA), fmt.Sprintf("%.2f", student.CGPA)})
			}
		}
		writer.Flush()
	case "txt":
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="classedge-report.txt"`)
		_, _ = fmt.Fprint(w, grading.FormatClassReport(records, metrics))
	default:
		writeJSON(w, http.StatusBadRequest, apiError{"Choose an export format: json, csv or txt."})
	}
}

// safeSpreadsheetCell prevents downloaded CSV data from being interpreted as
// a spreadsheet formula when values came from user-provided files.
func safeSpreadsheetCell(value string) string {
	trimmed := strings.TrimLeft(value, " \t\r\n")
	if len(trimmed) > 0 && strings.ContainsRune("=+-@", rune(trimmed[0])) {
		return "'" + value
	}
	return value
}
