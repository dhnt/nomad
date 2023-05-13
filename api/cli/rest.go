package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"syscall"
	"time"

	"github.com/dhnt/nomad/api"
)

const (
	OK  = syscall.Errno(0)
	ERR = syscall.Errno(0xff)
)

type Client struct {
	base *url.URL

	c *http.Client
}

func NewClient(baseUrl string) (*Client, error) {
	c := http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(r *http.Request, via []*http.Request) error {
			r.URL.Opaque = r.URL.Path
			return nil
		},
	}
	u, err := url.Parse(baseUrl)
	if err != nil {
		return nil, err
	}
	return &Client{
		base: u,
		c:    &c,
	}, nil
}

func (r *Client) Exec(request *api.RunReq, result *api.RunResult) error {
	log.Printf("exec: %v", request)

	u, err := r.base.Parse("/procs/")
	if err != nil {
		return err
	}

	b, err := json.Marshal(request)
	if err != nil {
		return err
	}
	body := bytes.NewReader(b)

	req, err := http.NewRequest("POST", u.String(), body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if !statusIsValid(resp) {
		return errors.New(resp.Status)
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(data, result); err != nil {
		return err
	}

	return nil
}

func (r *Client) Ps(result *[]api.Proc) error {
	log.Printf("ps")

	u, err := r.base.Parse("/procs/")
	if err != nil {
		return err
	}

	resp, err := r.c.Get(u.String())
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if !statusIsValid(resp) {
		return errors.New(resp.Status)
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(data, result); err != nil {
		return err
	}

	return nil
}

func (r *Client) Ps1(id string, result *api.Proc) error {
	log.Printf("ps")

	u, err := r.base.Parse("/procs/" + id)
	if err != nil {
		return err
	}

	resp, err := r.c.Get(u.String())
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return api.ErrorNotFound{
			Status: resp.Status,
		}
	}

	if !statusIsValid(resp) {
		return errors.New(resp.Status)
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(data, result); err != nil {
		return err
	}

	return nil
}

func (r *Client) Kill(id string) error {
	log.Printf("exec: %v", id)

	u, err := r.base.Parse("/procs/" + id)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("DELETE", u.String(), nil)
	if err != nil {
		return err
	}

	resp, err := r.c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if !statusIsValid(resp) {
		return errors.New(resp.Status)
	}

	return nil
}

func (r *Client) fs(call string, args *api.CallArgs, result interface{}) error {
	log.Printf("%v: %v", call, args)
	ref := fmt.Sprintf("/fs/%s", strings.ToLower(call))

	u, err := r.base.Parse(ref)
	if err != nil {
		return err
	}

	b, err := json.Marshal(args)
	if err != nil {
		return err
	}
	blen := len(b)
	body := bytes.NewReader(b)

	req, err := http.NewRequest("POST", u.String(), body)
	if err != nil {
		return err
	}
	if blen >= 0 {
		if blen == 0 {
			// Need this to FORCE the http client to send a
			// Content-Length header for size 0.
			req.TransferEncoding = []string{"identity"}
		}
		req.ContentLength = int64(blen)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if !statusIsValid(resp) {
		return errors.New(resp.Status)
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var cr api.CallResultRaw
	if err := json.Unmarshal(data, &cr); err != nil {
		return err
	}
	if cr.Status != OK {
		return cr.Status
	}

	if !isNilInterface(result) {
		if err := json.Unmarshal(cr.Data, result); err != nil {
			return err
		}
	}
	return nil
}

func (r *Client) Statfs(path string, rc *syscall.Statfs_t) error {
	return r.fs("statfs", &api.CallArgs{
		Path: path,
	}, rc)
}

func (r *Client) Stat(path string, rc *syscall.Stat_t) error {
	return r.fs("stat", &api.CallArgs{
		Path: path,
	}, rc)
}

func (r *Client) Lstat(path string, rc *syscall.Stat_t) error {
	return r.fs("lstat", &api.CallArgs{
		Path: path,
	}, rc)
}

func (r *Client) Setattr(path string, attr *api.Attr, rc *syscall.Stat_t) error {
	return r.fs("setattr", &api.CallArgs{
		Path: path,
		Attr: attr,
	}, rc)
}

func (r *Client) Readdir(path string, rc *[]api.DirEntry) error {
	return r.fs("readdir", &api.CallArgs{
		Path: path,
	}, rc)
}

func (r *Client) Open(path string, mode uint32, perm uint32, rc *syscall.Stat_t) error {
	return r.fs("open", &api.CallArgs{
		Path: path,
		Attr: &api.Attr{
			Mode: &mode,
			Perm: &perm,
		},
	}, rc)
}

func (r *Client) Create(path string, mode uint32, perm uint32, rc *syscall.Stat_t) error {
	return r.fs("create", &api.CallArgs{
		Path: path,
		Attr: &api.Attr{
			Mode: &mode,
			Perm: &perm,
		},
	}, rc)
}

func (r *Client) Opendir(path string) error {
	return r.fs("opendir", &api.CallArgs{
		Path: path,
	}, nil)
}

func (r *Client) Readlink(path string, link *string) error {
	return r.fs("readlink", &api.CallArgs{
		Path: path,
	}, link)
}

func (r *Client) Link(path string, link string, rc *syscall.Stat_t) error {
	return r.fs("link", &api.CallArgs{
		Path: path,
		Link: link,
	}, rc)
}

func (r *Client) Mkdir(path string, mode uint32, rc *syscall.Stat_t) error {
	return r.fs("mkdir", &api.CallArgs{
		Path: path,
		Attr: &api.Attr{
			Mode: &mode,
		},
	}, rc)
}

func (r *Client) Rmdir(path string) error {
	return r.fs("rmdir", &api.CallArgs{
		Path: path,
	}, nil)
}

func (r *Client) Unlink(path string) error {
	return r.fs("unlink", &api.CallArgs{
		Path: path,
	}, nil)
}

func (r *Client) Rename(path string, to string) error {
	return r.fs("rename", &api.CallArgs{
		Path: path,
		To:   to,
	}, nil)
}

func (r *Client) Symlink(path string, link string, rc *syscall.Stat_t) error {
	return r.fs("symlink", &api.CallArgs{
		Path: path,
		Link: link,
	}, rc)
}

func (r *Client) Chmod(path string, mode uint32) error {
	return r.fs("chmod", &api.CallArgs{
		Path: path,
		Attr: &api.Attr{
			Mode: &mode,
		},
	}, nil)
}

func (r *Client) Chown(path string, uid, gid int) error {
	return r.fs("chown", &api.CallArgs{
		Path: path,
		Attr: &api.Attr{
			Owner: &api.Owner{
				Uid: uid,
				Gid: gid,
			},
		},
	}, nil)
}

func (r *Client) Truncate(path string, size int64) error {
	return r.fs("truncate", &api.CallArgs{
		Path: path,
		Attr: &api.Attr{
			Size: &size,
		},
	}, nil)
}

func (r *Client) Read(path string, offset, size int64, info *api.BlobInfo) error {
	return r.fs("read", &api.CallArgs{
		Path: path,
		Attr: &api.Attr{
			Offset: &offset,
			Size:   &size,
		},
	}, info)
}

func (r *Client) Mknod(path string, mode uint32, rc *syscall.Stat_t) error {
	return r.fs("mknod", &api.CallArgs{
		Path: path,
		Attr: &api.Attr{
			Mode: &mode,
		},
	}, rc)
}

func (r *Client) Download(href string, data []byte) (int, error) {
	log.Printf("download: %v len: %v", href, len(data))
	resp, err := r.c.Get(href)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}
	n := copy(data, b)
	return n, nil
}

func (r *Client) Write(path string, offset, size int64, info *api.BlobInfo) error {
	return r.fs("write", &api.CallArgs{
		Path: path,
		Attr: &api.Attr{
			Offset: &offset,
			Size:   &size,
		},
	}, info)
}

func (r *Client) Upload(href string, data []byte) (int, error) {
	log.Printf("upload: %v len: %v", href, len(data))

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("blob", "blob")
	if err != nil {
		return 0, err
	}

	dr := bytes.NewReader(data)
	if _, err := io.Copy(fw, dr); err != nil {
		return 0, err
	}

	w.Close()

	//
	req, err := http.NewRequest("POST", href, &buf)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	resp, err := r.c.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}
	type BlobResult struct {
		N int
	}
	var result BlobResult
	if err := json.Unmarshal(b, &result); err != nil {
		return 0, err
	}

	return result.N, nil
}

func statusIsValid(resp *http.Response) bool {
	return resp.StatusCode/100 == 2
}

// func statusIsRedirect(resp *http.Response) bool {
// 	return resp.StatusCode/100 == 3
// }

func isNilInterface(i interface{}) bool {
	if i == nil {
		return true
	}
	iv := reflect.ValueOf(i)
	if !iv.IsValid() {
		return true
	}
	switch iv.Kind() {
	case reflect.Ptr, reflect.Slice, reflect.Map, reflect.Func, reflect.Interface:
		return iv.IsNil()
	default:
		return false
	}
}
