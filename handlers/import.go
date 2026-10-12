package handlers

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/israeloluwasegun293-stars/classedge/grading"
	"github.com/ledongthuc/pdf"
	"github.com/xuri/excelize/v2"
)

const maxUploadBytes = 10 << 20

var importHeaders = []string{"name", "matricno", "coursecode", "creditunits", "score", "previouscreditunits", "previousqualitypoints", "previouscgpa"}

// parseImportFile decodes the supported tabular formats into the exact same
// request structs used by the JSON grading endpoint.
func parseImportFile(file multipart.File, filename string) ([]GradeRequest, error) {
	data, err := io.ReadAll(io.LimitReader(file, maxUploadBytes+1))
	if err != nil {
		return nil, fmt.Errorf("could not read uploaded file: %w", err)
	}
	if len(data) > maxUploadBytes {
		return nil, fmt.Errorf("file exceeds the 10 MiB upload limit")
	}

	var rows [][]string
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".csv":
		rows, err = readCSVRows(bytes.NewReader(data), ',')
	case ".xlsx":
		var book *excelize.File
		book, err = excelize.OpenReader(bytes.NewReader(data), excelize.Options{UnzipSizeLimit: 50 << 20, UnzipXMLSizeLimit: 50 << 20})
		if err == nil {
			defer book.Close()
			sheets := book.GetSheetList()
			if len(sheets) == 0 {
				return nil, fmt.Errorf("workbook has no worksheets")
			}
			rows, err = book.GetRows(sheets[0])
			if err == nil && len(rows) > 1000 {
				return nil, fmt.Errorf("worksheet exceeds the 1,000-row import limit")
			}
		}
	case ".pdf":
		rows, err = readPDFRows(data)
	default:
		return nil, fmt.Errorf("unsupported file type %q; upload .csv, .xlsx, or text-based .pdf", filepath.Ext(filename))
	}
	if err != nil {
		return nil, fmt.Errorf("could not read file: %w", err)
	}
	return rowsToGradeRequests(rows)
}

func readCSVRows(src io.Reader, delimiter rune) ([][]string, error) {
	r := csv.NewReader(src)
	r.FieldsPerRecord = -1
	r.TrimLeadingSpace = true
	if delimiter != ',' {
		r.Comma = delimiter
	}
	var rows [][]string
	for {
		row, err := r.Read()
		if err == io.EOF {
			return rows, nil
		}
		if err != nil {
			return nil, err
		}
		rows = append(rows, row)
		if len(rows) > 1000 {
			return nil, fmt.Errorf("file exceeds the 1,000-row import limit")
		}
	}
}

