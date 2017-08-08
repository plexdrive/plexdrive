use rusqlite::Connection;

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
        debug!("{}", cause);
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
      Ok(()) => Ok(()),
      Err(cause) => {
        debug!("{}", cause);
        Err(cache::Error::OpenError(format!("Could not initialize cache {}", self.cache_file)))
      }
    }
  }

  fn store_files(&self, files: Vec<cache::File>) -> cache::CacheResult<()> {
    for file in files {
      info!("Storing file {}", file.name.unwrap());
    }

    Ok(())
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
        debug!("{}", cause);
        Err(cache::Error::StoreError(format!("Could not store token {} in cache", token)))
      }
    }
  }
}