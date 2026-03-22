//go:build arm64 && freebsd
// +build arm64,freebsd

package runner

// Created by cgo -godefs - DO NOT EDIT
// cgo -godefs types_freebsd.go

const (
	_C_SPECNAMELEN = 0x3f
)

type fiodgnameArg struct {
	Len       int32
	Pad_cgo_0 [4]byte
	Buf       *byte
}
