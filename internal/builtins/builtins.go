package builtins

import (
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
