// logging.go

// Package utils provides utility functions used throughout the application
package utils

import (
	"fmt"
)

// Color codes for terminal output
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorPurple = "\033[35m"
	colorCyan   = "\033[36m"
	colorBold   = "\033[1m"
)

// PrintHeader prints a header text with purple color
func PrintHeader(message string) {
	fmt.Println(colorPurple + colorBold + message + colorReset)
}

// PrintInfo prints an informational message with blue color
func PrintInfo(message string) {
	fmt.Println(colorBlue + message + colorReset)
}

// PrintSuccess prints a success message with green color
func PrintSuccess(message string) {
	fmt.Println(colorGreen + message + colorReset)
}

// PrintWarning prints a warning message with yellow color
func PrintWarning(message string) {
	fmt.Println(colorYellow + message + colorReset)
}

// PrintError prints an error message with red color and increments the global error count
func PrintError(message string) {
	fmt.Println(colorRed + message + colorReset)
}
