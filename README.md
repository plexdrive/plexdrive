<a href="https://github.com/dweidenfeld/plexdrive"><img src="logo/banner.png" alt="Plexdrive" /></a>
[![Build Status](https://travis-ci.org/dweidenfeld/plexdrive.svg?branch=master)](https://travis-ci.org/dweidenfeld/plexdrive)

__Plexdrive__ allows you to mount your Google Drive account as read-only fuse filesystem, with direct delete option on the filesystem.

The project is comparable to projects like [rclone](https://rclone.org/), 
[google-drive-ocamlfuse](https://github.com/astrada/google-drive-ocamlfuse) or 
[node-gdrive-fuse](https://github.com/thejinx0r/node-gdrive-fuse), 
but optimized for media streaming e.g. with plex ;)

Please note that plexdrive doesn't currently support writes (adding new files or modifications), it only supports reading existing files and deletion. 

I tried using rclone for a long time, but got API Quota errors every day and/or multiple times per day, so I decided to try node-gdrive-fuse. The problem here was that it missed some of my media files, so as a result I started implementing my own file system library.

_If you like the project, feel free to make a small [donation via PayPal](https://www.paypal.me/dowei). Otherwise support the project by implementing new functions / bugfixes yourself and create pull requests :)_

## Installation
1. First you need to install fuse on your system 
2. Then you should download the newest release from the [GitHub release page](https://github.com/dweidenfeld/plexdrive/releases).
3. Create your own client id and client secret (see [https://rclone.org/drive/#making-your-own-client-id](https://rclone.org/drive/#making-your-own-client-id)).
4. Sample command line for plexdrive
```
./plexdrive mount -c /root/.plexdrive -o allow_other /mnt/plexdrive
```

### Crypted mount with rclone
You can use [this tutorial](TUTORIAL.md) for instruction how to mount an encrypted rclone mount.

## Usage
```
Usage of ./plexdrive mount:
  --cache-file string
    	Path the the cache file (default "~/.plexdrive/cache.bolt")
  --chunk-check-threads int
    	The number of threads to use for checking chunk existence (default 2)
  --chunk-load-ahead int
    	The number of chunks that should be read ahead (default 3)
  --chunk-load-threads int
    	The number of threads to use for downloading chunks (default 2)
  --chunk-size string
    	The size of each chunk that is downloaded (units: B, K, M, G) (default "10M")
  -c, --config string
    	The path to the configuration directory (default "~/.plexdrive")
  --drive-id string
    	The ID of the shared drive to mount (including team drives)
  -o, --fuse-options string
    	Fuse mount options (e.g. -fuse-options allow_other,...)
  --gid int
    	Set the mounts GID (-1 = default permissions) (default -1)
  --max-chunks int
    	The maximum number of chunks to be stored on disk (default 10)
  --refresh-interval duration
    	The time to wait till checking for changes (default 1m0s)
  --root-node-id string
    	The ID of the root node to mount (use this for only mount a sub directory) (default "root")
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
Slack support is available on [our Slack channel](https://join.slack.com/t/plexdrive/shared_invite/MjM2MTMzMjY2MTc5LTE1MDQ2MDE4NDQtOTc0N2RiY2UxNw). 
Feel free to ask configuration and setup questions here.

### Supported FUSE mount options
* allow_other
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


### Root-Node-ID
You can use the option `root-node-id` to specify a folder id that should be mounted as
the root folder. This option will not prevent plexdrive from getting the changes for your
whole Google Drive structure. It will only "display" another folder as root instead of the
real root folder.
Don't expect any performance improvement or something else. This option is only for your
personal folder structuring.

#### Team Drive
You can pass the ID of a Team Drive as `drive-id` to get access to a Team drive, here's how:
* Open the Team Drive in your browser
* Note the format of the URL: https://drive.google.com/drive/u/0/folders/ABC123qwerty987
* The `drive-id` of this Team Drive is `ABC123qwerty987`
* Pass it with `--drive-id=ABC123qwerty987` argument to your `plexdrive mount` command

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
