package protosftp

import (
	"encoding"
	"errors"
	"fmt"
	"io"
	"reflect"
)

type Packet interface {
	encoding.BinaryMarshaler
	encoding.BinaryUnmarshaler
}

var (
	errShortPacket           = errors.New("packet too short")
	errUnknownExtendedPacket = errors.New("unknown extended packet")
)

func WritePacket(w io.Writer, m Packet) error {
	bb, err := m.MarshalBinary()
	if err != nil {
		return fmt.Errorf("binary marshaller failed: %v", err)
	}
	l := uint32(len(bb))
	hdr := []byte{byte(l >> 24), byte(l >> 16), byte(l >> 8), byte(l)}
	_, err = w.Write(hdr)
	if err != nil {
		return fmt.Errorf("failed to send packet header: %v", err)
	}
	_, err = w.Write(bb)
	if err != nil {
		return fmt.Errorf("failed to send packet body: %v", err)
	}
	return nil
}

func ReadPacket(r io.Reader) (Packet, error) {
	var b = []byte{0, 0, 0, 0}
	if _, err := io.ReadFull(r, b); err != nil {
		return nil, err
	}

	var pkt Packet

	l, _ := unmarshalUint32(b)

	if l > 1024*1024 {
		return nil, errors.New("packet too large")
	}

	if l <= 1 {
		return nil, errors.New("packet too small")
	}

	b = make([]byte, l)
	if _, err := io.ReadFull(r, b); err != nil {
		return nil, err
	}

	pktType := fxp(b[0])

	switch pktType {
	case FXP_INIT:
		pkt = &FxpInitPacket{}
	case FXP_LSTAT:
		pkt = &FxpLstatPacket{}
	case FXP_OPEN:
		pkt = &FxpOpenPacket{}
	case FXP_CLOSE:
		pkt = &FxpClosePacket{}
	case FXP_READ:
		pkt = &FxpReadPacket{}
	case FXP_WRITE:
		pkt = &FxpWritePacket{}
	case FXP_FSTAT:
		pkt = &FxpFstatPacket{}
	case FXP_SETSTAT:
		pkt = &FxpSetStatPacket{}
	case FXP_FSETSTAT:
		pkt = &FxpFSetStatPacket{}
	case FXP_OPENDIR:
		pkt = &FxpOpendirPacket{}
	case FXP_READDIR:
		pkt = &FxpReaddirPacket{}
	case FXP_REMOVE:
		pkt = &FxpRemovePacket{}
	case FXP_MKDIR:
		pkt = &FxpMkdirPacket{}
	case FXP_RMDIR:
		pkt = &FxpRmdirPacket{}
	case FXP_REALPATH:
		pkt = &FxpRealpathPacket{}
	case FXP_STAT:
		pkt = &FxpStatPacket{}
	case FXP_RENAME:
		pkt = &FxpRenamePacket{}
	case FXP_READLINK:
		pkt = &FxpReadlinkPacket{}
	case FXP_SYMLINK:
		pkt = &FxpSymlinkPacket{}
	default:
		return nil, fmt.Errorf("unhandled packet type: %s", pktType)
	}

	if err := pkt.UnmarshalBinary(b[1:]); err != nil {
		return nil, err
	}

	return pkt, nil
}

func marshalUint32(b []byte, v uint32) []byte {
	return append(b, byte(v>>24), byte(v>>16), byte(v>>8), byte(v))
}

func marshalUint64(b []byte, v uint64) []byte {
	return marshalUint32(marshalUint32(b, uint32(v>>32)), uint32(v))
}

func marshalString(b []byte, v string) []byte {
	return append(marshalUint32(b, uint32(len(v))), v...)
}

func marshalStatus(b []byte, err StatusError) []byte {
	b = marshalUint32(b, err.Code)
	b = marshalString(b, err.Msg)
	b = marshalString(b, err.Lang)
	return b
}

func marshal(b []byte, v interface{}) []byte {
	if v == nil {
		return b
	}
	switch v := v.(type) {
	case uint8:
		return append(b, v)
	case uint32:
		return marshalUint32(b, v)
	case uint64:
		return marshalUint64(b, v)
	case string:
		return marshalString(b, v)
	default:
		switch d := reflect.ValueOf(v); d.Kind() {
		case reflect.Struct:
			for i, n := 0, d.NumField(); i < n; i++ {
				b = append(marshal(b, d.Field(i).Interface()))
			}
			return b
		case reflect.Slice:
			for i, n := 0, d.Len(); i < n; i++ {
				b = append(marshal(b, d.Index(i).Interface()))
			}
			return b
		default:
			panic(fmt.Sprintf("marshal(%#v): cannot handle type %T", v, v))
		}
	}
}

