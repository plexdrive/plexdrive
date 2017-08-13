use std::sync::Mutex;
use std::collections::HashMap;

use chunk;

pub struct RAMManager<M> {
    manager: M,
    chunks: Mutex<HashMap<String, Vec<u8>>>
}

impl<M> RAMManager<M> where M: chunk::Manager + Sync + Send + 'static {
    pub fn new(manager: M) -> chunk::ChunkResult<RAMManager<M>> {
        Ok(RAMManager {
            manager: manager,
            chunks: Mutex::new(HashMap::new()),
        })
    }
}

impl<M> chunk::Manager for RAMManager<M> where M: chunk::Manager + Sync + Send + 'static {
    fn get_chunk<F>(&self, config: chunk::Config, callback: F)
        where F: FnOnce(chunk::ChunkResult<Vec<u8>>) + Send + 'static
    {
        // let id = format!("{}|{}", config., start);
        let mut chunks = self.chunks.lock().unwrap();

        self.manager.get_chunk(config, |result| {
            callback(result);

            // TODO: store chunk in 
        });
    }
}