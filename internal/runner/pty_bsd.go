//go:build freebsd || openbsd || netbsd
// +build freebsd openbsd netbsd

package runner

import (
	"fmt"

	"golang.org/x/sys/unix"
)

// BSD implementation using unix.Openpty when available (not for darwin).
func openpty() (masterFD, slaveFD int, err error) {
	m, s, err := unix.Openpty(nil, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("openpty failed: %w", err)
	}
	return m, s, nil
}
