package extradbx

import (
	"errors"
	"io"

	"github.com/dropbox/dropbox-sdk-go-unofficial/dropbox/files"
)

var ErrCanceled = errors.New("Upload canceled")

type Upload struct {
	curChunkOffset int
	offset         int64
	pipeReader     *io.PipeReader
	pipeWriter     *io.PipeWriter
	errChan        chan error
}

func NewUpload(client files.Client, fpath string) (*Upload, error) {
	u := &Upload{
		errChan: make(chan error, 1),
	}
	u.pipeReader, u.pipeWriter = io.Pipe()
	go doUpload(client, u.pipeReader, u.errChan, fpath)

	return u, nil
}

func doUpload(client files.Client, pipe *io.PipeReader, errChan chan error, fpath string) {
	var sessionId string
	var offset int64

	signalErr := func(err error) {
		pipe.CloseWithError(err)
		errChan <- err
	}

	// Upload in 80 meg chunks, 150 is the dropbox api limit.
	chunkSize := int64(80 * 1024 * 1024)

	for nLoops := 0; ; nLoops++ {
		limitedReader := &io.LimitedReader{R: pipe, N: chunkSize}
		if nLoops == 0 {
			res, err := client.UploadSessionStart(files.NewUploadSessionStartArg(), limitedReader)
			if err != nil {
				signalErr(err)
				return
			}
			sessionId = res.SessionId
		} else {
			appendArg := files.NewUploadSessionAppendArg(files.NewUploadSessionCursor(sessionId, uint64(offset)))
			err := client.UploadSessionAppendV2(appendArg, limitedReader)
			if err != nil {
				signalErr(err)
				return
			}
		}

		offset += (chunkSize - limitedReader.N)
		if limitedReader.N != 0 {
			break
		}
	}

	finishArg := files.NewUploadSessionFinishArg(files.NewUploadSessionCursor(sessionId, uint64(offset)), files.NewCommitInfo(fpath))
	_, err := client.UploadSessionFinish(finishArg, &io.LimitedReader{N: 0})
	if err != nil {
		signalErr(err)
		return
	}

	errChan <- nil
}

func (u *Upload) Write(buf []byte) (int, error) {
	return u.pipeWriter.Write(buf)
}

func (u *Upload) Close() error {
	err := u.pipeWriter.Close()
	if err != nil {
		return err
	}
	return <-u.errChan
}

func (u *Upload) Cancel() error {
	u.pipeWriter.CloseWithError(ErrCanceled)
	return nil
}
