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

// RunShell runs a command line inside a pseudoterminal so interactive programs
// behave correctly. Uses the linux pty implementation (openpty in pty_linux.go)
// and the passthrough helpers in pty_runner.go.
func RunShell(line string) error {
	masterFd, slaveFd, err := openpty()
	if err != nil {
		log.Debugf("openpty unavailable, using plain exec: %v", err)
		return runPlain(line)
	}

	master := os.NewFile(uintptr(masterFd), "pty-master")
	slave := os.NewFile(uintptr(slaveFd), "pty-slave")

	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "sh"
	}

	cmd := exec.Command(shell, "-c", line)
	log.Debugf("pty exec: %s -c %q", shell, line)
	cmd.Stdin = slave
	cmd.Stdout = slave
	cmd.Stderr = slave
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
		Ctty:   int(slave.Fd()),
	}

	// Switch stdin to passthrough mode before starting the child.
	stdinFd := int(os.Stdin.Fd())
	oldTermios, termErr := saveAndSetPassthrough(stdinFd)
	if termErr != nil {
		log.Debugf("passthrough mode failed: %v", termErr)
	}

	if err := cmd.Start(); err != nil {
		slave.Close()
		master.Close()
		if termErr == nil {
			restoreTermios(stdinFd, oldTermios)
		}
		return err
	}

	// The child inherited the slave fd; close our copy so the master gets
	// EOF/EIO promptly when the child exits.
	slave.Close()

	stopWinSize := propagateWinSize(masterFd)

	// Forward signals to the child's process group.
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM, syscall.SIGTSTP)
	go func() {
		for s := range sigc {
			if pgid, err := syscall.Getpgid(cmd.Process.Pid); err == nil {
				_ = syscall.Kill(-pgid, s.(syscall.Signal))
			}
		}
	}()

	var wg sync.WaitGroup
	var lastByte byte

	// Copy master → stdout. We wait on this goroutine to ensure all child
	// output is flushed before returning.
	wg.Add(1)
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
		}
	}()

	// Copy stdin → master. Uses a dup'd fd with poll(2) so we can stop
	// cleanly after the child exits without affecting the real stdin.
	//
	// We must not set O_NONBLOCK on the dup because dup'd fds share the
	// underlying file description — non-blocking mode would leak to the
	// original fd and break the editor's reads after we return.
	done := make(chan struct{})
	stdinDupFd, dupErr := syscall.Dup(stdinFd)
	if dupErr != nil {
		log.Debugf("dup(stdin) failed: %v; stdin forwarding disabled", dupErr)
	} else {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer syscall.Close(stdinDupFd)
			buf := make([]byte, 4096)
			pollFds := []unix.PollFd{{Fd: int32(stdinDupFd), Events: unix.POLLIN}}
			for {
				select {
				case <-done:
					return
				default:
				}
				// 20 ms timeout keeps the loop responsive without busy-spinning.
				n, err := unix.Poll(pollFds, 20)
				if err == syscall.EINTR || n == 0 {
					continue
				}
				if err != nil {
					return
				}
				nr, rerr := syscall.Read(stdinDupFd, buf)
				if nr > 0 {
					if _, werr := master.Write(buf[:nr]); werr != nil {
						return
					}
				}
				if rerr != nil || nr == 0 {
					return
				}
			}
		}()
	}

	// Wait for the child to exit, then tear down the copy goroutines.
	err = cmd.Wait()
	log.Debugf("child exited: %v", err)

	time.Sleep(20 * time.Millisecond) // let remaining output drain through the PTY
	close(done)                       // stop stdin→master polling
	master.Close()                    // EOF for master→stdout
	wg.Wait()

	signal.Stop(sigc)
	close(sigc)
	stopWinSize()

	if termErr == nil {
		restoreTermios(stdinFd, oldTermios)
	}

	// Ensure the next prompt starts on a fresh line.
	if lastByte != '\n' && lastByte != 0 {
		os.Stdout.WriteString("\r\n")
	}

	return err
}
