use std::sync::{Arc, Mutex};
use std::collections::HashMap;
use time;
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

    fn get_bus_for_id(&self, id: &str) -> (Arc<Mutex<bus::Bus<chunk::ChunkResult<Vec<u8>>>>>, bool) {
        let mut requests = self.requests.lock().unwrap();
        let (bus, exist) = match requests.get(id) {
            Some(bus) => (bus.clone(), true),
            None => (Arc::new(Mutex::new(bus::Bus::new(16))), false),
        };

        if !exist {
            requests.insert(id.to_owned(), bus.clone());
        }

        (bus, exist)
    }

    fn do_request(&self, bus: Arc<Mutex<bus::Bus<chunk::ChunkResult<Vec<u8>>>>>, config: &chunk::Config, retry: u8, delay: time::Duration) {
        let cfg = config.clone();
        self.client.do_http_request(&config.url, config.offset_start, config.offset_end, move |result| {
            match result {
                Ok(chunk) => {
                    bus.lock().unwrap().broadcast(Ok(chunk));
                },
                Err(cause) => {
                    // TODO: implement retry handling
                    // TODO: implement 4xx HTTP error handling
                    debug!("{:?}", cause);

                    bus.lock().unwrap().broadcast(Err(chunk::Error::RetrievalError(format!("Could not load chunk {} ({} - {})", cfg.url, cfg.start, cfg.size))));
                }
            };
        });
    } 
}

impl<C> chunk::Manager for RequestManager<C> where C: api::Client + Send + 'static {
    fn get_chunk<F>(&self, config: &chunk::Config, callback: F)
        where F: FnOnce(chunk::ChunkResult<Vec<u8>>) + Send + 'static
    {
        let (bus, exist) = self.get_bus_for_id(&config.id);
        let mut rx = bus.lock().unwrap().add_rx();

        if !exist {
            self.do_request(bus.clone(), config, 0, time::Duration::milliseconds(500));
        }

        callback(match rx.recv() {
            Ok(result) => result,
            Err(cause) => {
                debug!("{}", cause);

                Err(chunk::Error::RetrievalError(format!("Could not receive chunk {}", &config.id)))
            }
        });
    }
}