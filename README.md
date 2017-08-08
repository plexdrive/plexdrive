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
