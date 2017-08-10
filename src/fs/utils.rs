use time;
use fuse;

use cache;

/// Create file attributes based on the cached file
pub fn create_attrs_for_file(file: &cache::File, uid: u32, gid: u32) -> fuse::FileAttr {
    let time = time::Timespec::new(file.last_modified.timestamp(), 0);

    let perm = if file.is_dir {
        0o755
    } else {
        0o644
    };

    fuse::FileAttr{
      ino: file.inode.unwrap(),
      size: file.size,
      blocks: (file.size + 511) / 512,
      atime: time,
      mtime: time,
      ctime: time,
      crtime: time,
      kind: if file.is_dir { fuse::FileType::Directory } else { fuse::FileType::RegularFile },
      perm: perm,
      nlink: 0,
      uid: uid,
      gid: gid,
      rdev: 0,
      flags: 0,
    }
}

/// Get the fuse filetype for a file
pub fn get_filetype_for_file(file: &cache::File) -> fuse::FileType {
    if file.is_dir {
        fuse::FileType::Directory
    } else {
        fuse::FileType::RegularFile
    }
}