func readPDFRows(data []byte) (rows [][]string, err error) {
	defer func() {
		if recovered := recover(); recovered != nil {
			rows = nil
			err = fmt.Errorf("PDF parser could not read this document")
		}
	}()
	reader, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	if reader.NumPage() > 50 {
		return nil, fmt.Errorf("PDF exceeds the 50-page import limit")
	}
	type positionedSpan struct {
		x    float64
		text string
	}
	type positionedRow struct {
		spans []positionedSpan
	}
	var textRows []positionedRow
	for pageNo := 1; pageNo <= reader.NumPage(); pageNo++ {
		page := reader.Page(pageNo)
		if page.V.IsNull() {
			continue
		}
		pageRows, rowErr := page.GetTextByRow()
		if rowErr != nil {
			return nil, fmt.Errorf("could not extract text from page %d: %w", pageNo, rowErr)
		}
		sort.Slice(pageRows, func(i, j int) bool { return pageRows[i].Position < pageRows[j].Position })
		for _, row := range pageRows {
			positioned := positionedRow{}
			for _, span := range row.Content {
				if text := strings.TrimSpace(span.S); text != "" {
					positioned.spans = append(positioned.spans, positionedSpan{x: span.X, text: span.S})
				}
			}
			if len(positioned.spans) > 0 {
				textRows = append(textRows, positioned)
			}
		}
	}
	if len(textRows) == 0 {
		return nil, fmt.Errorf("PDF contains no selectable text; scanned PDFs need OCR before import")
	}

	// Find a row that looks like the import header and remember each field's
	// horizontal position. This handles normal spreadsheet-export PDFs whose
	// columns are aligned by whitespace rather than separated with pipes.
	var anchors []struct {
		key string
		x   float64
	}
	headerIndex := -1
	for i, row := range textRows {
		seen := make(map[string]bool)
		for _, span := range row.spans {
			if key := normalizeHeader(span.text); isImportHeader(key) {
				seen[key] = true
				anchors = append(anchors, struct {
					key string
					x   float64
				}{key: key, x: span.x})
			}
		}
		if len(seen) >= 5 {
			headerIndex = i
			break
		}
		anchors = nil
	}
	if headerIndex < 0 {
		return nil, fmt.Errorf("PDF table header was not recognized; include the required column names in a selectable-text header row")
	}
	sort.Slice(anchors, func(i, j int) bool { return anchors[i].x < anchors[j].x })
	indices := make(map[string]int, len(anchors))
	for i, anchor := range anchors {
		if _, duplicate := indices[anchor.key]; duplicate {
			return nil, fmt.Errorf("PDF header contains duplicate columns")
		}
		indices[anchor.key] = i
	}
	nameIndex, hasName := indices["name"]
	matricIndex, hasMatric := indices["matricno"]
	courseIndex, hasCourse := indices["coursecode"]
	if !hasName || !hasMatric || !hasCourse || courseIndex+1 >= len(anchors) {
		return nil, fmt.Errorf("PDF header must include name, matricNo, courseCode and course data columns")
	}
	textColumns := map[string]int{"name": nameIndex, "matricno": matricIndex, "coursecode": courseIndex}
	textPositions := make([]struct {
		index int
		x     float64
	}, 0, 3)
	for _, index := range textColumns {
		textPositions = append(textPositions, struct {
			index int
			x     float64
		}{index: index, x: anchors[index].x})
	}
	spacingValues := make([]float64, 0, len(anchors))
	for i := courseIndex + 1; i < len(anchors); i++ {
		spacingValues = append(spacingValues, anchors[i].x-anchors[i-1].x)
	}
	if len(spacingValues) == 0 {
		return nil, fmt.Errorf("PDF header has no numeric course data columns")
	}
	sort.Float64s(spacingValues)
	spacing := spacingValues[len(spacingValues)/2]
	if spacing <= 0 {
		return nil, fmt.Errorf("PDF header columns are not separated clearly")
	}
	dataNumericBase := math.Inf(1)
	for _, row := range textRows[headerIndex+1:] {
		for _, span := range row.spans {
			if span.x > anchors[courseIndex].x && span.x < dataNumericBase {
				dataNumericBase = span.x
			}
		}
	}
	if math.IsInf(dataNumericBase, 1) {
		return nil, fmt.Errorf("PDF contains no course data rows")
	}
	rows = append(rows, make([]string, len(anchors)))
	for i, anchor := range anchors {
		rows[0][i] = anchor.key
	}
	for _, row := range textRows[headerIndex+1:] {
		if len(rows) >= 1000 {
			return nil, fmt.Errorf("PDF exceeds the 1,000-row import limit")
		}
		cells := make([]strings.Builder, len(anchors))
		spans := append([]positionedSpan(nil), row.spans...)
		sort.Slice(spans, func(i, j int) bool { return spans[i].x < spans[j].x })
		for i := 0; i < len(spans); {
			j := i + 1
			var group strings.Builder
			group.WriteString(spans[i].text)
			for j < len(spans) && math.Abs(spans[j].x-spans[i].x) < 1 {
				group.WriteString(spans[j].text)
				j++
			}
			x := spans[i].x
			column, nearest := -1, math.Inf(1)
			for _, textColumn := range textPositions {
				if distance := math.Abs(x - textColumn.x); distance < nearest {
					column, nearest = textColumn.index, distance
				}
			}
			if nearest > 20 && x > anchors[courseIndex].x {
				numericOffset := int(math.Round((x - dataNumericBase) / spacing))
				column = courseIndex + 1 + numericOffset
				if column >= len(anchors) || math.Abs(x-(dataNumericBase+float64(numericOffset)*spacing)) > spacing*0.48 {
					column = -1
				}
			} else if nearest > 20 {
				column = -1
			}
			if column >= 0 {
				cells[column].WriteString(group.String())
			}
			i = j
		}
		values := make([]string, len(cells))
		for i := range cells {
			values[i] = strings.TrimSpace(cells[i].String())
		}
		if !blankRow(values) {
			rows = append(rows, values)
		}
	}
	if len(rows) < 2 {
		return nil, fmt.Errorf("PDF has a recognized header but no data rows")
	}
	return rows, nil
}

func isImportHeader(key string) bool {
	for _, header := range importHeaders {
		if key == header {
			return true
		}
	}
	return false
}

