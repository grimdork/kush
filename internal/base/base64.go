package base

import (
	"encoding/base64"
	"strings"
)

// EncodeBase64 returns a base64 encoding of s as a byte slice.
func EncodeBase64(s string) []byte {
	b := []byte(s)
	enc := base64.StdEncoding.EncodeToString(b)
	return []byte(enc)
}

// EncodeBase64URL returns a URL-safe base64 encoding of s as a byte slice.
func EncodeBase64URL(s string) []byte {
	b := []byte(s)
	enc := base64.URLEncoding.EncodeToString(b)
	return []byte(enc)
}

// DecodeBase64 decodes a base64-encoded byte slice. Returns nil on error.
func DecodeBase64(b []byte) []byte {
	if len(b) == 0 {
		return nil
	}
	s := string(b)
	out, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil
	}
	return out
}

// DecodeBase64URL decodes a URL-safe base64-encoded byte slice. Returns nil on error.
func DecodeBase64URL(b []byte) []byte {
	if len(b) == 0 {
		return nil
	}
	s := string(b)
	out, err := base64.URLEncoding.DecodeString(s)
	if err != nil {
		// try raw/rawstd padding-insensitive decode
		// attempt to replace URL-safe chars and pad
		s2 := strings.ReplaceAll(s, "-", "+")
		s2 = strings.ReplaceAll(s2, "_", "/")
		if m := len(s2) % 4; m != 0 {
			s2 += strings.Repeat("=", 4-m)
		}
		out2, err2 := base64.StdEncoding.DecodeString(s2)
		if err2 != nil {
			return nil
		}
		return out2
	}
	return out
}
