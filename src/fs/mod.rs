use std::fmt;
use std::sync::{Arc, Mutex};
use std::ffi;
use time;
use fuse;
use libc;

use cache;

#[derive(Debug)]
pub enum Error {
}
type FilesystemResult<T> = Result<T, Error>;

impl fmt::Display for Error {
  fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
    write!(f, "{:?}", self)
  }
}

#[derive(Debug)]
pub struct Filesystem<C> {
  cache: Arc<Mutex<C>>,
  uid: u32,
  gid: u32,
}

impl<C> Filesystem<C>
where
  C: cache::MetadataCache + Send + 'static,
{
  pub fn new(cache: Arc<Mutex<C>>, uid: u32, gid: u32) -> FilesystemResult<Filesystem<C>> {
    Ok(Filesystem { 
      cache: cache,
      uid: uid,
      gid: gid,
    })
  }
}

impl<C> fuse::Filesystem for Filesystem<C>
where
  C: cache::MetadataCache + Send + 'static,
{
  fn getattr(&mut self, _req: &fuse::Request, inode: u64, reply: fuse::ReplyAttr) {
    let file: cache::File = match self.cache.lock().unwrap().get_file(inode) {
      Ok(file) => file,
      Err(cause) => {
        warn!("{}", cause);

        return reply.error(libc::EIO);
      }
    };

    let time = time::Timespec::new(file.last_modified.timestamp(), 0);

    reply.attr(&time::Timespec::new(60, 0), &fuse::FileAttr{
      ino: file.inode.unwrap(),
      size: file.size,
      blocks: (file.size + 511) / 512,
      atime: time,
      mtime: time,
      ctime: time,
      crtime: time,
      kind: if file.is_dir { fuse::FileType::Directory } else { fuse::FileType::RegularFile },
      perm: 0,
      nlink: 0,
      uid: self.uid,
      gid: self.gid,
      rdev: 0,
      flags: 0,
    })
  }

  fn readdir (&mut self, _req: &fuse::Request, inode: u64, _fh: u64, _offset: u64, mut reply: fuse::ReplyDirectory) {
    let files: Vec<cache::File> = match self.cache.lock().unwrap().get_child_files_by_inode(inode) {
      Ok(files) => files,
      Err(cause) => {
        warn!("{}", cause);

        return reply.error(libc::EIO);
      }
    };

    reply.add(1, 0, fuse::FileType::Directory, ".");
    reply.add(1, 1, fuse::FileType::Directory, "..");

    let mut i = 2;
    for file in files {
      let filetype = if file.is_dir {
        fuse::FileType::Directory
      } else {
        fuse::FileType::RegularFile
      };

      reply.add(file.inode.unwrap(), i, filetype, &file.name);
      i += 1;
    }

    reply.ok()
  }

  fn lookup (&mut self, _req: &fuse::Request, parent_inode: u64, name: &ffi::OsStr, reply: fuse::ReplyEntry) {
    debug!("lookup file {:?}", name);

    let file: cache::File = match self.cache.lock().unwrap().get_child_file_by_inode_and_name(parent_inode, name.to_str().unwrap().to_owned()) {
      Ok(file) => file,
      Err(cause) => {
        debug!("{:?}", cause);

        return reply.error(libc::EIO);
      }
    };

    let time = time::Timespec::new(file.last_modified.timestamp(), 0);

    reply.entry(&time::Timespec::new(60, 0), &fuse::FileAttr{
      ino: file.inode.unwrap(),
      size: file.size,
      blocks: (file.size + 511) / 512,
      atime: time,
      mtime: time,
      ctime: time,
      crtime: time,
      kind: if file.is_dir { fuse::FileType::Directory } else { fuse::FileType::RegularFile },
      perm: 0,
      nlink: 0,
      uid: self.uid,
      gid: self.gid,
      rdev: 0,
      flags: 0,
    }, 0)
  }
}
