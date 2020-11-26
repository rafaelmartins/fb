package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/schollz/progressbar/v3"
	"gopkg.in/yaml.v2"
)

var (
	bar  *progressbar.ProgressBar
	conf *config

	toDelete = flag.Bool("d", false, "delete file by id or url, instead of uploading")
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

func handleFile(w *multipart.Writer, file string) error {
	part, err := w.CreateFormFile("file", filepath.Base(file))
	if err != nil {
		return err
	}

	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = io.Copy(part, f)
	return err
}

func handleUrl(w *multipart.Writer, u *url.URL) error {
	up := strings.Split(u.Path, "/")
	if len(up) == 0 {
		return errors.New("invalid url path")
	}
	fn := up[len(up)-1]
	if fn == "" {
		fn = "-"
	}

	part, err := w.CreateFormFile("file", fn)
	if err != nil {
		return err
	}

	c, err := http.Get(u.String())
	if err != nil {
		return err
	}
	defer c.Body.Close()

	length := int64(-1)
	if v := c.Header.Get("content-length"); v != "" {
		if l, err := strconv.ParseInt(v, 10, 64); err == nil {
			length = l
		}
	}

	bar = progressbar.DefaultBytes(length, "downloading")
	_, err = io.Copy(part, &pbReader{c.Body})
	return err
}

func handleStdin(w *multipart.Writer) error {
	part, err := w.CreateFormFile("file", "-")
	if err != nil {
		return err
	}

	_, err = io.Copy(part, os.Stdin)
	return err
}

func upload(files []string) error {
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)

	if len(files) == 0 {
		if err := handleStdin(writer); err != nil {
			return err
		}
	} else {
		withStdin := false
		for _, file := range files {
			if file == "-" {
				if withStdin {
					return errors.New("stdin can't be uploaded more than once")
				}
				withStdin = true
				if err := handleStdin(writer); err != nil {
					return err
				}
				continue
			}

			if u, err := url.Parse(file); err == nil && (u.Scheme == "http" || u.Scheme == "https") {
				if err := handleUrl(writer, u); err != nil {
					return err
				}
				continue
			}

			if err := handleFile(writer, file); err != nil {
				return err
			}
		}
	}

	if err := writer.Close(); err != nil {
		return err
	}

	length := int64(body.Len())
	bar = progressbar.DefaultBytes(length, "uploading  ")
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

func del(files []string) error {
	if len(files) == 0 {
		return errors.New("nothing selected")
	}

	if len(files) != 1 {
		return errors.New("only one file can be deleted")
	}

	bu, err := url.Parse(conf.Url)
	if err != nil {
		return err
	}

	u, err := url.Parse(files[0])
	if err != nil {
		return err
	}

	fu := files[0]

	if u.Scheme == "" && u.Host == "" && !strings.Contains(u.Path, "/") {
		fu = conf.Url
		if !strings.HasSuffix(fu, "/") {
			fu += "/"
		}
		fu += files[0]
	} else if u.Scheme != bu.Scheme || u.Host != bu.Host {
		return fmt.Errorf("invalid url: %s", files[0])
	}

	req, err := http.NewRequest("DELETE", fu, nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(conf.Username, conf.Password)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()

	switch resp.StatusCode {
	case 200:
		fmt.Println("ok")
	case 401:
		return errors.New("authentication failed. please check credentials/url")
	case 404:
		return errors.New("file not found")
	case 500:
		return errors.New("internal server error")
	default:
		return errors.New(resp.Status)
	}

	return nil
}

func main() {
	flag.Parse()

	var err error
	conf, err = readConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	f := upload
	if *toDelete {
		f = del
	}

	if err := f(flag.Args()); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
	}
}