func unmarshalUint32(b []byte) (uint32, []byte) {
	v := uint32(b[3]) | uint32(b[2])<<8 | uint32(b[1])<<16 | uint32(b[0])<<24
	return v, b[4:]
}

func unmarshalUint32Safe(b []byte) (uint32, []byte, error) {
	var v uint32
	if len(b) < 4 {
		return 0, nil, errShortPacket
	}
	v, b = unmarshalUint32(b)
	return v, b, nil
}

func unmarshalUint64(b []byte) (uint64, []byte) {
	h, b := unmarshalUint32(b)
	l, b := unmarshalUint32(b)
	return uint64(h)<<32 | uint64(l), b
}

func unmarshalUint64Safe(b []byte) (uint64, []byte, error) {
	var v uint64
	if len(b) < 8 {
		return 0, nil, errShortPacket
	}
	v, b = unmarshalUint64(b)
	return v, b, nil
}

func unmarshalString(b []byte) (string, []byte) {
	n, b := unmarshalUint32(b)
	return string(b[:n]), b[n:]
}

func unmarshalStringSafe(b []byte) (string, []byte, error) {
	n, b, err := unmarshalUint32Safe(b)
	if err != nil {
		return "", nil, err
	}
	if int64(n) > int64(len(b)) {
		return "", nil, errShortPacket
	}
	return string(b[:n]), b[n:], nil
}

type extensionPair struct {
	Name string
	Data string
}

func unmarshalExtensionPair(b []byte) (extensionPair, []byte, error) {
	var ep extensionPair
	var err error
	ep.Name, b, err = unmarshalStringSafe(b)
	if err != nil {
		return ep, b, err
	}
	ep.Data, b, err = unmarshalStringSafe(b)
	if err != nil {
		return ep, b, err
	}
	return ep, b, err
}

type FxpInitPacket struct {
	Version    uint32
	Extensions []extensionPair
}

func (p FxpInitPacket) MarshalBinary() ([]byte, error) {
	l := 1 + 4
	for _, e := range p.Extensions {
		l += 4 + len(e.Name) + 4 + len(e.Data)
	}

	b := make([]byte, 0, l)
	b = append(b, FXP_INIT)
	b = marshalUint32(b, p.Version)
	for _, e := range p.Extensions {
		b = marshalString(b, e.Name)
		b = marshalString(b, e.Data)
	}
	return b, nil
}

func (p *FxpInitPacket) UnmarshalBinary(b []byte) error {
	var err error
	if p.Version, b, err = unmarshalUint32Safe(b); err != nil {
		return err
	}
	for len(b) > 0 {
		var ep extensionPair
		ep, b, err = unmarshalExtensionPair(b)
		if err != nil {
			return err
		}
		p.Extensions = append(p.Extensions, ep)
	}
	return nil
}

type FxVersionPacket struct {
	Version    uint32
	Extensions []struct {
		Name, Data string
	}
}

func (p FxVersionPacket) MarshalBinary() ([]byte, error) {
	l := 1 + 4
	for _, e := range p.Extensions {
		l += 4 + len(e.Name) + 4 + len(e.Data)
	}

	b := make([]byte, 0, l)
	b = append(b, FXP_VERSION)
	b = marshalUint32(b, p.Version)
	for _, e := range p.Extensions {
		b = marshalString(b, e.Name)
		b = marshalString(b, e.Data)
	}
	return b, nil
}

func (p FxVersionPacket) UnmarshalBinary(b []byte) error {
	return errors.New("XXX: unimplemented")
}

func marshalIDString(packetType byte, id uint32, str string) ([]byte, error) {
	l := 1 + 4 +
		4 + len(str)

	b := make([]byte, 0, l)
	b = append(b, packetType)
	b = marshalUint32(b, id)
	b = marshalString(b, str)
	return b, nil
}

func unmarshalIDString(b []byte, id *uint32, str *string) error {
	var err error
	*id, b, err = unmarshalUint32Safe(b)
	if err != nil {
		return err
	}
	*str, b, err = unmarshalStringSafe(b)
	return err
}

