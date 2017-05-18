[![Build Status](https://travis-ci.org/dweidenfeld/plexdrive.svg?branch=master)](https://travis-ci.org/dweidenfeld/plexdrive)

# Plexdrive
Plexdrive allows you to mount your Google Drive account as fuse filesystem.

The project is comparable to projects like [rclone](https://rclone.org/) or [node-gdrive-fuse](https://github.com/thejinx0r/node-gdrive-fuse), but optimized for media streaming e.g. with plex ;)

I tried using rclone a long time, but got API Quota errors ever day, or more times a day. So I decided to try node-gdrive-fuse. The problem here was, that it missed some of my media files, so I started implementing my own file system library.

_If you like the project, feel free to make a small [donation via PayPal](https://www.paypal.me/dowei). Otherwise support the project by implementing new functions / bugfixes yourself and create pull requests :)_

## Installation
1. First you should install fuse on your system
2. Then you should download the newest release from the [GitHub release page](https://github.com/dweidenfeld/plexdrive/releases).
3. Create your own client id and client secret (see [https://rclone.org/drive/#making-your-own-client-id](https://rclone.org/drive/#making-your-own-client-id)).
4. Run the application like this
```
./plexdrive /path/to/my/mount
```

## Usage
```
Usage of ./plexdrive:
  --chunk-size string
    	The size of each chunk that is downloaded (units: B, K, M, G) (default "5M")
  --clear-chunk-age duration
    	The maximum age of a cached chunk file (default 30m0s)
  --clear-chunk-interval duration
    	The time to wait till clearing the chunk directory (default 1m0s)
  --clear-chunk-max-size string
    	The maximum size of the temporary chunk directory (units: B, K, M, G)
  -c, --config string
    	The path to the configuration directory (default "~/.plexdrive")
  -o, --fuse-options string
    	Fuse mount options (e.g. -fuse-options allow_other,...)
  --gid int
    	Set the mounts GID (-1 = default permissions) (default -1)
  --refresh-interval duration
    	The time to wait till checking for changes (default 5m0s)
  -t, --temp string
    	Path to a temporary directory to store temporary data (default "/tmp")
  --uid int
    	Set the mounts UID (-1 = default permissions) (default -1)
  --umask value
    	Override the default file permissions
  -v, --verbosity int
    	Set the log level (0 = error, 1 = warn, 2 = info, 3 = debug, 4 = trace)
  --version
    	Displays program's version information
```
### Support 
Slack support is available on [our Slack channel](https://plexdrive.slack.com/shared_invite/MTg1NTg5NzY2Njc4LTE0OTUwNDU3NzAtMjJjNWRiMTAxMg). Feel free to ask configuration and setup questions here.

### Supported FUSE mount options
* allow_other
* allow_root
* allow_dev
* allow_non_empty_mount
* allow_suid
* max_readahead=1234
* default_permissions
* excl_create
* fs_name=myname
* local_volume
* writeback_cache
* volume_name=myname
* read_only

### Cache by usage
If you set the --clear-chunk-age to e.g. 24 hours your files will be stored
for 24 hours on your harddisk. This prevents you from downloading the file
everytime it is accessed so will have a faster playback start, avoid stuttering
and spare API calls. 

Everytime a file is accessed it will the caching time will be extended.
E.g. You access a file at 20:00, then it will be deleted on the next day at
20:00. If you access the file e.g. at 18:00 the next day, the file will be
deleted the day after at 18:00 and so on.

# Init files
Personally I start the program with systemd. You can use this configuration
```
[Unit]
Description=Plexdrive
AssertPathIsDirectory=/mnt/drive
After=network-online.target

[Service]
Type=simple
ExecStart=/usr/bin/plexdrive -v 2 /mnt/drive
ExecStop=/bin/fusermount -u /mnt/drive
Restart=on-abort

[Install]
WantedBy=default.target
```

## Crypted mount with rclone
```
[Unit]
Description=Google Drive (rclone)
AssertPathIsDirectory=/mnt/media
After=plexdrive.service

[Service]
Type=simple
ExecStart=/usr/bin/rclone mount --config /root/.config/rclone/rclone.conf --allow-other ldrive: /mnt/media
ExecStop=/bin/fusermount -u /mnt/media
Restart=on-abort

[Install]
WantedBy=default.target
```
