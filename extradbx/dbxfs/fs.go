package dbxfs

import (
	"encoding/gob"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/andrewchambers/sftpplease/extradbx"
	"github.com/andrewchambers/sftpplease/vfs"
	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox"
	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox/files"
)

func init() {
	vfs.RegisterEngine("dropbox", vfsFactory)
}

func vfsFactory(token string) (vfs.VFS, error) {
	return Attach(dropbox.Config{
		Token: token,
	})
}

type Fs struct {
	api files.Client
}

type FileHandle struct {
	fs *Fs

	fpath  string
	dbxfid string
	isDir  bool

	dirEntTempFile *os.File
	dirEntDecoder  *gob.Decoder

	openForReading bool
	openForWriting bool

	readOffset int64
	reader     io.ReadCloser

	writeOffset int64
	writer      io.WriteCloser
}

type FileStat struct {
	FileMetadata   *files.FileMetadata
	FolderMetadata *files.FolderMetadata
}

func Attach(cfg dropbox.Config) (*Fs, error) {

	fs := &Fs{
		api: files.New(cfg),
	}

	return fs, nil
}

func doWithRetry(f func() error) error {
again:
	err := f()
	if err != nil {
		// XXX string checking?
		if strings.HasPrefix("too_many_write_operations", err.Error()) {
			// XXX Totally arbitrary
			time.Sleep(100 * time.Millisecond)
			goto again
		}
	}
	return err
}

func (fs *Fs) Create(fpath string) (*FileHandle, error) {

	fh := &FileHandle{
		fs:    fs,
		fpath: fpath,
	}

	fh.openForWriting = true
	return fh, nil
}

func (fs *Fs) Chmod(fpath string, mode os.FileMode) error {
	return nil
}

func (fs *Fs) Open(fpath string) (vfs.File, error) {
	if fpath == "/" {
		fpath = ""
	}

	st, err := dbxStat(fs.api, fpath)
	if err != nil {
		return nil, err
	}

	fh := &FileHandle{
		fs:     fs,
		fpath:  fpath,
		dbxfid: st.GetDropboxId(),
	}

	fh.isDir = st.IsDir()
	fh.openForReading = true

	return fh, nil
}

func (fs *Fs) OpenFile(fpath string, flags int, perm os.FileMode) (vfs.File, error) {
	switch flags & 3 {
	case os.O_RDONLY:
		return fs.Open(fpath)
	default:
		if flags&os.O_EXCL != 0 {
			_, err := fs.Stat(fpath)
			if err == nil {
				return nil, os.ErrExist
			}
		}

		return fs.Create(fpath)
	}
}

func (fs *Fs) Mkdir(fpath string, mode os.FileMode) error {
	_, err := fs.api.CreateFolderV2(files.NewCreateFolderArg(fpath))
	if err != nil {
		return err
	}
	return nil
}

func dbxMetadataToFileStat(md files.IsMetadata) (*FileStat, error) {
	fileMetadata, _ := md.(*files.FileMetadata)
	folderMetadata, _ := md.(*files.FolderMetadata)

	if fileMetadata == nil && folderMetadata == nil {
		return nil, ErrStatUnavailable
	}

	return &FileStat{
		FileMetadata:   fileMetadata,
		FolderMetadata: folderMetadata,
	}, nil

}

func dbxStat(api files.Client, fpath string) (*FileStat, error) {
	if fpath == "/" || fpath == "" {
		return &FileStat{
			FolderMetadata: &files.FolderMetadata{},
		}, nil
	}

	md, err := api.GetMetadata(files.NewGetMetadataArg(fpath))
	if err != nil {
		switch err := err.(type) {
		case files.GetMetadataAPIError:
			switch err.EndpointError.Path.Tag {
			case "not_found":
				return nil, os.ErrNotExist
			}
		}
		return nil, err
	}
	return dbxMetadataToFileStat(md)
}

func (fs *Fs) Stat(fpath string) (os.FileInfo, error) {
	return dbxStat(fs.api, fpath)
}

func (fs *Fs) Rename(from, to string) error {
	return doWithRetry(func() error {
		_, err := fs.api.MoveV2(files.NewRelocationArg(from, to))
		if err != nil {
			return err
		}
		return nil
	})
}

func (fs *Fs) Remove(fpath string) error {
	// XXX: Should we refuse to delete
	// non empty dirs for consistency?

	return doWithRetry(func() error {
		_, err := fs.api.DeleteV2(files.NewDeleteArg(fpath))
		if err != nil {
			return err
		}
		return nil
	})
}

func (fs *Fs) Close() error {
	return nil
}

func (f *FileHandle) Stat() (os.FileInfo, error) {
	return f.fs.Stat(f.fpath)
}

