package shell

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/grimdork/kush/internal/builtins"
)

// executePipeline runs a parsed pipeline, handling builtins with redirects and
// pipes. Returns true if at least the first segment was a builtin (meaning the
// shell should not fall through to RunShell).
//
// Supported patterns:
//   - builtin > file
//   - builtin >> file
//   - builtin | external_cmd
//   - builtin | external_cmd > file
//   - external>file  (passed to sh, but parsed correctly)
//
// If no segment is a builtin, returns false so the whole line goes to RunShell.
func executePipeline(segs []segment, bt *builtins.Builtins) bool {
	if len(segs) == 0 {
		return false
	}

	// If first segment is not a builtin, let RunShell handle the whole line
	// (sh already understands pipes and redirects).
	if !bt.IsBuiltin(segs[0].line) {
		return false
	}

	// Single segment with redirect only (no pipe).
	if len(segs) == 1 {
		seg := segs[0]
		if seg.redirectTo != "" {
			return redirectBuiltin(seg, bt)
		}
		// No redirect, no pipe — shouldn't reach here (hasPipelineOps was true),
		// but handle gracefully.
		return bt.Handle(seg.line)
	}

	// Pipeline: builtin | cmd [| cmd ...] [> file]
	// Run the builtin, capture its output, then feed it through the remaining
	// pipeline segments using sh.
	var buf bytes.Buffer
	if !bt.HandleTo(segs[0].line, &buf) {
		return false
	}

	// Build the rest of the pipeline as a shell command string.
	var rest strings.Builder
	for i := 1; i < len(segs); i++ {
		if i > 1 {
			rest.WriteString(" | ")
		}
		rest.WriteString(segs[i].line)
		if segs[i].redirectTo != "" {
			if segs[i].appendMode {
				rest.WriteString(" >> ")
			} else {
				rest.WriteString(" > ")
			}
			rest.WriteString(segs[i].redirectTo)
		}
	}

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "sh"
	}
	cmd := exec.Command(shell, "-c", rest.String())
	cmd.Stdin = &buf
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
	}
	return true
}

// redirectBuiltin runs a builtin with output redirected to a file.
func redirectBuiltin(seg segment, bt *builtins.Builtins) bool {
	flags := os.O_WRONLY | os.O_CREATE
	if seg.appendMode {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}
	f, err := os.OpenFile(seg.redirectTo, flags, 0644)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return true // was a builtin, just failed to redirect
	}
	defer f.Close()
	bt.HandleTo(seg.line, io.Writer(f))
	return true
}
