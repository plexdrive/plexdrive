use std::thread;
use std::time;
use std::sync::{Arc, Mutex};
use std::io::Read;
use std::str;
use hyper;
use hyper_rustls;
use yup_oauth2::{ApplicationSecret, Authenticator, DefaultAuthenticatorDelegate, DiskTokenStorage,
                 FlowType, GetToken};
use google_drive3 as drive3;

use api;
use cache;

static FILE_FIELDS: &'static str = "id, name, mimeType, modifiedTime, size, explicitlyTrashed, parents, capabilities/canTrash";
static CHANGE_FIELDS: &'static str = "nextPageToken, newStartPageToken, changes(removed, fileId, file(id, name, mimeType, modifiedTime, size, explicitlyTrashed, parents, capabilities/canTrash))";

/// The Client that holds the connection infos for the Google Drive API.
/// It does not hold a Google Drive API instance because
/// of lifetime complexity. It will create an instance on demand.
pub struct DriveClient {
    secret: ApplicationSecret,
    token_file: String,
}

impl DriveClient {
    /// Create a new client instance
    pub fn new(token_file: String, client_id: String, client_secret: String) -> DriveClient {
        let secret = ApplicationSecret {
            client_id: client_id,
            client_secret: client_secret,
            token_uri: "https://accounts.google.com/o/oauth2/token".to_owned(),
            auth_uri: "https://accounts.google.com/o/oauth2/auth".to_owned(),
            redirect_uris: vec!["urn:ietf:wg:oauth:2.0:oob".to_owned()],
            project_id: None,
            client_email: None,
            auth_provider_x509_cert_url: None,
            client_x509_cert_url: None,
        };

        DriveClient {
            secret: secret,
            token_file: token_file,
        }
    }

    /// Get the native Google Drive client
    fn get_native_client(&self)
                         -> drive3::Drive<hyper::Client,
                                          Authenticator<DefaultAuthenticatorDelegate,
                                                        DiskTokenStorage,
                                                        hyper::Client>> {
        let authenticator = Authenticator::new(
            &self.secret,
            DefaultAuthenticatorDelegate,
            hyper::Client::with_connector(hyper::net::HttpsConnector::new(
                hyper_rustls::TlsClient::new(),
            )),
            DiskTokenStorage::new(&self.token_file).unwrap(),
            Some(FlowType::InstalledInteractive),
        );

        drive3::Drive::new(
            hyper::Client::with_connector(hyper::net::HttpsConnector::new(
                hyper_rustls::TlsClient::new(),
            )),
            authenticator,
        )
    }
}

impl api::Client for DriveClient {
    fn authorize(&self) -> api::ClientResult<String> {
        let client = self.get_native_client();

        let about = match client.about()
                  .get()
                  .param("fields", "user")
                  .add_scope(drive3::Scope::Full)
                  .doit() {
            Ok((_, about)) => about,
            Err(cause) => return Err(api::Error::Authentication(format!("{}", cause))),
        };

        let user = match about.user {
            Some(user) => user,
            None => return Err(api::Error::MissingDataObject(String::from("User object not found in Google response"))),
        };

        match user.display_name {
            Some(name) => Ok(name),
            None => Err(api::Error::MissingDataObject(String::from("Users display name not found in Google response"))),
        }
    }

