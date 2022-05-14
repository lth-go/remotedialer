package remotedialer

import (
	"bytes"
	"errors"
	"io"
	"sync"
	"time"
)

const (
	MaxBuffer = 1 << 21
)

type readBuffer struct {
	id       int64
	cond     sync.Cond
	deadline time.Time
	buf      bytes.Buffer
	err      error
}

func newReadBuffer(id int64) *readBuffer {
	return &readBuffer{
		id: id,
		cond: sync.Cond{
			L: &sync.Mutex{},
		},
	}
}

func (r *readBuffer) Write(data []byte) error {
	r.cond.L.Lock()
	defer r.cond.L.Unlock()

	if r.err != nil {
		return r.err
	}

	reader := bytes.NewReader(data)

	if n, err := io.Copy(&r.buf, reader); err != nil {
		return err
	} else if n > 0 {
		r.cond.Broadcast()
	}

	return nil
}

func (r *readBuffer) Read(b []byte) (int, error) {
	r.cond.L.Lock()
	defer r.cond.L.Unlock()

	for {
		if r.buf.Len() > 0 {
			n, err := r.buf.Read(b)
			if err != nil {
				// The definition of bytes.Buffer is that this will always return nil because
				// we first checked that bytes.Buffer.Len() > 0. We assume that fact so just assert
				// that here.
				panic("bytes.Buffer returned err=\"" + err.Error() + "\" when buffer length was > 0")
			}
			r.cond.Broadcast()
			return n, nil
		}

		if r.buf.Cap() > MaxBuffer/8 {
			r.buf = bytes.Buffer{}
		}

		if r.err != nil {
			return 0, r.err
		}

		now := time.Now()
		if !r.deadline.IsZero() {
			if now.After(r.deadline) {
				return 0, errors.New("deadline exceeded")
			}
		}

		var t *time.Timer
		if !r.deadline.IsZero() {
			t = time.AfterFunc(r.deadline.Sub(now), func() { r.cond.Broadcast() })
		}
		r.cond.Wait()
		if t != nil {
			t.Stop()
		}
	}
}

func (r *readBuffer) Close(err error) error {
	r.cond.L.Lock()
	defer r.cond.L.Unlock()

	if r.err == nil {
		r.err = err
	}

	r.cond.Broadcast()

	return nil
}
