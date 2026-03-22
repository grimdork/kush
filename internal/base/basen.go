// Package base contains a small subset of helpers copied from github.com/grimdork/base
// for local use inside kush. Only the functions required by the scripting engine
// are implemented: base64 encode/decode and numeric base encoders/decoders.
package base

import (
	"strings"
)

// Numeric base encoder/decoder. We provide an alphabet of 64 characters and
// allow encoding/decoding in any base between 2 and 64 via the size parameter.

var alphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ-_"

// NumEncode encodes the given unsigned integer into a string using the
// first "size" characters of the alphabet. If size is out of range, it will
// be clamped to [2,64].
func NumEncode(n uint64, size int) string {
	if size < 2 {
		size = 2
	}
	if size > len(alphabet) {
		size = len(alphabet)
	}
	if n == 0 {
		return string(alphabet[0])
	}
	var b []byte
	for n > 0 {
		rem := int(n % uint64(size))
		b = append([]byte{alphabet[rem]}, b...)
		n = n / uint64(size)
	}
	return string(b)
}

// NumDecode decodes a string encoded with NumEncode using the provided size
// and returns the numeric value. If decoding fails, 0 is returned.
func NumDecode(s string, size int) uint64 {
	if size < 2 {
		size = 2
	}
	if size > len(alphabet) {
		size = len(alphabet)
	}
	var val uint64
	for i := 0; i < len(s); i++ {
		ch := s[i]
		idx := strings.IndexByte(alphabet, ch)
		if idx < 0 || idx >= size {
			return 0
		}
		val = val*uint64(size) + uint64(idx)
	}
	return val
}
