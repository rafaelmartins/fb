package fb

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
)

type Upload struct {
	buf    *bytes.Buffer
	writer *multipart.Writer
	fb     *Filebin
	done   bool
}

func (f *Filebin) NewUpload() (*Upload, error) {
	if err := f.check(); err != nil {
		return nil, err
	}

	buf := new(bytes.Buffer)
	writer := multipart.NewWriter(buf)

	return &Upload{
		buf:    buf,
		writer: writer,
		fb:     f,
		done:   false,
	}, nil
}

func (u *Upload) AddFile(filename string) error {
	if u.done {
		return ErrUploadDone
	}
	if filename == "" {
		return ErrNoFilename
	}

	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	part, err := u.writer.CreateFormFile("file", filepath.Base(filename))
	if err != nil {
		return err
	}

	_, err = io.Copy(part, f)
	return err
}

func (u *Upload) AddFromReader(filename string, reader io.Reader) error {
	if u.done {
		return ErrUploadDone
	}
	if filename == "" {
		return ErrNoFilename
	}

	part, err := u.writer.CreateFormFile("file", filepath.Base(filename))
	if err != nil {
		return err
	}

	_, err = io.Copy(part, reader)
	return err
}

func (u *Upload) Do(f UploadReportFunc) (string, error) {
	if u.done {
		return "", ErrUploadDone
	}

	if err := u.writer.Close(); err != nil {
		return "", err
	}

	u.done = true

	length := int64(u.buf.Len())
	req, err := http.NewRequest("POST", u.fb.Url, &report{
		read:   0,
		length: length,
		r:      u.buf,
		f:      f,
	})
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", u.writer.FormDataContentType())
	req.ContentLength = length
	req.SetBasicAuth(u.fb.Username, u.fb.Password)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case 200:
	case 400:
		return "", ErrBadRequest
	case 401:
		return "", ErrUnauthorized
	case 500:
		return "", ErrInternalServerError
	default:
		return "", errors.New(resp.Status)
	}

	r, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(r), nil
}
