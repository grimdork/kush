package scripting

import (
	"os"
	"unicode/utf8"

	"github.com/d5/tengo/v2"
)

func init() {
	RegisterFactory(func(e *Engine) map[string]*tengo.UserFunction {
		m := map[string]*tengo.UserFunction{}

		m["loadfile"] = &tengo.UserFunction{
			Name: "loadfile",
			Value: func(a ...tengo.Object) (tengo.Object, error) {
				if len(a) < 1 {
					return nil, tengo.ErrWrongNumArguments
				}
				p, ok := tengo.ToString(a[0])
				if !ok {
					return nil, tengo.ErrInvalidArgumentType{Name: "path", Expected: "string", Found: a[0].TypeName()}
				}
				b, err := os.ReadFile(p)
				if err != nil {
					return &tengo.String{Value: ""}, nil
				}
				return &tengo.String{Value: string(b)}, nil
			},
		}

		m["loadtext"] = &tengo.UserFunction{
			Name: "loadtext",
			Value: func(a ...tengo.Object) (tengo.Object, error) {
				if len(a) < 1 {
					return nil, tengo.ErrWrongNumArguments
				}
				p, ok := tengo.ToString(a[0])
				if !ok {
					return nil, tengo.ErrInvalidArgumentType{Name: "path", Expected: "string", Found: a[0].TypeName()}
				}
				b, err := os.ReadFile(p)
				if err != nil {
					return &tengo.String{Value: ""}, nil
				}
				if !utf8.Valid(b) {
					return &tengo.String{Value: ""}, nil
				}
				return &tengo.String{Value: string(b)}, nil
			},
		}

		return m
	})
}
