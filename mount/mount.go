package mount

import (
	"os"

	"fmt"

	"strings"

	"strconv"

	"time"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	log "github.com/Sirupsen/logrus"
	"github.com/dweidenfeld/plexdrive/chunk"
	"github.com/dweidenfeld/plexdrive/drive"
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

	log.WithField("Mountpath", mountpoint).Infof("Mounting")

	if _, err := os.Stat(mountpoint); os.IsNotExist(err) {
		log.WithField("Mountpath", mountpoint).Debug("Path doesn't exist, creating...")
		if err := os.MkdirAll(mountpoint, 0644); nil != err {
			log.WithField("Mountpath", mountpoint).WithField("Error", err).Debug("Error")
			return fmt.Errorf("Could not create mount directory %v", mountpoint)
		}
	}

	fuse.Debug = func(msg interface{}) {
		log.WithField("Mountpath", mountpoint).WithField("FUSE", true).Debug(msg)
	}

	// Set mount options
	options := []fuse.MountOption{
		fuse.NoAppleDouble(),
		fuse.NoAppleXattr(),
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
				log.WithField("Mountpath", mountpoint).
					WithField("Error", err).
					Debug("Could not parse max_readahead value")
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
		} else if "read_only" == option {
			options = append(options, fuse.ReadOnly())
		} else {
			log.WithField("Mountpath", mountpoint).
				WithField("Option", option).
				Warning("Fuse option is not supported, yet")
		}
	}

	c, err := fuse.Mount(mountpoint, options...)
	if err != nil {
		return err
	}
	defer c.Close()

	filesys := &FS{
		client:       client,
		chunkManager: chunkManager,
		uid:          uid,
		gid:          gid,
		umask:        umask,
	}
	if err := fs.Serve(c, filesys); err != nil {
		return err
	}

	// check if the mount process has an error to report
	<-c.Ready
	if err := c.MountError; nil != err {
		log.WithField("Mountpath", mountpoint).
			WithField("Error", err).
			Debug("Error")
		return fmt.Errorf("Error mounting FUSE")
	}

	return Unmount(mountpoint, true)
}

// Unmount unmounts the mountpoint
func Unmount(mountpoint string, notify bool) error {
	if notify {
		log.WithField("Mountpath", mountpoint).Info("Unmounting path")
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
}

// Root returns the root path
func (f *FS) Root() (fs.Node, error) {
	object, err := f.client.GetRoot()
	if nil != err {
		log.WithField("Error", err).Warning("Could not get root object")
		return nil, fmt.Errorf("Could not get root object")
	}
	return &Object{
		client:       f.client,
		chunkManager: f.chunkManager,
		object:       object,
		uid:          f.uid,
		gid:          f.gid,
		umask:        f.umask,
	}, nil
}

// Object represents one drive object
type Object struct {
	client       *drive.Client
	chunkManager *chunk.Manager
	object       *drive.APIObject
	uid          uint32
	gid          uint32
	umask        os.FileMode
}

// Attr returns the attributes for a directory
func (o *Object) Attr(ctx context.Context, attr *fuse.Attr) error {
	if o.object.IsDir {
		if o.umask > 0 {
			attr.Mode = os.ModeDir | o.umask
		} else {
			attr.Mode = os.ModeDir | 0755
		}
		attr.Size = 0
	} else {
		if o.umask > 0 {
			attr.Mode = o.umask
		} else {
			attr.Mode = 0644
		}
		attr.Size = o.object.Size
	}

	attr.Uid = uint32(o.uid)
	attr.Gid = uint32(o.gid)

	attr.Mtime = o.object.LastModified
	attr.Crtime = o.object.LastModified
	attr.Ctime = o.object.LastModified

	attr.Blocks = (attr.Size + 511) / 512

	return nil
}

// ReadDirAll shows all files in the current directory
func (o *Object) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	objects, err := o.client.GetObjectsByParent(o.object.ObjectID)
	if nil != err {
		log.WithField("ObjectID", o.object.ObjectID).
			WithField("ObjectName", o.object.Name).
			WithField("Error", err).
			Debug("Error")
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
func (o *Object) Lookup(ctx context.Context, name string) (fs.Node, error) {
	object, err := o.client.GetObjectByParentAndName(o.object.ObjectID, name)
	if nil != err {
		log.WithField("ObjectID", o.object.ObjectID).
			WithField("ObjectName", o.object.Name).
			WithField("Error", err).
			Debug("Error")
		return nil, fuse.ENOENT
	}

	return &Object{
		client:       o.client,
		chunkManager: o.chunkManager,
		object:       object,
		uid:          o.uid,
		gid:          o.gid,
		umask:        o.umask,
	}, nil
}

// Read reads some bytes or the whole file
func (o *Object) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
	before := time.Now()
	bytes, err := o.chunkManager.GetChunk(o.object, req.Offset, int64(req.Size))
	log.WithField("ObjectID", o.object.ObjectID).
		WithField("ObjectName", o.object.Name).
		WithField("Took", time.Now().Sub(before)).
		Debug("Get chunk time")
	if nil != err {
		log.WithField("ObjectID", o.object.ObjectID).
			WithField("ObjectName", o.object.Name).
			WithField("Error", err).
			Warning("Error")
		return fuse.EIO
	}

	resp.Data = bytes[:]
	return nil
}

// Remove deletes an element
func (o *Object) Remove(ctx context.Context, req *fuse.RemoveRequest) error {
	obj, err := o.client.GetObjectByParentAndName(o.object.ObjectID, req.Name)
	if nil != err {
		log.WithField("ObjectID", o.object.ObjectID).
			WithField("ObjectName", o.object.Name).
			WithField("Error", err).
			Warning("Error")
		return fuse.EIO
	}

	err = o.client.Remove(obj, o.object.ObjectID)
	if nil != err {
		log.WithField("ObjectID", o.object.ObjectID).
			WithField("ObjectName", o.object.Name).
			WithField("Error", err).
			Warning("Error")
		return fuse.EIO
	}

	return nil
}

// Mkdir creates a new directory
func (o *Object) Mkdir(ctx context.Context, req *fuse.MkdirRequest) (fs.Node, error) {
	newObj, err := o.client.Mkdir(o.object.ObjectID, req.Name)
	if nil != err {
		log.WithField("ObjectID", o.object.ObjectID).
			WithField("ObjectName", o.object.Name).
			WithField("Error", err).
			Warning("Error")
		return nil, fuse.EIO
	}

	return &Object{
		client: o.client,
		object: newObj,
		uid:    o.uid,
		gid:    o.gid,
		umask:  o.umask,
	}, nil
}

// Rename renames an element
func (o *Object) Rename(ctx context.Context, req *fuse.RenameRequest, newDir fs.Node) error {
	obj, err := o.client.GetObjectByParentAndName(o.object.ObjectID, req.OldName)
	if nil != err {
		log.WithField("ObjectID", o.object.ObjectID).
			WithField("ObjectName", o.object.Name).
			WithField("Error", err).
			Warning("Error")
		return fuse.EIO
	}

	destDir, ok := newDir.(*Object)
	if !ok {
		log.WithField("ObjectID", o.object.ObjectID).
			WithField("ObjectName", o.object.Name).
			WithField("Error", err).
			Warning("Error")
		return fuse.EIO
	}

	err = o.client.Rename(obj, o.object.ObjectID, destDir.object.ObjectID, req.NewName)
	if nil != err {
		log.WithField("ObjectID", o.object.ObjectID).
			WithField("ObjectName", o.object.Name).
			WithField("Error", err).
			Warning("Error")
		return fuse.EIO
	}

	return nil
}
