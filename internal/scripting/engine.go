// Package scripting provides the Tengo script runtime for kush.
package scripting

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/d5/tengo/v2"
	"github.com/d5/tengo/v2/stdlib"
	"github.com/grimdork/kush/internal/prompt"
)

// Engine holds the scripting runtime state.
type Engine struct {
	pp         *prompt.Provider
	blessedDir string // blessed script directory
}

// New creates a new scripting engine.
func New(pp *prompt.Provider) *Engine {
	dir := os.Getenv("KUSH_SCRIPTS")
	if dir == "" {
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, ".kush", "scripts")
	}
	return &Engine{pp: pp, blessedDir: dir}
}

// BlessedDir returns the path to the blessed scripts directory.
func (e *Engine) BlessedDir() string {
	return e.blessedDir
}

// ListBlessed returns the names of all .tengo scripts in the blessed directory
// (without extension), suitable for registering as builtins.
func (e *Engine) ListBlessed() []string {
	entries, err := os.ReadDir(e.blessedDir)
	if err != nil {
		return nil
	}
	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".tengo") {
			names = append(names, strings.TrimSuffix(name, ".tengo"))
		}
	}
	return names
}

// RunFile executes a Tengo script file with the given arguments.
func (e *Engine) RunFile(path string, args []string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read script: %w", err)
	}
	return e.run(string(data), path, args)
}

// RunBlessed executes a blessed script by name.
func (e *Engine) RunBlessed(name string, args []string) error {
	path := filepath.Join(e.blessedDir, name+".tengo")
	return e.RunFile(path, args)
}

// Eval executes a Tengo expression/script string.
func (e *Engine) Eval(code string) error {
	return e.run(code, "<eval>", nil)
}

func (e *Engine) run(code, filename string, args []string) error {
	script := tengo.NewScript([]byte(code))

	// Add standard library modules
	script.SetImports(stdlib.GetModuleMap(
		"math", "text", "times", "rand", "fmt", "json", "base64", "hex", "os",
	))

	// Set script args
	if args == nil {
		args = []string{}
	}
	argsObj := make([]tengo.Object, len(args))
	for i, a := range args {
		argsObj[i] = &tengo.String{Value: a}
	}
	_ = script.Add("args", &tengo.Array{Value: argsObj})

	// Add kush module functions as top-level variables
	_ = script.Add("env_get", &tengo.UserFunction{
		Name: "env_get",
		Value: func(a ...tengo.Object) (tengo.Object, error) {
			if len(a) < 1 {
				return nil, tengo.ErrWrongNumArguments
			}
			key, ok := tengo.ToString(a[0])
			if !ok {
				return nil, tengo.ErrInvalidArgumentType{Name: "key", Expected: "string", Found: a[0].TypeName()}
			}
			val := os.Getenv(key)
			if val == "" {
				return tengo.UndefinedValue, nil
			}
			return &tengo.String{Value: val}, nil
		},
	})

	_ = script.Add("env_set", &tengo.UserFunction{
		Name: "env_set",
		Value: func(a ...tengo.Object) (tengo.Object, error) {
			if len(a) < 2 {
				return nil, tengo.ErrWrongNumArguments
			}
			key, ok := tengo.ToString(a[0])
			if !ok {
				return nil, tengo.ErrInvalidArgumentType{Name: "key", Expected: "string", Found: a[0].TypeName()}
			}
			val, ok := tengo.ToString(a[1])
			if !ok {
				return nil, tengo.ErrInvalidArgumentType{Name: "value", Expected: "string", Found: a[1].TypeName()}
			}
			os.Setenv(key, val)
			if e.pp != nil {
				e.pp.Invalidate()
			}
			return tengo.UndefinedValue, nil
		},
	})

	_ = script.Add("cwd", &tengo.UserFunction{
		Name: "cwd",
		Value: func(a ...tengo.Object) (tengo.Object, error) {
			dir, err := os.Getwd()
			if err != nil {
				return &tengo.String{Value: ""}, nil
			}
			return &tengo.String{Value: dir}, nil
		},
	})

	_ = script.Add("print", &tengo.UserFunction{
		Name: "print",
		Value: func(a ...tengo.Object) (tengo.Object, error) {
			parts := make([]any, len(a))
			for i, obj := range a {
				s, _ := tengo.ToString(obj)
				parts[i] = s
			}
			fmt.Print(parts...)
			return tengo.UndefinedValue, nil
		},
	})

	_ = script.Add("println", &tengo.UserFunction{
		Name: "println",
		Value: func(a ...tengo.Object) (tengo.Object, error) {
			parts := make([]any, len(a))
			for i, obj := range a {
				s, _ := tengo.ToString(obj)
				parts[i] = s
			}
			fmt.Println(parts...)
			return tengo.UndefinedValue, nil
		},
	})

	_ = script.Add("printf", &tengo.UserFunction{
		Name: "printf",
		Value: func(a ...tengo.Object) (tengo.Object, error) {
			if len(a) < 1 {
				return nil, tengo.ErrWrongNumArguments
			}
			format, ok := tengo.ToString(a[0])
			if !ok {
				return nil, tengo.ErrInvalidArgumentType{Name: "format", Expected: "string", Found: a[0].TypeName()}
			}
			fmtArgs := make([]any, len(a)-1)
			for i, obj := range a[1:] {
				s, _ := tengo.ToString(obj)
				fmtArgs[i] = s
			}
			fmt.Printf(format, fmtArgs...)
			return tengo.UndefinedValue, nil
		},
	})

	// HTTP functions available to scripts
	_ = script.Add("http_get", &tengo.UserFunction{
		Name:  "http_get",
		Value: httpGetFunc,
	})
	_ = script.Add("http_post", &tengo.UserFunction{
		Name:  "http_post",
		Value: httpPostFunc,
	})

	_, err := script.RunContext(context.Background())
	if err != nil {
		return fmt.Errorf("%s: %w", filename, err)
	}
	return nil
}
