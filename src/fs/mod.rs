use std::fmt;
use std::sync::{Arc, Mutex};
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
  fn getattr(&mut self, req: &fuse::Request, inode: u64, reply: fuse::ReplyAttr) {
    debug!("Locking cache");
    let cache = self.cache.lock().unwrap();
    debug!("Cache locked");

    let file = match cache.get_file(inode) {
      Ok(file) => file,
      Err(cause) => {
        return warn!("{}", cause);
      }
    };

    debug!("getattr: {:?} / {:?} / {:?}", req, inode, file);

    reply.error(libc::ENOSYS);
  }
}
