package shell

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/grimdork/kush/internal/aliases"
	"github.com/grimdork/kush/internal/builtins"
	"github.com/grimdork/kush/internal/config"
	"github.com/grimdork/kush/internal/ed"
	"github.com/grimdork/kush/internal/log"
	"github.com/grimdork/kush/internal/runner"
)

// Run starts the simple REPL loop. It returns when the user exits or on error.
func Run() error {
	le, err := ed.New()
	if err != nil {
		return err
	}
	defer le.Close()

	// handle SIGHUP to reload aliases into the package cache
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
	// load aliases (optional files)
	al, _ := aliases.Load()
	// load config (optional)
	// load config (optional)
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

		// Ensure aliases are expanded right before execution to avoid stale state.
		if al == nil {
			al, _ = aliases.Load()
		}
		if al != nil {
			orig := line
			// allow a single extra pass so aliases like "la -> ls -la" can pick up
			// an alias for "ls" (common in bash where aliases apply after
			// expansion). Limit to two passes to avoid infinite loops from
			// recursive aliases.
			line = al.Expand(line)
			line = al.Expand(line)
			if log.Level() >= 2 && orig != line {
				log.Debugf("alias expanded: %q -> %q", orig, line)
			}
		}
		// Close the editor completely so the child runs on a normal terminal.
		le.Close()
		if err := runner.RunShell(line); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
		}
		// Recreate the editor for the next prompt.
		le, err = ed.New()
		if err != nil {
			return err
		}
	}
}
