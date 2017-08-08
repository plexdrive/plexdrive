use std::fs;
use std::path::Path;

use config;
use api::{Client, DriveClient};

/// Execute starts the initialization flow
pub fn execute(config_path: &str, client_id: &str, client_secret: &str) {
    let config_dir = Path::new(config_path);

    let config_file_buf = Path::new(config_path).join("config.json");
    let token_file_buf = Path::new(config_path).join("token.json");

    let config_file = config_file_buf.as_path();
    let token_file = token_file_buf.as_path();

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
        Ok(username) => info!("Initialization successful, {}", username),
        Err(cause) => {
            debug!("{}", cause);
            panic!("Initialization not successful")
        }
    }
}