type FxpReaddirPacket struct {
	ID     uint32
	Handle string
}

func (p FxpReaddirPacket) MarshalBinary() ([]byte, error) {
	return marshalIDString(FXP_READDIR, p.ID, p.Handle)
}

func (p *FxpReaddirPacket) UnmarshalBinary(b []byte) error {
	return unmarshalIDString(b, &p.ID, &p.Handle)
}

type FxpOpendirPacket struct {
	ID   uint32
	Path string
}

func (p FxpOpendirPacket) MarshalBinary() ([]byte, error) {
	return marshalIDString(FXP_OPENDIR, p.ID, p.Path)
}

func (p *FxpOpendirPacket) UnmarshalBinary(b []byte) error {
	return unmarshalIDString(b, &p.ID, &p.Path)
}

type FxpLstatPacket struct {
	ID   uint32
	Path string
}

func (p FxpLstatPacket) MarshalBinary() ([]byte, error) {
	return marshalIDString(FXP_LSTAT, p.ID, p.Path)
}

func (p *FxpLstatPacket) UnmarshalBinary(b []byte) error {
	return unmarshalIDString(b, &p.ID, &p.Path)
}

type FxpStatPacket struct {
	ID   uint32
	Path string
}

func (p FxpStatPacket) MarshalBinary() ([]byte, error) {
	return marshalIDString(FXP_STAT, p.ID, p.Path)
}

func (p *FxpStatPacket) UnmarshalBinary(b []byte) error {
	return unmarshalIDString(b, &p.ID, &p.Path)
}

type FxpFstatPacket struct {
	ID     uint32
	Handle string
}

func (p FxpFstatPacket) MarshalBinary() ([]byte, error) {
	return marshalIDString(FXP_FSTAT, p.ID, p.Handle)
}

func (p *FxpFstatPacket) UnmarshalBinary(b []byte) error {
	return unmarshalIDString(b, &p.ID, &p.Handle)
}

type FxpStatResponse struct {
	ID   uint32
	Info FileStat
}

func (p FxpStatResponse) MarshalBinary() ([]byte, error) {
	b := []byte{FXP_ATTRS}
	b = marshalUint32(b, p.ID)
	b = marshalFileStat(b, &p.Info)
	return b, nil
}

func (p FxpStatResponse) UnmarshalBinary(b []byte) error {
	return errors.New("unimplemented")
}

type FxpClosePacket struct {
	ID     uint32
	Handle string
}

func (p FxpClosePacket) MarshalBinary() ([]byte, error) {
	return marshalIDString(FXP_CLOSE, p.ID, p.Handle)
}

func (p *FxpClosePacket) UnmarshalBinary(b []byte) error {
	return unmarshalIDString(b, &p.ID, &p.Handle)
}

type FxpRemovePacket struct {
	ID       uint32
	Filename string
}

func (p FxpRemovePacket) MarshalBinary() ([]byte, error) {
	return marshalIDString(FXP_REMOVE, p.ID, p.Filename)
}

func (p *FxpRemovePacket) UnmarshalBinary(b []byte) error {
	return unmarshalIDString(b, &p.ID, &p.Filename)
}

type FxpRmdirPacket struct {
	ID   uint32
	Path string
}

func (p FxpRmdirPacket) MarshalBinary() ([]byte, error) {
	return marshalIDString(FXP_RMDIR, p.ID, p.Path)
}

func (p *FxpRmdirPacket) UnmarshalBinary(b []byte) error {
	return unmarshalIDString(b, &p.ID, &p.Path)
}

type FxpSymlinkPacket struct {
	ID         uint32
	Targetpath string
	Linkpath   string
}

func (p FxpSymlinkPacket) MarshalBinary() ([]byte, error) {
	l := 1 + 4 +
		4 + len(p.Targetpath) +
		4 + len(p.Linkpath)

	b := make([]byte, 0, l)
	b = append(b, FXP_SYMLINK)
	b = marshalUint32(b, p.ID)
	b = marshalString(b, p.Targetpath)
	b = marshalString(b, p.Linkpath)
	return b, nil
}

func (p *FxpSymlinkPacket) UnmarshalBinary(b []byte) error {
	var err error
	if p.ID, b, err = unmarshalUint32Safe(b); err != nil {
		return err
	} else if p.Targetpath, b, err = unmarshalStringSafe(b); err != nil {
		return err
	} else if p.Linkpath, b, err = unmarshalStringSafe(b); err != nil {
		return err
	}
	return nil
}

