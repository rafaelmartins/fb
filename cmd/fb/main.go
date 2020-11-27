package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/rafaelmartins/fb"
	"github.com/schollz/progressbar/v3"
	"gopkg.in/yaml.v2"
)

var (
	bar  *progressbar.ProgressBar
	fbin *fb.Filebin

	toDelete = flag.Bool("d", false, "delete file by id or url, instead of uploading")
)

func readConfig() (*fb.Filebin, error) {
	u, err := user.Current()
	if err != nil {
		return nil, err
	}

	f, err := os.Open(filepath.Join(u.HomeDir, ".fb.yml"))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	c := &fb.Filebin{}
	if err := yaml.NewDecoder(f).Decode(c); err != nil {
		return nil, err
	}
	return c, nil
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

func handleUrl(up *fb.Upload, u *url.URL) error {
	pieces := strings.Split(u.Path, "/")
	if len(pieces) == 0 {
		return errors.New("invalid url path")
	}
	filename := pieces[len(pieces)-1]
	if filename == "" {
		filename = "-"
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

	return up.AddFromReader(filename, &pbReader{c.Body})
}

func upload(files []string) error {
	up, err := fbin.NewUpload()
	if err != nil {
		return err
	}

	if len(files) == 0 {
		if err := up.AddFromReader("-", os.Stdin); err != nil {
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
				if err := up.AddFromReader("-", os.Stdin); err != nil {
					return err
				}
				continue
			}

			if u, err := url.Parse(file); err == nil && (u.Scheme == "http" || u.Scheme == "https") {
				if err := handleUrl(up, u); err != nil {
					return err
				}
				continue
			}

			if err := up.AddFile(file); err != nil {
				return err
			}
		}
	}

	pbInit := false
	body, err := up.Do(func(read int64, length int64) {
		if !pbInit {
			bar = progressbar.DefaultBytes(length, "uploading  ")
			pbInit = true
		}
		bar.Set64(read)
	})
	fmt.Println()
	if err != nil {
		return err
	}

	fmt.Print(body)
	return nil
}

func del(files []string) error {
	if len(files) == 0 {
		return errors.New("nothing selected")
	}

	if len(files) != 1 {
		return errors.New("only one file can be deleted")
	}

	return fbin.Delete(files[0])
}

func main() {
	flag.Parse()

	var err error
	fbin, err = readConfig()
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
