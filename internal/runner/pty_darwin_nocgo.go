//go:build darwin && !cgo
// +build darwin,!cgo

package runner

import "fmt"

// openpty stub for darwin when cgo is disabled. The cgo implementation uses
// the system openpty(3) wrapper and requires cgo. When cross-compiling with
// CGO_ENABLED=0 we provide this fallback so builds succeed and runShell will
// fall back to the plain exec path.
func openpty() (masterFD, slaveFD int, err error) {
	return 0, 0, fmt.Errorf("openpty: disabled on darwin when cgo is disabled")
}
