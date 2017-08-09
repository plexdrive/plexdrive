use std::fs;
use std::path::Path;

use config;
use api::{Client, DriveClient};
use cache::{MetadataCache, SqlCache, Change};

/// Execute starts the initialization flow
pub fn execute(config_path: &str, client_id: &str, client_secret: &str) {
    let config_dir = Path::new(config_path);

    let config_file_buf = Path::new(config_path).join("config.json");
    let token_file_buf = Path::new(config_path).join("token.json");
    let cache_file_buf = Path::new(config_path).join("cache.db");

    let config_file = config_file_buf.as_path();
    let token_file = token_file_buf.as_path();
    let cache_file = cache_file_buf.as_path();

    // create configuration directory
    if config_dir.exists() {
        fs::remove_dir_all(config_dir).expect("Could not delete existing configuration directory");
    }
    fs::create_dir_all(config_path).expect("Could not create configuration directory");

    // create configuration file
    match config::create(config_file.to_str().unwrap(), client_id, client_secret) {
        Ok(_) => (),
        Err(_) => panic!("Could not create configuration")
    }
    // create token file
    let drive_client = DriveClient::new(token_file.to_str().unwrap().to_owned(), client_id.to_owned(), client_secret.to_owned());
    
    match drive_client.authorize() {
        Ok(username) => info!("Google Drive initialization successful, {}", username),
        Err(cause) => {
            debug!("{:?}", cause);
            panic!("Google Drive initialization not successful");
        }
    };

    let mut cache = match SqlCache::new(cache_file.to_str().unwrap()) {
        Ok(cache) => cache,
        Err(cause) => panic!("{}", cause),
    };

    match cache.initialize() {
        Ok(_) => info!("Cache initialization successful"),
        Err(cause) => {
            debug!("{:?}", cause);
            panic!("Cache initialization not successful");
        }
    }

    let file = match drive_client.get_file("root") {
        Ok(file) => file,
        Err(cause) => panic!("{}", cause)
    };

    match cache.process_changes(vec![
        Change {
            removed: false,
            file_id: None,
            file: Some(file),
        }
    ]) {
        Ok(_) => info!("Root folder successfully added to cache"),
        Err(cause) => {
            debug!("{:?}", cause);
            panic!("Could not store root folder in cache")
        }
    }
}



