use api;
use chunk;

/// The request manager is just a wrapper for
/// doing requests and handle API errors, so that
/// it can retry the request
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
    fn get_chunk<F>(&self, config: chunk::Config, callback: F)
        where F: FnOnce(chunk::ChunkResult<Vec<u8>>) + Send + 'static
    {
        // TODO: implement retry handling
        // TODO: implement 4xx HTTP error handling

        match self.client.do_http_request(&config.url, config.start, config.start + config.size) {
            Ok(chunk) => callback(Ok(chunk)),
            Err(cause) => {
                debug!("{:?}", cause);

                callback(Err(chunk::Error::RetrievalError(format!("Could not load chunk {} ({} - {})", config.url, config.start, config.size))))
            }
        }
    }
}