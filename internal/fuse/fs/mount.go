package fs

import (
	"github.com/hanwen/go-fuse/v2/fs"
	"github.com/hanwen/go-fuse/v2/fuse"
)

type Options = fs.Options
type MountOptions = fuse.MountOptions

func Mount(dir string, root fs.InodeEmbedder, options *Options) (*fuse.Server, error) {
	return fs.Mount(dir, root, options)
}
