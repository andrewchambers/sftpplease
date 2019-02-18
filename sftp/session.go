package sftp

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/andrewchambers/sftpplease/sftp/protosftp"
	"github.com/andrewchambers/sftpplease/vfs"
)

var (
	ErrInvalidHandle    = errors.New("invalid handle")
	ErrUnsupported      = errors.New("unsupported operation")
	ErrBadRead          = errors.New("bad read")
	ErrTooManyOpenFiles = errors.New("too many open files")
)

type Options struct {
	Debug    bool
	MaxFiles int
	LogFunc  func(string, ...interface{})
}

type Session struct {
	Options *Options

	fs vfs.VFS

	files     map[string]*handle
	inbox     chan protosftp.Packet
	outbox    chan protosftp.Packet
	closed    chan struct{}
	closeOnce sync.Once
	wg        sync.WaitGroup

	fcounter int64
}

type handle struct {
	Id      string
	reqChan chan protosftp.Packet
}

func (s *Session) newFileHandle(f vfs.File) *handle {
	id := fmt.Sprintf("%d", s.fcounter)
	s.fcounter += 1

	h := &handle{
		Id:      id,
		reqChan: make(chan protosftp.Packet),
	}

	// Each file has it's own goroutine and request
	// channel. This makes it easier do concurrent operations
	// but still process file requests in the order they arrive.
	// Some file systems have strict ordering requirements.
	go func() {
		for req := range h.reqChan {
			switch req := req.(type) {
			case *protosftp.FxpFstatPacket:
				st, err := f.Stat()
				if err != nil {
					s.respondError(req.ID, err)
					continue
				}
				s.Respond(&protosftp.FxpStatResponse{
					ID:   req.ID,
					Info: fileStatToSFTPStat(st),
				})
			case *protosftp.FxpWritePacket:
				_, err := f.WriteAt(req.Data, int64(req.Offset))
				if err != nil {
					s.respondError(req.ID, err)
					continue
				}
				s.respondOk(req.ID)
			case *protosftp.FxpReadPacket:
				if req.Len > 1024*1024 {
					s.respondError(req.ID, ErrBadRead)
					continue
				}

				buf := make([]byte, req.Len, req.Len)

				n, err := f.ReadAt(buf, int64(req.Offset))
				if err != nil && n == 0 {
					s.respondError(req.ID, err)
					continue
				}

				s.Respond(&protosftp.FxpDataPacket{
					ID:     req.ID,
					Length: uint32(n),
					Data:   buf[:n],
				})
			case *protosftp.FxpReaddirPacket:
				stats, err := f.Readdir(64)
				if err != nil {
					s.respondError(req.ID, err)
					continue
				}

				resp := &protosftp.FxpNamePacket{ID: req.ID}
				for _, stat := range stats {
					resp.NameAttrs = append(resp.NameAttrs, protosftp.FxpNameAttr{
						Name:     stat.Name(),
						LongName: runLsStat(stat),
						Attrs:    fileStatToSFTPStat(stat),
					})
				}
				s.Respond(resp)
			case *protosftp.FxpClosePacket:
				err := f.Close()
				if err != nil {
					s.respondError(req.ID, err)
					return
				}
				s.respondOk(req.ID)
				return
			default:
				s.Logf("unsupported file request: %#v", req)
			}
		}
	}()

	return h
}

func (s *Session) Logf(format string, args ...interface{}) {
	s.Options.LogFunc(format, args...)
}

func (s *Session) Respond(resp protosftp.Packet) {
	select {
	case <-s.closed:
	case s.outbox <- resp:
	}
}

