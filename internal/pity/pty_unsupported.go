//go:build !linux && !freebsd && !openbsd && !netbsd && !darwin
// +build !linux,!freebsd,!openbsd,!netbsd,!darwin

package pity

import "errors"

// OpenPTY returns an error on unsupported platforms/builds.
func OpenPTY() (masterFD, slaveFD int, err error) {
	return 0, 0, errors.New("openpty not supported on this platform")
}
