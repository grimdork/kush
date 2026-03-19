//go:build darwin || freebsd || openbsd || netbsd || linux
// +build darwin freebsd openbsd netbsd linux

package runner

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/grimdork/kush/internal/log"

	"golang.org/x/sys/unix"
)

// propagateWinSize copies the real terminal's window size to the PTY master
// and starts a goroutine that listens for SIGWINCH to keep it in sync.
// Returns a cleanup function that stops the listener.
func propagateWinSize(masterFd int) func() {
	stdinFd := int(os.Stdin.Fd())

	// Set the initial size.
	if ws, err := unix.IoctlGetWinsize(stdinFd, unix.TIOCGWINSZ); err == nil {
		if err := unix.IoctlSetWinsize(masterFd, unix.TIOCSWINSZ, ws); err != nil {
			log.Debugf("winsize: initial set failed: %v", err)
		}
	} else {
		log.Debugf("winsize: get failed: %v", err)
	}

	// Listen for SIGWINCH and propagate.
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGWINCH)
	done := make(chan struct{})

	go func() {
		for {
			select {
			case <-sigc:
				if ws, err := unix.IoctlGetWinsize(stdinFd, unix.TIOCGWINSZ); err == nil {
					_ = unix.IoctlSetWinsize(masterFd, unix.TIOCSWINSZ, ws)
				}
			case <-done:
				return
			}
		}
	}()

	return func() {
		signal.Stop(sigc)
		close(done)
	}
}
