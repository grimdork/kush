package shell

import (
	"strings"
	"unicode"
)

// segment represents a single command in a pipeline, with optional output redirection.
type segment struct {
	line       string // the command text (trimmed)
	redirectTo string // file path for > or >>
	appendMode bool   // true for >>, false for >
}

// parsePipeline splits a command line into pipe-separated segments, each with
// optional output redirection. It respects single and double quotes so that
// operators inside quoted strings are treated as literals.
//
// Examples:
//
//	"ls -la | grep foo"        → [{line:"ls -la"}, {line:"grep foo"}]
//	"echo hello>file.txt"      → [{line:"echo hello", redirectTo:"file.txt"}]
//	"cat foo | sort >> out"    → [{line:"cat foo"}, {line:"sort", redirectTo:"out", appendMode:true}]
//	"echo 'a|b' > f"          → [{line:"echo 'a|b'", redirectTo:"f"}]
func parsePipeline(input string) []segment {
	// First split on pipe operators (outside quotes).
	parts := splitOutsideQuotes(input, '|')
	segments := make([]segment, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		seg := parseRedirect(p)
		segments = append(segments, seg)
	}
	return segments
}

// splitOutsideQuotes splits s on the given delimiter, ignoring delimiters
// inside single or double quotes. Backslash escapes are not handled (kush
// delegates to sh for complex quoting).
func splitOutsideQuotes(s string, delim rune) []string {
	var parts []string
	var cur strings.Builder
	inSingle := false
	inDouble := false
	for _, r := range s {
		if r == '\'' && !inDouble {
			inSingle = !inSingle
			cur.WriteRune(r)
			continue
		}
		if r == '"' && !inSingle {
			inDouble = !inDouble
			cur.WriteRune(r)
			continue
		}
		if r == delim && !inSingle && !inDouble {
			parts = append(parts, cur.String())
			cur.Reset()
			continue
		}
		cur.WriteRune(r)
	}
	parts = append(parts, cur.String())
	return parts
}

// parseRedirect extracts > or >> redirection from the end of a command segment.
// It handles both spaced ("cmd > file") and compact ("cmd>file") forms.
func parseRedirect(s string) segment {
	runes := []rune(s)
	inSingle := false
	inDouble := false

	// Scan for the last unquoted > that isn't part of >>
	// We want the rightmost redirect operator.
	lastGt := -1
	for i, r := range runes {
		if r == '\'' && !inDouble {
			inSingle = !inSingle
			continue
		}
		if r == '"' && !inSingle {
			inDouble = !inDouble
			continue
		}
		if r == '>' && !inSingle && !inDouble {
			lastGt = i
		}
	}

	if lastGt < 0 {
		return segment{line: s}
	}

	// Check for >>
	appendMode := false
	cmdEnd := lastGt
	fileStart := lastGt + 1
	if lastGt > 0 && runes[lastGt-1] == '>' {
		appendMode = true
		cmdEnd = lastGt - 1
	}

	cmd := strings.TrimRightFunc(string(runes[:cmdEnd]), unicode.IsSpace)
	file := strings.TrimSpace(string(runes[fileStart:]))

	if file == "" {
		// No file specified — return as-is (let sh deal with the error)
		return segment{line: s}
	}

	return segment{
		line:       cmd,
		redirectTo: file,
		appendMode: appendMode,
	}
}

// hasPipelineOps returns true if the line contains unquoted | or > operators.
func hasPipelineOps(s string) bool {
	inSingle := false
	inDouble := false
	for _, r := range s {
		if r == '\'' && !inDouble {
			inSingle = !inSingle
			continue
		}
		if r == '"' && !inSingle {
			inDouble = !inDouble
			continue
		}
		if (r == '|' || r == '>') && !inSingle && !inDouble {
			return true
		}
	}
	return false
}
