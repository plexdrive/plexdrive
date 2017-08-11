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
    fn get_chunk<F>(&self, url: &str, start: u64, offset: u64, callback: F)
        where F: FnOnce(chunk::ChunkResult<Vec<u8>>) + Send + 'static
    {
        let manager = self.manager.clone();
        let url = url.to_owned();
        let start = start.clone();
        let offset = offset.clone();

        self.pool.execute(move || {
            manager.get_chunk(&url, start, offset, callback);
        });
    }
}