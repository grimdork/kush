package builtins

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Builtins struct{}

func New() *Builtins { return &Builtins{} }

// Handle returns true if the line was handled by a builtin.
func (b *Builtins) Handle(line string) bool {
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return false
	}
	switch parts[0] {
	case "cd":
		dir := "~"
		if len(parts) > 1 {
			dir = parts[1]
		}
		if dir == "~" {
			dir = os.Getenv("HOME")
		}
		if err := os.Chdir(dir); err != nil {
			fmt.Fprintln(os.Stderr, "cd:", err)
		}
		return true
	case "history":
		home := os.Getenv("HOME")
		f := filepath.Join(home, ".kush_history")
		data, _ := os.ReadFile(f)
		fmt.Print(string(data))
		return true
	case "checksum":
		if len(parts) < 3 {
			fmt.Fprintln(os.Stderr, "usage: checksum [md5|sha1|sha256] file")
			return true
		}
		algo := parts[1]
		file := parts[2]
		switch algo {
		case "md5", "sha1", "sha256":
			// placeholder — implement later
			fmt.Printf("checksum %s %s\n", algo, file)
		default:
			fmt.Fprintln(os.Stderr, "unknown algorithm")
		}
		return true
	}
	return false
}
