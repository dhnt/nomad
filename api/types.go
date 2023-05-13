package api

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"syscall"
	"time"
)

type Owner struct {
	Uid int `json:"uid"`
	Gid int `json:"gid"`
}

type Attr struct {
	Mode *uint32 `json:"mode,omitempty"`
	Perm *uint32 `json:"perm,omitempty"`

	Size   *int64 `json:"size,omitempty"`
	Offset *int64 `json:"offset,omitempty"`

	Atime *time.Time `json:"atime,omitempty"`
	Mtime *time.Time `json:"mtime,omitempty"`
	Ctime *time.Time `json:"ctime,omitempty"`

	Owner *Owner `json:"owner,omitempty"`
}

type CallArgs struct {
	// file path or target/from for symlink/rename
	Path string `json:"path,omitempty"`
	Link string `json:"link,omitempty"`
	To   string `json:"to,omitempty"`

	Attr *Attr `json:"attr,omitempty"`
}

type CallResult struct {
	Status syscall.Errno `json:"status"`
	Error  string        `json:"error,omitempty"`
	Data   interface{}   `json:"data,omitempty"`
}

type CallResultRaw struct {
	Status syscall.Errno   `json:"status"`
	Error  string          `json:"error,omitempty"`
	Data   json.RawMessage `json:"data,omitempty"`
}

type FileInfo struct {
	Name    string    `json:"name"`
	Size    int64     `json:"size"`
	Mode    uint32    `json:"mode"`
	ModTime time.Time `json:"modtime"`
	IsDir   bool      `json:"isdir"`
}

type DirEntry struct {
	Name  string   `json:"name"`
	IsDir bool     `json:"isdir"`
	Type  uint32   `json:"type"`
	Info  FileInfo `json:"info"`
}

type BlobInfo struct {
	Path   string `json:"path"`
	Offset int64  `json:"offset"`
	Size   int64  `json:"size"`
	Perm   uint32 `json:"perm"`

	Href string `json:"href"`
}

type RunState int

const (
	Unknown RunState = 0

	Running RunState = 1
	Done    RunState = 2
	Failed  RunState = 3
)

type RunReq = Proc

type Proc struct {
	ID string `json:"id"`

	Command string   `json:"command"`
	Args    []string `json:"args"`
	Dir     string   `json:"dir"`
	Env     []string `json:"env"`

	Background bool `json:"bg"`

	// stdout/stderr redirect
	Outfile string `json:"outfile"`
	Errfile string `json:"errfile"`

	Resolve []string `json:"resolve"`

	Timeout int64 `json:"timeout"`

	Meta map[string]string `json:"meta"`

	//
	Pid   int      `json:"pid"`
	State RunState `json:"state"`

	Status int    `json:"status"`
	Error  string `json:"error,omitempty"`

	Created time.Time `json:"created"`
	Elapsed int64     `json:"elapsed"`

	Cancel context.CancelFunc `json:"-"`
}

type RunResult struct {
	ID string `json:"id"`

	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`

	Background bool   `json:"bg"`
	Outfile    string `json:"outfile,omitempty"`
	Errfile    string `json:"errfile,omitempty"`

	Status int    `json:"status"`
	Error  string `json:"error,omitempty"`

	Stdin  string `json:"stdin,omitempty"`
	Stdout string `json:"stdout,omitempty"`
	Stderr string `json:"stderr,omitempty"`
}

func ToErrno(err error) syscall.Errno {
	switch err {
	case nil:
		return syscall.Errno(0)
	case os.ErrPermission:
		return syscall.EPERM
	case os.ErrExist:
		return syscall.EEXIST
	case os.ErrNotExist:
		return syscall.ENOENT
	case os.ErrInvalid:
		return syscall.EINVAL
	}

	switch t := err.(type) {
	case syscall.Errno:
		return t
	case *os.SyscallError:
		return t.Err.(syscall.Errno)
	case *os.PathError:
		return ToErrno(t.Err)
	case *os.LinkError:
		return ToErrno(t.Err)
	}
	return syscall.ENOSYS
}

func ToFileInfo(info fs.FileInfo) FileInfo {
	if info == nil {
		return FileInfo{}
	}
	return FileInfo{
		info.Name(),
		info.Size(),
		uint32(info.Mode()),
		info.ModTime(),
		info.IsDir(),
	}
}

func ToDirEntry(entries []fs.DirEntry) []DirEntry {
	de := make([]DirEntry, len(entries))
	for i, v := range entries {
		info, _ := v.Info()
		de[i] = DirEntry{
			v.Name(),
			v.IsDir(),
			uint32(v.Type()),
			ToFileInfo(info),
		}
	}
	return de
}
