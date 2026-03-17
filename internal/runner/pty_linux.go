//go:build linux
// +build linux

package runner

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"
)

// Linux implementation using posix_openpt / ioctl TIOCSPTLCK + TIOCGPTN.
func openpty() (masterFD, slaveFD int, err error) {
	fd, err := unix.PosixOpenpt(unix.O_RDWR | unix.O_NOCTTY)
	if err != nil {
		return 0, 0, fmt.Errorf("posix_openpt: %w", err)
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
