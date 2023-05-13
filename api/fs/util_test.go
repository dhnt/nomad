package fs

import (
	"net/url"
	"reflect"
	"testing"

	"github.com/dhnt/nomad/api"
)

func TestEncodeBlobHref(t *testing.T) {
	bi := func(off, sz int64, perm uint32, p string) *api.BlobInfo {
		return &api.BlobInfo{
			Path:   p,
			Offset: off,
			Size:   sz,
			Perm:   perm,
		}
	}
	tests := []struct {
		base string
		href string
		bi   *api.BlobInfo
	}{
		{"http://localhost:58080", "http://localhost:58080/blob/eyJPIjoxMDAsIlMiOjY0LCJNIjo1MTEsIlAiOiIvdG1wL2ZpbGUifQ==", bi(100, 64, 0777, "/tmp/file")},
		{"http://localhost:58080/", "http://localhost:58080/blob/eyJPIjoxMDAsIlMiOjY0LCJNIjo1MTEsIlAiOiIvdG1wL2ZpbGUifQ==", bi(100, 64, 0777, "/tmp/file")},
	}

	for _, tc := range tests {
		base, _ := url.Parse(tc.base)
		s, _ := EncodeBlobHref(base, tc.bi)
		if s != tc.href {
			t.FailNow()
		}
		t.Log(s)
	}
}

func TestDecodeBlobHref(t *testing.T) {
	bi := func(off, sz int64, perm uint32, p string) *api.BlobInfo {
		return &api.BlobInfo{
			Path:   p,
			Offset: off,
			Size:   sz,
			Perm:   perm,
		}
	}
	tests := []struct {
		base string
		href string
		bi   *api.BlobInfo
	}{
		{"http://localhost:58080/blob", "http://localhost:58080/blob/eyJPIjoxMDAsIlMiOjY0LCJNIjo1MTEsIlAiOiIvdG1wL2ZpbGUifQ==", bi(100, 64, 0777, "/tmp/file")},
		{"http://localhost:58080/blob/", "http://localhost:58080/blob/eyJPIjoxMDAsIlMiOjY0LCJNIjo1MTEsIlAiOiIvdG1wL2ZpbGUifQ==", bi(100, 64, 0777, "/tmp/file")},
		{"http://localhost:58080/blob/", "/blob/eyJPIjoxMDAsIlMiOjY0LCJNIjo1MTEsIlAiOiIvdG1wL2ZpbGUifQ==", bi(100, 64, 0777, "/tmp/file")},
		{"http://localhost:58080/blob", "/blob/eyJPIjoxMDAsIlMiOjY0LCJNIjo1MTEsIlAiOiIvdG1wL2ZpbGUifQ==", bi(100, 64, 0777, "/tmp/file")},
	}

	for _, tc := range tests {
		base, _ := url.Parse(tc.base)
		bi, _ := DecodeBlobHref(base, tc.href)
		if !reflect.DeepEqual(tc.bi, bi) {
			t.FailNow()
		}
		t.Log(bi)
	}
}

func TestIsRegular(t *testing.T) {
	tests := []struct {
		mode     uint32
		expected bool
	}{
		{0x81a4, true},
		{33188, true},
		{32768, true},
		{0xffff0fff, false},
		{0xffff8fff, true},
		{0x71a4, false},
	}

	for _, tc := range tests {
		b := IsRegular(tc.mode)
		if b != tc.expected {
			t.FailNow()
		}
	}
}

// func TestCreate(t *testing.T) {
// 	tests := []struct {
// 		path string
// 		mode uint32
// 		perm uint32
// 	}{
// 		{"/tmp/test/xxx/a", 0x8841, 0x8841},
// 	}

// 	for _, tc := range tests {
// 		err := Create(tc.path, tc.mode|uint32(os.O_CREATE), tc.perm)
// 		if err != nil {
// 			t.FailNow()
// 		}
// 	}
// }
