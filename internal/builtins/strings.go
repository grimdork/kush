package builtins

import (
	"fmt"
	"strings"
)

func init() {
	Register("split", handleSplit)
}

func handleSplit(b *Builtins, line string) bool {
	tokens := shellSplit(line)
	for _, t := range tokens[1:] {
		fmt.Println(t)
	}
	return true
}

// shellSplit splits a command line respecting single and double quotes.
func shellSplit(line string) []string {
	var tokens []string
	var current strings.Builder
	inSingle := false
	inDouble := false

	for i := 0; i < len(line); i++ {
		c := line[i]
		switch {
		case c == '\'' && !inDouble:
			inSingle = !inSingle
		case c == '"' && !inSingle:
			inDouble = !inDouble
		case c == ' ' && !inSingle && !inDouble:
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		default:
			current.WriteByte(c)
		}
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}
	return tokens
}
