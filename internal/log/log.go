package log

import (
	"github.com/grimdork/climate/env"
	"github.com/grimdork/climate/loglines"
)

// Level returns the integer debug level (KUSH_DEBUG).
func Level() int { return int(env.GetInt("KUSH_DEBUG", 0)) }

func colorEnabled() bool {
	c := env.Get("KUSH_COLOR", "auto")
	return c != "never"
}

// Debugf prints when level >= 1.
func Debugf(format string, a ...interface{}) {
	if Level() < 1 {
		return
	}
	loglines.CMsg(format, a...)
}

func Infof(format string, a ...interface{}) {
	loglines.CMsg(format, a...)
}

func Warnf(format string, a ...interface{}) {
	loglines.CMsg(format, a...)
}

func Errorf(format string, a ...interface{}) {
	loglines.CMsg(format, a...)
}
