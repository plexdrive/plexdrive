use std::fmt;

mod ram;
mod thread;

pub use chunk::thread::ThreadManager;
pub use chunk::ram::RAMManager;

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

/// The Chunk Manager can handle saving and loading chunks
/// it will also buffer chunks in memory or disk or another
/// datasource depending on what manager you're using.
pub trait Manager {
    fn get_chunk<F>(&self, url: &str, start: u64, offset: u64, callback: F)
        where F: FnOnce(ChunkResult<Vec<u8>>) + Send + 'static;
}