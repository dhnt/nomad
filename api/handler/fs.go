package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"syscall"

	"github.com/dhnt/nomad/api"
	"github.com/dhnt/nomad/api/fs"
)

type FileHandler struct {
	prefix string
	node   *fs.FileNode
}

func NewFileHandler(prefix string, root string, baseUrl *url.URL) (*FileHandler, error) {
	node, err := fs.NewFileNode(root, baseUrl)
	if err != nil {
		return nil, err
	}
	return &FileHandler{
		prefix: prefix,
		node:   node,
	}, nil
}

func StatusText(code int) string {
	return http.StatusText(code)
}

var (
	errDestinationEqualsSource = errors.New("destination equals source")
	errDirectoryNotEmpty       = errors.New("directory not empty")
	errNotADirectory           = errors.New("not a directory")
	errPrefixMismatch          = errors.New("prefix mismatch")
	errUnsupportedMethod       = errors.New("unsupported method")
	errNotImplemented          = errors.New("not implemented")
)

type ETager interface {
	// ETag returns an ETag for the file.  This should be of the
	// form "value" or W/"value"
	//
	// If this returns error ErrNotImplemented then the error will
	// be ignored and the base implementation will be used
	// instead.
	ETag(ctx context.Context) (string, error)
}

func findETag(ctx context.Context, fi os.FileInfo) (string, error) {
	if do, ok := fi.(ETager); ok {
		etag, err := do.ETag(ctx)
		if err != errNotImplemented {
			return etag, err
		}
	}
	// The Apache http 2.4 web server by default concatenates the
	// modification time and size of a file. We replicate the heuristic
	// with nanosecond granularity.
	return fmt.Sprintf(`"%x%x"`, fi.ModTime().UnixNano(), fi.Size()), nil
}

func (h *FileHandler) stripPrefix(p string) (string, error) {
	if h.prefix == "" {
		return p, nil
	}
	if r := strings.TrimPrefix(p, h.prefix); len(r) < len(p) {
		return r, nil
	}
	return p, errPrefixMismatch
}

func (h *FileHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "OPTIONS":
		h.handleOptions(w, r)
	case "GET", "HEAD":
		h.handle(w, r)
	case "POST":
		h.handle(w, r)
	case "DELETE":
		h.handle(w, r)
	case "PUT", "PATCH":
		h.handle(w, r)
	default:
		notSupported(w, r, r.URL.Path)
	}
}

func (h *FileHandler) handleOptions(w http.ResponseWriter, r *http.Request) {
	_, err := h.stripPrefix(r.URL.Path)
	if err != nil {
		internalServerError(w, r, err)
		return
	}
	allow := "OPTIONS, GET, HEAD, POST, DELETE, PATCH, PUT"

	w.Header().Set("Allow", allow)
	w.Header().Set("X-Web-FS", "1.0.0")

	return
}

type StatfsOut struct {
	Blocks  uint64
	Bfree   uint64
	Bavail  uint64
	Files   uint64
	Ffree   uint64
	Bsize   uint32
	NameLen uint32
	Frsize  uint32
	Padding uint32
	Spare   [6]uint32
}

func ToStatfsOut(v *syscall.Statfs_t) (*StatfsOut, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var st StatfsOut
	if err := json.Unmarshal(b, &st); err != nil {
		return nil, err
	}

	return &st, nil
}

func (h *FileHandler) handle(w http.ResponseWriter, r *http.Request) {
	call, err := h.stripPrefix(r.URL.Path)
	if err != nil {
		internalServerError(w, r, err)
		return
	}

	var args api.CallArgs

	if err := json.NewDecoder(r.Body).Decode(&args); err != nil {
		internalServerError(w, r, err)
		return
	}

	log.Printf("%s %v", call, args)

	var data interface{}
	var sterr error

	switch call {
	case "statfs":
		v, err := h.node.Statfs(args.Path)
		sterr = err
		if data, err = ToStatfsOut(v); err != nil {
			internalServerError(w, r, err)
			return
		}
	case "lstat":
		data, sterr = h.node.Lstat(args.Path)
	case "stat":
		data, sterr = h.node.Stat(args.Path)
	case "mknod":
		data, sterr = h.node.Mknod(args.Path, args.Attr)
	case "mkdir":
		data, sterr = h.node.Mkdir(args.Path, args.Attr)
	case "rmdir":
		sterr = h.node.Rmdir(args.Path)
		data = ""
	case "unlink":
		sterr = h.node.Unlink(args.Path)
		data = ""
	case "rename":
		sterr = h.node.Rename(args.Path, args.To)
		data = ""
	case "symlink":
		data, sterr = h.node.Symlink(args.Path, args.Link)
	case "link":
		data, sterr = h.node.Link(args.Path, args.Link)
	case "readlink":
		data, sterr = h.node.Readlink(args.Path)
	case "open":
		data, sterr = h.node.Open(args.Path, args.Attr)
	case "create":
		data, sterr = h.node.Create(args.Path, args.Attr)
	case "opendir":
		sterr = h.node.Opendir(args.Path)
		data = ""
	case "readdir":
		data, sterr = h.node.Readdir(args.Path)
	case "chmod":
		sterr = h.node.Chmod(args.Path, args.Attr)
		data = ""
	case "chown":
		sterr = h.node.Chown(args.Path, args.Attr)
		data = ""
	case "truncate":
		sterr = h.node.Truncate(args.Path, args.Attr)
		data = ""
	case "setattr":
		data, sterr = h.node.Setattr(args.Path, args.Attr)
	case "read":
		data, sterr = h.node.Read(args.Path, args.Attr)
	case "write":
		data, sterr = h.node.Write(args.Path, args.Attr)
	default:
		notSupported(w, r, r.URL.Path)
		return
	}

	toMsg := func(e error) string {
		if e == nil {
			return ""
		}
		return e.Error()
	}
	jsonResponse(w, r, api.CallResult{
		Status: api.ToErrno(sterr),
		Error:  toMsg(sterr),
		Data:   data,
	})
}
