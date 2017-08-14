use std::thread;
use std::sync::{Arc, Mutex, mpsc};
use std::collections::HashMap;

use api;
use chunk;

/// The request manager is just a wrapper for
/// doing requests and handle API errors, so that
/// it can retry the request
pub struct RequestManager<C> {
    client: Arc<Mutex<C>>,
    requests: Mutex<HashMap<String, Arc<Mutex<mpsc::Receiver<chunk::ChunkResult<Vec<u8>>>>>>>
}

impl<C> RequestManager<C> where C: api::Client {
    pub fn new(client: C) -> chunk::ChunkResult<RequestManager<C>> {
        Ok(RequestManager {
            client: Arc::new(Mutex::new(client)),
            requests: Mutex::new(HashMap::new()),
        })
    }
}

impl<C> chunk::Manager for RequestManager<C> where C: api::Client + Send + 'static {
    fn get_chunk<F>(&self, config: chunk::Config, callback: F)
        where F: FnOnce(chunk::ChunkResult<Vec<u8>>) + Send + 'static
    {
        let (tx, rx, exist) = {
            let (tx, rx): (mpsc::Sender<chunk::ChunkResult<Vec<u8>>>, mpsc::Receiver<chunk::ChunkResult<Vec<u8>>>) = mpsc::channel();
            let mut requests = self.requests.lock().unwrap();
            let (found_rx, exist) = match requests.get(&config.id) {
                Some(rx) => (rx.clone(), true),
                None => (Arc::new(Mutex::new(rx)), false),
            };

            if !exist {
                requests.insert(config.id.clone(), found_rx.clone());
            }

            (tx, found_rx, exist)
        };

        if !exist {
            let client = self.client.clone();
            thread::spawn(move || {
                match client.lock().unwrap().do_http_request(&config.url, config.start, config.start + config.size) {
                    Ok(chunk) => {
                        tx.send(Ok(chunk)).unwrap();
                    },
                    Err(cause) => {
                        debug!("{:?}", cause);

                        tx.send(Err(chunk::Error::RetrievalError(format!("Could not load chunk {} ({} - {})", config.url, config.start, config.size)))).unwrap();
                    }
                };
            });
        }

        callback(rx.lock().unwrap().recv().unwrap());

        // TODO: implement retry handling
        // TODO: implement 4xx HTTP error handling
    }
}