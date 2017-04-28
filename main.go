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
	chunkSize := flag.Int64("chunkSize", 5120, "The chunk size to be downloaded in kb")
	tempDir := flag.String("tempDir", os.TempDir(), "Path to a temporary directory, where the chunks are stored")
	debug := flag.Bool("debug", false, "Set debug to true, to get more output (like fuse info)")
	flag.Parse()

	if err := os.MkdirAll(*configPath, os.ModeDir); nil != err {
		panic(fmt.Sprintf("Could not create configuration directory %v\n", configPath))
	}

	config := ReadConfig(filepath.Join(*configPath, "config.json"))
	chunkDir := filepath.Join(*tempDir, "chunks")
	if err := os.MkdirAll(chunkDir, 0777); nil != err {
		panic(err)
	}

	go CleanChunkDir(chunkDir)

	drive, err := NewDriveClient(config.Accounts, filepath.Join(*configPath, "token.json"), chunkDir)
	if nil != err {
		panic(err)
	}

	cache, err := NewDefaultCache(filepath.Join(*configPath, "cache.db"), drive)
	if nil != err {
		panic(err)
	}
	drive.Cache = cache

	if err := Mount(config, cache, *mountPoint, *debug, *chunkSize*1024); nil != err {
		panic(err)
	}

	cache.Close()
}
