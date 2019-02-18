package protosftp

import (
	"fmt"
)

const ProtocolVersion = 3

const (
	FXP_INIT           = 1
	FXP_VERSION        = 2
	FXP_OPEN           = 3
	FXP_CLOSE          = 4
	FXP_READ           = 5
	FXP_WRITE          = 6
	FXP_LSTAT          = 7
	FXP_FSTAT          = 8
	FXP_SETSTAT        = 9
	FXP_FSETSTAT       = 10
	FXP_OPENDIR        = 11
	FXP_READDIR        = 12
	FXP_REMOVE         = 13
	FXP_MKDIR          = 14
	FXP_RMDIR          = 15
	FXP_REALPATH       = 16
	FXP_STAT           = 17
	FXP_RENAME         = 18
	FXP_READLINK       = 19
	FXP_SYMLINK        = 20
	FXP_STATUS         = 101
	FXP_HANDLE         = 102
	FXP_DATA           = 103
	FXP_NAME           = 104
	FXP_ATTRS          = 105
	FXP_EXTENDED       = 200
	FXP_EXTENDED_REPLY = 201
)

const (
	FX_OK                = 0
	FX_EOF               = 1
	FX_NO_SUCH_FILE      = 2
	FX_PERMISSION_DENIED = 3
	FX_FAILURE           = 4
	FX_BAD_MESSAGE       = 5
	FX_NO_CONNECTION     = 6
	FX_CONNECTION_LOST   = 7
	FX_OP_UNSUPPORTED    = 8

	// see draft-ietf-secsh-filexfer-13
	// https://tools.ietf.org/html/draft-ietf-secsh-filexfer-13#section-9.1
	FX_INVALID_HANDLE              = 9
	FX_NO_SUCH_PATH                = 10
	FX_FILE_ALREADY_EXISTS         = 11
	FX_WRITE_PROTECT               = 12
	FX_NO_MEDIA                    = 13
	FX_NO_SPACE_ON_FILESYSTEM      = 14
	FX_QUOTA_EXCEEDED              = 15
	FX_UNKNOWN_PRINCIPAL           = 16
	FX_LOCK_CONFLICT               = 17
	FX_DIR_NOT_EMPTY               = 18
	FX_NOT_A_DIRECTORY             = 19
	FX_INVALID_FILENAME            = 20
	FX_LINK_LOOP                   = 21
	FX_CANNOT_DELETE               = 22
	FX_INVALID_PARAMETER           = 23
	FX_FILE_IS_A_DIRECTORY         = 24
	FX_BYTE_RANGE_LOCK_CONFLICT    = 25
	FX_BYTE_RANGE_LOCK_REFUSED     = 26
	FX_DELETE_PENDING              = 27
	FX_FILE_CORRUPT                = 28
	FX_OWNER_INVALID               = 29
	FX_GROUP_INVALID               = 30
	FX_NO_MATCHING_BYTE_RANGE_LOCK = 31
)

const (
	FXF_READ   = 0x00000001
	FXF_WRITE  = 0x00000002
	FXF_APPEND = 0x00000004
	FXF_CREAT  = 0x00000008
	FXF_TRUNC  = 0x00000010
	FXF_EXCL   = 0x00000020
)

const (
	FILEXFER_ATTR_SIZE        = 0x00000001
	FILEXFER_ATTR_UIDGID      = 0x00000002
	FILEXFER_ATTR_PERMISSIONS = 0x00000004
	FILEXFER_ATTR_ACMODTIME   = 0x00000008
	FILEXFER_ATTR_EXTENDED    = 0x80000000
)

const (
	S_IFMT   = 00170000
	S_IFSOCK = 0140000
	S_IFLNK  = 0120000
	S_IFREG  = 0100000
	S_IFBLK  = 0060000
	S_IFDIR  = 0040000
	S_IFCHR  = 0020000
	S_IFIFO  = 0010000
	S_ISUID  = 0004000
	S_ISGID  = 0002000
	S_ISVTX  = 0001000
)

type fxp uint8

