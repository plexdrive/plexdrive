package main

import (
	"os"

	"fmt"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	. "github.com/claudetech/loggo/default"
	"golang.org/x/net/context"
)

// Mount the fuse volume
func Mount(client *Drive, mountpoint string) error {
	Log.Debugf("Mounting path %v", mountpoint)

	if _, err := os.Stat(mountpoint); os.IsNotExist(err) {
		Log.Debugf("Mountpoint doesn't exist, creating...")
		if err := os.MkdirAll(mountpoint, 0644); nil != err {
			Log.Debugf("%v", err)
			return fmt.Errorf("Could not create mount directory %v", mountpoint)
		}
	}

	fuse.Debug = func(msg interface{}) {
		Log.Tracef("FUSE %v", msg)
	}

	c, err := fuse.Mount(mountpoint)
	if err != nil {
		return err
	}
	defer c.Close()

	object, err := client.GetObject("root")
	if nil != err {
		Log.Debugf("%v", err)
		return fmt.Errorf("Could not get root node")
	}

	filesys := &FS{
		client: client,
		rootID: object.ObjectID,
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
	client *Drive
	rootID string
}

// Root returns the root path
func (f *FS) Root() (fs.Node, error) {
	object, err := f.client.GetObject(f.rootID)
	if nil != err {
		Log.Warningf("%v", err)
		return nil, fmt.Errorf("Could not get root object")
	}
	return &Object{
		client: f.client,
		object: &object,
	}, nil
}

// Object represents one drive object
type Object struct {
	client *Drive
	object *APIObject
	buffer *Buffer
}

// Attr returns the attributes for a directory
func (o *Object) Attr(ctx context.Context, attr *fuse.Attr) error {
	if o.object.IsDir {
		attr.Mode = os.ModeDir | 0755
		attr.Size = 0
	} else {
		attr.Mode = 0644
		attr.Size = o.object.Size
	}

	attr.Mtime = o.object.LastModified
	attr.Crtime = o.object.LastModified
	attr.Ctime = o.object.LastModified

	return nil
}

// ReadDirAll shows all files in the current directory
func (o *Object) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	objects, err := o.client.GetObjectsByParent(o.object.ObjectID)
	if nil != err {
		Log.Warningf("%v", err)
		return nil, err
	}

	dirs := []fuse.Dirent{}
	for _, object := range objects {
		dirs = append(dirs, fuse.Dirent{
			Name: object.Name,
			Type: fuse.DT_File,
		})
	}
	return dirs, nil
}

// Lookup tests if a file is existent in the current directory
func (o *Object) Lookup(ctx context.Context, name string) (fs.Node, error) {
	object, err := o.client.GetObjectByParentAndName(o.object.ObjectID, name)
	if nil != err {
		Log.Debugf("%v", err)
		return nil, err
	}

	return &Object{
		client: o.client,
		object: &object,
	}, nil
}

// Open opens a file for reading
func (o *Object) Open(ctx context.Context, req *fuse.OpenRequest, resp *fuse.OpenResponse) (fs.Handle, error) {
	if req.Dir {
		return o, nil
	}

	buffer, err := o.client.Open(o.object)
	if nil != err {
		Log.Warningf("%v", err)
		return o, fuse.ENOENT
	}
	o.buffer = buffer

	return o, nil
}

// Release a stream
func (o *Object) Release(ctx context.Context, req *fuse.ReleaseRequest) error {
	if nil != o.buffer {
		if err := o.buffer.Close(); nil != err {
			Log.Debugf("%v", err)
			Log.Warningf("Could not close buffer stream")
		}
	}
	return nil
}

// Read reads some bytes or the whole file
func (o *Object) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
	buf, err := o.buffer.ReadBytes(req.Offset, int64(req.Size))
	if nil != err {
		Log.Warningf("%v", err)
		return err
	}

	resp.Data = buf[:]
	return nil
}
