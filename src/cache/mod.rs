use std::fmt;
use google_drive3 as drive3;

mod sql;

pub use cache::sql::SqlCache;

#[derive(Debug)]
pub enum Error {
    OpenError(String),
    StoreError(String),
}
pub type CacheResult<T> = Result<T, Error>;

impl fmt::Display for Error {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        write!(f, "{:?}", self)
    }
}

/// The MetadataCache caches all meta information about Google Files
/// so that you won't hit the API limits that fast.
pub trait MetadataCache {
    /// Initialize the cache
    fn initialize(&self) -> CacheResult<()>;

    /// Stores files in the cache
    fn store_files(&self, files: Vec<File>) -> CacheResult<()>;

    /// Get the cahnge token from cache or returns "1"
    fn get_change_token(&self) -> String;

    /// Stores the change token in cache
    fn store_change_token(&self, token: String) -> CacheResult<()>;
}

/// File is a Google Drive file representation that only contains the
/// necessary fields for the cache.
pub struct File {
  pub name: Option<String>
}

impl From<drive3::File> for File {
    fn from(file: drive3::File) -> File {
        File {
          name: file.name
        }
    }
}