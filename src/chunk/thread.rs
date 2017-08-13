use std::sync::Arc;
use threadpool;

use chunk;

pub struct ThreadManager<M> {
    manager: Arc<M>,
    pool: threadpool::ThreadPool,
}

impl<M> ThreadManager<M>
    where M: chunk::Manager + Sync + Send + 'static
{
    pub fn new(manager: M, threads: usize) -> chunk::ChunkResult<ThreadManager<M>> {
        Ok(ThreadManager {
               manager: Arc::new(manager),
               pool: threadpool::ThreadPool::new(threads),
           })
    }
}

impl<M> chunk::Manager for ThreadManager<M>
    where M: chunk::Manager + Sync + Send + 'static
{
    fn get_chunk<F>(&self, config: chunk::Config, callback: F)
        where F: FnOnce(chunk::ChunkResult<Vec<u8>>) + Send + 'static
    {
        let manager = self.manager.clone();
        let config = config.clone();

        self.pool.execute(move || {
            manager.get_chunk(config, callback);
        });
    }
}