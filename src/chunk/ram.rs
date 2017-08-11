use api;
use chunk;

pub struct RAMManager<C> {
    client: C
}

impl<C> RAMManager<C> where C: api::Client {
    pub fn new(client: C) -> chunk::ChunkResult<RAMManager<C>> {
        Ok(RAMManager {
            client
        })
    }
}

impl<C> chunk::Manager for RAMManager<C> where C: api::Client {
    fn get_chunk<F>(&self, url: &str, start: u64, offset: u64, callback: F)
        where F: FnOnce(chunk::ChunkResult<Vec<u8>>) + Send + 'static
    {
        match self.client.do_http_request(url, start, start + offset) {
            Ok(chunk) => callback(Ok(chunk)),
            Err(cause) => {
                debug!("{:?}", cause);

                callback(Err(chunk::Error::RetrievalError(format!("Could not load chunk {} ({} - {})", url, start, offset))))
            }
        }
    }
}