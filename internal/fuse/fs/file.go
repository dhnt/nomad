package fs

import (
	"context"
	"log"
	"sync"
	"syscall"

	"github.com/dhnt/nomad/api"
	"github.com/dhnt/nomad/api/cli"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

// newWebFile creates a FileHandle out of a file descriptor.
func newWebFile(fd string, c *cli.Client) fs.FileHandle {
	return &webFile{
		fd: fd,
		c:  c,
	}
}

type webFile struct {
	mu sync.Mutex

	// file path
	fd string
	c  *cli.Client
}

var _ = (fs.FileHandle)((*webFile)(nil))
var _ = (fs.FileReleaser)((*webFile)(nil))
var _ = (fs.FileGetattrer)((*webFile)(nil))
var _ = (fs.FileReader)((*webFile)(nil))
var _ = (fs.FileWriter)((*webFile)(nil))
var _ = (fs.FileGetlker)((*webFile)(nil))
var _ = (fs.FileSetlker)((*webFile)(nil))
var _ = (fs.FileSetlkwer)((*webFile)(nil))
var _ = (fs.FileLseeker)((*webFile)(nil))
var _ = (fs.FileFlusher)((*webFile)(nil))
var _ = (fs.FileFsyncer)((*webFile)(nil))
var _ = (fs.FileSetattrer)((*webFile)(nil))
var _ = (fs.FileAllocater)((*webFile)(nil))

func (f *webFile) Read(ctx context.Context, buf []byte, off int64) (res fuse.ReadResult, errno syscall.Errno) {
	log.Printf("Read %v %v", len(buf), off)

	f.mu.Lock()
	defer f.mu.Unlock()

	var bi api.BlobInfo
	if err := f.c.Read(f.fd, off, int64(len(buf)), &bi); err != nil {
		return nil, fs.ToErrno(err)
	}

	sz := bi.Size
	data := make([]byte, sz)
	n, err := f.c.Download(bi.Href, data)
	if err != nil {
		return nil, fs.ToErrno(err)
	}
	r := fuse.ReadResultData(data[:n])
	return r, fs.OK
}

func (f *webFile) Write(ctx context.Context, data []byte, off int64) (uint32, syscall.Errno) {
	log.Println("Write")

	f.mu.Lock()
	defer f.mu.Unlock()

	var bi api.BlobInfo
	if err := f.c.Write(f.fd, off, int64(len(data)), &bi); err != nil {
		return 0, fs.ToErrno(err)
	}
	n, err := f.c.Upload(bi.Href, data)
	return uint32(n), fs.ToErrno(err)
}

func (f *webFile) Release(ctx context.Context) syscall.Errno {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fd != "" {
		f.fd = ""
		return fs.OK
	}
	return syscall.EBADF
}

func (f *webFile) Flush(ctx context.Context) syscall.Errno {
	// TODO
	log.Println("TODO Flush")
	return fs.OK
}

func (f *webFile) Fsync(ctx context.Context, flags uint32) (errno syscall.Errno) {
	// TODO
	log.Println("TODO Fsync")
	return fs.OK
}

func (f *webFile) Getlk(ctx context.Context, owner uint64, lk *fuse.FileLock, flags uint32, out *fuse.FileLock) (errno syscall.Errno) {
	// return syscall.ENOTSUP
	log.Println("TODO Getlk")
	return fs.OK
}

func (f *webFile) Setlk(ctx context.Context, owner uint64, lk *fuse.FileLock, flags uint32) (errno syscall.Errno) {
	// return syscall.ENOTSUP
	log.Println("TODO Setlk")
	return fs.OK
}

func (f *webFile) Setlkw(ctx context.Context, owner uint64, lk *fuse.FileLock, flags uint32) (errno syscall.Errno) {
	// return syscall.ENOTSUP
	log.Println("TODO Setlkw")
	return fs.OK
}

func (f *webFile) Setattr(ctx context.Context, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {
	st := syscall.Stat_t{}
	attr := ToAttr(in)
	err := f.c.Setattr(f.fd, attr, &st)
	if err != nil {
		return fs.ToErrno(err)
	}

	out.FromStat(&st)
	return fs.OK
}

func (f *webFile) fchmod(mode uint32) syscall.Errno {
	f.mu.Lock()
	defer f.mu.Unlock()

	err := f.c.Chmod(f.fd, mode)
	return fs.ToErrno(err)
}

func (f *webFile) fchown(uid, gid int) syscall.Errno {
	f.mu.Lock()
	defer f.mu.Unlock()

	err := f.c.Chown(f.fd, uid, gid)
	return fs.ToErrno(err)
}

func (f *webFile) ftruncate(sz uint64) syscall.Errno {
	f.mu.Lock()
	defer f.mu.Unlock()

	err := f.c.Truncate(f.fd, int64(sz))
	return fs.ToErrno(err)
}

func (f *webFile) Getattr(ctx context.Context, out *fuse.AttrOut) syscall.Errno {
	f.mu.Lock()
	defer f.mu.Unlock()

	st := syscall.Stat_t{}
	err := f.c.Stat(f.fd, &st)
	if err != nil {
		return fs.ToErrno(err)
	}

	out.FromStat(&st)
	return fs.OK
}

func (f *webFile) Lseek(ctx context.Context, off uint64, whence uint32) (uint64, syscall.Errno) {
	// f.mu.Lock()
	// defer f.mu.Unlock()
	// n, err := unix.Seek(f.fd, int64(off), int(whence))
	// return uint64(n), ToErrno(err)
	log.Println("TODO Lseek")
	return 0, fs.OK
}

func (f *webFile) Allocate(ctx context.Context, off uint64, size uint64, mode uint32) syscall.Errno {
	log.Println("TODO Allocate")
	return fs.OK
}
