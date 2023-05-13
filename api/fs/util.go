package fs

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strings"
	"syscall"
	"time"

	"github.com/dhnt/nomad/api"
)

const _UTIME_OMIT = ((1 << 30) - 2)

// UtimeToTimespec converts a "Time" pointer as passed to Utimens to a
// "Timespec" that can be passed to the utimensat syscall.
// A nil pointer is converted to the special UTIME_OMIT value.
func UtimeToTimespec(t *time.Time) (ts syscall.Timespec) {
	if t == nil {
		ts.Nsec = _UTIME_OMIT
	} else {
		ts = syscall.NsecToTimespec(t.UnixNano())
		// Go bug https://github.com/golang/go/issues/12777
		if ts.Nsec < 0 {
			ts.Nsec = 0
		}
	}
	return ts
}

type blobInfoHref struct {
	O int64
	S int64
	M uint32
	P string
}

func EncodeBlobHref(base *url.URL, bi *api.BlobInfo) (string, error) {
	b, err := json.Marshal(blobInfoHref{bi.Offset, bi.Size, bi.Perm, bi.Path})
	if err != nil {
		return "", err
	}
	p := base64.StdEncoding.EncodeToString(b)
	return base.JoinPath("blob", p).String(), nil
}

func DecodeBlobHref(base *url.URL, href string) (*api.BlobInfo, error) {
	u, err := url.Parse(href)
	if err != nil {
		return nil, err
	}
	p := strings.TrimPrefix(strings.TrimPrefix(u.Path, base.Path), "/")
	data, err := base64.StdEncoding.DecodeString(p)
	if err != nil {
		return nil, err
	}
	var bi blobInfoHref
	if err := json.Unmarshal(data, &bi); err != nil {
		return nil, err
	}
	return &api.BlobInfo{
		Offset: bi.O,
		Size:   bi.S,
		Perm:   bi.M,
		Path:   bi.P,
	}, nil
}

func IsRegular(mode uint32) bool {
	return (mode & syscall.S_IFREG) > 0
}

func Create(path string, mode uint32, perm uint32) error {
	//  m := mode|uint32(os.O_CREATE)
	// mode |= os.O_CREATE
	// m := uint32(mode)
	im := int(mode)
	fd, err := syscall.Open(path, im, perm)
	if err != nil {
		return err
	}
	syscall.Close(fd)
	st := syscall.Stat_t{}
	if err := syscall.Fstat(fd, &st); err != nil {
		return err
	}

	return nil
}
