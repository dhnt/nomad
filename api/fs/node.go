package fs

import (
	"log"
	"net/url"
	"os"
	"path/filepath"
	"syscall"

	"github.com/dhnt/nomad/api"
)

type FileNode struct {
	Root string
	Dev  uint64

	baseUrl *url.URL
}

func NewFileNode(root string, u *url.URL) (*FileNode, error) {
	var st syscall.Stat_t
	if err := syscall.Stat(root, &st); err != nil {
		return nil, err
	}
	return &FileNode{
		Root:    root,
		Dev:     uint64(st.Dev),
		baseUrl: u,
	}, nil
}

// abs resolves the path
func (n *FileNode) abs(p string) string {
	return filepath.Join(n.Root, p)
}

// rel returns a relative path to the root
func (n *FileNode) rel(p string) (string, error) {
	return filepath.Rel(n.Root, p)
}

func (n *FileNode) Statfs(rel string) (*syscall.Statfs_t, error) {
	path := n.abs(rel)

	st := syscall.Statfs_t{}
	err := syscall.Statfs(path, &st)
	if err != nil {
		return nil, err
	}
	return &st, nil
}

func (n *FileNode) Mknod(rel string, attr *api.Attr) (*syscall.Stat_t, error) {
	if attr == nil || attr.Mode == nil {
		return nil, syscall.EINVAL
	}
	path := n.abs(rel)

	// syscall.S_IFREG
	if !IsRegular(*attr.Mode) {
		// syscall.S_IFCHR | syscall.S_IFBLK | syscall.S_IFIFO | syscall.S_IFSOCK
		return nil, syscall.ENOTSUP
	}
	// mknod requires root privilege
	// err := syscall.Mknod(path, mode, 0)
	_, err := os.Create(path)
	if err != nil {
		return nil, api.ToErrno(err)
	}

	st := syscall.Stat_t{}
	if err := syscall.Lstat(path, &st); err != nil {
		syscall.Rmdir(path)
		return nil, err
	}

	return &st, nil
}

func (n *FileNode) Mkdir(rel string, attr *api.Attr) (*syscall.Stat_t, error) {
	if attr == nil || attr.Mode == nil {
		return nil, syscall.EINVAL
	}
	path := n.abs(rel)

	err := os.Mkdir(path, os.FileMode(*attr.Mode))
	if err != nil {
		return nil, api.ToErrno(err)
	}
	st := syscall.Stat_t{}
	if err := syscall.Lstat(path, &st); err != nil {
		syscall.Rmdir(path)
		return nil, err
	}

	return &st, nil
}

func (n *FileNode) Rmdir(rel string) error {
	path := n.abs(rel)

	err := syscall.Rmdir(path)
	return err
}

func (n *FileNode) Unlink(rel string) error {
	path := n.abs(rel)

	err := syscall.Unlink(path)
	return err
}

func (n *FileNode) Rename(rel1, rel2 string) error {
	from := n.abs(rel1)
	to := n.abs(rel2)

	err := syscall.Rename(from, to)
	return err
}

func (n *FileNode) Symlink(rel string, dir string) (*syscall.Stat_t, error) {
	path := n.abs(rel)
	link := n.abs(dir)

	err := syscall.Symlink(path, link)
	if err != nil {
		return nil, err
	}
	st := syscall.Stat_t{}
	if err := syscall.Lstat(link, &st); err != nil {
		syscall.Unlink(link)
		return nil, err
	}

	return &st, nil
}

func (n *FileNode) Link(rel, dir string) (*syscall.Stat_t, error) {
	path := n.abs(rel)
	link := n.abs(dir)

	err := syscall.Link(path, link)
	if err != nil {
		return nil, err
	}
	st := syscall.Stat_t{}
	if err := syscall.Lstat(link, &st); err != nil {
		syscall.Unlink(link)
		return nil, err
	}

	return &st, nil
}

func (n *FileNode) Readlink(rel string) (string, error) {
	path := n.abs(rel)

	for l := 256; ; l *= 2 {
		buf := make([]byte, l)
		sz, err := syscall.Readlink(path, buf)
		if err != nil {
			return "", err
		}

		if sz < len(buf) {
			return n.rel(string(buf[:sz]))
		}
	}
}

func (n *FileNode) Open(rel string, attr *api.Attr) (*syscall.Stat_t, error) {
	if attr == nil || attr.Mode == nil || attr.Perm == nil {
		return nil, syscall.EINVAL
	}
	path := n.abs(rel)

	writing := *attr.Mode & syscall.O_CREAT
	log.Printf("writing %v", writing)

	fd, err := syscall.Open(path, int(*attr.Mode), *attr.Perm)
	if err != nil {
		return nil, err
	}
	defer syscall.Close(fd)

	st := syscall.Stat_t{}
	if err := syscall.Fstat(fd, &st); err != nil {
		return nil, err
	}

	return &st, nil
}

func (n *FileNode) Create(rel string, attr *api.Attr) (*syscall.Stat_t, error) {
	if attr == nil || attr.Mode == nil || attr.Perm == nil {
		return nil, syscall.EINVAL
	}
	path := n.abs(rel)

	mode := *attr.Mode | syscall.O_CREAT

	fd, err := syscall.Open(path, int(mode), *attr.Perm)
	if err != nil {
		return nil, err
	}
	defer syscall.Close(fd)

	st := syscall.Stat_t{}
	if err := syscall.Fstat(fd, &st); err != nil {
		return nil, err
	}

	return &st, nil
}

