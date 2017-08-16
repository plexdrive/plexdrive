use std::fmt;

use cache;

mod ram;
mod thread;
mod request;
mod utils;
mod preload;

pub use chunk::thread::ThreadManager;
pub use chunk::ram::RAMManager;
pub use chunk::request::RequestManager;
pub use chunk::preload::PreloadManager;

#[derive(Debug, Clone)]
pub enum Error {
    RetrievalError(String),
}
pub type ChunkResult<T> = Result<T, Error>;

impl fmt::Display for Error {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        write!(f, "{:?}", self)
    }
}

/// The configuration of a chunk request.
/// It holds all prepared and precalculated offsets
#[derive(Debug, Clone)]
pub struct Config {
    pub file_id: String,
    pub file_size: u64,
    pub id: String,
    pub url: String,
    pub start: u64,
    pub size: u64,
    pub chunk_offset: u64,
    pub offset_start: u64,
    pub offset_end: u64,
}

impl Config {
    pub fn from_request(file: &cache::File, start: u64, size: u64, chunk_size: u64) -> Config {
        let chunk_offset = start % chunk_size;
        let offset_start = start - chunk_offset;
        let offset_end = offset_start + chunk_size;

        Config {
            file_id: file.id.clone(),
            file_size: file.size,
            id: format!("{}:{}", &file.id, &offset_start),
            url: file.download_url.clone(),
            start: start,
            size: size,
            chunk_offset: chunk_offset,
            offset_start: offset_start,
            offset_end: offset_end,
        }
    }
}

/// The Chunk Manager can handle saving and loading chunks
/// it will also buffer chunks in memory or disk or another
/// datasource depending on what manager you're using.
pub trait Manager {
    fn get_chunk<F>(&self, config: &Config, callback: F)
        where F: FnOnce(ChunkResult<Vec<u8>>) + Send + 'static;
}