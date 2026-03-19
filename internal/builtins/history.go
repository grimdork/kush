package builtins

import (
	"fmt"
	"os"
	"path/filepath"
)

func init() { Register("history", historyHandler) }

func historyHandler(b *Builtins, line string) bool {
	home := os.Getenv("HOME")
	f := filepath.Join(home, ".kush_history")
	data, _ := os.ReadFile(f)
	fmt.Print(string(data))
	return true
}
