use std::sync::{Arc, Mutex};
use std::collections::HashMap;

use api;
use chunk;

pub struct RAMManager<C> {
    client: C,
    chunks: Mutex<HashMap<String, Vec<u8>>>
}

impl<C> RAMManager<C> where C: api::Client {
    pub fn new(client: C) -> chunk::ChunkResult<RAMManager<C>> {
        Ok(RAMManager {
            client: client,
            chunks: Mutex::new(HashMap::new()),
        })
    }
}

impl<C> chunk::Manager for RAMManager<C> where C: api::Client {
    fn get_chunk<F>(&self, url: &str, start: u64, offset: u64, callback: F)
        where F: FnOnce(chunk::ChunkResult<Vec<u8>>) + Send + 'static
    {
        let mut chunks = self.chunks.lock().unwrap();
        // TODO: resume here

        match self.client.do_http_request(url, start, start + offset) {
            Ok(chunk) => callback(Ok(chunk)),
            Err(cause) => {
                debug!("{:?}", cause);

                callback(Err(chunk::Error::RetrievalError(format!("Could not load chunk {} ({} - {})", url, start, offset))))
            }
        }
    }
}