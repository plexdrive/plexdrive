use std::fmt;
use std::sync::{Arc, Mutex};
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
}

impl<C> Filesystem<C>
where
  C: cache::MetadataCache + Send + 'static,
{
  pub fn new(cache: Arc<Mutex<C>>) -> FilesystemResult<Filesystem<C>> {
    Ok(Filesystem { cache: cache })
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

    reply.attr(&time::Timespec::new(1, 0), &fuse::FileAttr{
      ino: file.inode.unwrap_or(0),
      size: file.size,
      blocks: file.size,
      atime: time,
      mtime: time,
      ctime: time,
      crtime: time,
      kind: if file.is_dir { fuse::FileType::Directory } else { fuse::FileType::RegularFile },
      perm: 0,
      nlink: 0,
      uid: 0,
      gid: 0,
      rdev: 0,
      flags: 0,
    });
  }
}
