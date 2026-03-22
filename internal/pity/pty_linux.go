//go:build linux
// +build linux

package pity

import (
	"fmt"

	"golang.org/x/sys/unix"
)

// OpenPTY opens a new pseudoterminal and returns master and slave file
// descriptors (masterFd, slaveFd). Mirrors previous internal/runner/openpty
// linux implementation.
func OpenPTY() (masterFD, slaveFD int, err error) {
	fd, err := unix.Open("/dev/ptmx", unix.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		return 0, 0, fmt.Errorf("open /dev/ptmx: %w", err)
	}

	// unlockpt: clear the lock
	if err := unix.IoctlSetInt(fd, unix.TIOCSPTLCK, 0); err != nil {
		unix.Close(fd)
		return 0, 0, fmt.Errorf("TIOCSPTLCK: %w", err)
	}

	// get pty number
	pty, err := unix.IoctlGetInt(fd, unix.TIOCGPTN)
	if err != nil {
		unix.Close(fd)
		return 0, 0, fmt.Errorf("TIOCGPTN: %w", err)
	}

	name := fmt.Sprintf("/dev/pts/%d", pty)
	slave, err := unix.Open(name, unix.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		unix.Close(fd)
		return 0, 0, fmt.Errorf("open slave pt: %w", err)
	}
	return fd, slave, nil
}
