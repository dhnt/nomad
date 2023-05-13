package server

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
)

var (
	volumeRe = regexp.MustCompile(`^\/volumes\/(.+)$`)
)

type VolHandler struct {
	root string
}

func NewVolHandler(root string) *VolHandler {
	return &VolHandler{
		root: root,
	}
}

func (h *VolHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("content-type", "application/json")
	switch {
	case r.Method == http.MethodGet && volumeRe.MatchString(r.URL.Path):
		h.Download(w, r)
		return
	default:
		notFound(w, r, r.URL.Path)
		return
	}
}

func (h *VolHandler) resolvePath(name string) string {
	return filepath.Join(h.root, name)
}

func (h *VolHandler) Download(w http.ResponseWriter, r *http.Request) {
	matches := volumeRe.FindStringSubmatch(r.URL.Path)
	if len(matches) < 2 {
		notFound(w, r, r.URL.Path)
		return
	}
	pathname := h.resolvePath(matches[1])
	f, err := os.Open(pathname)
	defer f.Close()

	if err != nil {
		http.Error(w, "File not found.", 404)
		return
	}

	buf := make([]byte, 512)
	f.Read(buf)
	contentType := http.DetectContentType(buf)

	s, _ := f.Stat()
	size := strconv.FormatInt(s.Size(), 10)

	//
	w.Header().Set("Content-Type", contentType+";"+filepath.Base(pathname))
	w.Header().Set("Content-Length", size)

	f.Seek(0, 0)
	io.Copy(w, f)
}
