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
      Ok(()) => Ok(()),
      Err(cause) => {
        debug!("{:?}", cause);
        Err(cache::Error::OpenError(format!("Could not initialize cache {}", self.cache_file)))
      }
    }
  }

  fn store_files(&self, files: Vec<cache::File>) -> cache::CacheResult<()> {
    let mut file_inserts = Vec::new();
    let mut parent_delete_ids = Vec::new();
    let mut parent_inserts = Vec::new();
    for file in files {
      file_inserts.push(format!(
        "('{}', '{}', {}, {}, '{}', '{}', {})", 
        file.id.replace("'", "''"), 
        file.name.replace("'", "''"),
        if file.is_dir { 1 } else { 0 },
        file.size,
        file.last_modified.to_rfc3339(),
        file.download_url.replace("'", "''"),
        if file.can_trash { 1 } else { 0 }
      ));

      for parent in file.parents {
        parent_delete_ids.push(format!("'{}'", parent));

        parent_inserts.push(format!("('{}', '{}')", file.id, parent));
      }
    }

    let file_insert_query = format!("REPLACE INTO file (id, name, is_dir, size, last_modified, download_url, can_trash) VALUES {};", file_inserts.join(", "));
    let parent_delete_query = format!("DELETE FROM parent WHERE file_id IN ({});", parent_delete_ids.join(", "));
    let parent_insert_query = format!("REPLACE INTO parent (file_id, parent_id) VALUES {};", parent_inserts.join(", "));

    let mut query = file_insert_query;
    query.push_str(&parent_delete_query);
    query.push_str(&parent_insert_query);

    match self.connection.execute_batch(&query) {
      Ok(_) => Ok(()),
      Err(cause) => {
        debug!("{:?} / {}", cause, query);
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
}