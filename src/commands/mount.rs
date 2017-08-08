use std::path::Path;
use std::thread;
use std::sync::{Arc, Mutex};
use std::time;

use config;
use api::{Client, DriveClient};
use cache::SqlCache;

/// Execute starts the mount flow
pub fn execute(config_path: &str, mount_path: &str) {
    let config_file_buf = Path::new(config_path).join("config.json");
    let token_file_buf = Path::new(config_path).join("token.json");
    let cache_file_buf = Path::new(config_path).join("cache.db");

    let config_file = config_file_buf.as_path();
    let token_file = token_file_buf.as_path();
    let cache_file = cache_file_buf.as_path();

    let config = match config::load(config_file.to_str().unwrap()) {
        Ok(config) => config,
        Err(_) => panic!("Could not read configuration"),
    };

    let drive_client = DriveClient::new(token_file.to_str().unwrap().to_owned(), config.client_id, config.client_secret);

    let cache = match SqlCache::new(cache_file.to_str().unwrap()) {
        Ok(cache) => Arc::new(Mutex::new(cache)),
        Err(cause) => panic!("{}", cause)
    };

    // TODO: create a database instance and pass it to watch_changes
    drive_client.watch_changes(cache);

    // TODO: delete this whenever it is not useful anymore
    thread::sleep(time::Duration::new(5 * 60, 0));
}