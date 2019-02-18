package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"strings"

	"github.com/andrewchambers/sftpplease/cmd/sftpplease/scp"
	"github.com/andrewchambers/sftpplease/extraio"
	"github.com/andrewchambers/sftpplease/sftp"
	"github.com/andrewchambers/sftpplease/vfs"
	"github.com/anmitsu/go-shlex"

	_ "github.com/andrewchambers/sftpplease/extradbx/dbxfs"
	_ "github.com/andrewchambers/sftpplease/vfs/local"
)

func parseVFS(s string) (string, string) {
	idx := strings.Index(s, ":")
	if idx == -1 {
		return s, ""
	}
	return s[0:idx], s[idx+1:]
}

func main() {

	log.SetFlags(log.LstdFlags | log.Lshortfile)

	Debug := flag.Bool("debug", false, "enable debug logging")
	ReadOnly := flag.Bool("read-only", false, "only allow read access to the virtual file system")
	MaxFiles := flag.Int("max-files", 64, "maximum number of files allowed to be open concurrently")
	VFS := flag.String("vfs", "", "File system implementation. Valid values are 'local' and 'dropbox:TOKEN' ")

	flag.Parse()

	originalCommand := os.Getenv("SSH_ORIGINAL_COMMAND")

	cmdArgs, err := shlex.Split(originalCommand, true)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error parsing ssh command: %s", err)
		os.Exit(1)
	}

	if len(cmdArgs) == 0 {
		_, _ = fmt.Fprintf(os.Stderr, "expected a command, got none!\n")
		os.Exit(1)
	}

	vfsName, vfsOpts := parseVFS(*VFS)

	fs, err := vfs.Open(vfsName, vfsOpts)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error opening sftpplease vfs: %s", err)
		os.Exit(1)
	}

	if path.Base(cmdArgs[0]) == "sftp-server" {
		opts := &sftp.Options{
			Debug:       *Debug,
			MaxFiles:    *MaxFiles,
			LogFunc:     log.Printf,
			WriteAccess: !*ReadOnly,
		}

		sftp.Serve(opts, fs, &extraio.MergedReadWriteCloser{
			WC: os.Stdout,
			RC: os.Stdin,
		})
	} else if path.Base(cmdArgs[0]) == "scp" {
		if len(cmdArgs) == 1 {
			scp.Main([]string{}, fs)
		} else {
			scp.Main(cmdArgs[1:], fs)
		}
	} else {
		_, _ = fmt.Fprintf(os.Stderr, "unsupported command: %s", originalCommand)
		os.Exit(1)
	}

}
