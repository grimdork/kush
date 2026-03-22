package base_test

import (
	"crypto/rand"
	"reflect"
	"testing"

	"github.com/grimdork/kush/internal/base"
)

func TestBytesToBaseN_Roundtrip(t *testing.T) {
	sizes := []int{2, 8, 16, 63, 64}

	cases := [][]byte{
		{},
		{0},
		{1},
		{0, 1, 2, 3, 4},
	}

	// random cases
	for i := 0; i < 5; i++ {
		b := make([]byte, 1+i)
		if _, err := rand.Read(b); err == nil {
			cases = append(cases, b)
		}
	}

	for _, size := range sizes {
		for _, c := range cases {
			enc := base.BytesToBaseN(c, size)
			dec := base.BaseNToBytes(enc, size)
			if dec == nil {
				dec = []byte{}
			}

			// For inputs without leading zero bytes we expect exact roundtrip.
			if len(c) > 0 && c[0] != 0 {
				if !reflect.DeepEqual(c, dec) {
					t.Fatalf("roundtrip failed for size=%d: orig=%v dec=%v enc=%s", size, c, dec, enc)
				}
			} else {
				// For inputs that may have leading zeros (or empty) the numerical
				// representation loses leading zero bytes. In that case accept that
				// re-encoding the decoded bytes yields the same string encoding.
				enc2 := base.BytesToBaseN(dec, size)
				if enc != enc2 {
					t.Fatalf("idempotence failed for size=%d: orig=%v dec=%v enc=%s enc2=%s", size, c, dec, enc, enc2)
				}
			}
		}
	}
}

func TestBaseNToBytes_InvalidInput(t *testing.T) {
	// use a character that is invalid for base 2
	s := "z"
	res := base.BaseNToBytes(s, 2) // size 2 allows only two characters
	if res != nil {
		t.Fatalf("expected nil for invalid input, got %v", res)
	}
}

func TestSizeClamping(t *testing.T) {
	b := []byte("hello world")
	// request an oversized base; function should clamp to max alphabet size
	enc := base.BytesToBaseN(b, 1000)
	dec := base.BaseNToBytes(enc, 1000)
	if dec == nil {
		t.Fatalf("decode returned nil for clamped base")
	}
	if !reflect.DeepEqual(b, dec) {
		t.Fatalf("clamped roundtrip failed: orig=%v dec=%v", b, dec)
	}
}
