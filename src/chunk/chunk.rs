use chunk;

pub struct ChunkManager<M> {
    manager: M,
    chunk_size: u64,
}

impl<M> ChunkManager<M> where M: chunk::Manager {
    pub fn new(manager: M, chunk_size: u64) -> chunk::ChunkResult<ChunkManager<M>> {
        Ok(ChunkManager{
            manager: manager,
            chunk_size: chunk_size,
        })
    }
}

impl<M> chunk::Manager for ChunkManager<M> where M: chunk::Manager {
    fn get_chunk<F>(&self, url: &str, start: u64, offset: u64, callback: F)
        where F: FnOnce(chunk::ChunkResult<Vec<u8>>) + Send + 'static
    {
        // TODO: recalculate chunk size
    }
}