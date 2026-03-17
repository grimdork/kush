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

// RunShell runs the given command line inside a pseudoterminal so interactive
// programs behave correctly. It falls back to a plain exec.Command if Openpty
// is unavailable.
func RunShell(line string) error {
	// Save and restore terminal state for stdin to ensure we return the
	// terminal to its previous mode after the child exits (fixes "shell
	// broken after command returns" on macOS where PTY sessions can leave
	// the tty in raw mode).
	stdinFd := int(os.Stdin.Fd())
	var old unix.Termios
	if p, err := unix.IoctlGetTermios(stdinFd, unix.TIOCGETA); err == nil && p != nil {
		old = *p
		defer unix.IoctlSetTermios(stdinFd, unix.TIOCSETA, &old)
	}

	// attempt to open a pty
	var masterFd, slaveFd int
	var err error
	if masterFd, slaveFd, err = openpty(); err != nil {
		// fallback to simple exec
		log.Debugf("openpty failed, falling back to plain exec: %v", err)
		return runShellFallback(line)
	}
	defer unix.Close(masterFd)
	defer unix.Close(slaveFd)

	master := os.NewFile(uintptr(masterFd), "pty-master")
	defer master.Close()

	// Start the user's shell with the slave side as its stdio.
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "sh"
	}
	cmd := exec.Command(shell, "-lc", line)
	log.Debugf("starting shell via pty: %s -lc %q (masterfd=%d)", shell, line, masterFd)
	// make the child a process group leader so we can signal the whole group
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	slave := os.NewFile(uintptr(slaveFd), "pty-slave")
	defer slave.Close()
	cmd.Stdin = slave
	cmd.Stdout = slave
	cmd.Stderr = slave

	if err := cmd.Start(); err != nil {
		slave.Close()
		master.Close()
		return err
	}

	// propagate window size once (best-effort)
	propagateWinSize(masterFd)

	// Forward signals (SIGINT/SIGTERM/...) to child process group.
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM, syscall.SIGTSTP)
	go func() {
		for s := range sigc {
			// send signal to child process group
			pgid, err := syscall.Getpgid(cmd.Process.Pid)
			if err == nil {
				syscall.Kill(-pgid, s.(syscall.Signal))
			}
		}
	}()

	// copy goroutines use a waitgroup so we can ensure they exit cleanly
	// before returning. This avoids races where editor recreation and
	// leftover goroutines both touch the terminal causing a frozen shell.
	var wg sync.WaitGroup
	done := make(chan struct{})

	wg.Add(1)
	var lastByte byte = 0
	go func() {
		defer wg.Done()
		buf := make([]byte, 4096)
		for {
			n, err := master.Read(buf)
			if n > 0 {
				os.Stdout.Write(buf[:n])
				lastByte = buf[n-1]
			}
			if err != nil {
				return
			}
			select {
			case <-done:
				return
			default:
			}
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		buf := make([]byte, 4096)
		for {
			n, err := os.Stdin.Read(buf)
			if n > 0 {
				if _, werr := master.Write(buf[:n]); werr != nil {
					return
				}
			}
			if err != nil {
				return
			}
			select {
			case <-done:
				return
			default:
			}
		}
	}()

	// Wait for the child to exit.
	err = cmd.Wait()
	// small grace period to let IO drain
	time.Sleep(20 * time.Millisecond)
	// stop background copy goroutines and wait for them
	close(done)
	wg.Wait()
	// if the child didn't end with a newline, emit one so the prompt starts
	// on a fresh line — but only when necessary. We check lastByte captured
	// from the master->stdout copy goroutine above. Log values at debug level
	// to help diagnose lingering "press Enter" cases.
	wrote := false
	if lastByte != '\n' && lastByte != 0 {
		if _, werr := os.Stdout.WriteString("\r\n"); werr != nil {
			log.Debugf("failed to write trailing newline: %v", werr)
		} else {
			wrote = true
		}
	}
	log.Debugf("pty-runner: child exit, lastByte=%q wroteNewline=%v", lastByte, wrote)
	signal.Stop(sigc)
	close(sigc)
	return err
}

func runShellFallback(line string) error {
	// Save/restore stdin termios here too.
	stdinFd := int(os.Stdin.Fd())
	var old unix.Termios
	if p, err := unix.IoctlGetTermios(stdinFd, unix.TIOCGETA); err == nil && p != nil {
		old = *p
		defer unix.IoctlSetTermios(stdinFd, unix.TIOCSETA, &old)
	}

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "sh"
	}
	cmd := exec.Command(shell, "-lc", line)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = os.Environ()
	return cmd.Run()
}

func propagateWinSize(masterFd int) {
	// No-op for now; proper winsize propagation will be added with a full
	// openpty implementation.
	_ = masterFd
	return
}
