package main

import (
	"os"

	"golang.org/x/net/context"

	"fmt"

	"io"

	"log"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
)

// Mount the fuse volume
func Mount(config *Config, cache Cache, mountpoint string, debug bool) error {
	if _, err := os.Stat(mountpoint); os.IsNotExist(err) {
		if err := os.MkdirAll(mountpoint, os.ModeDir); nil != err {
			return fmt.Errorf("Could not create directory %v", mountpoint)
		}
	}

	if debug {
		fuse.Debug = func(msg interface{}) {
			log.Printf("FUSE DEBUG %v", msg)
		}
	}

	c, err := fuse.Mount(mountpoint)
	if err != nil {
		return err
	}
	defer c.Close()

	filesys := &FS{
		cache:     cache,
		blockSize: 500 * 1024 * 1024,
	}
	if err := fs.Serve(c, filesys); err != nil {
		return err
	}

	// check if the mount process has an error to report
	<-c.Ready
	if err := c.MountError; err != nil {
		return err
	}

	return nil
}

// FS the fuse filesystem
type FS struct {
	cache     Cache
	blockSize uint32
}

// Root returns the root path
func (f *FS) Root() (fs.Node, error) {
	rootID, err := f.cache.GetRootID()
	if nil != err {
		return nil, err
	}
	return &Object{
		cache:  f.cache,
		fileID: rootID,
	}, nil
}

// Statfs returns the filesystem stats such as block size
func (f *FS) Statfs(ctx context.Context, req *fuse.StatfsRequest, resp *fuse.StatfsResponse) error {
	resp.Bsize = f.blockSize
	resp.Bfree = 999 * 1024 * 1024 * 1024 * 1024
	resp.Bavail = resp.Bfree
	return nil
}

// Object represents one drive object
type Object struct {
	cache  Cache
	fileID string
	buffer *Buffer
}

// Attr returns the attributes for a directory
func (o *Object) Attr(ctx context.Context, attr *fuse.Attr) error {
	f, err := o.cache.GetObject(o.fileID)
	if nil != err {
		return err
	}

	if f.IsDir {
		attr.Mode = os.ModeDir | 0755
		attr.Size = 0
	} else {
		attr.Mode = 0644
		attr.Size = f.Size
	}

	attr.Mtime = f.MTime
	attr.Crtime = f.MTime
	attr.Ctime = f.MTime

	return nil
}

// Lookup tests if a file is existent in the current directory
func (o *Object) Lookup(ctx context.Context, name string) (fs.Node, error) {
	file, err := o.cache.GetObjectByNameAndParent(name, o.fileID)
	if nil != err {
		return nil, err
	}

	return &Object{
		cache:  o.cache,
		fileID: file.ID,
	}, nil
}

// ReadDirAll shows all files in the current directory
func (o *Object) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	dirs := []fuse.Dirent{}
	files, err := o.cache.GetObjectsByParent(o.fileID)
	if nil != err {
		return nil, err
	}
	for _, file := range files {
		dirs = append(dirs, fuse.Dirent{
			Name: file.Name,
			Type: fuse.DT_File,
		})
	}
	return dirs, nil
}

// Release releases the file
func (o *Object) Release(ctx context.Context, req *fuse.ReleaseRequest) error {
	o.cache.Release(o.fileID)
	if nil != o.buffer {
		if err := o.buffer.Close(); nil != err {
			return err
		}
		o.buffer = nil
	}
	return nil
}

// Read reads some bytes or the whole file
func (o *Object) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
	if nil == o.buffer {
		o.cache.Open(o.fileID)
		handle, err := o.cache.Download(o.fileID)
		if nil != err {
			return err
		}
		o.buffer = handle
	}

	buf := make([]byte, req.Size)
	n, err := io.ReadAtLeast(o.buffer, buf, req.Size)
	if err == io.ErrUnexpectedEOF || err == io.EOF {
		err = nil
	}
	resp.Data = buf[:n]

	return nil
}
