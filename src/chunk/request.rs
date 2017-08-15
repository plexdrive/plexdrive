use std::sync::{Arc, Mutex};
use std::collections::HashMap;
use bus;

use api;
use chunk;

/// The request manager is just a wrapper for
/// doing requests and handle API errors, so that
/// it can retry the request
pub struct RequestManager<C> {
    client: C,
    requests: Mutex<HashMap<String, Arc<Mutex<bus::Bus<chunk::ChunkResult<Vec<u8>>>>>>>
}

impl<C> RequestManager<C> where C: api::Client {
    pub fn new(client: C) -> chunk::ChunkResult<RequestManager<C>> {
        Ok(RequestManager {
            client: client,
            requests: Mutex::new(HashMap::new()),
        })
    }
}

impl<C> chunk::Manager for RequestManager<C> where C: api::Client + Send + 'static {
    fn get_chunk<F>(&self, config: chunk::Config, callback: F)
        where F: FnOnce(chunk::ChunkResult<Vec<u8>>) + Send + 'static
    {
        let (bus, exist) = {
            let mut requests = self.requests.lock().unwrap();
            let (bus, exist) = match requests.get(&config.id) {
                Some(bus) => (bus.clone(), true),
                None => (Arc::new(Mutex::new(bus::Bus::new(20))), false),
            };

            if !exist {
                requests.insert(config.id.clone(), bus.clone());
            }

            (bus, exist)
        };

        let mut rx = bus.lock().unwrap().add_rx();

        if !exist {
            let bus = bus.clone();
            let cfg = config.clone();
            self.client.do_http_request(&config.url, config.offset_start, config.offset_end, move |result| {
                match result {
                    Ok(chunk) => {
                        bus.lock().unwrap().broadcast(Ok(chunk));
                    },
                    Err(cause) => {
                        debug!("{:?}", cause);

                        bus.lock().unwrap().broadcast(Err(chunk::Error::RetrievalError(format!("Could not load chunk {} ({} - {})", cfg.url, cfg.start, cfg.size))));
                    }
                };
            });
        }

        callback(match rx.recv() {
            Ok(result) => result,
            Err(cause) => {
                debug!("{}", cause);

                Err(chunk::Error::RetrievalError(format!("Could not receive chunk {}", &config.id)))
            }
        });

        // TODO: implement retry handling
        // TODO: implement 4xx HTTP error handling
    }
}