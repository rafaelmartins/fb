package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"os/user"
	"path/filepath"

	"github.com/schollz/progressbar/v3"
	"gopkg.in/yaml.v2"
)

var (
	bar  *progressbar.ProgressBar
	conf *config
)

type config struct {
	Url      string `yaml:"url"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type pbReader struct {
	buf io.Reader
}

func (pb *pbReader) Read(p []byte) (int, error) {
	n, err := pb.buf.Read(p)
	if bar != nil {
		bar.Add(n)
	}
	return n, err
}

func readConfig() (*config, error) {
	u, err := user.Current()
	if err != nil {
		return nil, err
	}

	f, err := os.Open(filepath.Join(u.HomeDir, ".fb.yml"))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	c := &config{}
	if err := yaml.NewDecoder(f).Decode(c); err != nil {
		return nil, err
	}

	if c.Url == "" {
		return nil, errors.New("config: url not defined")
	}
	if c.Username == "" {
		return nil, errors.New("config: username not defined")
	}
	if c.Password == "" {
		return nil, errors.New("config: password not defined")
	}

	return c, nil
}

func upload(files []string) error {
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)

	if len(files) == 0 {
		part, err := writer.CreateFormFile("file", "-")
		if err != nil {
			return err
		}

		if _, err := io.Copy(part, os.Stdin); err != nil {
			return err
		}
	} else {
		withStdin := false
		for _, file := range files {
			part, err := writer.CreateFormFile("file", filepath.Base(file))
			if err != nil {
				return err
			}

			var f io.Reader
			if file == "-" {
				if withStdin {
					return errors.New("stdin can't be uploaded more than once")
				}
				withStdin = true
				f = os.Stdin
			} else {
				f, err = os.Open(file)
				if err != nil {
					return err
				}
			}

			if _, err := io.Copy(part, f); err != nil {
				return err
			}
		}
	}

	if err := writer.Close(); err != nil {
		return err
	}

	length := int64(body.Len())
	bar = progressbar.DefaultBytes(length, "uploading")
	req, err := http.NewRequest("POST", conf.Url, &pbReader{body})
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.ContentLength = length
	req.SetBasicAuth(conf.Username, conf.Password)

	resp, err := http.DefaultClient.Do(req)
	fmt.Println()
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case 200:
	case 400:
		return errors.New("server refused the uploaded file")
	case 401:
		return errors.New("authentication failed. please check credentials/url")
	case 500:
		return errors.New("internal server error")
	default:
		return errors.New(resp.Status)
	}

	r, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	fmt.Print(string(r))

	return nil
}

func main() {
	var err error
	conf, err = readConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
	}

	if err := upload(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
	}
}
