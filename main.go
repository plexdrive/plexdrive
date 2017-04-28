package main

import (
	"flag"
	"fmt"
	"os"
	"os/user"
	"path/filepath"

	"github.com/claudetech/loggo"
	. "github.com/claudetech/loggo/default"
)

func main() {
	// get the users home dir
	user, err := user.Current()
	if nil != err {
		panic(fmt.Sprintf("Could not read users homedir %v\n", err))
	}

	// parse the command line arguments
	argLogLevel := flag.Int("log-level", 0, "Set the log level (0 = error, 1 = warn, 2 = info, 3 = debug, 4 = trace)")
	argConfigPath := flag.String("config", filepath.Join(user.HomeDir, ".plexdrive"), "The path to the configuration directory")
	argTempPath := flag.String("temp", os.TempDir(), "Path to a temporary directory to store temporary data")
	argChunkSize := flag.Int64("chunk-size", 5120, "The size of each chunk that is downloaded (in kb)")
	flag.Parse()

	// check if mountpoint is specified
	argMountPoint := flag.Arg(0)
	if "" == argMountPoint {
		flag.Usage()
		panic(fmt.Errorf("Mountpoint not specified"))
	}

	// initialize the logger with the specific log level
	var logLevel loggo.Level
	switch *argLogLevel {
	case 1:
		logLevel = loggo.Warning
	case 2:
		logLevel = loggo.Info
	case 3:
		logLevel = loggo.Debug
	case 4:
		logLevel = loggo.Trace
	default:
		logLevel = loggo.Error
	}
	Log.SetLevel(logLevel)

	// debug all given parameters
	Log.Debugf("log-level  : %v", logLevel)
	Log.Debugf("config     : %v", *argConfigPath)
	Log.Debugf("temp       : %v", *argTempPath)
	Log.Debugf("chunk-size : %v", *argChunkSize)

	// create all directories
	if err := os.MkdirAll(*argConfigPath, 0644); nil != err {
		Log.Errorf("Could not create configuration directory")
		Log.Debugf("%v", err)
		os.Exit(1)
	}
	chunkPath := filepath.Join(*argTempPath, "chunks")
	if err := os.MkdirAll(chunkPath, 0644); nil != err {
		Log.Errorf("Could not create temp chunk directory")
		Log.Debugf("%v", err)
		os.Exit(2)
	}

	// read the configuration
	config, err := ReadConfig(filepath.Join(*argConfigPath, "config.json"))
	if nil != err {
		Log.Errorf("Could not read configuration")
		Log.Debugf("%v", err)
		os.Exit(3)
	}

	cache, err := NewCache(filepath.Join(*argConfigPath, "cache"))
	if nil != err {
		Log.Errorf("Could not initialize cache")
		Log.Debugf("%v", err)
		os.Exit(4)
	}
	defer cache.Close()

	_, err = NewDriveClient(config, cache)
	if nil != err {
		Log.Errorf("Could not initialize Google Drive Client")
		Log.Debugf("%v", err)
		os.Exit(5)
	}

	// drive.Cache = cache

	// go CleanChunkDir(chunkDir)
	// if err := Mount(config, cache, *mountPoint, *debug, *chunkSize*1024); nil != err {
	// 	panic(err)
	// }

	// cache.Close()
}
