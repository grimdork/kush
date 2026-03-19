package builtins

import (
	"fmt"
	"strings"
	"os/exec"

	"github.com/grimdork/kush/internal/log"
)

func init() { Register("which", whichHandler) }

func whichHandler(b *Builtins, line string) bool {
	parts := strings.Fields(line)
	if len(parts) == 1 {
		log.Errorf("usage: which prog [prog...]")
		return true
	}
	for _, a := range parts[1:] {
		if p, _ := exec.LookPath(a); p != "" {
			fmt.Println(p)
		} else {
			log.Errorf("%s: not found in PATH", a)
		}
	}
	return true
}
