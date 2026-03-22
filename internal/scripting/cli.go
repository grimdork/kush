package scripting

import (
	"fmt"
	"os"

	"github.com/d5/tengo/v2"
)

func init() {
	RegisterFactory(func(e *Engine) map[string]*tengo.UserFunction {
		m := map[string]*tengo.UserFunction{}

		m["cwd"] = &tengo.UserFunction{
			Name: "cwd",
			Value: func(a ...tengo.Object) (tengo.Object, error) {
				dir, err := os.Getwd()
				if err != nil {
					return &tengo.String{Value: ""}, nil
				}
				return &tengo.String{Value: dir}, nil
			},
		}

		m["printf"] = &tengo.UserFunction{
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
		}

		m["pr"] = &tengo.UserFunction{
			Name: "pr",
			Value: func(a ...tengo.Object) (tengo.Object, error) {
				parts := make([]any, len(a))
				for i, obj := range a {
					s, _ := tengo.ToString(obj)
					parts[i] = s
				}
				fmt.Println(parts...)
				return tengo.UndefinedValue, nil
			},
		}

		m["print"] = &tengo.UserFunction{
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
		}

		// getenv / setenv: setenv needs access to the engine's prompt provider
		m["getenv"] = &tengo.UserFunction{
			Name: "getenv",
			Value: func(a ...tengo.Object) (tengo.Object, error) {
				if len(a) < 1 {
					return nil, tengo.ErrWrongNumArguments
				}
				key, ok := tengo.ToString(a[0])
				if !ok {
					return nil, tengo.ErrInvalidArgumentType{Name: "key", Expected: "string", Found: a[0].TypeName()}
				}
				val := os.Getenv(key)
				return &tengo.String{Value: val}, nil
			},
		}

		m["setenv"] = &tengo.UserFunction{
			Name: "setenv",
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
		}

		return m
	})
}
