use std::fs;
use std::path::Path;
use std::io::Write;
use serde_json;

use config;

#[derive(Debug)]
pub enum Error {
    JsonParseError,
    ReadError(String),
    WriteError(String),
}

/// The configuraton object
#[derive(Serialize, Deserialize, Debug)]
pub struct Config {
    pub client_id: String,
    pub client_secret: String,
}

/// Load loads an existing configuration
pub fn load(config_file: &str) -> Result<Config, Error> {
    let file = match fs::File::open(&Path::new(config_file)) {
        Ok(file) => file,
        Err(cause) => {
            debug!("{:?}", cause);
            return Err(Error::ReadError(format!("Could not open config file {}", config_file)));
        }
    };

    let config: config::Config = match serde_json::from_reader(file) {
        Ok(config) => config,
        Err(cause) => {
            debug!("{:?}", cause);
            return Err(Error::ReadError(format!("Could not read config file {}", config_file)));
        }
    };

    Ok(config)
}

/// Create the configuration file
pub fn create(config_file: &str, client_id: &str, client_secret: &str) -> Result<(), Error> {
    let config = Config {
        client_id: client_id.to_owned(),
        client_secret: client_secret.to_owned(),
    };

    let config_json = match serde_json::to_string(&config) {
        Ok(json) => json,
        Err(cause) => {
            debug!("{:?}", cause);
            return Err(Error::JsonParseError)
        }
    };

    let mut file = match fs::File::create(&Path::new(config_file)) {
        Ok(file) => file,
        Err(cause) => {
            debug!("{:?}", cause);
            return Err(Error::WriteError(format!("Could not create config file {}", config_file)));
        }
    };

    match file.write_all(config_json.as_bytes()) {
        Ok(_) => (),
        Err(cause) => {
            debug!("{:?}", cause);
            return Err(Error::WriteError(format!("Could not write to config file {}", config_file)));
        }
    }

    Ok(())
}