#[macro_use]
extern crate log;
extern crate env_logger;
extern crate clap;
extern crate serde;
extern crate serde_json;
#[macro_use]
extern crate serde_derive;
extern crate hyper;
extern crate hyper_rustls;
extern crate yup_oauth2;
extern crate google_drive3;
extern crate rusqlite;
extern crate chrono;
extern crate fuse;
extern crate libc;
extern crate time;

mod commands;
mod config;
mod api;
mod cache;
mod fs;

use std::env;
use clap::{App, Arg, SubCommand};

fn main() {
    let home_dir_path = env::home_dir().expect("Could not get home directory");
    let home_dir = home_dir_path.to_string_lossy();

    let default_config_dir = format!("{}/.config/plexdrive", home_dir);

    let mut usage = App::new("plexdrive")
        .version("1.0.0")
        .author("Dominik Weidenfeld <dominik@sh0k.de>")
        .about(
            "A mounting tool for Google Drive, optimized for media streaming",
        )
        .arg(Arg::with_name("verbosity")
            .short("v")
            .multiple(true)
            .help("Set the level of verbosity"))
        .arg(Arg::with_name("config")
            .short("c")
            .takes_value(true)
            .default_value(&default_config_dir)
            .help("The configuration directory"))
        .subcommand(
            SubCommand::with_name("init")
                .about("initialize the application (authorization etc.)")
                .arg(Arg::with_name("client_id")
                    .long("client-id")
                    .takes_value(true)
                    .help("The Google Drive Client ID")
                    .required(true))
                .arg(Arg::with_name("client_secret")
                    .long("client-secret")
                    .takes_value(true)
                    .help("The Google Drive Client Secret")
                    .required(true)))
        .subcommand(
            SubCommand::with_name("mount")
                .about("mount Google Drive to the folder you specify")
                .arg(Arg::with_name("mount_path")
                    .index(1)
                    .help("The destination mount path")
                    .required(true)));
    let matches = usage.clone().get_matches();

    // initialize logger
    match matches.occurrences_of("verbosity") {
        1 => env::set_var("RUST_LOG", "plexdrive=error"),
        2 => env::set_var("RUST_LOG", "plexdrive=warn"),
        3 => env::set_var("RUST_LOG", "plexdrive=info"),
        4 => env::set_var("RUST_LOG", "plexdrive=debug"),
        5 => env::set_var("RUST_LOG", "plexdrive=trace"),
        0|_ => env::set_var("RUST_LOG", "plexdrive=warn")
    }
    env_logger::init().expect("Could not initialize logging system");

    // match subcommands
    if matches.subcommand_matches("init").is_some() {
        commands::init(matches);
    } else if matches.subcommand_matches("mount").is_some() {
        commands::mount(matches);
    } else {
        usage.print_help().expect("Could not print usage");
    }
}