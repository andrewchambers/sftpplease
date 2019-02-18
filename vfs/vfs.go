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
