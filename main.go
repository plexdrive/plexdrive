package main

import (
	"flag"

	"sh0k.de/plexdrive/config"
	"sh0k.de/plexdrive/mount"
	"sh0k.de/plexdrive/plexdrive"
)

func main() {
	configPath := flag.String("config", "config.json", "The path to the configuration file")
	tokenPath := flag.String("tokenpath", "token.json", "The path to store the token file")
	mountPoint := flag.String("mountpoint", "/tmp/drive", "The destination that should be used for mounting")
	driveDir := flag.String("driveDir", "/", "The drive folder that should be mounted")
	flag.Parse()

	config := config.ReadConfig(*configPath)

	drive, err := plexdrive.New(config.Accounts, *tokenPath, *driveDir)
	if nil != err {
		panic(err)
	}
	if err := mount.Mount(config, drive, *mountPoint); nil != err {
		panic(err)
	}
}