func (f fxp) String() string {
	switch f {
	case FXP_INIT:
		return "FXP_INIT"
	case FXP_VERSION:
		return "FXP_VERSION"
	case FXP_OPEN:
		return "FXP_OPEN"
	case FXP_CLOSE:
		return "FXP_CLOSE"
	case FXP_READ:
		return "FXP_READ"
	case FXP_WRITE:
		return "FXP_WRITE"
	case FXP_LSTAT:
		return "FXP_LSTAT"
	case FXP_FSTAT:
		return "FXP_FSTAT"
	case FXP_SETSTAT:
		return "FXP_SETSTAT"
	case FXP_FSETSTAT:
		return "FXP_FSETSTAT"
	case FXP_OPENDIR:
		return "FXP_OPENDIR"
	case FXP_READDIR:
		return "FXP_READDIR"
	case FXP_REMOVE:
		return "FXP_REMOVE"
	case FXP_MKDIR:
		return "FXP_MKDIR"
	case FXP_RMDIR:
		return "FXP_RMDIR"
	case FXP_REALPATH:
		return "FXP_REALPATH"
	case FXP_STAT:
		return "FXP_STAT"
	case FXP_RENAME:
		return "FXP_RENAME"
	case FXP_READLINK:
		return "FXP_READLINK"
	case FXP_SYMLINK:
		return "FXP_SYMLINK"
	case FXP_STATUS:
		return "FXP_STATUS"
	case FXP_HANDLE:
		return "FXP_HANDLE"
	case FXP_DATA:
		return "FXP_DATA"
	case FXP_NAME:
		return "FXP_NAME"
	case FXP_ATTRS:
		return "FXP_ATTRS"
	case FXP_EXTENDED:
		return "FXP_EXTENDED"
	case FXP_EXTENDED_REPLY:
		return "FXP_EXTENDED_REPLY"
	default:
		return "unknown"
	}
}

type fx uint8

func (f fx) String() string {
	switch f {
	case FX_OK:
		return "FX_OK"
	case FX_EOF:
		return "FX_EOF"
	case FX_NO_SUCH_FILE:
		return "FX_NO_SUCH_FILE"
	case FX_PERMISSION_DENIED:
		return "FX_PERMISSION_DENIED"
	case FX_FAILURE:
		return "FX_FAILURE"
	case FX_BAD_MESSAGE:
		return "FX_BAD_MESSAGE"
	case FX_NO_CONNECTION:
		return "FX_NO_CONNECTION"
	case FX_CONNECTION_LOST:
		return "FX_CONNECTION_LOST"
	case FX_OP_UNSUPPORTED:
		return "FX_OP_UNSUPPORTED"
	default:
		return "unknown"
	}
}

type unexpectedPacketErr struct {
	want, got uint8
}

func (u *unexpectedPacketErr) Error() string {
	return fmt.Sprintf("sftp: unexpected packet: want %v, got %v", fxp(u.want), fxp(u.got))
}

func unimplementedPacketErr(u uint8) error {
	return fmt.Errorf("sftp: unimplemented packet type: got %v", fxp(u))
}

type unexpectedIDErr struct{ want, got uint32 }

func (u *unexpectedIDErr) Error() string {
	return fmt.Sprintf("sftp: unexpected id: want %v, got %v", u.want, u.got)
}

func unimplementedSeekWhence(whence int) error {
	return fmt.Errorf("sftp: unimplemented seek whence %v", whence)
}

func unexpectedCount(want, got uint32) error {
	return fmt.Errorf("sftp: unexpected count: want %v, got %v", want, got)
}

type unexpectedVersionErr struct{ want, got uint32 }

func (u *unexpectedVersionErr) Error() string {
	return fmt.Sprintf("sftp: unexpected server version: want %v, got %v", u.want, u.got)
}

// A StatusError is returned when an SFTP operation fails, and provides
// additional information about the failure.
type StatusError struct {
	Code uint32
	Msg  string
	Lang string
}

func (s *StatusError) Error() string { return fmt.Sprintf("sftp: %q (%v)", s.Msg, fx(s.Code)) }
