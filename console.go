package main

// console.go preserves the ORIGINAL terminal-based grading program, now
// reachable with:  go run . -console
// The grading switch has been swapped for the shared gradeAndGPA() helper
// in grades.go so both modes use identical rules.

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func runConsole() {
	// demo-sized roster for the terminal experience
	const rosterSize = 3
	const courseCount = 2

	consoleReader := bufio.NewScanner(os.Stdin)

	classRoster := make([]StudentRecord, rosterSize)
	courseNames := [courseCount]string{"Course 1", "Course 2"}

	highestAvg := -1.0
	lowestAvg := 101.0
	highestGPA := -1.0
	lowestGPA := 6.0

	fmt.Println("================================================================")
	fmt.Println("              STUDENT RESULTS PROCESSING SYSTEM                 ")
	fmt.Println("================================================================")
	fmt.Printf("This Project is configured to process data for %d students across %d courses.\n\n", rosterSize, courseCount)

	for i := 0; i < rosterSize; i++ {
		fmt.Printf("\n--- Processing Record for Student %d of %d ---\n", i+1, rosterSize)

		var currentStudent StudentRecord

		for {
			fmt.Print("Enter Student Full Name: ")
			if consoleReader.Scan() {
				name := strings.TrimSpace(consoleReader.Text())
				if name != "" {
					currentStudent.Name = name
					break
				}
				fmt.Println("Error: Please provide an input, student name cannot be blank.")
			}
		}

		for {
			fmt.Print("Enter Matriculation Number: ")
			if consoleReader.Scan() {
				matric := strings.TrimSpace(consoleReader.Text())
				if matric != "" {
					currentStudent.MatrikNo = matric
					break
				}
				fmt.Println("Error: Matric number cannot be blank.")
			}
		}

		var sum float64 = 0
		for j := 0; j < courseCount; j++ {
			for {
				fmt.Printf("  Enter score for %s (0-100): ", courseNames[j])
				if consoleReader.Scan() {
					inputStr := strings.TrimSpace(consoleReader.Text())
					score, err := strconv.ParseFloat(inputStr, 64)

					if err == nil && score >= 0 && score <= 100 {
						currentStudent.Marks[j] = score
						sum += score
						break
					}
				}
				fmt.Println("INVALID INPUT! Score must be a valid number between 0 and 100.")
			}
		}

		currentStudent.TotalMarks = sum
		currentStudent.AverageMark = sum / float64(courseCount)
		currentStudent.GradeLeter, currentStudent.GPA = gradeAndGPA(currentStudent.AverageMark)

		if currentStudent.AverageMark > highestAvg {
			highestAvg = currentStudent.AverageMark
		}
		if currentStudent.AverageMark < lowestAvg {
			lowestAvg = currentStudent.AverageMark
		}
		if currentStudent.GPA > highestGPA {
			highestGPA = currentStudent.GPA
		}
		if currentStudent.GPA < lowestGPA {
			lowestGPA = currentStudent.GPA
		}

		classRoster[i] = currentStudent
	}

	fmt.Println("\n\n==========================================================================================")
	fmt.Println("                                   FINAL ACADEMIC REPORT                                  ")
	fmt.Println("==========================================================================================")
	fmt.Printf("%-22s %-15s %-12s %-15s %-8s %-5s\n", "STUDENT NAME", "MATRIC NO", "TOTAL SCORE", "AVERAGE SCORE", "GRADE", "GPA")
	fmt.Println("------------------------------------------------------------------------------------------")

	for _, student := range classRoster {
		fmt.Printf("%-22s %-15s %-12.2f %-15.2f %-8s %-5.2f\n",
			student.Name,
			student.MatrikNo,
			student.TotalMarks,
			student.AverageMark,
			student.GradeLeter,
			student.GPA,
		)
	}
	fmt.Println("==========================================================================================")

	fmt.Println("\n==================================================")
	fmt.Println("              CLASS METRICS SUMMARY               ")
	fmt.Println("==================================================")
	fmt.Printf("Highest Average Score:  %6.2f\n", highestAvg)
	fmt.Printf("Lowest Average Score:   %6.2f\n", lowestAvg)
	fmt.Printf("Highest Achieved GPA:   %6.2f (5.0 Max Scale)\n", highestGPA)
	fmt.Printf("Lowest Achieved GPA:    %6.2f\n", lowestGPA)
	fmt.Println("==================================================")
}
