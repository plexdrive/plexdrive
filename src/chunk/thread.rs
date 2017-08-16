use std::sync::{Arc, Mutex};
use threadpool;

use chunk;

/// The thread manager is a component for multi threaded
/// chunk loading from a datastore that is able to 
/// handle thread safe operations
pub struct ThreadManager<M> {
    manager: Arc<M>,
    pool: Arc<Mutex<threadpool::ThreadPool>>,
}

impl<M> ThreadManager<M>
    where M: chunk::Manager + Sync + Send + 'static
{
    pub fn new(manager: M, threads: usize) -> chunk::ChunkResult<ThreadManager<M>> {
        Ok(ThreadManager {
               manager: Arc::new(manager),
               pool: Arc::new(Mutex::new(threadpool::ThreadPool::new(threads))),
           })
    }
}

impl<M> chunk::Manager for ThreadManager<M>
    where M: chunk::Manager + Sync + Send + 'static
{
    fn get_chunk<F>(&self, config: &chunk::Config, callback: F)
        where F: FnOnce(chunk::ChunkResult<Arc<Vec<u8>>>) + Send + 'static
    {
        let manager = self.manager.clone();
        let config = config.clone();

        let pool = self.pool.clone();
        pool.lock().unwrap().execute(move || {
            manager.get_chunk(&config, callback);
        });
    }
}