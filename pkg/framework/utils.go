package framework

import (
	"strings"
)

// ParseArgs is a utility function to parse command line arguments
// while correctly handling spaces inside quotes.
func ParseArgs(input string) []string {
	var args []string
	var current strings.Builder
	inQuote := false
	quoteChar := rune(0)

	for _, r := range input {
		if (r == '"' || r == '\'') && (!inQuote || quoteChar == r) {
			inQuote = !inQuote
			if inQuote {
				quoteChar = r
			}
		} else if r == ' ' && !inQuote {
			if current.Len() > 0 {
				args = append(args, current.String())
				current.Reset()
			}
		} else {
			current.WriteRune(r)
		}
	}
	if current.Len() > 0 {
		args = append(args, current.String())
	}
	return args
}
