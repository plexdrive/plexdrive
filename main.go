package main

import (
	"flag"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
)

func main() {
	user, err := user.Current()
	if nil != err {
		panic(fmt.Sprintf("Could not read users homedir %v\n", err))
	}

	configPath := flag.String("config", filepath.Join(user.HomeDir, ".plexdrive"), "The path to the configuration directory")
	mountPoint := flag.String("mountpoint", "/tmp/drive", "The destination that should be used for mounting")
	flag.Parse()

	if err := os.MkdirAll(*configPath, os.ModeDir); nil != err {
		panic(fmt.Sprintf("Could not create configuration directory %v\n", configPath))
	}

	config := ReadConfig(filepath.Join(*configPath, "config.json"))

	drive, err := NewDriveClient(config.Accounts, filepath.Join(*configPath, "token.json"))
	if nil != err {
		panic(err)
	}

	cache, err := NewDefaultCache(filepath.Join(*configPath, "cache.db"), drive)
	if nil != err {
		panic(err)
	}

	if err := Mount(config, cache, *mountPoint); nil != err {
		panic(err)
	}

	cache.Close()
}
