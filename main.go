package main

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"

	"time"

	"strings"

	"github.com/claudetech/loggo"
	. "github.com/claudetech/loggo/default"
	flag "github.com/ogier/pflag"
	"golang.org/x/sys/unix"
)

func main() {
	// get the users home dir
	user, err := user.Current()
	if nil != err {
		panic(fmt.Sprintf("Could not read users homedir %v\n", err))
	}

	// parse the command line arguments
	argLogLevel := flag.IntP("verbosity", "v", 0, "Set the log level (0 = error, 1 = warn, 2 = info, 3 = debug, 4 = trace)")
	argConfigPath := flag.StringP("config", "c", filepath.Join(user.HomeDir, ".plexdrive"), "The path to the configuration directory")
	argTempPath := flag.StringP("temp", "t", os.TempDir(), "Path to a temporary directory to store temporary data")
	argChunkSize := flag.Int64("chunk-size", 5*1024*1024, "The size of each chunk that is downloaded (in byte)")
	argRefreshInterval := flag.Duration("refresh-interval", 5*time.Minute, "The time to wait till checking for changes")
	argClearInterval := flag.Duration("clear-chunk-interval", 1*time.Minute, "The time to wait till clearing the chunk directory")
	argClearChunkAge := flag.Duration("clear-chunk-age", 30*time.Minute, "The maximum age of a cached chunk file")
	argMountOptions := flag.StringP("fuse-options", "o", "", "Fuse mount options (e.g. -fuse-options allow_other,...)")
	argVersion := flag.Bool("version", false, "Displays program's version information")
	argUID := flag.Int64("uid", -1, "Set the mounts UID (-1 = default permissions)")
	argGID := flag.Int64("gid", -1, "Set the mounts GID (-1 = default permissions)")
	flag.Parse()

	// display version information
	if *argVersion {
		fmt.Println("2.0.0")
		return
	}

	// check if mountpoint is specified
	argMountPoint := flag.Arg(0)
	if "" == argMountPoint {
		flag.Usage()
		panic(fmt.Errorf("Mountpoint not specified"))
	}

	// calculate uid / gid
	uid := uint32(unix.Geteuid())
	gid := uint32(unix.Getegid())
	if *argUID > -1 {
		uid = uint32(*argUID)
	}
	if *argGID > -1 {
		gid = uint32(*argGID)
	}

	// parse the mount options
	var mountOptions []string
	if "" != *argMountOptions {
		mountOptions = strings.Split(*argMountOptions, ",")
	}

	// initialize the logger with the specific log level
	var logLevel loggo.Level
	switch *argLogLevel {
	case 0:
		logLevel = loggo.Error
	case 1:
		logLevel = loggo.Warning
	case 2:
		logLevel = loggo.Info
	case 3:
		logLevel = loggo.Debug
	case 4:
		logLevel = loggo.Trace
	default:
		logLevel = loggo.Warning
	}
	Log.SetLevel(logLevel)

	// debug all given parameters
	Log.Debugf("log-level            : %v", logLevel)
	Log.Debugf("config               : %v", *argConfigPath)
	Log.Debugf("temp                 : %v", *argTempPath)
	Log.Debugf("chunk-size           : %v", *argChunkSize)
	Log.Debugf("refresh-interval     : %v", *argRefreshInterval)
	Log.Debugf("clear-chunk-interval : %v", *argClearInterval)
	Log.Debugf("fuse-options         : %v", *argMountOptions)
	Log.Debugf("UID                  : %v", uid)
	Log.Debugf("GID                  : %v", gid)

	// create all directories
	if err := os.MkdirAll(*argConfigPath, 0766); nil != err {
		Log.Errorf("Could not create configuration directory")
		Log.Debugf("%v", err)
		os.Exit(1)
	}
	chunkPath := filepath.Join(*argTempPath, "chunks")
	if err := os.MkdirAll(chunkPath, 0777); nil != err {
		Log.Errorf("Could not create temp chunk directory")
		Log.Debugf("%v", err)
		os.Exit(2)
	}

	// set the global buffer configuration
	SetChunkPath(chunkPath)
	SetChunkSize(*argChunkSize)

	// read the configuration
	configPath := filepath.Join(*argConfigPath, "config.json")
	config, err := ReadConfig(configPath)
	if nil != err {
		config, err = CreateConfig(configPath)
		if nil != err {
			Log.Errorf("Could not read configuration")
			Log.Debugf("%v", err)
			os.Exit(3)
		}
	}

	cache, err := NewCache(*argConfigPath, *argLogLevel > 3)
	if nil != err {
		Log.Errorf("Could not initialize cache")
		Log.Debugf("%v", err)
		os.Exit(4)
	}
	defer cache.Close()

	drive, err := NewDriveClient(config, cache, *argRefreshInterval)
	if nil != err {
		Log.Errorf("Could not initialize Google Drive Client")
		Log.Debugf("%v", err)
		os.Exit(5)
	}

	go CleanChunkDir(chunkPath, *argClearInterval, *argClearChunkAge)
	if err := Mount(drive, argMountPoint, mountOptions, uid, gid); nil != err {
		Log.Debugf("%v", err)
		os.Exit(6)
	}
}