type FxpReadlinkPacket struct {
	ID   uint32
	Path string
}

func (p FxpReadlinkPacket) MarshalBinary() ([]byte, error) {
	return marshalIDString(FXP_READLINK, p.ID, p.Path)
}

func (p *FxpReadlinkPacket) UnmarshalBinary(b []byte) error {
	return unmarshalIDString(b, &p.ID, &p.Path)
}

type FxpRealpathPacket struct {
	ID   uint32
	Path string
}

func (p FxpRealpathPacket) MarshalBinary() ([]byte, error) {
	return marshalIDString(FXP_REALPATH, p.ID, p.Path)
}

func (p *FxpRealpathPacket) UnmarshalBinary(b []byte) error {
	return unmarshalIDString(b, &p.ID, &p.Path)
}

type FxpNameAttr struct {
	Name     string
	LongName string
	Attrs    FileStat
}

var EmptyFileStat = FileStat{}

func (p FxpNameAttr) MarshalBinary() ([]byte, error) {
	b := []byte{}
	b = marshalString(b, p.Name)
	b = marshalString(b, p.LongName)
	b = marshalFileStat(b, &p.Attrs)
	return b, nil
}

type FxpNamePacket struct {
	ID        uint32
	NameAttrs []FxpNameAttr
}

func (p FxpNamePacket) MarshalBinary() ([]byte, error) {
	b := []byte{}
	b = append(b, FXP_NAME)
	b = marshalUint32(b, p.ID)
	b = marshalUint32(b, uint32(len(p.NameAttrs)))
	for _, na := range p.NameAttrs {
		ab, err := na.MarshalBinary()
		if err != nil {
			return nil, err
		}

		b = append(b, ab...)
	}
	return b, nil
}

func (p FxpNamePacket) UnmarshalBinary(b []byte) error {
	return errors.New("unimplemented XXX")
}

type FxpOpenPacket struct {
	ID     uint32
	Path   string
	Pflags uint32
	Attrs  FileStat
}

func (p FxpOpenPacket) MarshalBinary() ([]byte, error) {
	l := 1 + 4 +
		4 + len(p.Path) +
		4 + 4

	b := make([]byte, 0, l)
	b = append(b, FXP_OPEN)
	b = marshalUint32(b, p.ID)
	b = marshalString(b, p.Path)
	b = marshalUint32(b, p.Pflags)
	b = marshalFileStat(b, &p.Attrs)
	return b, nil
}

func (p *FxpOpenPacket) UnmarshalBinary(b []byte) error {
	var err error
	if p.ID, b, err = unmarshalUint32Safe(b); err != nil {
		return err
	} else if p.Path, b, err = unmarshalStringSafe(b); err != nil {
		return err
	} else if p.Pflags, b, err = unmarshalUint32Safe(b); err != nil {
		return err
	} else if b, err = unmarshalFileStatSafe(b, &p.Attrs); err != nil {
		return err
	}
	return nil
}

type FxpReadPacket struct {
	ID     uint32
	Handle string
	Offset uint64
	Len    uint32
}

func (p FxpReadPacket) MarshalBinary() ([]byte, error) {
	l := 1 + 4 +
		4 + len(p.Handle) +
		8 + 4

	b := make([]byte, 0, l)
	b = append(b, FXP_READ)
	b = marshalUint32(b, p.ID)
	b = marshalString(b, p.Handle)
	b = marshalUint64(b, p.Offset)
	b = marshalUint32(b, p.Len)
	return b, nil
}

func (p *FxpReadPacket) UnmarshalBinary(b []byte) error {
	var err error
	if p.ID, b, err = unmarshalUint32Safe(b); err != nil {
		return err
	} else if p.Handle, b, err = unmarshalStringSafe(b); err != nil {
		return err
	} else if p.Offset, b, err = unmarshalUint64Safe(b); err != nil {
		return err
	} else if p.Len, b, err = unmarshalUint32Safe(b); err != nil {
		return err
	}
	return nil
}

type FxpRenamePacket struct {
	ID      uint32
	Oldpath string
	Newpath string
}

