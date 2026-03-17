package shell

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/grimdork/kush/internal/aliases"
	"github.com/grimdork/kush/internal/builtins"
	"github.com/grimdork/kush/internal/config"
	"github.com/grimdork/kush/internal/ed"
	"github.com/grimdork/kush/internal/log"
	"github.com/grimdork/kush/internal/runner"
)

// Run starts the REPL loop. It returns when the user exits or on error.
func Run() error {
	le, err := ed.New()
	if err != nil {
		return err
	}
	defer le.Close()

	// Reload aliases on SIGHUP so external tools can update the file and
	// signal running shells to pick up changes.
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGHUP)
	go func() {
		for range sigc {
			if _, err := aliases.Reload(); err != nil {
				log.Warnf("alias reload on SIGHUP failed: %v", err)
			} else {
				log.Debugf("aliases reloaded (SIGHUP)")
			}
		}
	}()

	bt := builtins.New()
	al, _ := aliases.Load()
	_, _ = config.Load()

	for {
		line, err := le.Prompt("$ ")
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