func Serve(opt *Options, fs vfs.VFS, rw io.ReadWriter) {

	s := &Session{
		Options: opt,
		fs:      fs,
		files:   make(map[string]*handle),
		inbox:   make(chan protosftp.Packet, 16),
		outbox:  make(chan protosftp.Packet, 16),
		closed:  make(chan struct{}),
	}

	shutdown := func() {
		s.closeOnce.Do(func() {
			close(s.closed)
		})
		s.wg.Done()
	}

	s.wg.Add(1)
	go func() {
		defer shutdown()
		for {
			req, err := protosftp.ReadPacket(rw)
			if err != nil {
				if s.Options.Debug {
					s.Logf("reading message failed: %s", err)
				}
				break
			}
			if s.Options.Debug {
				s.Logf("got packet: %#v", req)
			}
			select {
			case <-s.closed:
				return
			case s.inbox <- req:
			}
		}
	}()

	s.wg.Add(1)
	go func() {
		defer shutdown()
		for {
			select {
			case <-s.closed:
				return
			case resp := <-s.outbox:
				if s.Options.Debug {
					s.Logf("sending response: %#v", resp)
				}
				err := protosftp.WritePacket(rw, resp)
				if err != nil {
					s.Logf("writing response failed: %s", err)
					break
				}
			}
		}
	}()

	s.wg.Add(1)
	go func() {
		defer shutdown()
		for {
			select {
			case <-s.closed:
				return
			case req := <-s.inbox:
				switch req := req.(type) {
				case *protosftp.FxpClosePacket:
					s.handleClose(req)
				case *protosftp.FxpFstatPacket:
					s.handleFstat(req)
				case *protosftp.FxpInitPacket:
					s.handleInit(req)
				case *protosftp.FxpLstatPacket:
					s.handleLstat(req)
				case *protosftp.FxpMkdirPacket:
					s.handleMkdir(req)
				case *protosftp.FxpOpendirPacket:
					s.handleOpenDir(req)
				case *protosftp.FxpOpenPacket:
					s.handleOpen(req)
				case *protosftp.FxpReaddirPacket:
					s.handleReadDir(req)
				case *protosftp.FxpReadlinkPacket:
					s.handleReadLink(req)
				case *protosftp.FxpReadPacket:
					s.handleRead(req)
				case *protosftp.FxpRealpathPacket:
					s.handleRealPath(req)
				case *protosftp.FxpRemovePacket:
					s.handleRemove(req)
				case *protosftp.FxpRenamePacket:
					s.handleRename(req)
				case *protosftp.FxpRmdirPacket:
					s.handleRmdir(req)
				case *protosftp.FxpSetStatPacket:
					s.handleSetStat(req)
				case *protosftp.FxpStatPacket:
					s.handleStat(req)
				case *protosftp.FxpSymlinkPacket:
					s.handleSymlink(req)
				case *protosftp.FxpWritePacket:
					s.handleWrite(req)
				default:
					s.Logf("unimplemented request: %#v", req)
					return
				}
			}
		}
	}()

	s.wg.Wait()
}

func (s *Session) respondError(respId uint32, err error) {
	code := uint32(protosftp.FX_FAILURE)
	msg := "error"

	if err == io.EOF {
		code = protosftp.FX_EOF
		msg = err.Error()
	} else if err == os.ErrNotExist {
		code = protosftp.FX_NO_SUCH_FILE
		msg = err.Error()
	} else if os.IsPermission(err) {
		code = protosftp.FX_PERMISSION_DENIED
		msg = err.Error()
	} else if err == ErrUnsupported {
		code = protosftp.FX_OP_UNSUPPORTED
		msg = err.Error()
	} else {
		s.Logf("unhandled/unexpected error: %s", err)
	}

	s.Respond(protosftp.MakeStatus(respId, msg, code))
}

func (s *Session) respondOk(respId uint32) {
	s.Respond(protosftp.MakeStatus(respId, "", protosftp.FX_OK))
}

func (s *Session) handleInit(req *protosftp.FxpInitPacket) {
	s.Respond(&protosftp.FxVersionPacket{
		Version:    protosftp.ProtocolVersion,
		Extensions: nil,
	})
}

func (s *Session) handleRealPath(req *protosftp.FxpRealpathPacket) {
	p := req.Path
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	p = path.Clean(p)

	s.Respond(&protosftp.FxpNamePacket{
		ID: req.ID,
		NameAttrs: []protosftp.FxpNameAttr{{
			Name:     p,
			LongName: p, // XXX?
			Attrs:    protosftp.EmptyFileStat,
		}},
	})
}

func (s *Session) handleRemove(req *protosftp.FxpRemovePacket) {
	st, err := s.fs.Stat(req.Filename)
	if err != nil {
		s.respondError(req.ID, err)
		return
	}

	if st.IsDir() {
		s.respondError(req.ID, ErrUnsupported)
		return
	}

	err = s.fs.Remove(req.Filename)
	if err != nil {
		s.respondError(req.ID, err)
		return
	}

	s.respondOk(req.ID)
}

func (s *Session) handleRmdir(req *protosftp.FxpRmdirPacket) {

	st, err := s.fs.Stat(req.Path)
	if err != nil {
		s.respondError(req.ID, err)
		return
	}

	if !st.IsDir() {
		s.respondError(req.ID, ErrUnsupported)
		return
	}

	err = s.fs.Remove(req.Path)
	if err != nil {
		s.respondError(req.ID, err)
		return
	}

	s.respondOk(req.ID)
}

func (s *Session) handleFstat(req *protosftp.FxpFstatPacket) {

	h, ok := s.files[req.Handle]
	if !ok {
		s.respondError(req.ID, ErrInvalidHandle)
		return
	}

	h.reqChan <- req
}