func (p FxpRenamePacket) MarshalBinary() ([]byte, error) {
	l := 1 + 4 +
		4 + len(p.Oldpath) +
		4 + len(p.Newpath)

	b := make([]byte, 0, l)
	b = append(b, FXP_RENAME)
	b = marshalUint32(b, p.ID)
	b = marshalString(b, p.Oldpath)
	b = marshalString(b, p.Newpath)
	return b, nil
}

func (p *FxpRenamePacket) UnmarshalBinary(b []byte) error {
	var err error
	if p.ID, b, err = unmarshalUint32Safe(b); err != nil {
		return err
	} else if p.Oldpath, b, err = unmarshalStringSafe(b); err != nil {
		return err
	} else if p.Newpath, b, err = unmarshalStringSafe(b); err != nil {
		return err
	}
	return nil
}

type FxpWritePacket struct {
	ID     uint32
	Handle string
	Offset uint64
	Length uint32
	Data   []byte
}

func (p FxpWritePacket) MarshalBinary() ([]byte, error) {
	l := 1 + 4 +
		4 + len(p.Handle) +
		8 + 4 +
		len(p.Data)

	b := make([]byte, 0, l)
	b = append(b, FXP_WRITE)
	b = marshalUint32(b, p.ID)
	b = marshalString(b, p.Handle)
	b = marshalUint64(b, p.Offset)
	b = marshalUint32(b, p.Length)
	b = append(b, p.Data...)
	return b, nil
}

func (p *FxpWritePacket) UnmarshalBinary(b []byte) error {
	var err error
	if p.ID, b, err = unmarshalUint32Safe(b); err != nil {
		return err
	} else if p.Handle, b, err = unmarshalStringSafe(b); err != nil {
		return err
	} else if p.Offset, b, err = unmarshalUint64Safe(b); err != nil {
		return err
	} else if p.Length, b, err = unmarshalUint32Safe(b); err != nil {
		return err
	} else if uint32(len(b)) < p.Length {
		return errShortPacket
	}

	p.Data = append([]byte{}, b[:p.Length]...)
	return nil
}

type FxpMkdirPacket struct {
	ID    uint32
	Path  string
	Attrs FileStat
}

func (p FxpMkdirPacket) MarshalBinary() ([]byte, error) {
	l := 1 + 4 +
		4 + len(p.Path) +
		4 // uint32

	b := make([]byte, 0, l)
	b = append(b, FXP_MKDIR)
	b = marshalUint32(b, p.ID)
	b = marshalString(b, p.Path)
	b = marshalFileStat(b, &p.Attrs)
	return b, nil
}

func (p *FxpMkdirPacket) UnmarshalBinary(b []byte) error {
	var err error
	if p.ID, b, err = unmarshalUint32Safe(b); err != nil {
		return err
	} else if p.Path, b, err = unmarshalStringSafe(b); err != nil {
		return err
	} else if b, err = unmarshalFileStatSafe(b, &p.Attrs); err != nil {
		return err
	}
	return nil
}

type FxpSetStatPacket struct {
	ID    uint32
	Path  string
	Attrs FileStat
}

type FxpFSetStatPacket struct {
	ID     uint32
	Handle string
	Attrs  FileStat
}

func (p FxpSetStatPacket) MarshalBinary() ([]byte, error) {
	l := 1 + 4 +
		4 + len(p.Path) +
		4 // uint32 + uint64

	b := make([]byte, 0, l)
	b = append(b, FXP_SETSTAT)
	b = marshalUint32(b, p.ID)
	b = marshalString(b, p.Path)
	b = marshalFileStat(b, &p.Attrs)
	return b, nil
}

func (p FxpFSetStatPacket) MarshalBinary() ([]byte, error) {
	l := 1 + 4 +
		4 + len(p.Handle) +
		4 // uint32 + uint64

	b := make([]byte, 0, l)
	b = append(b, FXP_FSETSTAT)
	b = marshalUint32(b, p.ID)
	b = marshalString(b, p.Handle)
	b = marshalFileStat(b, &p.Attrs)
	return b, nil
}

func (p *FxpSetStatPacket) UnmarshalBinary(b []byte) error {
	var err error
	if p.ID, b, err = unmarshalUint32Safe(b); err != nil {
		return err
	} else if p.Path, b, err = unmarshalStringSafe(b); err != nil {
		return err
	} else if b, err = unmarshalFileStatSafe(b, &p.Attrs); err != nil {
		return err
	}
	return nil
}

