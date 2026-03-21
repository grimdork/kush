package builtins

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/grimdork/kush/internal/log"
)

func init() { Register("export", (*Builtins).handleExport) }

// handleExport processes the export builtin. It preserves trailing spaces
// and accepts single- or double-quoted values. Double-quoted values are
// processed with strconv.Unquote so Go-style escapes are interpreted.
func (b *Builtins) handleExport(line string) bool {
	// export KEY=VALUE or export KEY VALUE
	rest := strings.TrimSpace(strings.TrimPrefix(line, "export"))
	if rest == "" {
		// list environment
		for _, e := range os.Environ() {
			fmt.Println(e)
		}
		return true
	}
	var key, val string
	if strings.Contains(rest, "=") {
		parts2 := strings.SplitN(rest, "=", 2)
		key = strings.TrimSpace(parts2[0])
		val = parts2[1]
	} else {
		parts2 := strings.Fields(rest)
		if len(parts2) == 1 {
			// export KEY -> export existing value
			key = parts2[0]
			val = os.Getenv(key)
		} else if len(parts2) >= 2 {
			key = parts2[0]
			val = strings.Join(parts2[1:], " ")
		}
	}
	// Handle quoted values so trailing spaces can be preserved.
	if len(val) >= 2 {
		// Double-quoted: allow Go-style escapes inside
		if val[0] == '"' && val[len(val)-1] == '"' {
			unq, err := strconv.Unquote(val)
			if err == nil {
				val = unq
			} else {
				// Fallback: strip quotes but keep interior verbatim
				val = val[1 : len(val)-1]
			}
		} else if val[0] == '\'' && val[len(val)-1] == '\'' {
			// Single-quoted: treat contents literally (do not interpret escapes)
			val = val[1 : len(val)-1]
		}
	}
	if key != "" {
		os.Setenv(key, val)
		// If we have a prompt provider, invalidate its cache so the running REPL picks up changes immediately.
		if b.pp != nil {
			b.pp.Invalidate()
		}
	} else {
		log.Errorf("export: missing key")
	}
	return true
}
