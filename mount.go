package main

// import (
// 	"os"

// 	"golang.org/x/net/context"

// 	"fmt"

// 	"log"

// 	"bazil.org/fuse"
// 	"bazil.org/fuse/fs"
// )

// // Mount the fuse volume
// func Mount(config *Config, cache Cache, mountpoint string, debug bool, chunkSize int64) error {
// 	if _, err := os.Stat(mountpoint); os.IsNotExist(err) {
// 		if err := os.MkdirAll(mountpoint, os.ModeDir); nil != err {
// 			return fmt.Errorf("Could not create directory %v", mountpoint)
// 		}
// 	}

// 	if debug {
// 		fuse.Debug = func(msg interface{}) {
// 			log.Printf("FUSE DEBUG %v", msg)
// 		}
// 	}

// 	c, err := fuse.Mount(mountpoint)
// 	if err != nil {
// 		return err
// 	}
// 	defer c.Close()

// 	filesys := &FS{
// 		cache:     cache,
// 		chunkSize: chunkSize,
// 	}
// 	if err := fs.Serve(c, filesys); err != nil {
// 		return err
// 	}

// 	// check if the mount process has an error to report
// 	<-c.Ready
// 	if err := c.MountError; err != nil {
// 		return err
// 	}

// 	return nil
// }

// // FS the fuse filesystem
// type FS struct {
// 	cache     Cache
// 	chunkSize int64
// }

// // Root returns the root path
// func (f *FS) Root() (fs.Node, error) {
// 	rootID, err := f.cache.GetRootID()
// 	if nil != err {
// 		return nil, err
// 	}
// 	return &Object{
// 		cache:     f.cache,
// 		id:        rootID,
// 		chunkSize: f.chunkSize,
// 	}, nil
// }

// // Object represents one drive object
// type Object struct {
// 	cache     Cache
// 	id        string
// 	apiObject *APIObject
// 	buffer    *Buffer
// 	chunkSize int64
// }

// // Attr returns the attributes for a directory
// func (o *Object) Attr(ctx context.Context, attr *fuse.Attr) error {
// 	f, err := o.cache.GetObject(o.id, false)
// 	if nil != err {
// 		return err
// 	}

// 	if f.IsDir {
// 		attr.Mode = os.ModeDir | 0755
// 		attr.Size = 0
// 	} else {
// 		attr.Mode = 0644
// 		attr.Size = f.Size
// 	}

// 	attr.Mtime = f.MTime
// 	attr.Crtime = f.MTime
// 	attr.Ctime = f.MTime

// 	return nil
// }

// // Lookup tests if a file is existent in the current directory
// func (o *Object) Lookup(ctx context.Context, name string) (fs.Node, error) {
// 	file, err := o.cache.GetObjectByNameAndParent(name, o.id)
// 	if nil != err {
// 		return nil, err
// 	}

// 	return &Object{
// 		cache:     o.cache,
// 		id:        file.ID,
// 		apiObject: file,
// 	}, nil
// }

// // ReadDirAll shows all files in the current directory
// func (o *Object) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
// 	dirs := []fuse.Dirent{}
// 	files, err := o.cache.GetObjectsByParent(o.id, false)
// 	if nil != err {
// 		return nil, err
// 	}
// 	for _, file := range files {
// 		dirs = append(dirs, fuse.Dirent{
// 			Name: file.Name,
// 			Type: fuse.DT_File,
// 		})
// 	}
// 	return dirs, nil
// }

// // Open opens a file for reading
// func (o *Object) Open(ctx context.Context, req *fuse.OpenRequest, resp *fuse.OpenResponse) (fs.Handle, error) {
// 	if req.Dir {
// 		return o, nil
// 	}

// 	buffer, err := o.cache.Open(o.apiObject, o.chunkSize)
// 	if nil != err {
// 		log.Printf("Could not open file for reading %v", err)
// 		return o, fuse.ENOENT
// 	}
// 	o.buffer = buffer

// 	return o, nil
// }

// // Release a stream
// func (o *Object) Release(ctx context.Context, req *fuse.ReleaseRequest) error {
// 	if nil != o.buffer {
// 		if err := o.buffer.Close(); nil != err {
// 			log.Printf("Could not close file %v", err)
// 		}
// 	}
// 	return nil
// }

// // Read reads some bytes or the whole file
// func (o *Object) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
// 	buf, err := o.buffer.ReadBytes(req.Offset, int64(req.Size))
// 	if nil != err {
// 		log.Printf("Could not read bytes %v", err)
// 		return err
// 	}

// 	resp.Data = buf[:]
// 	return nil
// }
