package builtins

import (
	"os"
	"strings"

	"github.com/grimdork/kush/internal/log"
)

func init() {
	Register("cd", cdHandler)
}

func cdHandler(b *Builtins, line string) bool {
	parts := strings.Fields(line)
	dir := "~"
	if len(parts) > 1 {
		dir = parts[1]
	}
	if dir == "~" {
		dir = os.Getenv("HOME")
	}
	if err := os.Chdir(dir); err != nil {
		log.Errorf("cd: %v", err)
	}
	return true
}
