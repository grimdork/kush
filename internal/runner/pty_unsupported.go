//go:build !linux && !darwin
// +build !linux,!darwin

package runner

import "fmt"

func openpty() (masterFD, slaveFD int, err error) {
	return 0, 0, fmt.Errorf("openpty: unsupported platform")
}
