package fb

import (
	"errors"
)

var (
	ErrNoUrl      = errors.New("filebin: no url set")
	ErrNoUsername = errors.New("filebin: no username set")
	ErrNoPassword = errors.New("filebin: no password set")

	ErrNoFilename = errors.New("filebin: no filename provided")
	ErrNoIdOrUrl  = errors.New("filebin: no id/url provided")

	ErrUploadDone = errors.New("filebin: upload already done")

	ErrBadRequest          = errors.New("filebin: server refused the uploaded file")
	ErrUnauthorized        = errors.New("filebin: authentication failed. please check credentials/url")
	ErrNotFound            = errors.New("filebin: file not found")
	ErrInternalServerError = errors.New("filebin: internal server error")
)

type Filebin struct {
	Url      string `json:"url" yaml:"url"`
	Username string `json:"username" yaml:"username"`
	Password string `json:"password" yaml:"password"`
}

func (f *Filebin) check() error {
	if f.Url == "" {
		return ErrNoUrl
	}
	if f.Username == "" {
		return ErrNoUsername
	}
	if f.Password == "" {
		return ErrNoPassword
	}
	return nil
}
