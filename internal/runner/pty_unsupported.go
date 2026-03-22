//go:build !(linux || darwin || freebsd || openbsd || netbsd)
// +build !linux,!darwin,!freebsd,!openbsd,!netbsd

package runner

import "fmt"

func openpty() (masterFD, slaveFD int, err error) {
	return 0, 0, fmt.Errorf("openpty: unsupported platform")
}
