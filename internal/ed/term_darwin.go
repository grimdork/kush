//go:build darwin
// +build darwin

package lineeditor

import (
	"golang.org/x/sys/unix"
)

// SaveTermios saves the current termios for fd on darwin.
func SaveTermios(fd int) (unix.Termios, error) {
	// On darwin the wrapper returns a pointer; reuse it safely.
	t := unix.Termios{}
	p, err := unix.IoctlGetTermios(fd, unix.TIOCGETA)
	if err != nil {
		return t, err
	}
	if p == nil {
		return t, unix.EINVAL
	}
	return *p, nil
}

// RestoreTermios restores a previously saved termios for fd on darwin.
func RestoreTermios(fd int, t unix.Termios) error {
	return unix.IoctlSetTermios(fd, unix.TIOCSETA, &t)
}

// SetRaw sets terminal to a raw-ish mode and returns previous termios on darwin.
func SetRaw(fd int) (unix.Termios, error) {
	old, err := SaveTermios(fd)
	if err != nil {
		return old, err
	}
	raw := old
	raw.Iflag &^= unix.IGNBRK | unix.BRKINT | unix.PARMRK | unix.ISTRIP | unix.INLCR | unix.IGNCR | unix.ICRNL | unix.IXON
	raw.Oflag &^= unix.OPOST
	raw.Cflag &^= unix.CSIZE | unix.PARENB
	raw.Cflag |= unix.CS8
	raw.Lflag &^= unix.ECHO | unix.ECHONL | unix.ICANON | unix.ISIG | unix.IEXTEN
	raw.Cc[unix.VMIN] = 1
	raw.Cc[unix.VTIME] = 0
	if err := RestoreTermios(fd, raw); err != nil {
		return old, err
	}
	return old, nil
}