func (f *FileHandle) Readdir(n int) ([]os.FileInfo, error) {
	if !f.isDir {
		return nil, ErrNotDir
	}

	if !f.openForReading {
		return nil, ErrNotOpen
	}

	if f.dirEntTempFile == nil {
		dirEntTempFile, err := ioutil.TempFile("", "")
		if err != nil {
			return nil, err
		}
		encoder := gob.NewEncoder(dirEntTempFile)
		res, err := f.fs.api.ListFolder(files.NewListFolderArg(f.fpath))
		if err != nil {
			return nil, err
		}
	HasMore:

		for _, entry := range res.Entries {
			fileStat, err := dbxMetadataToFileStat(entry)
			if err != nil {
				return nil, err
			}
			err = encoder.Encode(fileStat)
			if err != nil {
				return nil, err
			}
		}

		if res.HasMore {
			arg := files.NewListFolderContinueArg(res.Cursor)
			res, err = f.fs.api.ListFolderContinue(arg)
			if err != nil {
				return nil, err
			}
			goto HasMore
		}

		err = dirEntTempFile.Sync()
		if err != nil {
			return nil, err
		}

		_, err = dirEntTempFile.Seek(0, io.SeekStart)
		if err != nil {
			return nil, err
		}

		f.dirEntTempFile = dirEntTempFile
		f.dirEntDecoder = gob.NewDecoder(dirEntTempFile)
	}

	stats := []os.FileInfo{}
	for len(stats) != n || n <= 0 {
		st := &FileStat{}
		err := f.dirEntDecoder.Decode(st)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		stats = append(stats, st)
	}

	if len(stats) == 0 && n > 0 {
		return stats, io.EOF
	}

	return stats, nil
}

func (f *FileHandle) Readdirnames(n int) ([]string, error) {
	names := []string{}
	info, err := f.Readdir(n)
	if info != nil {
		for _, st := range info {
			names = append(names, st.Name())
		}
	}
	return names, err
}

func (f *FileHandle) ReadAt(b []byte, off int64) (int, error) {

	if f.isDir {
		return 0, ErrNotFile
	}

	if !f.openForReading {
		return 0, ErrNotOpen
	}

	if off != f.readOffset {
		return 0, ErrBadReadWriteOffset
	}

	// Lazily open reader in case it is never used
	if f.reader == nil {
		_, contents, err := f.fs.api.Download(files.NewDownloadArg(f.dbxfid))
		if err != nil {
			return 0, err
		}
		f.reader = contents
	}

	ntot := 0
	for len(b) != 0 {
		n, err := f.reader.Read(b)
		b = b[n:]
		ntot += n
		f.readOffset += int64(n)
		if err != nil {
			return ntot, err
		}
	}
	return ntot, nil
}

func (f *FileHandle) Read(b []byte) (int, error) {
	return f.ReadAt(b, f.readOffset)
}

func (f *FileHandle) Write(b []byte) (int, error) {
	return f.WriteAt(b, f.writeOffset)
}

func (f *FileHandle) WriteAt(b []byte, off int64) (int, error) {

	if f.isDir {
		return 0, ErrNotFile
	}

	if !f.openForWriting {
		return 0, ErrNotOpen
	}

	// Lazily open writer in case it is never used
	if f.writer == nil {
		writer, err := extradbx.NewUpload(f.fs.api, f.fpath)
		if err != nil {
			return 0, err
		}
		f.writer = writer
	}

	if off != f.writeOffset {
		return 0, ErrBadReadWriteOffset
	}

	n, err := f.writer.Write(b)
	f.writeOffset += int64(n)
	return n, err
}

func (f *FileHandle) Close() error {

	f.openForReading = false
	f.openForWriting = false

	if f.dirEntTempFile != nil {
		name := f.dirEntTempFile.Name()
		_ = f.dirEntTempFile.Close()
		_ = os.Remove(name)
		f.dirEntTempFile = nil
		f.dirEntDecoder = nil
	}

	if f.reader != nil {
		_ = f.reader.Close()
		f.reader = nil
	}

	if f.writer != nil {
		err := f.writer.Close()
		if err != nil {
			return err
		}
		f.writer = nil
	}

	return nil
}

func (f *FileHandle) Chmod(mode os.FileMode) error {
	return f.fs.Chmod(f.fpath, mode)
}

func (f *FileHandle) Name() string {
	return f.fpath
}

func (st *FileStat) Name() string {
	if st.IsDir() {
		return st.FolderMetadata.Name
	} else {
		return st.FileMetadata.Name
	}
}

func (st *FileStat) GetDropboxId() string {
	if st.IsDir() {
		return st.FolderMetadata.Id
	} else {
		return st.FileMetadata.Id
	}
}

func (st *FileStat) Size() int64 {
	if st.IsDir() {
		return 0
	}
	return int64(st.FileMetadata.Size)
}

func (st *FileStat) Mode() os.FileMode {
	if st.IsDir() {
		return os.ModeDir | 0755
	}
	return 0755
}

func (st *FileStat) ModTime() time.Time {
	if st.IsDir() {
		return time.Now() // what is better?
	} else {
		return st.FileMetadata.ClientModified
	}
}

func (st *FileStat) IsDir() bool {
	return st.FolderMetadata != nil
}

func (st *FileStat) Sys() interface{} {
	return nil
}
