package base

import (
	"math/big"
	"strings"
)

// BytesToBaseN encodes arbitrary bytes into a base-N string using the
// first "size" characters of the alphabet. If size is out of range it will
// be clamped to [2, len(alphabet)].
func BytesToBaseN(b []byte, size int) string {
	if size < 2 {
		size = 2
	}
	if size > len(alphabet) {
		size = len(alphabet)
	}
	if len(b) == 0 {
		return string(alphabet[0])
	}
	n := new(big.Int).SetBytes(b)
	zero := big.NewInt(0)
	base := big.NewInt(int64(size))
	if n.Cmp(zero) == 0 {
		return string(alphabet[0])
	}
	var out []byte
	mod := new(big.Int)
	for n.Cmp(zero) > 0 {
		n.DivMod(n, base, mod)
		rem := int(mod.Int64())
		out = append([]byte{alphabet[rem]}, out...)
	}
	return string(out)
}

// BaseNToBytes decodes a base-N string (using the same alphabet) back into
// the original bytes. Returns nil on invalid input or failure.
func BaseNToBytes(s string, size int) []byte {
	if size < 2 {
		size = 2
	}
	if size > len(alphabet) {
		size = len(alphabet)
	}
	base := big.NewInt(int64(size))
	n := big.NewInt(0)
	for i := 0; i < len(s); i++ {
		ch := s[i]
		idx := strings.IndexByte(alphabet, ch)
		if idx < 0 || idx >= size {
			return nil
		}
		n.Mul(n, base)
		n.Add(n, big.NewInt(int64(idx)))
	}
	if n.Sign() == 0 {
		return []byte{}
	}
	return n.Bytes()
}
