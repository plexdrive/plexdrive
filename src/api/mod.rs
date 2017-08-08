use std::fmt;

mod drive;

pub use api::drive::DriveClient;
use cache;

/// The errors that could occurr during execution
#[derive(Debug)]
pub enum Error {
    Authentication(String),
    MissingDataObject(String),
}
type ClientResult<T> = Result<T, Error>;

impl fmt::Display for Error {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        write!(f, "{:?}", self)
    }
}


pub trait Client {
    /// Authorize the first request with full Google Drive scope and return the username
    /// if the request succeeds
    fn authorize(&self) -> ClientResult<String>;

    /// Watch continuosly asynchronously for changes.
    /// If changes were found they'll get stored in the internal persistence unit
    fn watch_changes<C>(&self, cache: C) where C: cache::MetadataCache + Send + 'static;
}
