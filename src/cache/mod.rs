use std::fmt;
use google_drive3 as drive3;
use chrono;

mod sql;

pub use cache::sql::SqlCache;

#[derive(Debug)]
pub enum Error {
    OpenError(String),
    StoreError(String),
}
pub type CacheResult<T> = Result<T, Error>;

impl fmt::Display for Error {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        write!(f, "{:?}", self)
    }
}

/// The MetadataCache caches all meta information about Google Files
/// so that you won't hit the API limits that fast.
pub trait MetadataCache {
    /// Initialize the cache
    fn initialize(&self) -> CacheResult<()>;

    /// Stores files in the cache
    fn store_files(&self, files: Vec<File>) -> CacheResult<()>;

    /// Get the cahnge token from cache or returns "1"
    fn get_change_token(&self) -> String;

    /// Stores the change token in cache
    fn store_change_token(&self, token: String) -> CacheResult<()>;
}

/// File is a Google Drive file representation that only contains the
/// necessary fields for the cache.
#[derive(Debug)]
pub struct File {
    pub id: String,
    pub name: String,
    pub is_dir: bool,
    pub size: u32,
    pub last_modified: chrono::DateTime<chrono::FixedOffset>,
    pub download_url: String,
    pub can_trash: bool,
    pub parents: Vec<String>
}

impl From<drive3::File> for File {
    fn from(file: drive3::File) -> File {
        let id = file.id.expect("Missing Google Drive file attribute: id");
        let name = file.name.expect(&format!(
            "Missing Google Drive file attribute: name for file {}",
            id
        ));

        let modified_time =
            match chrono::DateTime::parse_from_rfc3339(&file.modified_time.expect(&format!(
                "Missing Google Drive file attribute: modified_time for file {} ({})",
                id.clone(),
                name.clone()
            ))) {
                Ok(time) => time,
                Err(cause) => {
                    debug!("{:?}", cause);
                    warn!("Could not get modified time for {} ({})", id.clone(), name.clone());

                    chrono::DateTime::parse_from_rfc3339("1970-01-01T13:37:00.000+00:00").unwrap()
                }
            };

        let can_trash = match file.capabilities {
            Some(capabilities) => capabilities.can_trash.expect(&format!(
                "Missing Google Drive file attribute: capabilities/can_trash for file {} ({})",
                id.clone(),
                name.clone()
            )),
            None => false,
        };

        File {
            id: id.clone(),
            name: name.clone(),
            is_dir: file.mime_type.expect(&format!(
                "Missing Google Drive file attribute: mime_type for file {} ({})",
                id.clone(),
                name.clone()
            )) == "application/vnd.google-apps.folder",
            size: file.size
                .unwrap_or(String::from("0"))
                .parse()
                .expect(&format!(
                    "Could not parse file size for file {} ({})",
                    id.clone(),
                    name.clone()
                )),
            last_modified: modified_time,
            download_url: format!("https://www.googleapis.com/drive/v3/files/{}?alt=media", id),
            can_trash: can_trash,
            parents: file.parents.unwrap_or(vec![])
        }
    }
}