func (s *Session) handleStat(req *protosftp.FxpStatPacket) {

	st, err := s.fs.Stat(req.Path)
	if err != nil {
		s.respondError(req.ID, err)
		return
	}

	s.Respond(&protosftp.FxpStatResponse{
		ID:   req.ID,
		Info: fileStatToSFTPStat(st),
	})
}

func (s *Session) handleSetStat(req *protosftp.FxpSetStatPacket) {

	if req.Attrs.Flags&protosftp.FILEXFER_ATTR_PERMISSIONS != 0 {
		err := s.fs.Chmod(req.Path, os.FileMode(req.Attrs.Mode))
		if err != nil {
			s.respondError(req.ID, err)
			return
		}

	}

	if req.Attrs.Flags&protosftp.FILEXFER_ATTR_SIZE != 0 {
		s.respondError(req.ID, ErrUnsupported)
		return
	}

	if req.Attrs.Flags&protosftp.FILEXFER_ATTR_ACMODTIME != 0 {
		s.respondError(req.ID, ErrUnsupported)
		return
	}

	s.respondOk(req.ID)
}

func (s *Session) handleLstat(req *protosftp.FxpLstatPacket) {

	st, err := s.fs.Stat(req.Path)
	if err != nil {
		s.respondError(req.ID, err)
		return
	}

	s.Respond(&protosftp.FxpStatResponse{
		ID:   req.ID,
		Info: fileStatToSFTPStat(st),
	})
}

func (s *Session) handleReadDir(req *protosftp.FxpReaddirPacket) {

	h, ok := s.files[req.Handle]
	if !ok {
		s.respondError(req.ID, ErrInvalidHandle)
		return
	}
	h.reqChan <- req
}

func (s *Session) handleClose(req *protosftp.FxpClosePacket) {

	h, ok := s.files[req.Handle]
	if !ok {
		s.respondError(req.ID, ErrInvalidHandle)
		return
	}
	delete(s.files, req.Handle)
	h.reqChan <- req
}

func (s *Session) handleOpen(req *protosftp.FxpOpenPacket) {

	if len(s.files) > s.Options.MaxFiles {
		s.respondError(req.ID, ErrTooManyOpenFiles)
		return
	}

	flags := 0

	if req.Pflags&protosftp.FXF_READ != 0 && req.Pflags&protosftp.FXF_WRITE != 0 {
		flags = os.O_RDWR
		req.Pflags &= ^uint32(protosftp.FXF_READ | protosftp.FXF_WRITE)
	} else if req.Pflags&protosftp.FXF_READ != 0 {
		flags = os.O_RDONLY
		req.Pflags &= ^uint32(protosftp.FXF_READ)
	} else if req.Pflags&protosftp.FXF_WRITE != 0 {
		flags = os.O_WRONLY
		req.Pflags &= ^uint32(protosftp.FXF_WRITE)
	}

	if req.Pflags&protosftp.FXF_TRUNC != 0 {
		flags |= os.O_TRUNC
		req.Pflags &= ^uint32(protosftp.FXF_TRUNC)
	}

	if req.Pflags&protosftp.FXF_CREAT != 0 {
		flags |= os.O_CREATE
		req.Pflags &= ^uint32(protosftp.FXF_CREAT)
	}

	if req.Pflags&protosftp.FXF_APPEND != 0 {
		flags |= os.O_APPEND
		req.Pflags &= ^uint32(protosftp.FXF_APPEND)
	}

	if req.Pflags&protosftp.FXF_EXCL != 0 {
		flags |= os.O_EXCL
		req.Pflags &= ^uint32(protosftp.FXF_EXCL)
	}

	if req.Pflags != 0 {
		s.respondError(req.ID, ErrUnsupported)
		return
	}

	mode := os.FileMode(0755)
	if req.Attrs.Flags&protosftp.FILEXFER_ATTR_PERMISSIONS != 0 {
		mode = os.FileMode(req.Attrs.Mode & 0777)
	}

	f, err := s.fs.OpenFile(req.Path, flags, mode)
	if err != nil {
		s.respondError(req.ID, err)
		return
	}

	handle := s.newFileHandle(f)

	s.files[handle.Id] = handle

	s.Respond(&protosftp.FxpHandlePacket{ID: req.ID, Handle: handle.Id})
}

func (s *Session) handleMkdir(req *protosftp.FxpMkdirPacket) {

	mode := os.FileMode(0755)
	if req.Attrs.Flags&protosftp.FILEXFER_ATTR_PERMISSIONS != 0 {
		mode = os.FileMode(req.Attrs.Mode & 0777)
	}

	err := s.fs.Mkdir(req.Path, mode)
	if err != nil {
		s.respondError(req.ID, err)
		return
	}

	s.respondOk(req.ID)
}

