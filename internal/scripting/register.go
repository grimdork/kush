package scripting

import (
	"sync"

	"github.com/d5/tengo/v2"
)

var (
	mu        sync.RWMutex
	factories []func(e *Engine) map[string]*tengo.UserFunction
)

// RegisterFactory registers a factory that will be invoked with the current
// Engine pointer when building the builtin map.
func RegisterFactory(f func(e *Engine) map[string]*tengo.UserFunction) {
	mu.Lock()
	defer mu.Unlock()
	factories = append(factories, f)
}

// Builtins invokes all registered factories and merges their maps.
func Builtins(e *Engine) map[string]*tengo.UserFunction {
	mu.RLock()
	fs := make([]func(e *Engine) map[string]*tengo.UserFunction, len(factories))
	copy(fs, factories)
	mu.RUnlock()
	res := make(map[string]*tengo.UserFunction)
	for _, f := range fs {
		m := f(e)
		for k, v := range m {
			res[k] = v
		}
	}
	return res
}
