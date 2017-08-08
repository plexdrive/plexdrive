use std::thread;
use std::time;
use hyper;
use hyper_rustls;
use yup_oauth2::{ApplicationSecret, Authenticator, DefaultAuthenticatorDelegate, DiskTokenStorage,
                 FlowType};
use google_drive3 as drive3;

use api;
use cache;

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
    fn get_native_client(
        &self,
    ) -> drive3::Drive<
        hyper::Client,
        Authenticator<DefaultAuthenticatorDelegate, DiskTokenStorage, hyper::Client>,
    > {
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

        let about = match client
            .about()
            .get()
            .param("fields", "user")
            .add_scope(drive3::Scope::Full)
            .doit()
        {
            Ok((_, about)) => about,
            Err(cause) => return Err(api::Error::Authentication(format!("{}", cause))),
        };

        let user = match about.user {
            Some(user) => user,
            None => {
                return Err(api::Error::MissingDataObject(
                    String::from("User object not found in Google response"),
                ))
            }
        };

        match user.display_name {
            Some(name) => Ok(name),
            None => Err(api::Error::MissingDataObject(String::from(
                "Users display name not found in Google response",
            ))),
        }
    }

    fn watch_changes<C>(&self, cache: C)
    where
        C: cache::MetadataCache + Send + 'static,
    {
        let client = self.get_native_client();

        thread::spawn(move || {
            loop {
                let changelist = match client
                    .changes()
                    .list(&cache.get_change_token())
                    .add_scope(drive3::Scope::Full)
                    .page_size(999)
                    .doit()
                {
                    Ok((_, changes)) => changes,
                    Err(cause) => {
                        warn!("Could not get changes because of {}", cause);
                        continue;
                    }
                };

                let changes = match changelist.changes {
                    Some(changes) => changes,
                    None => continue,
                };

                cache.store_files(
                    changes
                        .into_iter()
                        .map(|change| match change.file {
                            Some(file) => Some(cache::File::from(file)),
                            None => None,
                        })
                        .filter(|file| file.is_some())
                        .map(|file| file.unwrap())
                        .collect(),
                );

                match cache.store_change_token(match changelist.next_page_token {
                    Some(token) => token,
                    None => match changelist.new_start_page_token {
                        Some(token) => token,
                        None => {
                            thread::sleep(time::Duration::new(60, 0));
                            continue
                        }
                    }
                }) {
                    Ok(_) => (),
                    Err(cause) => warn!("{}", cause),
                }
            }
        });
    }
}
