package extradbx

import (
	"crypto/rand"
	"crypto/sha256"
	"io"
	"os"
	"reflect"
	"testing"

	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox"
	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox/files"
)

func TestUpload(t *testing.T) {

	token := os.Getenv("SFTPPLEASE_TESTDROPBOXTOKEN")

	dbxCfg := dropbox.Config{
		Token: token,
	}

	api := files.New(dbxCfg)

	fpath := "/sftpplease_upload_testfile.bin"

	_, _ = api.DeleteV2(files.NewDeleteArg(fpath))
	defer api.DeleteV2(files.NewDeleteArg(fpath))

	// Upload upload different segment sizes.
	for _, nBytes := range []int64{0, 100, 40 * 1024 * 1024, 200 * 1024 * 1024} {
		t.Logf("testing upload of %d bytes", nBytes)

		writer, err := NewUpload(files.New(dbxCfg), fpath)
		if err != nil {
			t.Fatal(err)
		}

		sum1 := sha256.New()

		n, err := io.Copy(sum1, io.TeeReader(io.LimitReader(rand.Reader, nBytes), writer))
		if err != nil {
			t.Fatal(err)
		}
		if n != nBytes {
			t.Fatal("uploaded incorrect number of bytes")
		}

		err = writer.Close()
		if err != nil {
			t.Fatal(err)
		}

		sum2 := sha256.New()
		_, contents, err := api.Download(files.NewDownloadArg(fpath))
		if err != nil {
			t.Fatal(err)
		}

		n, err = io.Copy(sum2, contents)
		if err != nil {
			t.Fatal(err)
		}
		if n != nBytes {
			t.Fatal("downloaded incorrect number of bytes")
		}

		err = contents.Close()
		if err != nil {
			t.Fatal(err)
		}

		if !reflect.DeepEqual(sum1.Sum(nil), sum2.Sum(nil)) {
			t.Fatal("file contents of upload and download differ")
		}

		_, err = api.DeleteV2(files.NewDeleteArg(fpath))
		if err != nil {
			t.Fatal(err)
		}
	}
}
