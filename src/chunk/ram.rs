use chunk;

pub struct RAMManager {

}

impl RAMManager {
    pub fn new() -> chunk::ChunkResult<RAMManager> {
        Ok(RAMManager{})
    }
}

impl chunk::Manager for RAMManager {
    fn get_chunk<F>(&self, url: &str, start: u64, offset: u64, callback: F) where F: FnOnce(chunk::ChunkResult<Vec<u8>>) + Send + 'static {
        callback(Err(chunk::Error::NotImplemented));
    }
}