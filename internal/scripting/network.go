package scripting

import (
	"net"
	"sort"
	"strconv"
	"time"

	"github.com/d5/tengo/v2"
)

func init() {
	RegisterFactory(func(e *Engine) map[string]*tengo.UserFunction {
		m := map[string]*tengo.UserFunction{}

		m["checkport"] = &tengo.UserFunction{
			Name: "checkport",
			Value: func(a ...tengo.Object) (tengo.Object, error) {
				if len(a) < 2 {
					return nil, tengo.ErrWrongNumArguments
				}
				host, ok := tengo.ToString(a[0])
				if !ok {
					return nil, tengo.ErrInvalidArgumentType{Name: "host", Expected: "string", Found: a[0].TypeName()}
				}
				var portStr string
				if s, ok := tengo.ToString(a[1]); ok {
					portStr = s
				} else if i, ok := tengo.ToInt(a[1]); ok {
					portStr = strconv.Itoa(int(i))
				} else {
					return nil, tengo.ErrInvalidArgumentType{Name: "port", Expected: "string|int", Found: a[1].TypeName()}
				}
				addr := net.JoinHostPort(host, portStr)
				timeout := 2 * time.Second
				conn, err := net.DialTimeout("tcp", addr, timeout)
				if err != nil {
					return tengo.FalseValue, nil
				}
				_ = conn.Close()
				return tengo.TrueValue, nil
			},
		}

		m["ping"] = &tengo.UserFunction{
			Name: "ping",
			Value: func(a ...tengo.Object) (tengo.Object, error) {
				if len(a) < 1 {
					return nil, tengo.ErrWrongNumArguments
				}
				host, ok := tengo.ToString(a[0])
				if !ok {
					return nil, tengo.ErrInvalidArgumentType{Name: "host", Expected: "string", Found: a[0].TypeName()}
				}
				addr := net.JoinHostPort(host, "80")
				start := time.Now()
				conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
				if err != nil {
					return &tengo.Int{Value: -1}, nil
				}
				_ = conn.Close()
				ms := time.Since(start).Milliseconds()
				return &tengo.Int{Value: int64(ms)}, nil
			},
		}

		m["dig"] = &tengo.UserFunction{
			Name: "dig",
			Value: func(a ...tengo.Object) (tengo.Object, error) {
				if len(a) < 1 {
					return nil, tengo.ErrWrongNumArguments
				}
				host, ok := tengo.ToString(a[0])
				if !ok {
					return nil, tengo.ErrInvalidArgumentType{Name: "host", Expected: "string", Found: a[0].TypeName()}
				}
				ips, err := net.LookupIP(host)
				if err != nil {
					return nil, nil
				}
				var v4s []string
				var v6s []string
				for _, ip := range ips {
					if ip == nil {
						continue
					}
					if ip.To4() != nil {
						v4s = append(v4s, ip.String())
					} else {
						v6s = append(v6s, ip.String())
					}
				}
				sort.Strings(v4s)
				sort.Strings(v6s)

				// build helper functions
				ipv4Map := map[string]tengo.Object{}
				ipv4Map["first"] = &tengo.UserFunction{
					Name: "first",
					Value: func(a ...tengo.Object) (tengo.Object, error) {
						if len(v4s) == 0 {
							return &tengo.String{Value: ""}, nil
						}
						return &tengo.String{Value: v4s[0]}, nil
					},
				}
				ipv4Map["all"] = &tengo.UserFunction{
					Name: "all",
					Value: func(a ...tengo.Object) (tengo.Object, error) {
						return toTengoArray(v4s), nil
					},
				}

				ipv6Map := map[string]tengo.Object{}
				ipv6Map["first"] = &tengo.UserFunction{
					Name: "first",
					Value: func(a ...tengo.Object) (tengo.Object, error) {
						if len(v6s) == 0 {
							return &tengo.String{Value: ""}, nil
						}
						return &tengo.String{Value: v6s[0]}, nil
					},
				}
				ipv6Map["all"] = &tengo.UserFunction{
					Name: "all",
					Value: func(a ...tengo.Object) (tengo.Object, error) {
						return toTengoArray(v6s), nil
					},
				}

				res := &tengo.Map{Value: map[string]tengo.Object{
					"ipv4": &tengo.Map{Value: ipv4Map},
					"ipv6": &tengo.Map{Value: ipv6Map},
				}}
				return res, nil
			},
		}

		// HTTP wrappers (use the http helpers defined elsewhere in the package)
		m["httpget"] = &tengo.UserFunction{Name: "httpget", Value: httpGetFunc}
		m["httppost"] = &tengo.UserFunction{Name: "httppost", Value: httpPostFunc}

		return m
	})
}
