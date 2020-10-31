package mount

import (
	"os"
	"runtime"
	"sync"

	"fmt"

	"strings"

	"strconv"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	. "github.com/claudetech/loggo/default"
	"github.com/okzk/sdnotify"
	"github.com/plexdrive/plexdrive/chunk"
	"github.com/plexdrive/plexdrive/drive"
	"golang.org/x/net/context"
)

// Mount the fuse volume
func Mount(
	client *drive.Client,
	chunkManager *chunk.Manager,
	mountpoint string,
	mountOptions []string,
	uid, gid uint32,
	umask os.FileMode) error {

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
	}
	directIO := false
	maxReadahead := uint32(128 << 10) // 128 KB is the FUSE default
	for _, option := range mountOptions {
		if "allow_other" == option {
			options = append(options, fuse.AllowOther())
		} else if "allow_root" == option {
			return fmt.Errorf("The allow_root mount option is no longer supported")
		} else if "allow_dev" == option {
			options = append(options, fuse.AllowDev())
		} else if "allow_non_empty_mount" == option {
			options = append(options, fuse.AllowNonEmptyMount())
		} else if "allow_suid" == option {
			options = append(options, fuse.AllowSUID())
		} else if strings.HasPrefix(option, "max_readahead=") {
			data := strings.Split(option, "=")
			value, err := strconv.ParseUint(data[1], 10, 32)
			if nil != err {
				Log.Debugf("%v", err)
				return fmt.Errorf("Could not parse max_readahead value")
			}
			maxReadahead = uint32(value)
		} else if "default_permissions" == option {
			options = append(options, fuse.DefaultPermissions())
		} else if "direct_io" == option {
			directIO = true
		} else if "excl_create" == option {
			options = append(options, fuse.ExclCreate())
		} else if strings.HasPrefix(option, "fs_name") {
			data := strings.Split(option, "=")
			options = append(options, fuse.FSName(data[1]))
		} else if "local_volume" == option {
			options = append(options, fuse.LocalVolume())
		} else if "writeback_cache" == option {
			options = append(options, fuse.WritebackCache())
		} else if strings.HasPrefix(option, "volume_name") {
			data := strings.Split(option, "=")
			options = append(options, fuse.VolumeName(data[1]))
		} else if "read_only" == option {
			options = append(options, fuse.ReadOnly())
		} else {
			Log.Warningf("Fuse option %v is not supported, yet", option)
		}
	}
	options = append(options, fuse.MaxReadahead(maxReadahead))

	c, err := fuse.Mount(mountpoint, options...)
	if err != nil {
		return err
	}
	defer c.Close()

	if err := sdnotify.Ready(); err != nil && err != sdnotify.ErrSdNotifyNoSocket {
		Log.Errorf("Failed to notify systemd: %v", err)
	} else {
		Log.Debugf("Notify systemd: ready")
	}

	srv := fs.New(c, nil)

	filesys := &FS{
		client:       client,
		chunkManager: chunkManager,
		uid:          uid,
		gid:          gid,
		umask:        umask,
		directIO:     directIO,
		objectCache:  make(map[string]*drive.APIObject, 0),
	}

	if p := c.Protocol(); p.HasInvalidate() {
		client.NotifyFsChanges = true
		go watchObjectChanges(srv, filesys)
		Log.Debugf("Invalidation watcher started")
	} else {
		Log.Warningf("FUSE version does not support invalidations")
	}

	if err := srv.Serve(filesys); err != nil {
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

func watchObjectChanges(srv *fs.Server, fs *FS) {
	for {
		select {
		case objects, more := <-fs.client.ChangedObjects:
			if !more {
				return
			}
			for _, object := range objects {
				o := Object{fs, object.ObjectID}
				fs.lock.Lock()
				if _, exists := fs.objectCache[o.objectID]; exists {
					fs.objectCache[o.objectID] = object
				}
				fs.lock.Unlock()
				if err := srv.InvalidateNodeData(o); err != nil && err != fuse.ErrNotCached {
					Log.Warningf("Failed to invalidate object %v", object.ObjectID)
				} else {
					Log.Debugf("Invalidated object %v", object.ObjectID)
				}
			}
		}
	}
}

// Unmount unmounts the mountpoint
func Unmount(mountpoint string, notify bool) error {
	if notify {
		Log.Infof("Unmounting path %v", mountpoint)
	}
	if err := sdnotify.Stopping(); nil != err {
		Log.Debugf("Notify systemd: stopping")
	}
	fuse.Unmount(mountpoint)
	return nil
}

// FS the fuse filesystem
type FS struct {
	client       *drive.Client
	chunkManager *chunk.Manager
	uid          uint32
	gid          uint32
	umask        os.FileMode
	directIO     bool
	lock         sync.RWMutex
	objectCache  map[string]*drive.APIObject
}

// NewObject returns a new drive object and caches the api object
func (f *FS) NewObject(object *drive.APIObject) Object {
	o := Object{f, object.ObjectID}
	f.lock.Lock()
	f.objectCache[o.objectID] = object
	f.lock.Unlock()
	runtime.SetFinalizer(&o, f.removeCache)
	return o
}

// removeCache removes an unreferenced api object from the cache
func (f *FS) removeCache(o *Object) {
	f.lock.Lock()
	delete(f.objectCache, o.objectID)
	f.lock.Unlock()
}

// Root returns the root path
func (f *FS) Root() (fs.Node, error) {
	object, err := f.client.GetRoot()
	if nil != err {
		Log.Warningf("%v", err)
		return nil, fmt.Errorf("Could not get root object")
	}
	return f.NewObject(object), nil
}

// Object represents one drive object
type Object struct {
	fs       *FS
	objectID string
}

// GetObject returns the associated api object
func (o Object) GetObject() (object *drive.APIObject, err error) {
	o.fs.lock.RLock()
	object, exists := o.fs.objectCache[o.objectID]
	o.fs.lock.RUnlock()
	if !exists {
		object, err = o.fs.client.GetObject(o.objectID)
		if nil != err {
			return
		}
		o.fs.lock.Lock()
		o.fs.objectCache[o.objectID] = object
		o.fs.lock.Unlock()
	}
	return
}

// Attr returns the attributes for a directory
func (o Object) Attr(ctx context.Context, attr *fuse.Attr) error {
	object, err := o.GetObject()
	if nil != err {
		Log.Errorf("%v", err)
		return fuse.ENOENT
	}
	if object.IsDir {
		if o.fs.umask > 0 {
			attr.Mode = os.ModeDir | o.fs.umask
		} else {
			attr.Mode = os.ModeDir | 0755
		}
		attr.Size = 0
	} else {
		if o.fs.umask > 0 {
			attr.Mode = o.fs.umask
		} else {
			attr.Mode = 0644
		}
		attr.Size = object.Size
	}

	attr.Uid = o.fs.uid
	attr.Gid = o.fs.gid

	attr.Mtime = object.LastModified
	attr.Crtime = object.LastModified
	attr.Ctime = object.LastModified

	attr.Blocks = (attr.Size + 511) >> 9

	return nil
}

// ReadDirAll shows all files in the current directory
func (o Object) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	objects, err := o.fs.client.GetObjectsByParent(o.objectID)
	if nil != err {
		Log.Debugf("%v", err)
		return nil, fuse.ENOENT
	}

	dirs := []fuse.Dirent{}
	for _, object := range objects {
		if object.IsDir {
			dirs = append(dirs, fuse.Dirent{
				Name: object.Name,
				Type: fuse.DT_Dir,
			})
		} else {
			dirs = append(dirs, fuse.Dirent{
				Name: object.Name,
				Type: fuse.DT_File,
			})
		}
	}
	return dirs, nil
}

