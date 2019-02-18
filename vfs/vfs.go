package vfs

import (
	"fmt"
	"os"
)

type File interface {
	Name() string
	Chmod(mode os.FileMode) error
	Read(buf []byte) (int, error)
	ReadAt(buf []byte, offset int64) (int, error)
	Readdir(n int) ([]os.FileInfo, error)
	Readdirnames(n int) ([]string, error)
	Write(buf []byte) (int, error)
	WriteAt(buf []byte, off int64) (int, error)
	Stat() (os.FileInfo, error)
	Close() error
}

type VFS interface {
	Chmod(name string, mode os.FileMode) error
	Open(path string) (File, error)
	OpenFile(name string, flag int, perm os.FileMode) (File, error)
	Mkdir(path string, perm os.FileMode) error
	Stat(path string) (os.FileInfo, error)
	Rename(from, to string) error
	Remove(path string) error
	Close() error
}

type NewVFSFunc func(string) (VFS, error)

func Open(engineName, params string) (VFS, error) {
	fn, ok := vfsFactories[engineName]
	if !ok {
		return nil, fmt.Errorf("no vfs called '%s'", engineName)
	}
	return fn(params)
}

func RegisterEngine(name string, fn NewVFSFunc) {
	vfsFactories[name] = fn
}

var vfsFactories map[string]NewVFSFunc

func init() {
	vfsFactories = make(map[string]NewVFSFunc)
}

type ReadOnlyVFS struct {
	Fs VFS
}

func (rofs *ReadOnlyVFS) Chmod(name string, mode os.FileMode) error {
	return os.ErrPermission
}

func (rofs *ReadOnlyVFS) Open(path string) (File, error) {
	f, err := rofs.Fs.Open(path)
	if err != nil {
		return nil, err
	}
	return &ReadOnlyFile{F: f}, nil
}

func (rofs *ReadOnlyVFS) OpenFile(name string, flag int, perm os.FileMode) (File, error) {
	writeAttempt := true
	if (flag & os.O_WRONLY) == 0 {
		if (flag & os.O_RDWR) == 0 {
			if flag&3 == os.O_RDONLY {
				if flag&(os.O_CREATE|os.O_TRUNC|os.O_APPEND) == 0 {
					writeAttempt = false
				}
			}
		}
	}
	if writeAttempt {
		return nil, os.ErrPermission
	}

	f, err := rofs.Fs.OpenFile(name, flag, perm)
	if err != nil {
		return nil, err
	}
	return &ReadOnlyFile{F: f}, nil
}

func (rofs *ReadOnlyVFS) Mkdir(path string, perm os.FileMode) error {
	return os.ErrPermission
}

func (rofs *ReadOnlyVFS) Stat(path string) (os.FileInfo, error) {
	return rofs.Fs.Stat(path)
}

func (rofs *ReadOnlyVFS) Rename(from, to string) error {
	return os.ErrPermission
}

func (rofs *ReadOnlyVFS) Remove(path string) error {
	return os.ErrPermission
}

func (rofs *ReadOnlyVFS) Close() error {
	return rofs.Fs.Close()
}

type ReadOnlyFile struct {
	F File
}

func (rof *ReadOnlyFile) Name() string {
	return rof.F.Name()
}

func (rof *ReadOnlyFile) Chmod(mode os.FileMode) error {
	return os.ErrPermission
}

func (rof *ReadOnlyFile) Read(buf []byte) (int, error) {
	return rof.F.Read(buf)
}

func (rof *ReadOnlyFile) ReadAt(buf []byte, offset int64) (int, error) {
	return rof.F.ReadAt(buf, offset)
}

func (rof *ReadOnlyFile) Readdir(n int) ([]os.FileInfo, error) {
	return rof.F.Readdir(n)
}

func (rof *ReadOnlyFile) Readdirnames(n int) ([]string, error) {
	return rof.F.Readdirnames(n)
}

func (rof *ReadOnlyFile) Write(buf []byte) (int, error) {
	return 0, os.ErrPermission
}

func (rof *ReadOnlyFile) WriteAt(buf []byte, off int64) (int, error) {
	return 0, os.ErrPermission
}

func (rof *ReadOnlyFile) Stat() (os.FileInfo, error) {
	return rof.F.Stat()
}

func (rof *ReadOnlyFile) Close() error {
	return rof.F.Close()
}
