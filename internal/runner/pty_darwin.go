//go:build darwin
// +build darwin

package runner

import "fmt"

// Darwin/macOS: not implemented in this minimal initial pass. The runner will
// fall back to non-PTY execution on macOS until a more complete darwin
// implementation is provided.
func openpty() (masterFD, slaveFD int, err error) {
	return 0, 0, fmt.Errorf("openpty: not implemented on darwin yet")
}
