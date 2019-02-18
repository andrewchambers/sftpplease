package scp

import (
	"io"
	"time"
)

type BwStats struct {
	Last   time.Time /* time of last observed event */
	Wnd    uint      /* unmetered bytes */
	Thresh uint      /* delay after at least this much bytes */
	Rate   uint      /* bandwidth limit in bits/second */
}

func NewBwStats(rate uint) *BwStats {
	return &BwStats{Wnd: 0, Thresh: rate, Rate: rate}
}

func CapReader(r io.Reader, st *BwStats) io.Reader {
	if st == nil {
		panic("nil stats")
	}
	return &BwCapReader{r, st}
}

func CapWriter(w io.Writer, st *BwStats) io.Writer {
	if st == nil {
		panic("nil stats")
	}
	return &BwCapWriter{w, st}
}

type BwCapReader struct {
	Base  io.Reader
	Stats *BwStats
}

func (r *BwCapReader) Read(p []byte) (int, error) {
	n, err := r.Base.Read(p)
	bwCap(r.Stats, n)
	return n, err
}

type BwCapWriter struct {
	Base  io.Writer
	Stats *BwStats
}

func (w *BwCapWriter) Write(p []byte) (int, error) {
	n, err := w.Base.Write(p)
	bwCap(w.Stats, n)
	return n, err
}

func bwCap(st *BwStats, transfered int) {
	if transfered <= 0 {
		return
	}
	if st.Last.IsZero() {
		st.Last = time.Now()
		return
	}
	st.Wnd += uint(transfered)
	if st.Wnd < st.Thresh {
		return
	}

	bits := st.Wnd * 8
	exp := time.Duration((1e9 * bits) / st.Rate)
	ahead := exp - time.Since(st.Last)

	if ahead > 0 {
		if ahead.Seconds() > 1 {
			st.Thresh /= 2
		} else if ahead < 10*time.Millisecond {
			st.Thresh *= 2
		}
		time.Sleep(ahead)
	}

	st.Wnd = 0
	st.Last = time.Now()
}
