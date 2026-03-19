package builtins

// registry holds builtin handlers registered via init().
var registry = make(map[string]func(*Builtins, string) bool)

// Register registers a builtin handler by name. Call from init() in per-builtin files.
func Register(name string, h func(*Builtins, string) bool) {
	registry[name] = h
}

// collectHandlers returns a copy of the registry mapped to a Builtins instance.
func collectHandlers(b *Builtins) map[string]func(string) bool {
	h := make(map[string]func(string) bool)
	for k, fn := range registry {
		// wrap to bind the Builtins receiver
		localFn := fn
		h[k] = func(line string) bool { return localFn(b, line) }
	}
	return h
}
