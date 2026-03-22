//go:build freebsd || openbsd || netbsd || darwin
// +build freebsd openbsd netbsd darwin

package termio

import (
	"golang.org/x/sys/unix"
)

// SaveAndSetPassthrough for BSD/Darwin uses TIOCGETA/TIOCSETA variant.
func SaveAndSetPassthrough(fd int) (unix.Termios, error) {
	p, err := unix.IoctlGetTermios(fd, unix.TIOCGETA)
	if err != nil {
		return unix.Termios{}, err
	}
	old := *p

	raw := old
	raw.Lflag &^= unix.ICANON | unix.ECHO | unix.ECHONL | unix.ISIG | unix.IEXTEN
	raw.Iflag &^= unix.ICRNL | unix.INLCR | unix.IGNCR | unix.IXON
	// Leave Oflag untouched — OPOST must stay enabled.
	raw.Cc[unix.VMIN] = 1
	raw.Cc[unix.VTIME] = 0

	if err := unix.IoctlSetTermios(fd, unix.TIOCSETA, &raw); err != nil {
		return old, err
	}
	return old, nil
}

// RestoreTermios restores the termios state for fd.
func RestoreTermios(fd int, t unix.Termios) error {
	return unix.IoctlSetTermios(fd, unix.TIOCSETA, &t)
}
