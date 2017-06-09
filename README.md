[![Build Status](https://travis-ci.org/dweidenfeld/plexdrive.svg?branch=master)](https://travis-ci.org/dweidenfeld/plexdrive)

# Plexdrive
Plexdrive allows you to mount your Google Drive account as read-only fuse filesystem.

The project is comparable to projects like [rclone](https://rclone.org/), 
[google-drive-ocamlfuse](https://github.com/astrada/google-drive-ocamlfuse) or 
[node-gdrive-fuse](https://github.com/thejinx0r/node-gdrive-fuse), 
but optimized for media streaming e.g. with plex ;)

Please note that plexdrive doesn't currently support writes (adding new files or modifications), it only supports reading existing files and deletion. 

I tried using rclone for a long time, but got API Quota errors every day and/or multiple times per day, so I decided to try node-gdrive-fuse. The problem here was that it missed some of my media files, so as a result I started implementing my own file system library.

_If you like the project, feel free to make a small [donation via PayPal](https://www.paypal.me/dowei). Otherwise support the project by implementing new functions / bugfixes yourself and create pull requests :)_

## Installation
1. First you need to install fuse and mongodb on your system 
2. Then you should download the newest release from the [GitHub release page](https://github.com/dweidenfeld/plexdrive/releases).
3. Create your own client id and client secret (see [https://rclone.org/drive/#making-your-own-client-id](https://rclone.org/drive/#making-your-own-client-id)).
4. Run the application like this
```
./plexdrive -m localhost /path/to/my/mount
```

### Crypted mount with rclone
You can use [this tutorial](TUTORIAL.md) for instruction how to mount an encrypted rclone mount.

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
  --mongo-database string
    	MongoDB database (default "plexdrive")
  -m, --mongo-host string
    	MongoDB host (default "localhost")
  --mongo-password string
    	MongoDB password
  --mongo-user string
    	MongoDB username
  --refresh-interval duration
    	The time to wait till checking for changes (default 5m0s)
  --root-node-id string
    	The ID of the root node to mount (use this for only mount a sub directory) (default "root")
  --speed-limit string
    	This value limits the download speed, e.g. 5M = 5MB/s per chunk (units: B, K, M, G)
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
Slack support is available on [our Slack channel](https://plexdrive.slack.com/shared_invite/MTg1NTg5NzY2Njc4LTE0OTUwNDU3NzAtMjJjNWRiMTAxMg). 
Feel free to ask configuration and setup questions here.

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

If you activate the option `clear-chunk-max-size` you will automatically disable
the cache cleaning by time. So it will only delete the oldest chunk file when it 
needs the space.

**This function does not limit the storage to the given size**. It will only say
"if you reach the given limit, check if you can clean up old stuff". So if you have
of at most 60gb to be sure it will not override the 100gb limit. The implementation is 
a limit of e.g. 100gb available for chunks, you should specify the clear-chunk-max-size 
done that way, because a hard checking routine could make the playback unstable and 
present buffering because the cleaning of the old chunks off the file system is a low 
priority over streaming your files.


### Root-Node-ID
You can use the option `root-node-id` to specify a folder id that should be mounted as
the root folder. This option will not prevent plexdrive from getting the changes for your
whole Google Drive structure. It will only "display" another folder as root instead of the
real root folder.
Don't expect any performance improvement or something else. This option is only for your
personal folder structuring.

# Contribute
If you want to support the project by implementing functions / fixing bugs
yourself feel free to do so!

1. Fork the repository
2. Clone it to your [golang workspace](https://golang.org/doc/code.html) $GOPATH/src/github.com/username/plexdrive
3. Implement your changes
4. Test your changes (e.g. `go build && ./plexdrive -v3 /tmp/drive`)
5. Format everything with [gofmt](https://golang.org/cmd/gofmt/) (
(I recommend working with [VSCode](https://code.visualstudio.com/) and [VSCode-Go](https://github.com/lukehoban/vscode-go))
6. Create a pull request