    fn do_http_request(&self,
                       url: &str,
                       start_offset: u64,
                       end_offset: u64)
                       -> api::ClientResult<Vec<u8>> {
        let mut authenticator = Authenticator::new(
            &self.secret,
            DefaultAuthenticatorDelegate,
            hyper::Client::with_connector(hyper::net::HttpsConnector::new(
                hyper_rustls::TlsClient::new(),
            )),
            DiskTokenStorage::new(&self.token_file).unwrap(),
            Some(FlowType::InstalledInteractive),
        );

        let scopes = vec!["https://www.googleapis.com/auth/drive"];
        let token = match authenticator.token(&scopes) {
            Ok(token) => token,
            Err(cause) => {
                debug!("{:?}", cause);

                return Err(api::Error::Authentication(String::from("Could not get token for Google Drive access")));
            }
        };

        let http_client = hyper::Client::with_connector(hyper::net::HttpsConnector::new(
            hyper_rustls::TlsClient::new()
        ));

        let mut response: hyper::client::Response = match http_client.request(hyper::method::Method::Get, url)
                .header(hyper::header::Authorization(hyper::header::Bearer{ token: token.access_token }))
                .header(hyper::header::Range::Bytes(vec![hyper::header::ByteRangeSpec::FromTo(start_offset, end_offset)]))
                .send() {
                    Ok(response) => response,
                    Err(cause) => {
                        debug!("{:?}", cause);

                        return Err(api::Error::HttpRequestError(format!("Could not request URL {}", url)))
                    }
                };

        let length: usize = (end_offset - start_offset) as usize;
        let mut buffer = vec![0u8; length];

        let n = match response.read(buffer.as_mut_slice()) {
            Ok(n) => n,
            Err(cause) => {
                debug!("{:?}", cause);

                return Err(api::Error::HttpReadError(format!("Could not read content of http response for URL {}", url)));
            }
        };

        let body = &buffer[0 .. n];

        if response.status != hyper::status::StatusCode::PartialContent {
            return Err(api::Error::HttpInvalidStatus(response.status, match str::from_utf8(body) {
                Ok(content) => content.to_owned(),
                Err(cause) => {
                    debug!("{:?}", cause);
                    String::from("")
                }
            }));
        }

        Ok(body.to_vec())
    }

    fn watch_changes<C>(&self, cache: Arc<Mutex<C>>)
        where C: cache::MetadataCache + Send + 'static
    {
        let client = self.get_native_client();

        let cache = cache.clone();
        thread::spawn(move || {
            let mut first_run = true;
            let mut change_count = 0;
            loop {
                let changelist = match client.changes()
                          .list(&cache.lock().unwrap().get_change_token())
                          .add_scope(drive3::Scope::Full)
                          .param("fields", CHANGE_FIELDS)
                          .include_removed(true)
                          .page_size(999)
                          .doit() {
                    Ok((_, changes)) => changes,
                    Err(cause) => {
                        warn!("Could not get changes because of {}", cause);
                        continue;
                    }
                };

                let changes = match changelist.changes {
                    Some(changes) => changes,
                    None => {
                        warn!("No changes found");
                        continue;
                    }
                };

                let changes: Vec<cache::Change> =
                    changes.into_iter().map(|change| cache::Change::from(change)).collect();

                change_count += changes.len();

                match cache.lock().unwrap().process_changes(changes) {
                    Ok(_) => (),
                    Err(cause) => panic!("{}", cause),
                }

                match cache.lock().unwrap().store_change_token(match changelist.next_page_token {
                    Some(token) => token,
                    None => match changelist.new_start_page_token.clone() {
                        Some(token) => token,
                        None => {
                            warn!("Could not get next start token for watching changes");
                            continue;
                        }
                    }
                }) {
                    Ok(_) => (),
                    Err(cause) => warn!("{}", cause),
                }

                if change_count > 0 {
                    info!("Processed {} changes", change_count);
                }

                if changelist.new_start_page_token.is_some() {
                    change_count = 0;
                    if first_run {
                        info!("Cache building finished!");
                        first_run = false;
                    }

                    // sleep 60 seconds and wait for new changes
                    thread::sleep(time::Duration::new(60, 0));
                }
            }
        });
    }

    fn get_file(&self, id: &str) -> api::ClientResult<cache::File> {
        let client = self.get_native_client();

        match client.files()
                  .get(id)
                  .add_scope(drive3::Scope::Full)
                  .param("fields", FILE_FIELDS)
                  .doit() {
            Ok((_, file)) => Ok(cache::File::from(file)),
            Err(cause) => {
                debug!("{}", cause);
                Err(api::Error::FileNotFound(format!("File {} could not be fetched from API", id)))
            }
        }
    }
}
