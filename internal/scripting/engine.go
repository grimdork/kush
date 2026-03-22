// Package scripting provides the Tengo script runtime for kush.
package scripting

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/d5/tengo/v2"
	"github.com/d5/tengo/v2/stdlib"
	"github.com/grimdork/kush/internal/base"
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
		} else if strings.HasSuffix(name, ".t") {
			names = append(names, strings.TrimSuffix(name, ".t"))
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

// RunBlessed executes a blessed script by name. It will prefer .tengo, then .t.
func (e *Engine) RunBlessed(name string, args []string) error {
	p1 := filepath.Join(e.blessedDir, name+".tengo")
	if _, err := os.Stat(p1); err == nil {
		return e.RunFile(p1, args)
	}
	p2 := filepath.Join(e.blessedDir, name+".t")
	if _, err := os.Stat(p2); err == nil {
		return e.RunFile(p2, args)
	}
	return fmt.Errorf("blessed script not found: %s", name)
}

// Eval executes a Tengo expression/script string.
func (e *Engine) Eval(code string) error {
	return e.run(code, "<eval>", nil)
}

func toTengoArray(vals []string) *tengo.Array {
	arr := make([]tengo.Object, len(vals))
	for i, v := range vals {
		arr[i] = &tengo.String{Value: v}
	}
	return &tengo.Array{Value: arr}
}

func (e *Engine) run(code, filename string, args []string) error {
	// If code starts with a shebang (#!), strip the first line so the Tengo parser
	// doesn't choke on it. This lets scripts start with "#!/usr/bin/env kush".
	if strings.HasPrefix(code, "#!") {
		if idx := strings.Index(code, "\n"); idx >= 0 {
			code = code[idx+1:]
		} else {
			code = ""
		}
	}

	// Debug: show the start of the script (helpful when diagnosing shebang/parse issues)
	// NOTE: left in temporarily to verify shebang handling.
	if len(code) > 0 {
		// intentionally quiet here; don't print script contents in normal runs
	}
	script := tengo.NewScript([]byte(code))

	// Add standard library modules
	script.SetImports(stdlib.GetModuleMap(
		"math", "text", "times", "rand", "fmt", "json", "base64", "hex", "os",
	))

	// Set script args (args slice contains user args only, program name stripped)
	if args == nil {
		args = []string{}
	}
	argsArr := toTengoArray(args)
	_ = script.Add("args", argsArr)

	// Provide a `kush` object with metadata: name, path, args
	progName := filepath.Base(filename)
	kushMap := &tengo.Map{Value: map[string]tengo.Object{
		"name": &tengo.String{Value: progName},
		"path": &tengo.String{Value: filename},
		"args": argsArr,
	}}
	_ = script.Add("kush", kushMap)

	// CLI type constants
	_ = script.Add("FLAG", &tengo.Int{Value: 0})
	_ = script.Add("STRING", &tengo.Int{Value: 1})
	_ = script.Add("INT", &tengo.Int{Value: 2})

	// parseKeywords(spec, type, spec, type, ...) -> map (does not exit the process)
	_ = script.Add("parseKeywords", &tengo.UserFunction{
		Name: "parseKeywords",
		Value: func(a ...tengo.Object) (tengo.Object, error) {
			// specs come in pairs: specString, typeInt
			if len(a) == 0 || len(a)%2 != 0 {
				return nil, tengo.ErrWrongNumArguments
			}
			type specDef struct {
				short    string
				long     string
				required bool
				help     string
				typeID   int
				present  bool
			}
			defs := make([]specDef, 0, len(a)/2)
			for i := 0; i < len(a); i += 2 {
				specS, ok := tengo.ToString(a[i])
				if !ok {
					return nil, tengo.ErrInvalidArgumentType{Name: "spec", Expected: "string", Found: a[i].TypeName()}
				}
				typeID, ok := tengo.ToInt(a[i+1])
				if !ok {
					return nil, tengo.ErrInvalidArgumentType{Name: "type", Expected: "int", Found: a[i+1].TypeName()}
				}
				parts := strings.SplitN(specS, ":", 4)
				short := ""
				long := ""
				help := ""
				if len(parts) > 0 {
					short = parts[0]
				}
				if len(parts) > 1 {
					long = parts[1]
				}
				if len(parts) > 3 {
					help = parts[3]
				}
				req := false
				if strings.HasPrefix(short, "_") {
					req = true
					short = strings.TrimPrefix(short, "_")
				}
				if strings.HasPrefix(long, "_") {
					req = true
					long = strings.TrimPrefix(long, "_")
				}
				defs = append(defs, specDef{short: short, long: long, required: req, help: help, typeID: int(typeID)})
			}

			res := make(map[string]tengo.Object)
			byShort := map[string]*specDef{}
			byLong := map[string]*specDef{}
			for i := range defs {
				d := &defs[i]
				if d.short != "" {
					byShort[d.short] = d
				}
				if d.long != "" {
					byLong[d.long] = d
				}
				if d.typeID == 0 {
					if d.short != "" {
						res[d.short] = tengo.FalseValue
					}
					if d.long != "" {
						res[d.long] = tengo.FalseValue
					}
				}
			}

			// parse the current args slice
			i := 0
			for i < len(args) {
				tok := args[i]
				if strings.HasPrefix(tok, "--") {
					nameVal := strings.TrimPrefix(tok, "--")
					if strings.Contains(nameVal, "=") {
						parts := strings.SplitN(nameVal, "=", 2)
						name := parts[0]
						val := parts[1]
						if d, ok := byLong[name]; ok {
							d.present = true
							switch d.typeID {
							case 0:
								res[d.long] = tengo.TrueValue
								if d.short != "" {
									res[d.short] = tengo.TrueValue
								}
							case 1:
								res[d.long] = &tengo.String{Value: val}
								if d.short != "" {
									res[d.short] = &tengo.String{Value: val}
								}
							case 2:
								if v := parseInt(val); v == 0 && val != "0" {
									return nil, fmt.Errorf("invalid integer for --%s: %s", name, val)
								}
								res[d.long] = &tengo.Int{Value: int64(parseInt(val))}
								if d.short != "" {
									res[d.short] = &tengo.Int{Value: int64(parseInt(val))}
								}
							}
						}
					} else {
						name := nameVal
						if d, ok := byLong[name]; ok {
							d.present = true
							switch d.typeID {
							case 0:
								res[d.long] = tengo.TrueValue
								if d.short != "" {
									res[d.short] = tengo.TrueValue
								}
							case 1:
								if i+1 < len(args) {
									val := args[i+1]
									res[d.long] = &tengo.String{Value: val}
									if d.short != "" {
										res[d.short] = &tengo.String{Value: val}
									}
									i++
								} else {
									return nil, fmt.Errorf("missing value for --%s", name)
								}
							case 2:
								if i+1 < len(args) {
									val := args[i+1]
									if v := parseInt(val); v == 0 && val != "0" {
										return nil, fmt.Errorf("invalid integer for --%s: %s", name, val)
									}
									res[d.long] = &tengo.Int{Value: int64(parseInt(val))}
									if d.short != "" {
										res[d.short] = &tengo.Int{Value: int64(parseInt(val))}
									}
									i++
								} else {
									return nil, fmt.Errorf("missing value for --%s", name)
								}
							}
						}
					}
				} else if strings.HasPrefix(tok, "-") && len(tok) >= 2 {
					shorts := tok[1:]
					if len(shorts) > 1 {
						for j := 0; j < len(shorts); j++ {
							sch := string(shorts[j])
							if d, ok := byShort[sch]; ok {
								d.present = true
								switch d.typeID {
								case 0:
									res[d.short] = tengo.TrueValue
									if d.long != "" {
										res[d.long] = tengo.TrueValue
									}

								case 1:
									val := shorts[j+1:]
									if val == "" && i+1 < len(args) {
										val = args[i+1]
										i++
									}
									res[d.short] = &tengo.String{Value: val}
									if d.long != "" {
										res[d.long] = &tengo.String{Value: val}
									}

								case 2:
									val := shorts[j+1:]
									if val == "" && i+1 < len(args) {
										val = args[i+1]
										i++
									}
									if v := parseInt(val); v == 0 && val != "0" {
										return nil, fmt.Errorf("invalid integer for -%s: %s", sch, val)
									}
									res[d.short] = &tengo.Int{Value: int64(parseInt(val))}
									if d.long != "" {
										res[d.long] = &tengo.Int{Value: int64(parseInt(val))}
									}
								}
							}
						}
					} else {
						sch := string(shorts[0])
						if d, ok := byShort[sch]; ok {
							d.present = true
							switch d.typeID {
							case 0:
								res[d.short] = tengo.TrueValue
								if d.long != "" {
									res[d.long] = tengo.TrueValue
								}
							case 1:
								if i+1 < len(args) {
									val := args[i+1]
									res[d.short] = &tengo.String{Value: val}
									if d.long != "" {
										res[d.long] = &tengo.String{Value: val}
									}
									i++
								} else {
									return nil, fmt.Errorf("missing value for -%s", sch)
								}
							case 2:
								if i+1 < len(args) {
									val := args[i+1]
									if v := parseInt(val); v == 0 && val != "0" {
										return nil, fmt.Errorf("invalid integer for -%s: %s", sch, val)
									}
									res[d.short] = &tengo.Int{Value: int64(parseInt(val))}
									if d.long != "" {
										res[d.long] = &tengo.Int{Value: int64(parseInt(val))}
									}
									i++
								} else {
									return nil, fmt.Errorf("missing value for -%s", sch)
								}
							}
						}
					}
				} else {
					// positional arg, stop parsing
					break
				}
				i++
			}

			// capture remaining args and update global args/kush.args
			remaining := []string{}
			if i < len(args) {
				remaining = args[i:]
			}
			// update top-level args object and kush.args
			newArgsArr := toTengoArray(remaining)
			argsArr.Value = newArgsArr.Value
			kushMap.Value["args"] = newArgsArr

			// If help requested or missing required, return an error with help text
			helpLines := []string{}
			for _, d := range defs {
				helpLines = append(helpLines, fmt.Sprintf("-%s --%s\t%s\t%s", d.short, d.long, d.help, func() string {
					if d.required {
						return "(required)"
					}
					return ""
				}()))
			}
			helpText := strings.Join(helpLines, "\n")

			if h, ok := res["h"]; ok {
				if hb, _ := tengo.ToBool(h); hb {
					return nil, fmt.Errorf("%s", helpText)
				}
			}

			for _, d := range defs {
				if d.required && !d.present {
					return nil, fmt.Errorf("%s", helpText)
				}
			}

			return &tengo.Map{Value: res}, nil
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

	// Handy aliases and small network helpers
	_ = script.Add("getenv", &tengo.UserFunction{
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
	})

	_ = script.Add("setenv", &tengo.UserFunction{
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
	})

	_ = script.Add("pr", &tengo.UserFunction{
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
	})

	// alias style names for convenience - keep short names, avoid underscored variants
	// Note: getenv/setenv and checkport are the preferred names.

	// alias for port_check behaviour (checkport kept as the canonical name)
	_ = script.Add("checkport", &tengo.UserFunction{
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
	})

	// expose only the short env helpers
	_ = script.Add("getenv", &tengo.UserFunction{
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
	})

	_ = script.Add("setenv", &tengo.UserFunction{
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
	})

	// print (no trailing newline)
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

	// ping(host) -> int ms (simple TCP connect to port 80). Returns -1 on failure.
	_ = script.Add("ping", &tengo.UserFunction{
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
	})

	// dig(host) -> object with ipv4 and ipv6 helpers:
	// d.ipv4.first() -> string
	// d.ipv4.all()   -> array[string] (alphanumerically sorted)
	_ = script.Add("dig", &tengo.UserFunction{
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
	})

	// ---- New helpers from github.com/grimdork/base and simple loaders ----
	_ = script.Add("encode64", &tengo.UserFunction{
		Name: "encode64",
		Value: func(a ...tengo.Object) (tengo.Object, error) {
			if len(a) < 1 {
				return nil, tengo.ErrWrongNumArguments
			}
			s, ok := tengo.ToString(a[0])
			if !ok {
				return nil, tengo.ErrInvalidArgumentType{Name: "s", Expected: "string", Found: a[0].TypeName()}
			}
			return &tengo.String{Value: string(base.EncodeBase64(s))}, nil
		},
	})

	_ = script.Add("encode64url", &tengo.UserFunction{
		Name: "encode64url",
		Value: func(a ...tengo.Object) (tengo.Object, error) {
			if len(a) < 1 {
				return nil, tengo.ErrWrongNumArguments
			}
			s, ok := tengo.ToString(a[0])
			if !ok {
				return nil, tengo.ErrInvalidArgumentType{Name: "s", Expected: "string", Found: a[0].TypeName()}
			}
			return &tengo.String{Value: string(base.EncodeBase64URL(s))}, nil
		},
	})

	_ = script.Add("decode64", &tengo.UserFunction{
		Name: "decode64",
		Value: func(a ...tengo.Object) (tengo.Object, error) {
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
		},
	})

	_ = script.Add("decode64url", &tengo.UserFunction{
		Name: "decode64url",
		Value: func(a ...tengo.Object) (tengo.Object, error) {
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
		},
	})

	_ = script.Add("encoden", &tengo.UserFunction{
		Name: "encoden",
		Value: func(a ...tengo.Object) (tengo.Object, error) {
			if len(a) < 1 {
				return nil, tengo.ErrWrongNumArguments
			}
			s, ok := tengo.ToString(a[0])
			if !ok {
				return nil, tengo.ErrInvalidArgumentType{Name: "id", Expected: "string", Found: a[0].TypeName()}
			}
			id, err := strconv.ParseUint(s, 10, 64)
			if err != nil {
				return &tengo.String{Value: ""}, nil
			}
			sz := 64
			if len(a) >= 2 {
				if v, ok := tengo.ToInt(a[1]); ok {
					sz = v
				}
			}
			res := base.NumEncode(uint64(id), sz)
			return &tengo.String{Value: res}, nil
		},
	})

	_ = script.Add("decoden", &tengo.UserFunction{
		Name: "decoden",
		Value: func(a ...tengo.Object) (tengo.Object, error) {
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
			v := base.NumDecode(s, sz)
			return &tengo.String{Value: strconv.FormatUint(uint64(v), 10)}, nil
		},
	})

	_ = script.Add("loadfile", &tengo.UserFunction{
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
	})

	_ = script.Add("loadtext", &tengo.UserFunction{
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
	})

	// HTTP functions available to scripts (no underscores)
	_ = script.Add("httpget", &tengo.UserFunction{
		Name:  "httpget",
		Value: httpGetFunc,
	})
	_ = script.Add("httppost", &tengo.UserFunction{
		Name:  "httppost",
		Value: httpPostFunc,
	})

	_, err := script.RunContext(context.Background())
	if err != nil {
		sMsg := err.Error()
		// If the script produced a help/usage-style error (multi-line with option
		// markers), print the help text directly and don't treat it as a fatal
		// error. This avoids the top-level log.Fatal from prefixing the message
		// with a timestamp which looks noisy to users.
		sMsgLower := strings.ToLower(sMsg)
		if strings.Contains(sMsg, "--") || strings.Contains(sMsgLower, "show help") || strings.Contains(sMsgLower, "--help") {
			// Strip the "Runtime Error:" prefix and any trailing location stack
			// ("\n\tat ...") so the output is just the help text.
			clean := sMsg
			if idx := strings.Index(clean, "\n\tat "); idx >= 0 {
				clean = clean[:idx]
			}
			clean = strings.TrimSpace(strings.TrimPrefix(clean, "Runtime Error:"))
			fmt.Println(clean)
			return nil
		}
		return fmt.Errorf("%s: %w", filename, err)
	}
	return nil
}

func parseInt(s string) int {
	var v int
	fmt.Sscanf(s, "%d", &v)
	return v
}
