//go:build linux
// +build linux

package runner

import (
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/grimdork/kush/internal/log"

	"golang.org/x/sys/unix"
)

// saveAndSetPassthrough puts the terminal into passthrough mode for PTY
// forwarding: disables ICANON, ECHO, ISIG and IEXTEN so keypresses reach the
// child immediately, but preserves OPOST so any stderr output (e.g. debug
// logs) still gets proper CR+LF translation. Returns the previous termios for
// restoration after the child exits.
func saveAndSetPassthrough(fd int) (unix.Termios, error) {
	p, err := unix.IoctlGetTermios(fd, unix.TCGETS)
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

	if err := unix.IoctlSetTermios(fd, unix.TCSETS, &raw); err != nil {
		return old, err
	}
	return old, nil
}

func restoreTermios(fd int, t unix.Termios) error {
	return unix.IoctlSetTermios(fd, unix.TCSETS, &t)
}

func propagateWinSize(masterFd int) {
	// TODO: copy the real terminal's winsize to the PTY master and listen
	// for SIGWINCH to keep it in sync.
	_ = masterFd
}
