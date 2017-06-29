package main

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"

	"time"

	"strings"

	"syscall"

	"os/signal"

	"runtime"

	"github.com/claudetech/loggo"
	. "github.com/claudetech/loggo/default"
	"github.com/dweidenfeld/plexdrive/chunk"
	"github.com/dweidenfeld/plexdrive/config"
	"github.com/dweidenfeld/plexdrive/drive"
	"github.com/dweidenfeld/plexdrive/mount"
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
	argRootNodeID := flag.String("root-node-id", "root", "The ID of the root node to mount (use this for only mount a sub directory)")
	argConfigPath := flag.StringP("config", "c", filepath.Join(user.HomeDir, ".plexdrive"), "The path to the configuration directory")
	argTempPath := flag.StringP("temp", "t", os.TempDir(), "Path to a temporary directory to store temporary data")
	argMongoURL := flag.StringP("mongo-host", "m", "localhost", "MongoDB host")
	argMongoUser := flag.String("mongo-user", "", "MongoDB username")
	argMongoPass := flag.String("mongo-password", "", "MongoDB password")
	argMongoDatabase := flag.String("mongo-database", "plexdrive", "MongoDB database")
	argChunkSize := flag.String("chunk-size", "5M", "The size of each chunk that is downloaded (units: B, K, M, G)")
	argChunkLoadThreads := flag.Int("chunk-load-threads", runtime.NumCPU(), "The number of threads to use for downloading chunks")
	argChunkLoadAhead := flag.Int("chunk-load-ahead", 2, "The number of chunks that should be read ahead")
	argChunkLoadTimeout := flag.Duration("chunk-load-timeout", 10*time.Second, "Duration to wait for a chunk to be loaded")
	argChunkLoadRetries := flag.Int("chunk-load-retries", 3, "Number of retries to load a chunk")
	argMaxChunks := flag.Int("max-chunks", 10, "The maximum number of chunks to be stored on disk")
	argRefreshInterval := flag.Duration("refresh-interval", 5*time.Minute, "The time to wait till checking for changes")
	argMountOptions := flag.StringP("fuse-options", "o", "", "Fuse mount options (e.g. -fuse-options allow_other,...)")
	argVersion := flag.Bool("version", false, "Displays program's version information")
	argUID := flag.Int64("uid", -1, "Set the mounts UID (-1 = default permissions)")
	argGID := flag.Int64("gid", -1, "Set the mounts GID (-1 = default permissions)")
	argUmask := flag.Uint32("umask", 0, "Override the default file permissions")
	// argDownloadSpeedLimit := flag.String("speed-limit", "", "This value limits the download speed, e.g. 5M = 5MB/s per chunk (units: B, K, M, G)")
	flag.Parse()

	// display version information
	if *argVersion {
		fmt.Println("%VERSION%")
		return
	}

	// check if mountpoint is specified
	argMountPoint := flag.Arg(0)
	if "" == argMountPoint {
		flag.Usage()
		fmt.Println()
		panic(fmt.Errorf("Mountpoint not specified"))
	}
	if "" == *argMongoURL {
		flag.Usage()
		fmt.Println()
		panic(fmt.Errorf("MongoDB URL not specified"))
	}
	if "" == *argMongoDatabase {
		flag.Usage()
		fmt.Println()
		panic(fmt.Errorf("MongoDB database not specified"))
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

	// parse filemode
	umask := os.FileMode(*argUmask)

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
	Log.Debugf("verbosity            : %v", logLevel)
	Log.Debugf("root-node-id         : %v", *argRootNodeID)
	Log.Debugf("config               : %v", *argConfigPath)
	Log.Debugf("temp                 : %v", *argTempPath)
	Log.Debugf("mongo-host           : %v", *argMongoURL)
	Log.Debugf("mongo-user           : %v", *argMongoUser)
	Log.Debugf("mongo-password       : %v", *argMongoPass)
	Log.Debugf("mongo-database       : %v", *argMongoDatabase)
	Log.Debugf("chunk-size           : %v", *argChunkSize)
	Log.Debugf("chunk-load-threads   : %v", *argChunkLoadThreads)
	Log.Debugf("chunk-load-ahead     : %v", *argChunkLoadAhead)
	Log.Debugf("chunk-load-timeout   : %v", *argChunkLoadTimeout)
	Log.Debugf("chunk-load-retries   : %v", *argChunkLoadRetries)
	Log.Debugf("max-chunks           : %v", *argMaxChunks)
	Log.Debugf("refresh-interval     : %v", *argRefreshInterval)
	Log.Debugf("fuse-options         : %v", *argMountOptions)
	Log.Debugf("UID                  : %v", uid)
	Log.Debugf("GID                  : %v", gid)
	Log.Debugf("Umask                : %v", umask)
	// Log.Debugf("speed-limit          : %v", *argDownloadSpeedLimit)
	// version missing here

	// create all directories
	if err := os.MkdirAll(*argConfigPath, 0766); nil != err {
		Log.Errorf("Could not create configuration directory")
		Log.Debugf("%v", err)
		os.Exit(1)
	}
	chunkPath := filepath.Join(*argTempPath, "chunks")

	// set the global buffer configuration
	chunkSize, err := parseSizeArg(*argChunkSize)
	if nil != err {
		Log.Errorf("%v", err)
		os.Exit(2)
	}

	// read the configuration
	configPath := filepath.Join(*argConfigPath, "config.json")
	cfg, err := config.Read(configPath)
	if nil != err {
		cfg, err = config.Create(configPath)
		if nil != err {
			Log.Errorf("Could not read configuration")
			Log.Debugf("%v", err)
			os.Exit(3)
		}
	}

	cache, err := drive.NewCache(*argMongoURL, *argMongoUser, *argMongoPass, *argMongoDatabase, *argConfigPath, *argLogLevel > 3)
	if nil != err {
		Log.Errorf("%v", err)
		os.Exit(4)
	}
	defer cache.Close()

	client, err := drive.NewClient(cfg, cache, *argRefreshInterval, *argRootNodeID)
	if nil != err {
		Log.Errorf("%v", err)
		os.Exit(4)
	}

	chunkManager, err := chunk.NewManager(
		chunkPath,
		chunkSize,
		*argChunkLoadAhead,
		*argChunkLoadThreads,
		client.GetNativeClient(),
		*argMaxChunks,
		*argChunkLoadTimeout,
		*argChunkLoadRetries)
	if nil != err {
		Log.Errorf("%v", err)
		os.Exit(4)
	}

	// check os signals like SIGINT/TERM
	checkOsSignals(argMountPoint)
	if err := mount.Mount(client, chunkManager, argMountPoint, mountOptions, uid, gid, umask); nil != err {
		Log.Debugf("%v", err)
		os.Exit(5)
	}
}

