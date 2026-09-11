package main

// console.go preserves the ORIGINAL terminal-based grading program, now
// reachable with:  go run . -console
// It asks how many students you are calculating for, walks you through
// every student's courses (code, credit units, score) plus any prior
// semester totals, and prints the same formatted report the web UI uses.

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/israeloluwasegun293-stars/classedge/grading"
)

func runConsole() {
	consoleReader := bufio.NewScanner(os.Stdin)

	fmt.Println("================================================================")
	fmt.Println("              STUDENT RESULTS PROCESSING SYSTEM                 ")
	fmt.Println("================================================================")

	// Ask how many students to process, with validation.
	var rosterSize int
	for {
		fmt.Printf("How many students are you calculating for? (1-%d): ", grading.MaxStudents)
		if consoleReader.Scan() {
			n, err := strconv.Atoi(strings.TrimSpace(consoleReader.Text()))
			if err == nil && n >= 1 && n <= grading.MaxStudents {
				rosterSize = n
				break
			}
		}
		fmt.Printf("INVALID INPUT! Enter a number between 1 and %d.\n", grading.MaxStudents)
	}

	fmt.Printf("\nThis Project is configured to process data for %d students across up to %d courses.\n", rosterSize, grading.MaxCourses)
	fmt.Println("Every course needs its Credit Units — the CGPA is weighted by them.")
	fmt.Println()

	classRoster := make([]grading.StudentRecord, 0, rosterSize)
	metrics := grading.NewClassMetrics()

	for i := 0; i < rosterSize; i++ {
		fmt.Printf("\n--- Processing Record for Student %d of %d ---\n", i+1, rosterSize)

		var name, matric string
		for {
			fmt.Print("Enter Student Full Name: ")
			if consoleReader.Scan() {
				name = strings.TrimSpace(consoleReader.Text())
				if name != "" {
					break
				}
				fmt.Println("Error: Please provide an input, student name cannot be blank.")
			}
		}

		for {
			fmt.Print("Enter Matriculation Number: ")
			if consoleReader.Scan() {
				matric = strings.TrimSpace(consoleReader.Text())
				if matric != "" {
					break
				}
				fmt.Println("Error: Matric number cannot be blank.")
			}
		}

		// Prior semesters (for the cumulative CGPA). Enter 0 for a fresh student.
		// A previous CGPA alone is not enough to merge a semester — the credit
		// units are the weight — so we always collect ΣCU, plus either the
		// previous CGPA (TQP is reconstructed as CGPA × TCU) or the exact ΣQP.
		prior := grading.PriorRecord{}
		for {
			fmt.Print("  Previous credit units (from earlier semesters, 0 if none): ")
			if consoleReader.Scan() {
				n, err := strconv.Atoi(strings.TrimSpace(consoleReader.Text()))
				if err == nil && n >= 0 {
					prior.TotalCreditUnits = n
					break
				}
			}
			fmt.Println("  INVALID INPUT! Enter 0 or a positive whole number.")
		}
		if prior.TotalCreditUnits > 0 {
			var mode int
			for {
				fmt.Print("  Do you know your previous [1] CGPA or [2] total quality points? ")
				if consoleReader.Scan() {
					n, err := strconv.Atoi(strings.TrimSpace(consoleReader.Text()))
					if err == nil && (n == 1 || n == 2) {
						mode = n
						break
					}
				}
				fmt.Println("  INVALID INPUT! Enter 1 or 2.")
			}
			if mode == 1 {
				for {
					fmt.Print("  Previous CGPA (0.00-5.00): ")
					if consoleReader.Scan() {
						g, err := strconv.ParseFloat(strings.TrimSpace(consoleReader.Text()), 64)
						if err == nil && g > 0 && g <= 5.0 {
							prior.PreviousCGPA = g
							break
						}
					}
					fmt.Println("  INVALID INPUT! Enter a CGPA between 0 and 5.0.")
				}
			} else {
				for {
					fmt.Print("  Previous total quality points: ")
					if consoleReader.Scan() {
						qp, err := strconv.ParseFloat(strings.TrimSpace(consoleReader.Text()), 64)
						if err == nil && qp >= 0 && qp <= float64(prior.TotalCreditUnits)*5.0 {
							prior.TotalQualityPoints = qp
							break
						}
					}
					fmt.Printf("  INVALID INPUT! Enter a number between 0 and %.2f (5.0 × credit units).\n",
						float64(prior.TotalCreditUnits)*5.0)
				}
			}
		}

		// How many courses for this student.
		var courseCount int
		for {
			fmt.Printf("  How many courses for %s? (1-%d): ", name, grading.MaxCourses)
			if consoleReader.Scan() {
				n, err := strconv.Atoi(strings.TrimSpace(consoleReader.Text()))
				if err == nil && n >= 1 && n <= grading.MaxCourses {
					courseCount = n
					break
				}
			}
			fmt.Printf("  INVALID INPUT! Enter a number between 1 and %d.\n", grading.MaxCourses)
		}

		courses := make([]grading.CourseInput, courseCount)
		for j := 0; j < courseCount; j++ {
			var code string
			var cu int
			var score float64

			for {
				fmt.Printf("  Course %d code/name: ", j+1)
				if consoleReader.Scan() {
					code = strings.TrimSpace(consoleReader.Text())
					if code != "" {
						break
					}
				}
				fmt.Println("  Error: course code cannot be blank.")
			}

			for {
				fmt.Printf("  Credit Units for %s (1-%d): ", code, grading.MaxCreditUnits)
				if consoleReader.Scan() {
					n, err := strconv.Atoi(strings.TrimSpace(consoleReader.Text()))
					if err == nil && n >= grading.MinCreditUnits && n <= grading.MaxCreditUnits {
						cu = n
						break
					}
				}
				fmt.Printf("  INVALID INPUT! Enter valid credit units (e.g., %d to %d).\n",
					grading.MinCreditUnits, grading.MaxCreditUnits)
			}

			for {
				fmt.Printf("  Enter score for %s (0-100): ", code)
				if consoleReader.Scan() {
					s, err := strconv.ParseFloat(strings.TrimSpace(consoleReader.Text()), 64)
					if err == nil && s >= 0 && s <= 100 {
						score = s
						break
					}
				}
				fmt.Println("  INVALID INPUT! Score must be a valid number between 0 and 100.")
			}

			courses[j] = grading.CourseInput{Code: code, CreditUnits: cu, Score: score}
		}

		rec, err := grading.ProcessStudent(name, matric, courses, prior)
		if err != nil {
			fmt.Println(err.Error())
			i-- // retry this student
			continue
		}

		fmt.Print(grading.FormatCourseBreakdown(rec))
		classRoster = append(classRoster, rec)
		metrics.Add(rec)
	}
	metrics.Finalize()

	fmt.Println()
	fmt.Print(grading.FormatClassReport(classRoster, metrics))
}