// Lookup tests if a file is existent in the current directory
func (o Object) Lookup(ctx context.Context, name string) (fs.Node, error) {
	object, err := o.fs.client.GetObjectByParentAndName(o.objectID, name)
	if nil != err {
		Log.Tracef("%v", err)
		return nil, fuse.ENOENT
	}

	return o.fs.NewObject(object), nil
}

// Read reads some bytes or the whole file
func (o Object) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
	object, err := o.GetObject()
	if nil != err {
		Log.Errorf("%v", err)
		return fuse.EIO
	}
	data, err := o.fs.chunkManager.GetChunk(object, req.Offset, int64(req.Size))
	if nil != err {
		Log.Warningf("%v", err)
		return fuse.EIO
	}

	resp.Data = data
	return nil
}

// Open a file
func (o Object) Open(ctx context.Context, req *fuse.OpenRequest, resp *fuse.OpenResponse) (fs.Handle, error) {
	if o.fs.directIO {
		// Force use of Direct I/O, even if the app did not request it (direct_io mount option)
		resp.Flags |= fuse.OpenDirectIO
	}
	if o.fs.client.NotifyFsChanges {
		// We can actively invalidate kernel cache, use more aggressive caching
		resp.Flags |= fuse.OpenKeepCache
	}
	return o, nil
}

// Remove deletes an element
func (o Object) Remove(ctx context.Context, req *fuse.RemoveRequest) error {
	object, err := o.fs.client.GetObjectByParentAndName(o.objectID, req.Name)
	if nil != err {
		Log.Warningf("%v", err)
		return fuse.EIO
	}

	err = o.fs.client.Remove(object, o.objectID)
	if nil != err {
		Log.Warningf("%v", err)
		return fuse.EIO
	}

	return nil
}

// Mkdir creates a new directory
func (o Object) Mkdir(ctx context.Context, req *fuse.MkdirRequest) (fs.Node, error) {
	object, err := o.fs.client.Mkdir(o.objectID, req.Name)
	if nil != err {
		Log.Warningf("%v", err)
		return nil, fuse.EIO
	}

	return o.fs.NewObject(object), nil
}

// Rename renames an element
func (o Object) Rename(ctx context.Context, req *fuse.RenameRequest, newDir fs.Node) error {
	obj, err := o.fs.client.GetObjectByParentAndName(o.objectID, req.OldName)
	if nil != err {
		Log.Warningf("%v", err)
		return fuse.EIO
	}

	destDir, ok := newDir.(Object)
	if !ok {
		Log.Warningf("%v", err)
		return fuse.EIO
	}

	err = o.fs.client.Rename(obj, o.objectID, destDir.objectID, req.NewName)
	if nil != err {
		Log.Warningf("%v", err)
		return fuse.EIO
	}

	return nil
}