func checkOsSignals(mountpoint string) {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT)

	go func() {
		for sig := range signals {
			if sig == syscall.SIGINT {
				if err := mount.Unmount(mountpoint, false); nil != err {
					Log.Warningf("%v", err)
				}
			}
		}
	}()
}

func parseSizeArg(input string) (int64, error) {
	if "" == input {
		return 0, nil
	}

	suffix := input[len(input)-1]
	suffixLen := 1
	var multiplier float64
	switch suffix {
	case '0', '1', '2', '3', '4', '5', '6', '7', '8', '9', '.':
		suffixLen = 0
	case 'b', 'B':
		multiplier = 1
	case 'k', 'K':
		multiplier = 1024
	case 'm', 'M':
		multiplier = 1024 * 1024
	case 'g', 'G':
		multiplier = 1024 * 1024 * 1024
	default:
		return 0, fmt.Errorf("Invalid unit %v for %v", suffix, input)
	}
	input = input[:len(input)-suffixLen]
	value, err := strconv.ParseFloat(input, 64)
	if nil != err {
		Log.Debugf("%v", err)
		return 0, fmt.Errorf("Could not parse numeric value %v", input)
	}
	if value < 0 {
		return 0, fmt.Errorf("Numeric value must not be negative %v", input)
	}
	value *= multiplier
	return int64(value), nil
}
