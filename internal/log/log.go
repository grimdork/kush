// Package log provides levelled debug/info/warn/error logging to stderr,
// controlled by the KUSH_DEBUG environment variable (0 = quiet, 1+ = verbose).
package log

import (
	"github.com/grimdork/climate/env"
	"github.com/grimdork/climate/loglines"
)

// Level returns the integer debug level from KUSH_DEBUG (default 0).
func Level() int { return int(env.GetInt("KUSH_DEBUG", 0)) }

// Debugf prints to stderr when KUSH_DEBUG >= 1.
func Debugf(format string, a ...interface{}) {
	if Level() < 1 {
		return
	}
	loglines.Err(format, a...)
}

// Infof prints to stderr unconditionally.
func Infof(format string, a ...interface{}) {
	loglines.Err(format, a...)
}

// Warnf prints to stderr unconditionally.
func Warnf(format string, a ...interface{}) {
	loglines.Err(format, a...)
}

// Errorf prints to stderr unconditionally.
func Errorf(format string, a ...interface{}) {
	loglines.Err(format, a...)
}
