package runner

import (
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// RunShell runs the given command line inside a pseudoterminal so interactive
// programs behave correctly. It falls back to a plain exec.Command if Openpty
// is unavailable.
func RunShell(line string) error {
	// attempt to open a pty
	var masterFd, slaveFd int
	var err error
	if masterFd, slaveFd, err = openpty(); err != nil {
		// fallback to simple exec
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

	// Copy pty output to our stdout in a goroutine; stdin is attached by setting
	// the terminal to raw mode outside this function (the line editor will
	// usually manage that). We still copy bytes from the master to stdout so
	// interactive programs render.
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := master.Read(buf)
			if n > 0 {
				os.Stdout.Write(buf[:n])
			}
			if err != nil {
				return
			}
		}
	}()

	// Connect os.Stdin to master in a blocking copy until child exits or io error.
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := os.Stdin.Read(buf)
			if n > 0 {
				master.Write(buf[:n])
			}
			if err != nil {
				return
			}
		}
	}()

	// Wait for the child to exit.
	err = cmd.Wait()
	// small grace period to let IO drain
	time.Sleep(20 * time.Millisecond)
	signal.Stop(sigc)
	close(sigc)
	return err
}

func runShellFallback(line string) error {
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