func (n *FileNode) Opendir(rel string) error {
	path := n.abs(rel)

	fd, err := syscall.Open(path, syscall.O_DIRECTORY, 0755)
	if err != nil {
		return err
	}
	syscall.Close(fd)
	return nil
}

func (n *FileNode) Readdir(rel string) ([]api.DirEntry, error) {
	path := n.abs(rel)

	ent, err := os.ReadDir(path)
	return api.ToDirEntry(ent), err
}

func (n *FileNode) Lstat(rel string) (*syscall.Stat_t, error) {
	path := n.abs(rel)

	st := syscall.Stat_t{}
	err := syscall.Lstat(path, &st)

	if err != nil {
		return nil, err
	}
	return &st, nil
}

func (n *FileNode) Stat(rel string) (*syscall.Stat_t, error) {
	path := n.abs(rel)

	st := syscall.Stat_t{}
	err := syscall.Stat(path, &st)

	if err != nil {
		return nil, err
	}
	return &st, nil
}

func (n *FileNode) Chmod(rel string, attr *api.Attr) error {
	if attr == nil || attr.Mode == nil {
		return syscall.EINVAL
	}

	path := n.abs(rel)

	if err := syscall.Chmod(path, *attr.Mode); err != nil {
		return err
	}

	return nil
}

func (n *FileNode) Chown(rel string, attr *api.Attr) error {
	if attr == nil || attr.Owner == nil {
		return syscall.EINVAL
	}

	path := n.abs(rel)
	owner := attr.Owner
	if err := syscall.Chown(path, owner.Uid, owner.Gid); err != nil {
		return err
	}

	return nil
}

func (n *FileNode) Truncate(rel string, attr *api.Attr) error {
	if attr == nil || attr.Size == nil {
		return syscall.EINVAL
	}

	path := n.abs(rel)
	if err := syscall.Truncate(path, int64(*attr.Size)); err != nil {
		return err
	}

	return nil
}

func (n *FileNode) Setattr(rel string, attr *api.Attr) (*syscall.Stat_t, error) {
	if attr == nil {
		return nil, syscall.EINVAL
	}
	path := n.abs(rel)

	if attr.Mode != nil {
		if err := syscall.Chmod(path, *attr.Mode); err != nil {
			return nil, err
		}
	}

	if attr.Owner != nil {
		owner := attr.Owner
		if err := syscall.Chown(path, owner.Uid, owner.Gid); err != nil {
			return nil, err
		}
	}

	var ts [2]syscall.Timespec
	ts[0] = UtimeToTimespec(attr.Atime)
	ts[1] = UtimeToTimespec(attr.Mtime)

	if err := syscall.UtimesNano(path, ts[:]); err != nil {
		return nil, err
	}

	if attr.Size != nil {
		if err := syscall.Truncate(path, int64(*attr.Size)); err != nil {
			return nil, err
		}
	}

	st := syscall.Stat_t{}
	err := syscall.Lstat(path, &st)

	if err != nil {
		return nil, err
	}
	return &st, nil
}

func (n *FileNode) Read(rel string, attr *api.Attr) (*api.BlobInfo, error) {
	if attr == nil || attr.Offset == nil || attr.Size == nil {
		return nil, syscall.EINVAL
	}

	path := n.abs(rel)

	st := syscall.Stat_t{}
	err := syscall.Stat(path, &st)
	if err != nil {
		return nil, err
	}

	offset := *attr.Offset

	// disallow reading beyond the size
	if offset > st.Size {
		return nil, syscall.EINVAL
	}

	min := func(x, y int64) int64 {
		if x > y {
			return y
		}
		return x
	}

	allR := uint32(syscall.S_IRUSR | syscall.S_IRGRP | syscall.S_IROTH)
	perm := uint32(st.Mode) & (allR)
	bi := &api.BlobInfo{
		Path:   rel,
		Offset: *attr.Offset,
		Size:   min(*attr.Size, st.Size-offset),
		Perm:   perm,
		Href:   "",
	}
	bi.Href, err = EncodeBlobHref(n.baseUrl, bi)
	if err != nil {
		return nil, err
	}
	return bi, nil
}

func (n *FileNode) Write(rel string, attr *api.Attr) (*api.BlobInfo, error) {
	if attr == nil || attr.Offset == nil || attr.Size == nil {
		return nil, syscall.EINVAL
	}

	path := n.abs(rel)

	st := syscall.Stat_t{}
	err := syscall.Stat(path, &st)
	if err != nil {
		return nil, err
	}

	allW := uint32(syscall.S_IWUSR | syscall.S_IWGRP | syscall.S_IWOTH)
	perm := uint32(st.Mode) & (allW)
	bi := &api.BlobInfo{
		Path:   rel,
		Offset: *attr.Offset,
		Size:   *attr.Size,
		Perm:   perm,
		Href:   "",
	}
	bi.Href, err = EncodeBlobHref(n.baseUrl, bi)
	if err != nil {
		return nil, err
	}
	return bi, nil
}
