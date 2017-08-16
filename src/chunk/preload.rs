use std::sync::Arc;

use chunk;

/// The preload manager will start N preloads when a request
/// comes in
pub struct PreloadManager<M> {
    manager: M,
    preload: u64,
    chunk_size: u64,
}

impl<M> PreloadManager<M> where M: chunk::Manager + Sync + Send + 'static {
    pub fn new(manager: M, preload: u64, chunk_size: u64) -> chunk::ChunkResult<PreloadManager<M>> {
        Ok(PreloadManager {
            manager: manager,
            preload: preload,
            chunk_size: chunk_size,
        })
    }
}

impl<M> chunk::Manager for PreloadManager<M> where M: chunk::Manager + Sync + Send + 'static {
    fn get_chunk<F>(&self, config: &chunk::Config, callback: F)
        where F: FnOnce(chunk::ChunkResult<Arc<Vec<u8>>>) + Send + 'static
    {
        self.manager.get_chunk(config, callback);

        for i in 1..(self.preload + 1) {
            let mut config = config.clone();
            config.offset_start += i * self.chunk_size;

            if config.offset_start >= config.file_size {
                break;
            }

            config.offset_end += config.offset_start + self.chunk_size;
            config.id = format!("{}:{}", config.file_id, config.offset_start);

            trace!("Starting preload: {}", config.id);
            self.manager.get_chunk(&config, move |_result| {});
        }
    }
}