package shell

import (
	"fmt"

	"os"
	"os/signal"
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
)

// Run starts the REPL loop. It returns when the user exits or on error.
func Run() error {
	le, err := ed.New()
	if err != nil {
		return err
	}
	defer le.Close()

	bt := builtins.New()
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

		if bt.Handle(line) {
			continue
		}

		// Expand aliases before execution. A two-pass scheme allows
		// chained aliases (e.g. la → ls -la, ls → ls --color=yes) while
		// avoiding infinite loops or duplicate flags.
		if al == nil {
			al, _ = aliases.Load()
		}
		if al != nil {
			orig := line
			first := al.Expand(line)
			if first != line {
				origTok := strings.Fields(line)
				newTok := strings.Fields(first)
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

		// Close the editor so the child gets a normal terminal, then
		// recreate it for the next prompt.
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
