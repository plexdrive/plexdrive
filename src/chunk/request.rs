use api;
use chunk;

pub struct RequestManager<C> {
    client: C,
}

impl<C> RequestManager<C> where C: api::Client {
    pub fn new(client: C) -> chunk::ChunkResult<RequestManager<C>> {
        Ok(RequestManager {
            client: client,
        })
    }
}

impl<C> chunk::Manager for RequestManager<C> where C: api::Client {
    fn get_chunk<F>(&self, url: &str, start: u64, offset: u64, callback: F)
        where F: FnOnce(chunk::ChunkResult<Vec<u8>>) + Send + 'static
    {
        // TODO: implement retry handling
        // TODO: implement 4xx HTTP error handling

        match self.client.do_http_request(url, start, start + offset) {
            Ok(chunk) => callback(Ok(chunk)),
            Err(cause) => {
                debug!("{:?}", cause);

                callback(Err(chunk::Error::RetrievalError(format!("Could not load chunk {} ({} - {})", url, start, offset))))
            }
        }
    }
}