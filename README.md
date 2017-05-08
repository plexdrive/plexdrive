![https://travis-ci.org/dweidenfeld/plexdrive](https://travis-ci.org/dweidenfeld/plexdrive.svg?branch=master "Travis Build")

# Plexdrive
Plexdrive allows you to mount your Google Drive account as fuse filesystem.

The project is comparable to projects like [rclone](https://rclone.org/) or [node-gdrive-fuse](https://github.com/thejinx0r/node-gdrive-fuse), but optimized for media streaming e.g. with plex ;)

I tried using rclone a long time, but got API Quota errors ever day, or more times a day. So I decided to try node-gdrive-fuse. The problem here was, that it missed some of my media files, so I started implementing my own file system library.

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
  -chunk-size int
    	The size of each chunk that is downloaded (in byte) (default 5242880)
  -clear-chunk-interval duration
    	The number of minutes to wait till clearing the chunk directory (default 1m0s)
  -config string
    	The path to the configuration directory (default "/home/user/.plexdrive")
  -fuse-options string
    	Fuse mount options (e.g. -fuse-options allow_other,...)
  -log-level int
    	Set the log level (0 = error, 1 = warn, 2 = info, 3 = debug, 4 = trace)
  -refresh-interval duration
    	The number of minutes to wait till checking for changes (default 5m0s)
  -temp string
    	Path to a temporary directory to store temporary data (default "/tmp")
```

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

# Init files
Personally I start the program with systemd. You can use this configuration
```
[Unit]
Description=Plexdrive
AssertPathIsDirectory=/mnt/drive
After=network-online.target

[Service]
Type=simple
ExecStart=/usr/bin/plexdrive -log-level 2 /mnt/drive
ExecStop=/bin/fusermount -u /mnt/drive
Restart=on-abort

[Install]
WantedBy=default.target
```
