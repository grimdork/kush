package builtins

import (
	"fmt"
	"strings"

	"github.com/grimdork/kush/internal/log"
)

func init() { Register("checksum", checksumHandler) }

func checksumHandler(b *Builtins, line string) bool {
	parts := strings.Fields(line)
	if len(parts) < 3 {
		log.Errorf("usage: checksum [md5|sha1|sha256] file")
		return true
	}
	algo := parts[1]
	file := parts[2]
	switch algo {
	case "md5", "sha1", "sha256":
		// placeholder — implement later
		fmt.Printf("checksum %s %s\n", algo, file)
	default:
		log.Errorf("unknown algorithm")
	}
	return true
}
