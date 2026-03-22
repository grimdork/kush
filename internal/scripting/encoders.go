package scripting

import (
	"github.com/d5/tengo/v2"
	"github.com/grimdork/kush/internal/base"
)

func init() {
	RegisterFactory(func(e *Engine) map[string]*tengo.UserFunction {
		m := map[string]*tengo.UserFunction{}

		m["encode64"] = &tengo.UserFunction{
			Name:  "encode64",
			Value: handleEncode64,
		}

		m["encode64url"] = &tengo.UserFunction{
			Name:  "encode64url",
			Value: handleEncode64URL,
		}

		m["decode64"] = &tengo.UserFunction{
			Name:  "decode64",
			Value: handleDecode64,
		}

		m["decode64url"] = &tengo.UserFunction{
			Name:  "decode64url",
			Value: handleDecode64URL,
		}

		// keep encoden/decoden here (bytes-based)
		m["encoden"] = &tengo.UserFunction{
			Name:  "encoden",
			Value: handleEncoden,
		}

		m["decoden"] = &tengo.UserFunction{
			Name:  "decoden",
			Value: handleDecoden,
		}

		return m
	})
}

func handleEncode64(a ...tengo.Object) (tengo.Object, error) {
	if len(a) < 1 {
		return nil, tengo.ErrWrongNumArguments
	}
	s, ok := tengo.ToString(a[0])
	if !ok {
		return nil, tengo.ErrInvalidArgumentType{Name: "s", Expected: "string", Found: a[0].TypeName()}
	}
	return &tengo.String{Value: string(base.EncodeBase64(s))}, nil
}

func handleEncode64URL(a ...tengo.Object) (tengo.Object, error) {
	if len(a) < 1 {
		return nil, tengo.ErrWrongNumArguments
	}
	s, ok := tengo.ToString(a[0])
	if !ok {
		return nil, tengo.ErrInvalidArgumentType{Name: "s", Expected: "string", Found: a[0].TypeName()}
	}
	return &tengo.String{Value: string(base.EncodeBase64URL(s))}, nil
}

func handleDecode64(a ...tengo.Object) (tengo.Object, error) {
	if len(a) < 1 {
		return nil, tengo.ErrWrongNumArguments
	}
	s, ok := tengo.ToString(a[0])
	if !ok {
		return nil, tengo.ErrInvalidArgumentType{Name: "s", Expected: "string", Found: a[0].TypeName()}
	}
	b := base.DecodeBase64([]byte(s))
	if b == nil {
		return &tengo.String{Value: ""}, nil
	}
	return &tengo.String{Value: string(b)}, nil
}

func handleDecode64URL(a ...tengo.Object) (tengo.Object, error) {
	if len(a) < 1 {
		return nil, tengo.ErrWrongNumArguments
	}
	s, ok := tengo.ToString(a[0])
	if !ok {
		return nil, tengo.ErrInvalidArgumentType{Name: "s", Expected: "string", Found: a[0].TypeName()}
	}
	b := base.DecodeBase64URL([]byte(s))
	if b == nil {
		return &tengo.String{Value: ""}, nil
	}
	return &tengo.String{Value: string(b)}, nil
}

func handleEncoden(a ...tengo.Object) (tengo.Object, error) {
	if len(a) < 1 {
		return nil, tengo.ErrWrongNumArguments
	}
	s, ok := tengo.ToString(a[0])
	if !ok {
		return nil, tengo.ErrInvalidArgumentType{Name: "data", Expected: "string", Found: a[0].TypeName()}
	}
	sz := 64
	if len(a) >= 2 {
		if v, ok := tengo.ToInt(a[1]); ok {
			sz = v
		}
	}
	res := base.BytesToBaseN([]byte(s), sz)
	return &tengo.String{Value: res}, nil
}

func handleDecoden(a ...tengo.Object) (tengo.Object, error) {
	if len(a) < 1 {
		return nil, tengo.ErrWrongNumArguments
	}
	s, ok := tengo.ToString(a[0])
	if !ok {
		return nil, tengo.ErrInvalidArgumentType{Name: "s", Expected: "string", Found: a[0].TypeName()}
	}
	sz := 64
	if len(a) >= 2 {
		if v, ok := tengo.ToInt(a[1]); ok {
			sz = v
		}
	}
	b := base.BaseNToBytes(s, sz)
	if b == nil {
		return &tengo.String{Value: ""}, nil
	}
	return &tengo.String{Value: string(b)}, nil
}
