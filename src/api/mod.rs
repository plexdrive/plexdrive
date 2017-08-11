use std::fmt;
use std::sync::{Arc, Mutex};
use hyper;

mod drive;

pub use api::drive::DriveClient;
use cache;

/// The errors that could occurr during execution
#[derive(Debug)]
pub enum Error {
    Authentication(String),
    MissingDataObject(String),
    FileNotFound(String),
    HttpRequestError(String),
    HttpInvalidStatus(hyper::status::StatusCode, String),
    HttpReadError(String),
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

    /// Get the http client with embedded token credentials for custom requests
    /// against the API
    fn do_http_request(&self, url: &str, start_offset: u64, end_offset: u64) -> ClientResult<Vec<u8>>;

    /// Watch continuosly asynchronously for changes.
    /// If changes were found they'll get stored in the internal persistence unit
    fn watch_changes<C>(&self, cache: Arc<Mutex<C>>) where C: cache::MetadataCache + Send + 'static;

    /// Get a file with a specific ID and transform it
    /// to the cached representation
    fn get_file(&self, id: &str) -> ClientResult<cache::File>;
}
