use clap;

mod init;
mod mount;

/// Initialize the application so that plexdrive can authorize against Google Drive
pub fn init<'a>(params: clap::ArgMatches<'a>) {
    let command_params = params.subcommand_matches("init").expect("Could not parse command parameters");
    let config_dir = params.value_of("config").expect("Could not read config directory");
    let client_id = command_params.value_of("client_id").expect("Could not read client id");
    let client_secret = command_params.value_of("client_secret").expect("Could not read client secret");

    debug!("Config       : {}", config_dir);
    debug!("ClientID     : {}", client_id);
    debug!("ClientSecret : {}", client_secret);

    init::execute(config_dir, client_id, client_secret);
}

/// Mount Google Drive to the local file system
pub fn mount<'a>(params: clap::ArgMatches<'a>) {
    let command_params = params.subcommand_matches("mount").expect("Could not parse command parameters");
    let config_dir = params.value_of("config").expect("Could not read config directory");
    let mount_path = command_params.value_of("mount_path").expect("Could not read mount path");
    let uid = command_params.value_of("uid").expect("Could not read uid");
    let gid = command_params.value_of("gid").expect("Could not read gid");

    debug!("Config       : {}", config_dir);
    debug!("MountPath    : {}", mount_path);

    mount::execute(
        config_dir, 
        mount_path, 
        uid.parse().expect("Could not parse uid"), 
        gid.parse().expect("Could not parse gid")
    );
}