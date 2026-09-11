package main

// console.go preserves the ORIGINAL terminal-based grading program, now
// reachable with:  go run . -console
// It asks how many students you are calculating for, walks you through
// every student, and prints the same formatted report the web UI uses.

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

	fmt.Printf("\nThis Project is configured to process data for %d students across up to %d courses.\n\n", rosterSize, grading.MaxCourses)

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

		// Ask how many courses for this student.
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

		marks := make([]float64, courseCount)
		for j := 0; j < courseCount; j++ {
			for {
				fmt.Printf("  Enter score for Course %d (0-100): ", j+1)
				if consoleReader.Scan() {
					inputStr := strings.TrimSpace(consoleReader.Text())
					score, err := strconv.ParseFloat(inputStr, 64)
					if err == nil && score >= 0 && score <= 100 {
						marks[j] = score
						break
					}
				}
				fmt.Println("  INVALID INPUT! Score must be a valid number between 0 and 100.")
			}
		}

		rec, err := grading.ProcessStudent(name, matric, marks)
		if err != nil {
			fmt.Println(err.Error())
			i-- // retry this student
			continue
		}

		classRoster = append(classRoster, rec)
		metrics.Add(rec)
	}
	metrics.Finalize()

	fmt.Println()
	fmt.Print(grading.FormatClassReport(classRoster, metrics))
}
