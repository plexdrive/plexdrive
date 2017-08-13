use std::fmt;

use cache;

mod ram;
mod thread;
mod request;

pub use chunk::thread::ThreadManager;
pub use chunk::ram::RAMManager;
pub use chunk::request::RequestManager;

#[derive(Debug)]
pub enum Error {
    RetrievalError(String),
}
pub type ChunkResult<T> = Result<T, Error>;

impl fmt::Display for Error {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        write!(f, "{:?}", self)
    }
}

#[derive(Debug, Clone)]
pub struct Config { // TODO: Rename to ChunkConfig / create ManagerConfig
    pub url: String,
    pub start: u64,
    pub size: u64,
    // TODO: file offset
}

impl Config {
    pub fn from_request(file: &cache::File, start: u64, size: u64) -> Config {
        Config {
            url: file.download_url.clone(),
            start: start,
            size: size,
        }
    }
}

/// The Chunk Manager can handle saving and loading chunks
/// it will also buffer chunks in memory or disk or another
/// datasource depending on what manager you're using.
pub trait Manager {
    fn get_chunk<F>(&self, config: Config, callback: F)
        where F: FnOnce(ChunkResult<Vec<u8>>) + Send + 'static;
}