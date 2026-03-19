package prompt

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Provider returns the prompt string, supporting static and dynamic commands
// with optional TTL caching and timeout.
type Provider struct {
	Static        string
	Cmd           string
	TTL           time.Duration
	Timeout       time.Duration
	AllowExternal bool // if true, allow [cmd] tokens

	mu     sync.Mutex
	last   string
	lastAt time.Time
}

// Get returns the current prompt string. If KUSH_PROMPT or Cmd is set, it will
// be evaluated/expanded (with per-prompt timeout) and cached per TTL.
func (p *Provider) Get() string {
	if p == nil {
		return "$ "
	}
	// Use Cmd if present, otherwise Static as raw input. We'll expand tokens in
	// the resulting string so callers may place % tokens, [..] commands, or {..}
	// script calls.
	raw := ""
	if v := os.Getenv("KUSH_PROMPT"); v != "" {
		raw = v
	} else if p.Cmd != "" {
		raw = p.Cmd
	} else if p.Static != "" {
		raw = p.Static
	} else {
		raw = "$ "
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.TTL > 0 && time.Since(p.lastAt) < p.TTL && p.last != "" && os.Getenv("KUSH_PROMPT") == "" {
		// if KUSH_PROMPT set, bypass cached value to allow immediate reloads
		return p.last
	}

	// Context for external runs
	ctx := context.Background()
	if p.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, p.Timeout)
		defer cancel()
	}

	val := expandPrompt(ctx, raw, p.AllowExternal)
	if val == "" && p.Static != "" {
		val = p.Static
	}
	if val == "" {
		val = "$ "
	}
	p.last = val
	p.lastAt = time.Now()
	return val
}

// expandPrompt replaces tokens in the raw prompt string. Supported tokens:
//  - %% -> literal %
//  - %T, %t, %H, %h, %m, %s, %p, %P
//  - [cmd] -> execute external command via sh -c (allowed only when allowExternal=true)
//  - {script} -> execute a local script file (if executable) via sh -c
// Backslash can escape special chars: \[ \{ \% \\
func expandPrompt(ctx context.Context, raw string, allowExternal bool) string {
	var out bytes.Buffer
	r := []rune(raw)
	for i := 0; i < len(r); i++ {
		switch r[i] {
		case '\\':
			// escape next char if present
			if i+1 < len(r) {
				out.WriteRune(r[i+1])
				i++
			}
		case '%':
			// builtin tokens: single letter or short keyword
			if i+1 < len(r) {
				n := r[i+1]
				switch n {
				case '%':
					out.WriteRune('%')
				case 'T':
					out.WriteString(time.Now().Format("2006-01-02 15:04:05"))
				case 't':
					out.WriteString(time.Now().Format("15:04:05"))
				case 'H':
					h, _ := os.Hostname()
					out.WriteString(h)
				case 'h':
					out.WriteString(time.Now().Format("15"))
				case 'm':
					out.WriteString(time.Now().Format("04"))
				case 's':
					out.WriteString(time.Now().Format("05"))
				case 'p':
					cwd, _ := os.Getwd()
					parts := strings.Split(cwd, "/")
					if len(parts) > 0 {
						out.WriteString(parts[len(parts)-1])
					}
				case 'P':
					cwd, _ := os.Getwd()
					out.WriteString(cwd)
				default:
					// unknown, emit % and the char
					out.WriteRune('%')
					out.WriteRune(n)
				}
				i++
			} else {
				// trailing %, emit it
				out.WriteRune('%')
			}
		case '[':
			// external command until closing ]
			j := i + 1
			for j < len(r) && r[j] != ']' {
				j++
			}
			if j >= len(r) {
				// no closing, emit [
				out.WriteRune('[')
				continue
			}
			cmd := string(r[i+1 : j])
			if allowExternal {
				res := runCommand(ctx, cmd)
				out.WriteString(res)
			} else {
				// external disabled; emit nothing or placeholder
			}
			i = j
		case '{':
			// script file path until closing }
			j := i + 1
			for j < len(r) && r[j] != '}' {
				j++
			}
			if j >= len(r) {
				out.WriteRune('{')
				continue
			}
			path := string(r[i+1 : j])
			// if file exists and is executable, run it. Otherwise, try as shell command
			res := ""
			if fi, err := os.Stat(path); err == nil && fi.Mode().IsRegular() && fi.Mode().Perm()&0111 != 0 {
				res = runCommand(ctx, path)
			} else {
				// not executable file; attempt to run via sh -c
				res = runCommand(ctx, path)
			}
			out.WriteString(res)
			i = j
		default:
			out.WriteRune(r[i])
		}
	}
	return out.String()
}

// runCommand executes cmd via sh -c and returns trimmed stdout on success.
func runCommand(ctx context.Context, cmd string) string {
	c := exec.CommandContext(ctx, "sh", "-c", cmd)
	c.Env = os.Environ()
	b, err := c.Output()
	if err != nil {
		return ""
	}
	return strings.TrimRight(string(b), "\n")
}

// Invalidate clears cached prompt so Get() will re-evaluate immediately.
func (p *Provider) Invalidate() {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lastAt = time.Time{}
	p.last = ""
}
