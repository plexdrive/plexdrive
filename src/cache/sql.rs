use chrono;
use rusqlite::{Connection, Row};

use cache;

/// An SQLite based metadata cache
pub struct SqlCache {
  cache_file: String,
  connection: Connection,
}

impl SqlCache {
  pub fn new(cache_file: &str) -> cache::CacheResult<SqlCache> {
    let connection: Connection = match Connection::open(cache_file) {
      Ok(connection) => connection,
      Err(cause) => {
        debug!("{:?}", cause);
        return Err(cache::Error::OpenError(format!("Could not open cache {}", cache_file)))
      }
    };

    Ok(SqlCache{
      cache_file: cache_file.to_owned(),
      connection: connection,
    })
  }
}

impl cache::MetadataCache for SqlCache {
  fn initialize(&self) -> cache::CacheResult<()> {
    match self.connection.execute_batch(include_str!("sql/init.sql")) {
      Ok(_) => Ok(()),
      Err(cause) => {
        debug!("{:?}", cause);
        Err(cache::Error::OpenError(format!("Could not initialize cache {}", self.cache_file)))
      }
    }
  }

  fn process_changes(&mut self, changes: Vec<cache::Change>) -> cache::CacheResult<()> {
    let transaction = match self.connection.transaction() {
      Ok(transaction) => transaction,
      Err(cause) => {
        debug!("{:?}", cause);
        return Err(cache::Error::StoreError(String::from("Could not start transaction")));
      }
    };

    for change in changes {
      if change.removed {
        let id = match change.file_id {
          Some(id) => id,
          None => {
            warn!("Could not get file id from change");
            continue
          }
        };

        let file_removed = match transaction.execute("DELETE FROM file WHERE id = ?", &[ &id ]) {
          Ok(_) => true,
          Err(cause) => {
            debug!("{:?}", cause);
            warn!("Could not delete file {}", &id);

            false
          }
        };

        if file_removed {
          match transaction.execute("DELETE FROM parent WHERE file_id = ?", &[ &id ]) {
            Ok(_) => (),
            Err(cause) => {
              debug!("{:?}", cause);
              warn!("Could not delete parents for file {}", &id);
            }
          }
        }
      } else {
        let file = match change.file {
          Some(file) => file,
          None => {
            warn!("Could not get file from change");
            continue
          }
        };

        let file_inserted = match transaction.execute(
          "REPLACE INTO file (id, name, is_dir, size, last_modified, download_url, can_trash) VALUES (?, ?, ?, ?, ?, ?, ?);",
          &[ &file.id, &file.name, &file.is_dir, &format!("{}", file.size), &file.last_modified.to_rfc3339(), &file.download_url, &file.can_trash ])
          {
            Ok(_) => true,
            Err(cause) => {
              debug!("{:?}", cause);
              warn!("Could not insert file {} ({})", &file.id, &file.name);

              false
            }
          };

        if file_inserted {
          for parent in file.parents {
            match transaction.execute("DELETE FROM parent WHERE file_id = ?", &[ &file.id ]) {
              Ok(_) => (),
              Err(cause) => {
                debug!("{:?}", cause);
                warn!("Could not delete old parents for file {} ({})", &file.id, &file.name);
              }
            }

            match transaction.execute("REPLACE INTO parent (file_id, parent_id) VALUES (?, ?)", &[ &file.id, &parent ]) {
              Ok(_) => (),
              Err(cause) => {
                debug!("{:?}", cause);
                warn!("Could not insert parents for file {} ({})", &file.id, &file.name);
              }
            }
          }
        }
      }
    }

    match transaction.commit() {
      Ok(_) => Ok(()),
      Err(cause) => {
        debug!("{:?}", cause);
        Err(cache::Error::StoreError(String::from("Could not store batch in cache")))
      }
    }
  }

  fn get_change_token(&self) -> String {
    let token = self.connection.query_row("SELECT token FROM token WHERE id = 1", &[], |row| {
      row.get(0)
    });

    match token {
      Ok(token) => token,
      Err(_) => "1".to_owned()
    }
  }

  fn store_change_token(&self, token: String) -> cache::CacheResult<()> {
    match self.connection.execute("REPLACE INTO token (id, token) VALUES (1, ?)", &[&token]) {
      Ok(_) => Ok(()),
      Err(cause) => {
        debug!("{:?}", cause);
        Err(cache::Error::StoreError(format!("Could not store token {} in cache", token)))
      }
    }
  }

