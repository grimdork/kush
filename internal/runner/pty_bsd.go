//go:build darwin || freebsd || openbsd || netbsd
// +build darwin freebsd openbsd netbsd

package runner

import (
	"fmt"

	"golang.org/x/sys/unix"
)

// BSD/Darwin implementation using unix.Openpty when available.
func openpty() (masterFD, slaveFD int, err error) {
	m, s, err := unix.Openpty(nil, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("openpty failed: %w", err)
	}
	return m, s, nil
}
