package builtins

import "strings"

func init() { Register("help", helpHandler) }

func helpHandler(b *Builtins, line string) bool {
	parts := strings.Fields(line)
	printHelp(parts)
	return true
}
