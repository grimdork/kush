package shell

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/grimdork/kush/internal/aliases"
	"github.com/grimdork/kush/internal/builtins"
	"github.com/grimdork/kush/internal/config"
	"github.com/grimdork/kush/internal/ed"
	"github.com/grimdork/kush/internal/log"
	"github.com/grimdork/kush/internal/prompt"
	"github.com/grimdork/kush/internal/runner"
	"github.com/grimdork/kush/internal/scripting"
)

// parseArgs splits a command line into arguments similar to a simple shell.
// Supports single quotes (literal), double quotes (with backslash escapes),
// and backslash escapes outside quotes.
func parseArgs(line string) []string {
	var out []string
	var buf strings.Builder
	inSingle := false
	inDouble := false
	escape := false
	for i := 0; i < len(line); i++ {
		c := line[i]
		if escape {
			buf.WriteByte(c)
			escape = false
			continue
		}
		if c == '\\' {
			escape = true
			continue
		}
		if inSingle {
			if c == '\'' {
				inSingle = false
			} else {
				buf.WriteByte(c)
			}
			continue
		}
		if inDouble {
			if c == '"' {
				inDouble = false
			} else {
				buf.WriteByte(c)
			}
			continue
		}
		// not in any quote
		if c == '\'' {
			inSingle = true
			continue
		}
		if c == '"' {
			inDouble = true
			continue
		}
		if c == ' ' || c == '\t' || c == '\n' {
			if buf.Len() > 0 {
				out = append(out, buf.String())
				buf.Reset()
			}
			continue
		}
		buf.WriteByte(c)
	}
	if buf.Len() > 0 {
		out = append(out, buf.String())
	}
	return out
}

// Run starts the REPL loop. It returns when the user exits or on error.
func Run() error {
	le, err := ed.New()
	if err != nil {
		return err
	}
	defer le.Close()

	al, _ := aliases.Load()
	cfg, _ := config.Load()

	// build prompt provider from env/config
	pp := &prompt.Provider{Static: "$ ", TTL: 0}
	if os.Getenv("KUSH_PROMPT_ALLOW_EXTERNAL") == "1" {
		pp.AllowExternal = true
	}
	if v := os.Getenv("PROMPT"); v != "" {
		pp.Static = v
	}
	if v := os.Getenv("PROMPT_CMD"); v != "" {
		pp.Cmd = v
	}
	if cfg != nil {
		if v, ok := cfg["PROMPT"]; ok && v != "" {
			pp.Static = v
		}
		if v, ok := cfg["PROMPT_CMD"]; ok && v != "" {
			pp.Cmd = v
		}
		if v, ok := cfg["PROMPT_TTL"]; ok && v != "" {
			if d, err := time.ParseDuration(v); err == nil {
				pp.TTL = d
			}
		}
		if v, ok := cfg["PROMPT_TIMEOUT_MS"]; ok && v != "" {
			if ms, err := strconv.ParseInt(v, 10, 64); err == nil {
				pp.Timeout = time.Duration(ms) * time.Millisecond
			}
		}
	}

	// After prompt provider is constructed, create builtins with access to it so builtins can invalidate the prompt cache.
	bt := builtins.New(pp)

	// Register blessed Tengo scripts as builtins.
	eng := scripting.New(pp)
	for _, name := range eng.ListBlessed() {
		scriptName := name // capture for closure
		bt.RegisterHandler(scriptName, func(line string) bool {
			tokens := parseArgs(line)
			var args []string
			if len(tokens) > 1 {
				args = tokens[1:]
			}
			if err := eng.RunBlessed(scriptName, args); err != nil {
				fmt.Fprintln(os.Stderr, "error:", err)
			}
			return true
		})
	}

	// Reload config on SIGHUP and update prompt provider
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGHUP)
	go func() {
		for range sigc {
			if _, err := aliases.Reload(); err != nil {
				log.Warnf("alias reload on SIGHUP failed: %v", err)
			} else {
				log.Debugf("aliases reloaded (SIGHUP)")
			}
			if c, err := config.Load(); err == nil {
				if v, ok := c["PROMPT"]; ok {
					pp.Static = v
				}
				if v, ok := c["PROMPT_CMD"]; ok {
					pp.Cmd = v
				}
				if v, ok := c["PROMPT_TTL"]; ok {
					if d, err := time.ParseDuration(v); err == nil {
						pp.TTL = d
					}
				}
				if v, ok := c["PROMPT_TIMEOUT_MS"]; ok {
					if ms, err := strconv.ParseInt(v, 10, 64); err == nil {
						pp.Timeout = time.Duration(ms) * time.Millisecond
					}
				}
			}
			// Also reload KUSH_ environment-controlled prompt immediately
			if v := os.Getenv("KUSH_PROMPT"); v != "" {
				// clear provider cache so next Get() re-evaluates
				pp.Invalidate()
			}
		}
	}()

	for {
		line, err := le.Prompt(pp.Get())
		if err != nil {
			if err == ed.ErrEOF {
				fmt.Println()
				return nil
			}
			return err
		}

		if len(line) == 0 {
			continue
		}

		// Expand aliases before dispatch.
		if al == nil {
			al, _ = aliases.Load()
		}
		if al != nil {
			orig := line
			first := al.Expand(line)
			if first != line {
				origTok := parseArgs(line)
				newTok := parseArgs(first)
				if len(origTok) > 0 && len(newTok) > 0 && origTok[0] != newTok[0] {
					line = al.Expand(first)
				} else {
					line = first
				}
			}
			if log.Level() >= 2 && orig != line {
				log.Debugf("alias: %q -> %q", orig, line)
			}
		}

		// Check if the line has pipeline/redirect operators and involves builtins.
		if hasPipelineOps(line) {
			if handled := executePipeline(parsePipeline(line), bt); handled {
				continue
			}
		}

		// Simple builtin (no pipes or redirects).
		if bt.Handle(line) {
			continue
		}

		// If the line refers to a .t or .tengo file, run it with the scripting engine
		// so scripts execute like normal programs. Require the file to be executable
		// (+x). Check the blessed scripts dir first, then PATH.
		toks := parseArgs(line)
		if len(toks) > 0 {
			first := toks[0]
			ext := filepath.Ext(first)
			if ext == ".t" || ext == ".tengo" {
				var path string
				// If an explicit path was provided (contains / or starts with .), use it.
				if strings.Contains(first, "/") || strings.HasPrefix(first, ".") {
					path = first
				} else {
					// Check blessed scripts directory first.
					blessed := eng.BlessedDir()
					if blessed != "" {
						cand := filepath.Join(blessed, first)
						if fi, err := os.Stat(cand); err == nil && !fi.IsDir() {
							path = cand
						}
					}
					// If not found in blessed dir, fall back to PATH lookup.
					if path == "" {
						if p, err := exec.LookPath(first); err == nil {
							path = p
						}
					}
				}

				if path != "" {
					if fi, err := os.Stat(path); err == nil && !fi.IsDir() {
						// Require executable bit
						if fi.Mode()&0111 == 0 {
							fmt.Fprintf(os.Stderr, "error: script not executable: %s\n", path)
						} else {
							args := []string{}
							if len(toks) > 1 {
								args = toks[1:]
							}
							if err := eng.RunFile(path, args); err != nil {
								fmt.Fprintln(os.Stderr, "error:", err)
							}
							continue
						}
					}
				}
			}
		}

		// External command — close the editor so the child gets a
		// normal terminal, then recreate it for the next prompt.
		le.Close()
		if err := runner.RunShell(line); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
		}
		le, err = ed.New()
		if err != nil {
			return err
		}
	}
}
