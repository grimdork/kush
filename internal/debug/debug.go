package debug

import (
	"os"
	"strconv"
)

var lvlSet bool
var lvl int

// Level returns KUSH_DEBUG as integer. Not thread-safe but good enough for startup config.
func Level() int {
	if lvlSet {
		return lvl
	}
	s := os.Getenv("KUSH_DEBUG")
	if s == "" {
		lvl = 0
		lvlSet = true
		return lvl
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		lvl = 0
	} else {
		lvl = v
	}
	lvlSet = true
	return lvl
}