func (p *FxpFSetStatPacket) UnmarshalBinary(b []byte) error {
	var err error
	if p.ID, b, err = unmarshalUint32Safe(b); err != nil {
		return err
	} else if p.Handle, b, err = unmarshalStringSafe(b); err != nil {
		return err
	} else if b, err = unmarshalFileStatSafe(b, &p.Attrs); err != nil {
		return err
	}
	return nil
}

type FxpHandlePacket struct {
	ID     uint32
	Handle string
}

func (p FxpHandlePacket) MarshalBinary() ([]byte, error) {
	b := []byte{FXP_HANDLE}
	b = marshalUint32(b, p.ID)
	b = marshalString(b, p.Handle)
	return b, nil
}

func (p *FxpHandlePacket) UnmarshalBinary(b []byte) error {
	return errors.New("unimplemented")
}

type FxpStatusPacket struct {
	ID          uint32
	StatusError StatusError
}

func (p FxpStatusPacket) MarshalBinary() ([]byte, error) {
	b := []byte{FXP_STATUS}
	b = marshalUint32(b, p.ID)
	b = marshalStatus(b, p.StatusError)
	return b, nil
}

func (p FxpStatusPacket) UnmarshalBinary(b []byte) error {
	return errors.New("unimplemented XXX")
}

type FxpDataPacket struct {
	ID     uint32
	Length uint32
	Data   []byte
}

func (p FxpDataPacket) MarshalBinary() ([]byte, error) {
	b := []byte{FXP_DATA}
	b = marshalUint32(b, p.ID)
	b = marshalUint32(b, p.Length)
	b = append(b, p.Data[:p.Length]...)
	return b, nil
}

func (p *FxpDataPacket) UnmarshalBinary(b []byte) error {
	var err error
	if p.ID, b, err = unmarshalUint32Safe(b); err != nil {
		return err
	} else if p.Length, b, err = unmarshalUint32Safe(b); err != nil {
		return err
	} else if uint32(len(b)) < p.Length {
		return errors.New("truncated packet")
	}

	p.Data = make([]byte, p.Length)
	copy(p.Data, b)
	return nil
}

type StatExtended struct {
	ExtType string
	ExtData string
}

type FileStat struct {
	Flags    uint32
	Size     uint64
	Mode     uint32
	Mtime    uint32
	Atime    uint32
	UID      uint32
	GID      uint32
	Extended []StatExtended
}

func marshalFileStat(b []byte, stat *FileStat) []byte {

	b = marshalUint32(b, stat.Flags)
	if stat.Flags&FILEXFER_ATTR_SIZE != 0 {
		b = marshalUint64(b, stat.Size)
	}
	if stat.Flags&FILEXFER_ATTR_UIDGID != 0 {
		b = marshalUint32(b, stat.UID)
		b = marshalUint32(b, stat.GID)
	}
	if stat.Flags&FILEXFER_ATTR_PERMISSIONS != 0 {
		b = marshalUint32(b, stat.Mode)
	}
	if stat.Flags&FILEXFER_ATTR_ACMODTIME != 0 {
		b = marshalUint32(b, stat.Atime)
		b = marshalUint32(b, stat.Mtime)
	}

	return b
}

func unmarshalFileStatSafe(b []byte, stat *FileStat) ([]byte, error) {
	var err error

	if stat.Flags, b, err = unmarshalUint32Safe(b); err != nil {
		return nil, err
	}

	if stat.Flags&FILEXFER_ATTR_UIDGID != 0 {
		if stat.UID, b, err = unmarshalUint32Safe(b); err != nil {
			return nil, err
		}
		if stat.GID, b, err = unmarshalUint32Safe(b); err != nil {
			return nil, err
		}
	}

	if stat.Flags&FILEXFER_ATTR_PERMISSIONS != 0 {
		if stat.Mode, b, err = unmarshalUint32Safe(b); err != nil {
			return nil, err
		}
	}

	if stat.Flags&FILEXFER_ATTR_ACMODTIME != 0 {
		if stat.Atime, b, err = unmarshalUint32Safe(b); err != nil {
			return nil, err
		}
		if stat.Mtime, b, err = unmarshalUint32Safe(b); err != nil {
			return nil, err
		}
	}

	return b, nil
}

func MakeStatus(id uint32, msg string, code uint32) *FxpStatusPacket {
	ret := &FxpStatusPacket{
		ID: id,
		StatusError: StatusError{
			Code: code,
			Msg:  msg,
		},
	}
	return ret
}