func (s *Session) handleOpenDir(req *protosftp.FxpOpendirPacket) {

	if len(s.files) > s.Options.MaxFiles {
		s.respondError(req.ID, ErrTooManyOpenFiles)
		return
	}

	f, err := s.fs.Open(req.Path)
	if err != nil {
		s.respondError(req.ID, err)
		return
	}

	handle := s.newFileHandle(f)

	s.files[handle.Id] = handle

	s.Respond(&protosftp.FxpHandlePacket{ID: req.ID, Handle: handle.Id})
}

func (s *Session) handleWrite(req *protosftp.FxpWritePacket) {

	h, ok := s.files[req.Handle]
	if !ok {
		s.respondError(req.ID, ErrInvalidHandle)
		return
	}
	h.reqChan <- req
}

func (s *Session) handleRead(req *protosftp.FxpReadPacket) {

	h, ok := s.files[req.Handle]
	if !ok {
		s.respondError(req.ID, ErrInvalidHandle)
		return
	}
	h.reqChan <- req
}

func (s *Session) handleRename(req *protosftp.FxpRenamePacket) {
	err := s.fs.Rename(req.Oldpath, req.Newpath)
	if err != nil {
		s.respondError(req.ID, err)
		return
	}
	s.respondOk(req.ID)
}

func (s *Session) handleReadLink(req *protosftp.FxpReadlinkPacket) {
	s.respondError(req.ID, ErrUnsupported)
}

func (s *Session) handleSymlink(req *protosftp.FxpSymlinkPacket) {
	s.respondError(req.ID, ErrUnsupported)
}

func fileStatToSFTPStat(stat os.FileInfo) protosftp.FileStat {
	// XXX there are a lot more flags...
	// Maybe there should be a whitelist
	mode := uint32(stat.Mode() & 0777)
	if stat.IsDir() {
		mode |= protosftp.S_IFDIR
	} else {
		mode |= protosftp.S_IFREG
	}

	return protosftp.FileStat{
		Flags: protosftp.FILEXFER_ATTR_SIZE | protosftp.FILEXFER_ATTR_PERMISSIONS | protosftp.FILEXFER_ATTR_ACMODTIME,
		Size:  uint64(stat.Size()),
		Atime: uint32(stat.ModTime().Unix()),
		Mtime: uint32(stat.ModTime().Unix()),
		Mode:  mode,
	}
}

func runLsTypeWord(mode os.FileMode) string {
	// find first character, the type char
	// b     Block special file.
	// c     Character special file.
	// d     Directory.
	// l     Symbolic link.
	// s     Socket link.
	// p     FIFO.
	// -     Regular file.
	tc := '-'
	if mode.IsDir() {
		tc = 'd'
	}

	// owner
	orc := '-'
	if (mode & 0400) != 0 {
		orc = 'r'
	}
	owc := '-'
	if (mode & 0200) != 0 {
		owc = 'w'
	}
	oxc := '-'
	ox := (mode & 0100) != 0
	if ox {
		oxc = 'x'
	}

	// group
	grc := '-'
	if (mode & 040) != 0 {
		grc = 'r'
	}
	gwc := '-'
	if (mode & 020) != 0 {
		gwc = 'w'
	}
	gxc := '-'
	gx := (mode & 010) != 0
	if gx {
		gxc = 'x'
	}

	// all / others
	arc := '-'
	if (mode & 04) != 0 {
		arc = 'r'
	}
	awc := '-'
	if (mode & 02) != 0 {
		awc = 'w'
	}
	axc := '-'
	ax := (mode & 01) != 0
	if ax {
		axc = 'x'
	}

	return fmt.Sprintf("%c%c%c%c%c%c%c%c%c%c", tc, orc, owc, oxc, grc, gwc, gxc, arc, awc, axc)
}

func runLsStat(stat os.FileInfo) string {
	// example from openssh sftp server:
	// crw-rw-rw-    1 root     wheel           0 Jul 31 20:52 ttyvd
	// format:
	// {directory / char device / etc}{rwxrwxrwx}  {number of links} owner group size month day [time (this year) | year (otherwise)] name

	typeword := runLsTypeWord(stat.Mode())
	numLinks := 1

	monthStr := stat.ModTime().Month().String()[0:3]
	day := stat.ModTime().Day()
	year := stat.ModTime().Year()
	now := time.Now()
	isOld := stat.ModTime().Before(now.Add(-time.Hour * 24 * 365 / 2))

	yearOrTime := fmt.Sprintf("%02d:%02d", stat.ModTime().Hour(), stat.ModTime().Minute())
	if isOld {
		yearOrTime = fmt.Sprintf("%d", year)
	}

	return fmt.Sprintf("%s %4d %-8s %-8s %8d %s %2d %5s %s", typeword, numLinks, "user", "user", stat.Size(), monthStr, day, yearOrTime, stat.Name())
}
