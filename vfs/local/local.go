package local

import (
	"os"

	"github.com/andrewchambers/sftpplease/vfs"
)

func init() {
	vfs.RegisterEngine("local", vfsFactory)
}

func vfsFactory(token string) (vfs.VFS, error) {
	return &Fs{}, nil
}

type Fs struct {
}

func (fs *Fs) Chmod(fpath string, mode os.FileMode) error {
	return os.Chmod(fpath, mode)
}

func (fs *Fs) Open(fpath string) (vfs.File, error) {
	return os.Open(fpath)
}

func (fs *Fs) OpenFile(fpath string, flags int, perm os.FileMode) (vfs.File, error) {
	return os.OpenFile(fpath, flags, perm)
}

func (fs *Fs) Mkdir(fpath string, mode os.FileMode) error {
	return os.Mkdir(fpath, mode)
}

func (fs *Fs) Stat(fpath string) (os.FileInfo, error) {
	return os.Stat(fpath)
}

func (fs *Fs) Rename(from, to string) error {
	return os.Rename(from, to)
}

func (fs *Fs) Remove(fpath string) error {
	return os.Remove(fpath)
}

func (fs *Fs) Close() error {
	return nil
}
