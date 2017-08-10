use std::fmt;
use google_drive3 as drive3;
use chrono;

mod sql;

pub use cache::sql::SqlCache;

#[derive(Debug)]
pub enum Error {
    OpenError(String),
    StoreError(String),
    NotFound(String),
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

    /// Processes the changes delivered by Google
    fn process_changes(&mut self, files: Vec<Change>) -> CacheResult<()>;

    /// Get the cahnge token from cache or returns "1"
    fn get_change_token(&self) -> String;

    /// Stores the change token in cache
    fn store_change_token(&self, token: String) -> CacheResult<()>;

    /// Get a file by its inode id
    fn get_file(&self, inode: u64) -> CacheResult<File>;

    /// Get children of a file/folder by its inode id
    fn get_child_files_by_inode(&self, inode: u64) -> CacheResult<Vec<File>>;

    /// Get child of a folder by its inode id and name
    fn get_child_file_by_inode_and_name(&self, inode: u64, name: String) -> CacheResult<File>;
}

/// Change is a wrapper for files that can indicate if a file has been
/// deleted or has been added.
pub struct Change {
    pub removed: bool,
    pub file_id: Option<String>,
    pub file: Option<File>,
}

impl From<drive3::Change> for Change {
    fn from(change: drive3::Change) -> Change {
        let mut removed = change.removed.unwrap_or(false);

        let file = match change.file {
            Some(file) => {
                let explicitly_trashed = file.explicitly_trashed.unwrap_or(false);
                if !removed && explicitly_trashed {
                    removed = true;
                }

                Some(file)
            },
            None => None,
        };

        Change {
            removed: removed,
            file_id: change.file_id,
            file: match file {
                Some(file) => Some(File::from(file)),
                None => None,
            },
        }
    }
}

/// File is a Google Drive file representation that only contains the
/// necessary fields for the cache.
#[derive(Debug)]
pub struct File {
    pub inode: Option<u64>,
    pub id: String,
    pub name: String,
    pub is_dir: bool,
    pub size: u64,
    pub last_modified: chrono::DateTime<chrono::FixedOffset>,
    pub download_url: String,
    pub can_trash: bool,
    pub parents: Vec<String>,
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
                &id,
                &name
            ))) {
                Ok(time) => time,
                Err(cause) => {
                    debug!("{:?}", cause);
                    warn!("Could not get modified time for {} ({})", &id, &name);

                    chrono::DateTime::parse_from_rfc3339("1970-01-01T13:37:00.000+00:00").unwrap()
                }
            };

        let can_trash = match file.capabilities {
            Some(capabilities) => capabilities.can_trash.expect(&format!(
                "Missing Google Drive file attribute: capabilities/can_trash for file {} ({})",
                &id,
                &name
            )),
            None => false,
        };

        let size = match file.size.unwrap_or(String::from("0")).parse() {
            Ok(size) => size,
            Err(cause) => {
                debug!("{:?}", cause);
                warn!("Could not parse file size for file {} ({})", &id, &name);

                0
            }
        };

        let is_dir = file.mime_type.expect(&format!(
            "Missing Google Drive file attribute: mime_type for file {} ({})",
            &id,
            &name
        )) == "application/vnd.google-apps.folder";

        let download_url = format!("https://www.googleapis.com/drive/v3/files/{}?alt=media", &id); 

        File {
            inode: None,
            id: id,
            name: name,
            is_dir: is_dir,
            size: size,
            last_modified: modified_time,
            download_url: download_url,
            can_trash: can_trash,
            parents: file.parents.unwrap_or(vec![]),
        }
    }
}