  fn get_file(&self, inode: u64) -> cache::CacheResult<cache::File> {
    let mut stmt = match self.connection.prepare("SELECT inode, id, name, is_dir, size, last_modified, download_url, can_trash FROM file WHERE inode = ? LIMIT 1") {
      Ok(stmt) => stmt,
      Err(cause) => {
        debug!("{:?}", cause);
        return Err(cache::Error::NotFound(format!("Could not prepare cache query for inode {}", &inode)))
      }
    };

    let result = match stmt.query_map(&[ &format!("{}", inode) ], |row| { convert_to_file(row) }) {
      Ok(mut rows) => match rows.next() {
        Some(result) => result.unwrap(),
        None => return Err(cache::Error::NotFound(format!("Could not find inode {}", &inode)))
      },
      Err(cause) => {
        debug!("{:?}", cause);
        
        return Err(cache::Error::NotFound(format!("Could not execute cache query for inode {}", &inode)))
      }
    };

    Ok(result)
  }

  fn get_child_files_by_inode(&self, inode: u64) -> cache::CacheResult<Vec<cache::File>> {
    let mut stmt = match self.connection.prepare("SELECT inode, id, name, is_dir, size, last_modified, download_url, can_trash FROM file WHERE id IN (SELECT file_id FROM parent INNER JOIN file ON (file.id = parent.parent_id AND file.inode = ?))") {
      Ok(stmt) => stmt,
      Err(cause) => {
        debug!("{:?}", cause);
        return Err(cache::Error::NotFound(format!("Could not prepare cache query for parent inode {}", &inode)))
      }
    };

    let rows = match stmt.query_map(&[ &format!("{}", inode) ], |row| { convert_to_file(row) }) {
      Ok(rows) => rows,
      Err(cause) => {
        debug!("{:?}", cause);

        return Err(cache::Error::NotFound(format!("Could not find parent inode {}", &inode)))
      }
    };

    let mut results = Vec::new();
    for row in rows {
      let result = match row {
        Ok(result) => Some(result),
        Err(_) => {
          debug!("Could not map row {:?}", row);
          
          None
        }
      };

      if result.is_some() {
        results.push(result.unwrap());
      }
    }

    Ok(results)
  }

  fn get_child_file_by_inode_and_name(&self, inode: u64, name: String) -> cache::CacheResult<cache::File> {
    let mut stmt = match self.connection.prepare("SELECT inode, id, name, is_dir, size, last_modified, download_url, can_trash FROM file WHERE id IN (SELECT file_id FROM parent INNER JOIN file ON (file.id = parent.parent_id AND file.inode = ?)) AND name = ?") {
      Ok(stmt) => stmt,
      Err(cause) => {
        debug!("{:?}", cause);
        return Err(cache::Error::NotFound(format!("Could not prepare cache query for parent inode {} and name {}", &inode, &name)))
      }
    };

    let result = match stmt.query_map(&[ &format!("{}", inode), &name ], |row| { convert_to_file(row) }) {
      Ok(mut rows) => match rows.next() {
        Some(result) => result.unwrap(),
        None => return Err(cache::Error::NotFound(format!("Could not find inode {} and name {}", &inode, &name)))
      },
      Err(cause) => {
        debug!("{:?}", cause);
        
        return Err(cache::Error::NotFound(format!("Could not execute cache query for inode {} and name {}", &inode, &name)))
      }
    };

    Ok(result)
  }
}

fn convert_to_file(row: &Row) -> cache::File {
  let inode: u32 = row.get(0);
  let id = row.get(1);
  let name = row.get(2);

  let sql_is_dir: u8 = row.get(3);
  let is_dir = if sql_is_dir == 1 { true } else { false };

  let sql_size: String = row.get(4);
  let size = match sql_size.parse() {
    Ok(size) => size,
    Err(_) => 0,
  };

  let sql_date: String = row.get(5);
  let last_modified = match chrono::DateTime::parse_from_rfc3339(&sql_date) {
    Ok(date) => date,
    Err(cause) => {
      debug!("{:?}", cause);
      warn!("Could not get modified time for {} ({})", &id, &name);

      chrono::DateTime::parse_from_rfc3339("1970-01-01T13:37:00.000+00:00").unwrap()
    }
  };

  let sql_can_trash: u8 = row.get(7);
  let can_trash = if sql_can_trash == 1 { true } else { false };

  cache::File {
    inode: Some(inode as u64),
    id: id,
    name: name,
    is_dir: is_dir,
    size: size,
    last_modified: last_modified,
    download_url: row.get(6),
    can_trash: can_trash,
    parents: vec![],
  }
}