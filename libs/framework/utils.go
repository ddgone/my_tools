package framework

import (
	"fmt"
	"strings"
	"unicode"
)

// ParseArgs parses a CLI-like argument string into argv tokens.
// It supports:
// - spaces as separators
// - single and double quoted strings
// - escaped quotes and backslashes inside quoted strings
// - escaped spaces/quotes/backslashes outside quoted strings
func ParseArgs(input string) ([]string, error) {
	var args []string
	var current strings.Builder
	var quote rune
	tokenStarted := false

	runes := []rune(input)
	for i := 0; i < len(runes); i++ {
		r := runes[i]

		if quote == 0 && unicode.IsSpace(r) {
			if tokenStarted {
				args = append(args, current.String())
				current.Reset()
				tokenStarted = false
			}
			continue
		}

		switch r {
		case '\\':
			if i+1 >= len(runes) {
				if quote != 0 {
					return nil, fmt.Errorf("参数解析失败：尾部转义符不完整")
				}
				current.WriteRune(r)
				tokenStarted = true
				continue
			}

			next := runes[i+1]
			switch quote {
			case 0:
				if unicode.IsSpace(next) || next == '"' || next == '\'' || next == '\\' {
					current.WriteRune(next)
					i++
				} else {
					current.WriteRune(r)
				}
			case '"':
				if next == '"' || next == '\\' {
					current.WriteRune(next)
					i++
				} else {
					current.WriteRune(r)
				}
			case '\'':
				if next == '\'' || next == '\\' {
					current.WriteRune(next)
					i++
				} else {
					current.WriteRune(r)
				}
			}
			tokenStarted = true
		case '"', '\'':
			if quote == 0 {
				quote = r
				tokenStarted = true
				continue
			}
			if quote == r {
				quote = 0
				continue
			}
			current.WriteRune(r)
			tokenStarted = true
		default:
			current.WriteRune(r)
			tokenStarted = true
		}
	}

	if quote != 0 {
		if quote == '"' {
			return nil, fmt.Errorf("参数解析失败：双引号未闭合")
		}
		return nil, fmt.Errorf("参数解析失败：单引号未闭合")
	}

	if tokenStarted {
		args = append(args, current.String())
	}
	return args, nil
}
