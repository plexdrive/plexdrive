package mount

import (
	"os"
	"reflect"
	"time"

	"golang.org/x/net/context"

	"fmt"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
	"sh0k.de/plexdrive/config"
	"sh0k.de/plexdrive/plexdrive"
)

// Mount the fuse volume
func Mount(config *config.Config, drive *plexdrive.Drive, mountpoint string) error {
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
		drive: drive,
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
	drive *plexdrive.Drive
}

// Root returns the root path
func (f *FS) Root() (fs.Node, error) {
	rootID, err := f.drive.GetRootID()
	if nil != err {
		return nil, err
	}
	return &Object{
		drive:  f.drive,
		fileID: rootID,
	}, nil
}

// Object represents one drive object
type Object struct {
	drive  *plexdrive.Drive
	fileID string
}

// Attr returns the attributes for a directory
func (o *Object) Attr(ctx context.Context, attr *fuse.Attr) error {
	f, err := o.drive.GetFile(o.fileID)
	if nil != err {
		return err
	}

	if f.MimeType == "application/vnd.google-apps.folder" {
		attr.Mode = os.ModeDir | 0555
		attr.Size = 0
	} else {
		attr.Mode = 0444
		attr.Size = uint64(f.Size)
	}

	mtime, err := time.Parse(time.RFC3339, f.ModifiedTime)
	if nil == err {
		attr.Mtime = mtime
	}
	crtime, err := time.Parse(time.RFC3339, f.CreatedTime)
	if nil == err {
		attr.Crtime = crtime
		attr.Ctime = crtime
	}

	return nil
}

// Lookup tests if a file is existent in the current directory
func (o *Object) Lookup(ctx context.Context, name string) (fs.Node, error) {
	file, err := o.drive.GetFileByNameAndParent(name, o.fileID)
	if nil != err {
		return nil, err
	}

	return &Object{
		drive:  o.drive,
		fileID: file.Id,
	}, nil
}

// ReadDirAll shows all files in the current directory
func (o *Object) ReadDirAll(ctx context.Context) ([]fuse.Dirent, error) {
	dirs := []fuse.Dirent{}
	files, err := o.drive.GetFilesIn(o.fileID)
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

// ReadAll reads the content of a file completely
func (o *Object) ReadAll(ctx context.Context) ([]byte, error) {
	return []byte("abcd"), nil
}

// Read reads a file partially
func (o *Object) Read(ctx context.Context, req *fuse.ReadRequest, resp *fuse.ReadResponse) error {
	return nil
}

func inArray(val interface{}, array interface{}) (exists bool, index int) {
	exists = false
	index = -1

	switch reflect.TypeOf(array).Kind() {
	case reflect.Slice:
		s := reflect.ValueOf(array)

		for i := 0; i < s.Len(); i++ {
			if reflect.DeepEqual(val, s.Index(i).Interface()) == true {
				index = i
				exists = true
				return
			}
		}
	}

	return
}
