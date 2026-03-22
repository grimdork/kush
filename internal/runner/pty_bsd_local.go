//go:build freebsd || openbsd || netbsd
// +build freebsd openbsd netbsd

package runner

import (
	"errors"
	"os"
	"syscall"
	"unsafe"
)

// posixOpenpt wraps the posix_openpt syscall.
func posixOpenpt(oflag int) (fd int, err error) {
	r0, _, e1 := syscall.Syscall(syscall.SYS_POSIX_OPENPT, uintptr(oflag), 0, 0)
	fd = int(r0)
	if e1 != 0 {
		err = e1
	}
	return fd, err
}

// openpty implementation for BSD-like systems without external deps.
// It mirrors the behaviour of libutil openpty: return master and slave fds.
func openpty() (masterFD, slaveFD int, err error) {
	fd, err := posixOpenpt(syscall.O_RDWR | syscall.O_CLOEXEC)
	if err != nil {
		return 0, 0, err
	}
	p := os.NewFile(uintptr(fd), "/dev/ptmx")
	// Ensure we close on error
	defer func() {
		if err != nil {
			_ = p.Close()
		}
	}()

	sname, err := ptsname(p)
	if err != nil {
		return 0, 0, err
	}

	t, err := os.OpenFile("/dev/"+sname, os.O_RDWR, 0)
	if err != nil {
		return 0, 0, err
	}
	return int(p.Fd()), int(t.Fd()), nil
}

func isptmaster(f *os.File) (bool, error) {
	err := ioctl(f, syscall.TIOCPTMASTER, 0)
	return err == nil, err
}

// helper constants/types for BSD ptsname ioctl
var (
	emptyFiodgnameArg fiodgnameArg
	ioctlFIODGNAME    = _IOW('f', 120, unsafe.Sizeof(emptyFiodgnameArg))
)

func ptsname(f *os.File) (string, error) {
	master, err := isptmaster(f)
	if err != nil {
		return "", err
	}
	if !master {
		return "", syscall.EINVAL
	}

	const n = _C_SPECNAMELEN + 1
	buf := make([]byte, n)
	arg := fiodgnameArg{Len: int32(n), Buf: (*byte)(unsafe.Pointer(&buf[0]))}
	if err := ioctl(f, ioctlFIODGNAME, uintptr(unsafe.Pointer(&arg))); err != nil {
		return "", err
	}

	for i, c := range buf {
		if c == 0 {
			return string(buf[:i]), nil
		}
	}
	return "", errors.New("FIODGNAME string not NUL-terminated")
}

// ioctl helper: use syscall.Syscall to invoke ioctl
func ioctl(f *os.File, cmd, ptr uintptr) error {
	_, _, e := syscall.Syscall(syscall.SYS_IOCTL, f.Fd(), cmd, ptr)
	if e != 0 {
		return e
	}
	return nil
}
