//go:build freebsd || openbsd || netbsd
// +build freebsd openbsd netbsd

package runner

import (
	"fmt"

	"github.com/creack/pty"
)

// BSD implementation using creack/pty for portability.
func openpty() (masterFD, slaveFD int, err error) {
	m, s, err := pty.Open()
	if err != nil {
		return 0, 0, fmt.Errorf("openpty failed: %w", err)
	}
	return int(m.Fd()), int(s.Fd()), nil
}
