package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// we define our struct for taking informations of the students
type StudentRecord struct {
	Name        string
	MatrikNo    string
	Marks       [5]float64
	TotalMarks  float64
	AverageMark float64
	GradeLeter  string
	GPA         float64
}

func main() {
	// course work requires 100 student inputs so we have a constant defined for 100 students
	const totalStudents = 3
	const totalCourses = 2

	// we use our bufio package for efficient reading for the termainal
	consoleReader := bufio.NewScanner(os.Stdin)

	// we declaring arrayys and slicess for our data
	classRoster := make([]StudentRecord, totalStudents)
	courseNames := [totalCourses]string{"Course 1", "Course 2"}

	// we use this to track the student performancees
	highestAvg := -1.0
	lowestAvg := 101.0
	highestGPA := -1.0
	lowestGPA := 6.0

	fmt.Println("================================================================")
	fmt.Println("              STUDENT RESULTS PROCESSING SYSTEM                 ")
	fmt.Println("================================================================")
	fmt.Printf("This Project is configured to process data for %d students across %d courses.\n\n", totalStudents, totalCourses)

	// Data entry for each students
	for i := 0; i < totalStudents; i++ {
		fmt.Printf("\n--- Processing Record for Student %d of %d ---\n", i+1, totalStudents)

		var currentStudent StudentRecord

		// We capture the student full name safely handling spacess
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

		// we capture the Matriculation numnber
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

		// we cature valid course scores only
		var sum float64 = 0
		for j := 0; j < totalCourses; j++ {
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

		// the academic calculation logic
		currentStudent.TotalMarks = sum
		currentStudent.AverageMark = sum / float64(totalCourses)

		avg := currentStudent.AverageMark
		switch {
		case avg >= 70:
			currentStudent.GradeLeter = "A"
			currentStudent.GPA = 5.0
		case avg >= 60:
			currentStudent.GradeLeter = "B"
			currentStudent.GPA = 4.0
		case avg >= 50:
			currentStudent.GradeLeter = "C"
			currentStudent.GPA = 3.0
		case avg >= 45:
			currentStudent.GradeLeter = "D"
			currentStudent.GPA = 2.0
		case avg >= 40:
			currentStudent.GradeLeter = "E"
			currentStudent.GPA = 1.0
		default:
			currentStudent.GradeLeter = "F"
			currentStudent.GPA = 0.0
		}

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

		// students name being added  tot the roaster slice
		classRoster[i] = currentStudent
	}

	// our result is nbeing displayed here with a properlay formatted characters with fmt package
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

	//summary displayed
	fmt.Println("\n==================================================")
	fmt.Println("              CLASS METRICS SUMMARY               ")
	fmt.Println("==================================================")
	fmt.Printf("Highest Average Score:  %6.2f\n", highestAvg)
	fmt.Printf("Lowest Average Score:   %6.2f\n", lowestAvg)
	fmt.Printf("Highest Achieved GPA:   %6.2f (5.0 Max Scale)\n", highestGPA)
	fmt.Printf("Lowest Achieved GPA:    %6.2f\n", lowestGPA)
	fmt.Println("==================================================")
}
