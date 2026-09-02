package apiclient

import (
	"bytes"
	"io"
	"net/http"
)

type recorder struct {
	code   int
	header http.Header
	snap   http.Header
	buf    bytes.Buffer
}

func newRecorder() *recorder { return &recorder{header: http.Header{}} }

func (r *recorder) Header() http.Header { return r.header }

func (r *recorder) WriteHeader(code int) {
	if r.code == 0 {
		r.code = code
		r.snap = r.header.Clone()
	}
}

func (r *recorder) Write(b []byte) (int, error) {
	if r.code == 0 {
		r.code = http.StatusOK
		r.snap = r.header.Clone()
	}
	return r.buf.Write(b)
}

func (r *recorder) Flush() {}

func (r *recorder) result() *http.Response {
	if r.code == 0 {
		r.code = http.StatusOK
	}
	if r.snap == nil {
		r.snap = r.header.Clone()
	}
	body := r.buf.Bytes()
	return &http.Response{
		Status:        http.StatusText(r.code),
		StatusCode:    r.code,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        r.snap,
		Body:          io.NopCloser(bytes.NewReader(body)),
		ContentLength: int64(len(body)),
		Request:       &http.Request{Method: http.MethodGet},
	}
}
