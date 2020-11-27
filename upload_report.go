package fb

import (
	"io"
)

type UploadReportFunc func(read int64, length int64)

type report struct {
	read   int64
	length int64
	r      io.Reader
	f      UploadReportFunc
}

func (r *report) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	if r.f != nil {
		r.read += int64(n)
		r.f(r.read, r.length)
	}
	return n, err
}
