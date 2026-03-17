//go:build linux
// +build linux

package lineeditor

import (
	"golang.org/x/sys/unix"
)

// SaveTermios saves the current termios for fd on linux.
func SaveTermios(fd int) (unix.Termios, error) {
	p, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		return unix.Termios{}, err
	}
	return *p, nil
}

// RestoreTermios restores a previously saved termios for fd on linux.
func RestoreTermios(fd int, t unix.Termios) error {
	return unix.IoctlSetTermios(fd, unix.TCSETS, &t)
}

// SetRaw sets terminal to raw-like mode and returns previous termios on linux.
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
