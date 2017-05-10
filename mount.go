package main

import (
	"os"

	"fmt"

	"strings"

	"strconv"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	. "github.com/claudetech/loggo/default"
	"golang.org/x/net/context"
)

// Mount the fuse volume
func Mount(client *Drive, mountpoint string, mountOptions []string, uid, gid uint32) error {
	Log.Infof("Mounting path %v", mountpoint)

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

	// Set mount options
	options := []fuse.MountOption{
		fuse.NoAppleDouble(),
		fuse.NoAppleXattr(),
		fuse.ReadOnly(),
	}
	for _, option := range mountOptions {
		if "allow_other" == option {
			options = append(options, fuse.AllowOther())
		} else if "allow_root" == option {
			options = append(options, fuse.AllowRoot())
		} else if "allow_dev" == option {
			options = append(options, fuse.AllowDev())
		} else if "allow_non_empty_mount" == option {
			options = append(options, fuse.AllowNonEmptyMount())
		} else if "allow_suid" == option {
			options = append(options, fuse.AllowSUID())
		} else if strings.Contains(option, "max_readahead=") {
			data := strings.Split(option, "=")
			value, err := strconv.ParseUint(data[1], 10, 32)
			if nil != err {
				Log.Debugf("%v", err)
				return fmt.Errorf("Could not parse max_readahead value")
			}
			options = append(options, fuse.MaxReadahead(uint32(value)))
		} else if "default_permissions" == option {
			options = append(options, fuse.DefaultPermissions())
		} else if "excl_create" == option {
			options = append(options, fuse.ExclCreate())
		} else if strings.Contains(option, "fs_name") {
			data := strings.Split(option, "=")
			options = append(options, fuse.FSName(data[1]))
		} else if "local_volume" == option {
			options = append(options, fuse.LocalVolume())
		} else if "writeback_cache" == option {
			options = append(options, fuse.WritebackCache())
		} else if strings.Contains(option, "volume_name") {
			data := strings.Split(option, "=")
			options = append(options, fuse.VolumeName(data[1]))
		} else {
			Log.Warningf("Fuse option %v is not supported, yet", option)
		}
	}

	c, err := fuse.Mount(mountpoint, options...)
	if err != nil {
		return err
	}
	defer c.Close()

	filesys := &FS{
		client: client,
		uid:    uid,
		gid:    gid,
	}
	if err := fs.Serve(c, filesys); err != nil {
		return err
	}

	// check if the mount process has an error to report
	<-c.Ready
	if err := c.MountError; nil != err {
		Log.Debugf("%v", err)
		return fmt.Errorf("Error mounting FUSE")
	}

	return Unmount(mountpoint, true)
}

// Unmount unmounts the mountpoint
func Unmount(mountpoint string, notify bool) error {
	if notify {
		Log.Infof("Unmounting path %v", mountpoint)
	}
	if err := fuse.Unmount(mountpoint); nil != err {
		Log.Debugf("%v", err)
		return fmt.Errorf("Could not unmount %v", mountpoint)
	}
	return nil
}

// FS the fuse filesystem
type FS struct {
	client *Drive
	uid    uint32
	gid    uint32
}

// Root returns the root path
func (f *FS) Root() (fs.Node, error) {
	object, err := f.client.GetRoot()
	if nil != err {
		Log.Warningf("%v", err)
		return nil, fmt.Errorf("Could not get root object")
	}
	return &Object{
		client: f.client,
		object: object,
		uid:    f.uid,
		gid:    f.gid,
	}, nil
}

// Object represents one drive object
type Object struct {
	client *Drive
	object *APIObject
	buffer *Buffer
	uid    uint32
	gid    uint32
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

	attr.Uid = uint32(o.uid)
	attr.Gid = uint32(o.gid)

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
		return nil, fuse.ENOENT
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
		return nil, fuse.ENOENT
	}

	return &Object{
		client: o.client,
		object: object,
		uid:    o.uid,
		gid:    o.gid,
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
	buf, err := o.buffer.ReadBytes(req.Offset, int64(req.Size), false)
	if nil != err {
		Log.Warningf("%v", err)
		return fuse.EIO
	}

	resp.Data = buf[:]
	return nil
}
