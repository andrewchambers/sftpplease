package extraio

import (
	"io"
)

type MergedReadWriteCloser struct {
	RC io.ReadCloser
	WC io.WriteCloser
}

func (m *MergedReadWriteCloser) Read(buf []byte) (int, error) {
	return m.RC.Read(buf)
}

func (m *MergedReadWriteCloser) Write(buf []byte) (int, error) {
	return m.WC.Write(buf)
}

func (m *MergedReadWriteCloser) Close() error {
	err1 := m.RC.Close()
	err2 := m.WC.Close()
	if err1 != nil {
		return err1
	}
	if err2 != nil {
		return err2
	}
	return nil
}
