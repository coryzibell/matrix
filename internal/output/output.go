// Package output provides ANSI color-coded terminal output utilities.
//
// Supports colored headers, labeled items, and success messages with automatic
// color disabling via the NoColor flag. All output goes to stdout.
//
// Example:
//
//	output.Header("Processing files")
//	output.Item("Status", "complete")
//	output.Success("All done!")
package output

import "fmt"

// ANSI color codes
const (
	Green  = "\033[32m"
	Cyan   = "\033[36m"
	Yellow = "\033[33m"
	Red    = "\033[31m"
	Dim    = "\033[2m"
	Reset  = "\033[0m"
)

// NoColor disables color output when true
var NoColor bool

// color wraps text in an ANSI color code if NoColor is false
func color(colorCode, text string) string {
	if NoColor {
		return text
	}
	return colorCode + text + Reset
}

// Header prints colored header text in cyan
func Header(text string) {
	fmt.Println(color(Cyan, text))
}

// Item prints a labeled item with the label in yellow
func Item(label string, value string) {
	fmt.Printf("%s %s\n", color(Yellow, label+":"), value)
}

// Success prints green success text
func Success(text string) {
	fmt.Println(color(Green, text))
}
