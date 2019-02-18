package dbxfs

import (
	"errors"
)

var (
	ErrNotFile            = errors.New("not a file")
	ErrNotDir             = errors.New("not a directory")
	ErrBadPath            = errors.New("bad path")
	ErrNotOpen            = errors.New("file not open")
	ErrBadOpenFileOptions = errors.New("bad open file options")
	ErrNotSupported       = errors.New("not supported")
	ErrStatUnavailable    = errors.New("stat unavailable")
	ErrBadReadWriteOffset = errors.New("bad read/write offset")
	ErrUnimplemented      = errors.New("unimplemented")
)
