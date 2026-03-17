package main

import (
	"fmt"
	"log"
	"os"

	"github.com/grimdork/kush/internal/builtins"
	"github.com/grimdork/kush/internal/ed"
	"github.com/grimdork/kush/internal/runner"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	le, err := lineeditor.New()
	if err != nil {
		return err
	}
	defer le.Close()

	bt := builtins.New()

	for {
		line, err := le.Prompt("$ ")
		if err != nil {
			if err == lineeditor.ErrEOF {
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

		// Close the editor completely so the child runs on a normal terminal.
		le.Close()
		if err := runner.RunShell(line); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
		}
		// Recreate the editor for the next prompt.
		le, err = lineeditor.New()
		if err != nil {
			return err
		}
	}
}
