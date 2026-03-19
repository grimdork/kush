package builtins

import (
	"io"
	"os"
	"strings"

	"github.com/grimdork/kush/internal/prompt"
)

// Builtins provides handling for built-in commands that are executed directly by the shell rather than via exec.
type Builtins struct {
	pp       *prompt.Provider
	handlers map[string]func(string) bool
}

// New returns a new Builtins instance. Pass a prompt.Provider so builtins can invalidate the prompt cache on env changes.
func New(pp *prompt.Provider) *Builtins {
	b := &Builtins{pp: pp, handlers: make(map[string]func(string) bool)}
	// Populate handlers from package registry (registered via init() in per-builtin files).
	b.handlers = collectHandlers(b)
	return b
}

// RegisterHandler adds a dynamic builtin handler at runtime.
func (b *Builtins) RegisterHandler(name string, fn func(string) bool) {
	b.handlers[name] = fn
}

// IsBuiltin returns true if the first token of line matches a registered builtin.
func (b *Builtins) IsBuiltin(line string) bool {
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return false
	}
	_, ok := b.handlers[parts[0]]
	return ok
}

// Handle returns true if the line was handled by a builtin.
func (b *Builtins) Handle(line string) bool {
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return false
	}
	if h, ok := b.handlers[parts[0]]; ok {
		return h(line)
	}
	return false
}

// HandleTo runs a builtin with stdout redirected to w. Returns true if the
// line was handled. If w is nil, stdout is used as normal.
func (b *Builtins) HandleTo(line string, w io.Writer) bool {
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return false
	}
	h, ok := b.handlers[parts[0]]
	if !ok {
		return false
	}
	if w == nil {
		return h(line)
	}
	// Temporarily redirect os.Stdout to a pipe, copy output to w.
	origStdout := os.Stdout
	pr, pw, err := os.Pipe()
	if err != nil {
		return h(line) // fallback to normal
	}
	os.Stdout = pw
	done := make(chan struct{})
	go func() {
		io.Copy(w, pr)
		close(done)
	}()
	result := h(line)
	pw.Close()
	<-done
	pr.Close()
	os.Stdout = origStdout
	return result
}
