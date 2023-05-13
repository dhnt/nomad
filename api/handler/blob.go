package handler

import (
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/dhnt/nomad/api"
	"github.com/dhnt/nomad/api/fs"
)

const maxUploadSize = 100 * 1024 * 1024
const blobFilename = "blob"

var (
	blobRe = regexp.MustCompile(`^\/blob\/(.+)$`)
)

type blobHandler struct {
	prefix string
	root   string
}

func NewBlobHandler(prefix string, root string) *blobHandler {
	return &blobHandler{
		prefix: prefix,
		root:   root,
	}
}

func (h *blobHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("content-type", "application/json")
	switch {
	case r.Method == http.MethodGet && blobRe.MatchString(r.URL.Path):
		h.Download(w, r)
		return
	case r.Method == http.MethodPost && blobRe.MatchString(r.URL.Path):
		h.Upload(w, r)
		return
	default:
		notSupported(w, r, r.URL.Path)
		return
	}
}

func (h *blobHandler) resolvePath(name string) string {
	return filepath.Join(h.root, name)
}

func (h *blobHandler) Download(w http.ResponseWriter, r *http.Request) {
	bi, err := h.readBlobInfo(r)
	if err != nil {
		internalServerError(w, r, err)
	}
	log.Printf("blob info offset: %v size: %v path: %v", bi.Offset, bi.Size, bi.Path)

	p := h.resolvePath(bi.Path)

	f, err := os.Open(p)
	defer f.Close()

	if err != nil {
		notFound(w, r, p)
		return
	}

	buf := make([]byte, 512)
	f.Read(buf)
	contentType := http.DetectContentType(buf)

	s, _ := f.Stat()
	size := strconv.FormatInt(s.Size(), 10)

	//
	w.Header().Set("Content-Type", contentType+";"+filepath.Base(p))
	w.Header().Set("Content-Length", size)

	f.Seek(bi.Offset, 0)

	n, err := io.CopyN(w, f, bi.Size)
	if err != nil {
		internalServerError(w, r, err)
	}

	log.Printf("bytes read: %v", n)
}

func (h *blobHandler) Upload(w http.ResponseWriter, r *http.Request) {
	bi, err := h.readBlobInfo(r)
	if err != nil {
		internalServerError(w, r, err)
	}
	log.Printf("blob info offset: %v size: %v path: %v", bi.Offset, bi.Size, bi.Path)

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		internalServerError(w, r, err)
		return
	}

	file, hl, err := r.FormFile(blobFilename)
	if err != nil {
		internalServerError(w, r, err)
		return
	}
	defer file.Close()

	log.Printf("file name: %v size: %v mime: %v", hl.Filename, hl.Size, hl.Header)

	data, err := ioutil.ReadAll(file)
	if err != nil {
		internalServerError(w, r, err)
		return
	}
	n, err := h.write(bi, data)
	if err != nil {
		internalServerError(w, r, err)
	}

	log.Printf("data read: %v written: %v", len(data), n)
	type BlobResult struct {
		N int
	}
	jsonResponse(w, r, BlobResult{
		N: n,
	})
}

func (h *blobHandler) readBlobInfo(r *http.Request) (*api.BlobInfo, error) {
	base, _ := url.Parse(h.prefix)
	bi, err := fs.DecodeBlobHref(base, r.URL.Path)
	return bi, err
}

func (h *blobHandler) write(bi *api.BlobInfo, data []byte) (int, error) {
	p := h.resolvePath(bi.Path)

	f, err := os.OpenFile(p, os.O_RDWR, 0644)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	if _, err := f.Seek(bi.Offset, 0); err != nil {
		return 0, err
	}
	n, err := f.WriteAt(data, bi.Offset)
	if err != nil {
		return 0, err
	}
	return n, nil
}
