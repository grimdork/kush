package lineeditor

import (
	"golang.org/x/sys/unix"
	"sync"
)

var passthroughStore sync.Map

type passthroughStoreValue struct {
	old unix.Termios
	// ps kept for future; use interface{} to avoid referencing passthroughState
	ps interface{}
}
