use std::sync::{Arc, RwLock};
use std::collections::HashMap;

use chunk;

/// The RAM manager stores chunks in RAM if the chunk could not be found on RAM
/// it will pass the request to the next manager
pub struct RAMManager<M> {
    manager: M,
    chunks: Arc<RwLock<HashMap<String, Vec<u8>>>>
}

impl<M> RAMManager<M> where M: chunk::Manager + Sync + Send + 'static {
    pub fn new(manager: M) -> chunk::ChunkResult<RAMManager<M>> {
        Ok(RAMManager {
            manager: manager,
            chunks: Arc::new(RwLock::new(HashMap::new())),
        })
    }
}

impl<M> chunk::Manager for RAMManager<M> where M: chunk::Manager + Sync + Send + 'static {
    fn get_chunk<F>(&self, config: chunk::Config, callback: F)
        where F: FnOnce(chunk::ChunkResult<Vec<u8>>) + Send + 'static
    {
        debug!("Checking {} in RAM", &config.id);

        let chunks = self.chunks.clone();

        match chunks.read().unwrap().get(&config.id) {
            Some(chunk) => {
                debug!("Found {} in RAM", &config.id);

                callback(Ok(chunk::utils::cut_chunk(chunk, config.chunk_offset as usize, config.size as usize)));
            },
            None => {
                debug!("Not Found {} in RAM", &config.id);

                let chunks = self.chunks.clone();
                self.manager.get_chunk(config.clone(), move |result| {
                    match result {
                        Ok(chunk) => {
                            callback(Ok(chunk::utils::cut_chunk(&chunk, config.chunk_offset as usize, config.size as usize)));

                            chunks.write().unwrap().insert(config.id.clone(), chunk);
                        },
                        Err(cause) => {
                            callback(Err(cause));
                            warn!("Could not store chunk {}", config.id);
                        }
                    };
                })
            }
        };
    }
}