func rowsToGradeRequests(rows [][]string) ([]GradeRequest, error) {
	if len(rows) < 2 {
		return nil, fmt.Errorf("file must contain a header row and at least one data row")
	}
	headers := make(map[string]int, len(rows[0]))
	for i, header := range rows[0] {
		key := normalizeHeader(header)
		if key != "" {
			if _, duplicate := headers[key]; duplicate {
				return nil, fmt.Errorf("duplicate column %q", header)
			}
			headers[key] = i
		}
	}
	for _, required := range importHeaders[:5] {
		if _, ok := headers[required]; !ok {
			return nil, fmt.Errorf("missing required column %q", required)
		}
	}

	studentByMatric := make(map[string]int)
	students := make([]GradeRequest, 0)
	for rowNo, row := range rows[1:] {
		get := func(key string) string {
			idx, ok := headers[key]
			if !ok || idx >= len(row) {
				return ""
			}
			return strings.TrimSpace(row[idx])
		}
		if blankRow(row) {
			continue
		}
		name, matric, code := get("name"), get("matricno"), get("coursecode")
		if name == "" || matric == "" || code == "" {
			return nil, fmt.Errorf("row %d: name, matricNo and courseCode are required", rowNo+2)
		}
		units, err := strconv.Atoi(get("creditunits"))
		if err != nil {
			return nil, fmt.Errorf("row %d: creditUnits must be a whole number", rowNo+2)
		}
		score, err := strconv.ParseFloat(get("score"), 64)
		if err != nil {
			return nil, fmt.Errorf("row %d: score must be a number", rowNo+2)
		}
		prior := grading.PriorRecord{}
		priorCU, err := optionalInt(get("previouscreditunits"))
		if err != nil {
			return nil, fmt.Errorf("row %d: previousCreditUnits must be a whole number", rowNo+2)
		}
		prior.TotalCreditUnits = priorCU
		prior.TotalQualityPoints, err = optionalFloat(get("previousqualitypoints"))
		if err != nil {
			return nil, fmt.Errorf("row %d: previousQualityPoints must be a number", rowNo+2)
		}
		prior.PreviousCGPA, err = optionalFloat(get("previouscgpa"))
		if err != nil {
			return nil, fmt.Errorf("row %d: previousCGPA must be a number", rowNo+2)
		}

		idx, exists := studentByMatric[matric]
		if !exists {
			if len(students) >= grading.MaxStudents {
				return nil, fmt.Errorf("file contains more than %d students", grading.MaxStudents)
			}
			idx = len(students)
			studentByMatric[matric] = idx
			students = append(students, GradeRequest{Name: name, MatrikNo: matric, Prior: prior})
		} else {
			current := students[idx]
			if current.Name != name || current.Prior != prior {
				return nil, fmt.Errorf("row %d: student name and previous totals must match other rows for matricNo %q", rowNo+2, matric)
			}
		}
		students[idx].Courses = append(students[idx].Courses, grading.CourseInput{Code: code, CreditUnits: units, Score: score})
		if len(students[idx].Courses) > grading.MaxCourses {
			return nil, fmt.Errorf("row %d: student %q exceeds the %d-course limit", rowNo+2, matric, grading.MaxCourses)
		}
	}
	if len(students) == 0 {
		return nil, fmt.Errorf("file has no student rows")
	}
	return students, nil
}

func normalizeHeader(header string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(strings.TrimSpace(header)) {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	switch b.String() {
	case "student", "studentname", "fullname":
		return "name"
	case "matric", "matriculation", "matriculationno", "matricnumber":
		return "matricno"
	case "course", "coursecodeorname":
		return "coursecode"
	case "cu", "units":
		return "creditunits"
	case "mark", "percentage", "scorepercent":
		return "score"
	case "priorcu", "previouscu":
		return "previouscreditunits"
	case "priorqp", "previousqp":
		return "previousqualitypoints"
	case "priorcgpa":
		return "previouscgpa"
	default:
		return b.String()
	}
}

func optionalInt(v string) (int, error) {
	if strings.TrimSpace(v) == "" {
		return 0, nil
	}
	return strconv.Atoi(strings.TrimSpace(v))
}

func optionalFloat(v string) (float64, error) {
	if strings.TrimSpace(v) == "" {
		return 0, nil
	}
	return strconv.ParseFloat(strings.TrimSpace(v), 64)
}

func blankRow(row []string) bool {
	for _, cell := range row {
		if strings.TrimSpace(cell) != "" {
			return false
		}
	}
	return true
}
