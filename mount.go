package main

import (
	"os"

	"golang.org/x/net/context"

	"fmt"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
)

// Mount the fuse volume
func Mount(config *Config, cache Cache, mountpoint string) error {
	if _, err := os.Stat(mountpoint); os.IsNotExist(err) {
		if err := os.MkdirAll(mountpoint, os.ModeDir); nil != err {
			return fmt.Errorf("Could not create directory %v", mountpoint)
		}
	}

	c, err := fuse.Mount(mountpoint)
	if err != nil {
		return err
	}
	defer c.Close()

	filesys := &FS{
		cache: cache,
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
	cache Cache
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

// Object represents one drive object
type Object struct {
	cache  Cache
	fileID string
}

// Attr returns the attributes for a directory
func (o *Object) Attr(ctx context.Context, attr *fuse.Attr) error {
	f, err := o.cache.GetObject(o.fileID)
	if nil != err {
		return err
	}

	if f.IsDir {
		attr.Mode = os.ModeDir | 0555
		attr.Size = 0
	} else {
		attr.Mode = 0444
		attr.Size = uint64(f.Size)
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

// Open opens a file handle
// func (o *Object) Open(ctx context.Context, req *fuse.OpenRequest, resp *fuse.OpenResponse) (fs.Handle, error) {
// 	handle, err := o.cache.Download(o.fileID)
// 	if nil != err {
// 		return nil, err
// 	}

// 	resp.Flags |= fuse.OpenNonSeekable

// 	return &FileHandle{
// 		handle: handle,
// 	}, nil
// }

// // FileHandle handles the open files
// type FileHandle struct {
// 	handle io.ReadCloser
// }

// // Release releases the file
// func (h *FileHandle) Release(ctx context.Context, req *fuse.ReleaseRequest) error {
// 	return h.handle.Close()
// }

// // Read reads some bytes or the whole file
// func (h *FileHandle) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
// 	buf := make([]byte, req.Size)
// 	n, err := io.ReadFull(h.handle, buf)
// 	if err == io.ErrUnexpectedEOF || err == io.EOF {
// 		err = nil
// 	}
// 	resp.Data = buf[:n]
// 	return err
// }
