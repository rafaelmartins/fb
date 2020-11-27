package fb

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

func (f *Filebin) Delete(idOrUrl string) error {
	if err := f.check(); err != nil {
		return err
	}

	if idOrUrl == "" {
		return ErrNoIdOrUrl
	}

	bu, err := url.Parse(f.Url)
	if err != nil {
		return err
	}

	u, err := url.Parse(idOrUrl)
	if err != nil {
		return err
	}

	fu := idOrUrl

	if u.Scheme == "" && u.Host == "" && !strings.Contains(u.Path, "/") {
		fu = f.Url
		if !strings.HasSuffix(fu, "/") {
			fu += "/"
		}
		fu += idOrUrl
	} else if u.Scheme != bu.Scheme || u.Host != bu.Host {
		return fmt.Errorf("filebin: invalid url: %s", idOrUrl)
	}

	req, err := http.NewRequest("DELETE", fu, nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(f.Username, f.Password)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()

	switch resp.StatusCode {
	case 200:
	case 401:
		return ErrUnauthorized
	case 404:
		return ErrNotFound
	case 500:
		return ErrInternalServerError
	default:
		return errors.New(resp.Status)
	}

	return nil
}
