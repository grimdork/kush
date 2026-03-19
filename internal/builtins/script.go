package builtins

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/grimdork/kush/internal/scripting"
)

func init() {
	Register("run", (*Builtins).handleRun)
	Register("eval", (*Builtins).handleEval)
}

func (b *Builtins) handleRun(line string) bool {
	tokens := shellSplit(line)
	if len(tokens) < 2 {
		fmt.Fprintln(os.Stderr, "usage: run <script.tengo> [args...]")
		return true
	}

	eng := scripting.New(b.pp)
	scriptName := tokens[1]
	args := tokens[2:]

	// If the script name has no path separator and no extension, check blessed dir first
	if !strings.Contains(scriptName, string(filepath.Separator)) && !strings.HasSuffix(scriptName, ".tengo") {
		if err := eng.RunBlessed(scriptName, args); err == nil {
			return true
		}
		// Fall through to try as a file path with .tengo appended
		scriptName += ".tengo"
	}

	if err := eng.RunFile(scriptName, args); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
	}
	return true
}

func (b *Builtins) handleEval(line string) bool {
	// Everything after "eval " is the code
	idx := strings.Index(line, " ")
	if idx < 0 || strings.TrimSpace(line[idx:]) == "" {
		fmt.Fprintln(os.Stderr, "usage: eval '<tengo expression>'")
		return true
	}
	code := strings.TrimSpace(line[idx+1:])

	// Strip surrounding quotes if present (convenience for eval 'code' or eval "code")
	if len(code) >= 2 {
		if (code[0] == '\'' && code[len(code)-1] == '\'') ||
			(code[0] == '"' && code[len(code)-1] == '"') {
			code = code[1 : len(code)-1]
		}
	}

	eng := scripting.New(b.pp)
	if err := eng.Eval(code); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
	}
	return true
}
