//go:build darwin
// +build darwin

package runner

/*
#include <stdlib.h>
#include <util.h>

static int openpty_wrapper(int *amaster, int *aslave) {
    return openpty(amaster, aslave, NULL, NULL, NULL);
}
*/
import "C"
import (
	"fmt"
)

// openpty on darwin: call the system openpty via cgo wrapper.
func openpty() (masterFD, slaveFD int, err error) {
	var m C.int
	var s C.int
	ret := C.openpty_wrapper(&m, &s)
	if ret != 0 {
		return 0, 0, fmt.Errorf("openpty failed: %d", int(ret))
	}
	return int(m), int(s), nil
}
