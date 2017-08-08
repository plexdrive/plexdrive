use rusqlite::{Connection, Error};

use cache;

pub struct SqlCache {
  connection: Connection
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

    match initialize_db(&connection) {
      Ok(()) => (),
      Err(cause) => {
        debug!("{}", cause);
        return Err(cache::Error::OpenError(format!("Could not initialize cache {}", cache_file)));
      }
    }

    Ok(SqlCache{
      connection: connection
    })
  }
}

impl cache::MetadataCache for SqlCache {
  fn store_files(&self, files: Vec<cache::File>) {
    for file in files {
      info!("Storing file {}", file.name.unwrap());
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
        debug!("{}", cause);
        Err(cache::Error::StoreError(format!("Could not store token {} in cache", token)))
      }
    }
  }
}

fn initialize_db(conn: &Connection) -> Result<(), Error> {
  conn.execute_batch("
    BEGIN;

    CREATE TABLE IF NOT EXISTS token (
      id    INTEGER PRIMARY KEY,
      token TEXT
    );

    COMMIT;
  ")
}