package fs

import (
	"context"
	"log"
	"net/url"
	"path/filepath"
	"syscall"

	"github.com/dhnt/nomad/api"
	"github.com/dhnt/nomad/api/cli"

	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

// WebRoot holds the parameters for creating a new web
// filesystem. Web filesystem delegate their operations to an
// underlying POSIX file system on remote machine.
type WebRoot struct {
	// The path to the root of the underlying file system.
	Path string

	// The device on which the Path resides. This must be set if
	// the underlying filesystem crosses file systems.
	Dev uint64

	c *cli.Client

	// NewNode returns a new InodeEmbedder to be used to respond
	// to a LOOKUP/CREATE/MKDIR/MKNOD opcode. If not set, use a
	// WebNode.
	NewNode func(rootData *WebRoot, parent *fs.Inode, name string, st *syscall.Stat_t) fs.InodeEmbedder
}

func (r *WebRoot) newNode(parent *fs.Inode, name string, st *syscall.Stat_t) fs.InodeEmbedder {
	log.Printf("new node %v", name)

	if r.NewNode != nil {
		return r.NewNode(r, parent, name, st)
	}
	return &WebNode{
		RootData: r,
		c:        r.c,
	}
}

func (r *WebRoot) idFromStat(st *syscall.Stat_t) fs.StableAttr {
	// We compose an inode number by the underlying inode, and
	// mixing in the device number. In traditional filesystems,
	// the inode numbers are small. The device numbers are also
	// small (typically 16 bit). Finally, we mask out the root
	// device number of the root, so a web FS that does not
	// encompass multiple mounts will reflect the inode numbers of
	// the underlying filesystem
	swapped := (uint64(st.Dev) << 32) | (uint64(st.Dev) >> 32)
	swappedRootDev := (r.Dev << 32) | (r.Dev >> 32)
	return fs.StableAttr{
		Mode: uint32(st.Mode),
		Gen:  1,
		// This should work well for traditional backing FSes,
		// not so much for other go-fuse FS-es
		Ino: (swapped ^ swappedRootDev) ^ st.Ino,
	}
}

// WebNode is a filesystem node in a web file system. It is
// public so it can be used as a basis for other web based
// filesystems. See NewWebFile or WebRoot for more
// information.
type WebNode struct {
	fs.Inode

	// RootData points back to the root of the web filesystem.
	RootData *WebRoot

	c *cli.Client
}

var _ = (fs.NodeStatfser)((*WebNode)(nil))
var _ = (fs.NodeLookuper)((*WebNode)(nil))
var _ = (fs.NodeMknoder)((*WebNode)(nil))
var _ = (fs.NodeMkdirer)((*WebNode)(nil))
var _ = (fs.NodeRmdirer)((*WebNode)(nil))
var _ = (fs.NodeUnlinker)((*WebNode)(nil))
var _ = (fs.NodeRenamer)((*WebNode)(nil))
var _ = (fs.NodeCreater)((*WebNode)(nil))
var _ = (fs.NodeSymlinker)((*WebNode)(nil))
var _ = (fs.NodeLinker)((*WebNode)(nil))
var _ = (fs.NodeReadlinker)((*WebNode)(nil))
var _ = (fs.NodeOpener)((*WebNode)(nil))
var _ = (fs.NodeOpendirer)((*WebNode)(nil))
var _ = (fs.NodeReaddirer)((*WebNode)(nil))
var _ = (fs.NodeGetattrer)((*WebNode)(nil))
var _ = (fs.NodeSetattrer)((*WebNode)(nil))

func (n *WebNode) Statfs(ctx context.Context, out *fuse.StatfsOut) syscall.Errno {
	s := syscall.Statfs_t{}
	err := n.c.Statfs(n.path(), &s)
	if err != nil {
		return fs.ToErrno(err)
	}

	out.FromStatfsT(&s)
	return fs.OK
}

// path returns the full path to the file in the underlying file
// system.
func (n *WebNode) path() string {
	path := n.Path(n.Root())
	return filepath.Join(n.RootData.Path, path)
}

func (n *WebNode) Lookup(ctx context.Context, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	log.Printf("lookup %v", name)

	p := filepath.Join(n.path(), name)
	st := syscall.Stat_t{}
	err := n.c.Lstat(p, &st)
	if err != nil {
		return nil, fs.ToErrno(err)
	}

	out.Attr.FromStat(&st)
	node := n.RootData.newNode(n.EmbeddedInode(), name, &st)
	ch := n.NewInode(ctx, node, n.RootData.idFromStat(&st))
	return ch, 0
}

func (n *WebNode) Mknod(ctx context.Context, name string, mode, rdev uint32, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	log.Printf("mknod %v", name)

	p := filepath.Join(n.path(), name)
	st := syscall.Stat_t{}
	err := n.c.Mknod(p, mode, &st)
	if err != nil {
		return nil, fs.ToErrno(err)
	}

	out.Attr.FromStat(&st)
	node := n.RootData.newNode(n.EmbeddedInode(), name, &st)
	ch := n.NewInode(ctx, node, n.RootData.idFromStat(&st))

	return ch, 0
}

func (n *WebNode) Mkdir(ctx context.Context, name string, mode uint32, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	p := filepath.Join(n.path(), name)
	st := syscall.Stat_t{}

	err := n.c.Mkdir(p, mode, &st)
	if err != nil {
		return nil, fs.ToErrno(err)
	}

	out.Attr.FromStat(&st)
	node := n.RootData.newNode(n.EmbeddedInode(), name, &st)
	ch := n.NewInode(ctx, node, n.RootData.idFromStat(&st))

	return ch, 0
}

func (n *WebNode) Rmdir(ctx context.Context, name string) syscall.Errno {
	p := filepath.Join(n.path(), name)
	err := n.c.Rmdir(p)
	return fs.ToErrno(err)
}

func (n *WebNode) Unlink(ctx context.Context, name string) syscall.Errno {
	p := filepath.Join(n.path(), name)
	err := n.c.Unlink(p)
	return fs.ToErrno(err)
}

func (n *WebNode) Rename(ctx context.Context, name string, newParent fs.InodeEmbedder, newName string, flags uint32) syscall.Errno {
	p1 := filepath.Join(n.path(), name)
	p2 := filepath.Join(n.RootData.Path, newParent.EmbeddedInode().Path(nil), newName)

	err := n.c.Rename(p1, p2)
	return fs.ToErrno(err)
}

func (n *WebNode) Create(ctx context.Context, name string, mode uint32, perm uint32, out *fuse.EntryOut) (inode *fs.Inode, fh fs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	log.Printf("create %v", name)
	p := filepath.Join(n.path(), name)
	st := syscall.Stat_t{}
	err := n.c.Create(p, mode, perm, &st)
	if err != nil {
		return nil, nil, 0, fs.ToErrno(err)
	}

	node := n.RootData.newNode(n.EmbeddedInode(), name, &st)
	ch := n.NewInode(ctx, node, n.RootData.idFromStat(&st))
	lf := newWebFile(p, n.c)

	out.FromStat(&st)
	return ch, lf, 0, 0
}

func (n *WebNode) Symlink(ctx context.Context, target, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	p := filepath.Join(n.path(), name)
	st := syscall.Stat_t{}

	err := n.c.Symlink(target, p, &st)
	if err != nil {
		return nil, fs.ToErrno(err)
	}

	node := n.RootData.newNode(n.EmbeddedInode(), name, &st)
	ch := n.NewInode(ctx, node, n.RootData.idFromStat(&st))

	out.Attr.FromStat(&st)
	return ch, 0
}

func (n *WebNode) Link(ctx context.Context, target fs.InodeEmbedder, name string, out *fuse.EntryOut) (*fs.Inode, syscall.Errno) {
	p := filepath.Join(n.path(), name)
	st := syscall.Stat_t{}

	err := n.c.Link(filepath.Join(n.RootData.Path, target.EmbeddedInode().Path(nil)), p, &st)
	if err != nil {
		return nil, fs.ToErrno(err)
	}

	node := n.RootData.newNode(n.EmbeddedInode(), name, &st)
	ch := n.NewInode(ctx, node, n.RootData.idFromStat(&st))

	out.Attr.FromStat(&st)
	return ch, 0
}

func (n *WebNode) Readlink(ctx context.Context) ([]byte, syscall.Errno) {
	var link string
	err := n.c.Readlink(n.path(), &link)
	if err != nil {
		return nil, fs.ToErrno(err)
	}
	return []byte(link), 0
}

func (n *WebNode) Open(ctx context.Context, mode uint32) (fh fs.FileHandle, fuseFlags uint32, errno syscall.Errno) {
	log.Println("open")

	p := n.path()
	err := n.c.Open(p, mode, 0, nil)
	if err != nil {
		return nil, 0, fs.ToErrno(err)
	}
	lf := newWebFile(p, n.c)
	return lf, 0, 0
}

func (n *WebNode) Opendir(ctx context.Context) syscall.Errno {
	err := n.c.Opendir(n.path())
	if err != nil {
		return fs.ToErrno(err)
	}
	return fs.OK
}

func (n *WebNode) Readdir(ctx context.Context) (fs.DirStream, syscall.Errno) {
	var result []api.DirEntry
	err := n.c.Readdir(n.path(), &result)
	if err != nil {
		return nil, fs.ToErrno(err)
	}

	entries := make([]fuse.DirEntry, len(result))
	for i, v := range result {
		entries[i] = fuse.DirEntry{
			Mode: uint32(v.Info.Mode),
			Name: v.Name,
			// TODO Ino:,
		}
	}

	return fs.NewListDirStream(entries), fs.OK
}

func (n *WebNode) Getattr(ctx context.Context, f fs.FileHandle, out *fuse.AttrOut) syscall.Errno {
	p := n.path()

	var err error
	st := syscall.Stat_t{}
	if &n.Inode == n.Root() {
		err = n.c.Stat(p, &st)
	} else {
		err = n.c.Lstat(p, &st)
	}

	if err != nil {
		return fs.ToErrno(err)
	}
	out.FromStat(&st)
	return fs.OK
}

func (n *WebNode) Setattr(ctx context.Context, f fs.FileHandle, in *fuse.SetAttrIn, out *fuse.AttrOut) syscall.Errno {
	p := n.path()

	st := syscall.Stat_t{}
	attr := ToAttr(in)
	err := n.c.Setattr(p, attr, &st)
	if err != nil {
		return fs.ToErrno(err)
	}

	out.FromStat(&st)

	return fs.OK
}

// NewWebRoot returns a root node for a web file system whose
// root is at the given root. This node implements all NodeXxxxer
// operations available.
func NewWebRoot(baseUrl string) (fs.InodeEmbedder, error) {
	u, err := url.Parse(baseUrl)
	if err != nil {
		return nil, err
	}
	rootPath := u.Path
	c, err := cli.NewClient(u.String())
	if err != nil {
		return nil, err
	}
	var st syscall.Stat_t
	if err := c.Stat(rootPath, &st); err != nil {
		return nil, err
	}

	root := &WebRoot{
		Path: rootPath,
		Dev:  uint64(st.Dev),
		c:    c,
	}

	return root.newNode(nil, "", &st), nil
}

func ToAttr(in *fuse.SetAttrIn) *api.Attr {
	attr := api.Attr{}

	if v, ok := in.GetMode(); ok {
		attr.Mode = &v
	}

	if v, ok := in.GetSize(); ok {
		iv := int64(v)
		attr.Size = &iv
	}

	if v, ok := in.GetATime(); ok {
		attr.Atime = &v
	}
	if v, ok := in.GetMTime(); ok {
		attr.Mtime = &v
	}
	if v, ok := in.GetCTime(); ok {
		attr.Ctime = &v
	}

	owner := api.Owner{
		Uid: -1,
		Gid: -1,
	}
	if v, ok := in.GetUID(); ok {
		owner.Uid = int(v)
	}
	if v, ok := in.GetGID(); ok {
		owner.Gid = int(v)
	}
	if owner.Uid != -1 || owner.Gid != -1 {
		attr.Owner = &owner
	}

	return &attr
}
