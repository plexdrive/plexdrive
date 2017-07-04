package main

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"

	elastic "gopkg.in/olivere/elastic.v5"
	elogrus "gopkg.in/sohlich/elogrus.v2"

	"time"

	"strings"

	"syscall"

	"os/signal"

	"runtime"

	"github.com/Sirupsen/logrus"
	"github.com/dweidenfeld/plexdrive/alog"
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
	argLogLevel := flag.IntP("verbosity", "v", 0, "Set the log level (0 = error, 1 = warn, 2 = info, 3 = debug, 4 = trace, 5 = trace + fuse)")
	argRootNodeID := flag.String("root-node-id", "root", "The ID of the root node to mount (use this for only mount a sub directory)")
	argConfigPath := flag.StringP("config", "c", filepath.Join(user.HomeDir, ".plexdrive"), "The path to the configuration directory")
	argTempPath := flag.StringP("temp", "t", os.TempDir(), "Path to a temporary directory to store temporary data")
	argLogFormat := flag.String("log-format", "console", "Logging format (console, json, elastic)")
	argLogElasticURL := flag.String("log-elastic-url", "http://localhost:9200", "Elasticsearch URL")
	argLogElasticUser := flag.String("log-elastic-user", "elastic", "Elasticsearch username")
	argLogElasticPass := flag.String("log-elastic-pass", "changeme", "Elasticsearch password")
	argLogElasticIndex := flag.String("log-elastic-index", "plexdrive", "Elasticsearch index")
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
	alog.Start()
	var logLevel logrus.Level
	fuseLogging := false
	switch *argLogLevel {
	case 0:
		logLevel = logrus.FatalLevel
	case 1:
		logLevel = logrus.ErrorLevel
	case 2:
		logLevel = logrus.WarnLevel
	case 3:
		logLevel = logrus.InfoLevel
	case 4:
		logLevel = logrus.DebugLevel
	case 5:
		logLevel = logrus.DebugLevel
		fuseLogging = true
	default:
		logLevel = logrus.WarnLevel
	}
	logger := logrus.New()
	alog.SetLogger(logger)
	logger.Level = logLevel

	switch *argLogFormat {
	case "json":
		logger.Formatter = &logrus.JSONFormatter{}
	case "elastic":
		client, err := elastic.NewClient(
			elastic.SetURL(*argLogElasticURL),
			elastic.SetBasicAuth(*argLogElasticUser, *argLogElasticPass),
			elastic.SetSniff(false))
		if err != nil {
			flag.Usage()
			alog.Panic(nil, err)
		}
		hook, err := elogrus.NewElasticHook(client, *argLogElasticURL, logLevel, *argLogElasticIndex)
		if err != nil {
			flag.Usage()
			alog.Panic(nil, err)
		}
		logger.Hooks.Add(hook)
	}

	// debug all given parameters
	pLog := logrus.WithField("verbosity", logLevel)
	pLog = pLog.WithField("root-node-id", *argRootNodeID)
	pLog = pLog.WithField("config", *argConfigPath)
	pLog = pLog.WithField("temp", *argTempPath)
	if *argLogFormat == "elastic" {
		pLog = pLog.WithField("log-elastic-url", *argLogElasticURL)
		pLog = pLog.WithField("log-elastic-user", *argLogElasticUser)
		pLog = pLog.WithField("log-elastic-index", *argLogElasticIndex)
	}
	pLog = pLog.WithField("chunk-size", *argChunkSize)
	pLog = pLog.WithField("chunk-load-threads", *argChunkLoadThreads)
	pLog = pLog.WithField("chunk-load-ahead", *argChunkLoadAhead)
	pLog = pLog.WithField("chunk-load-timeout", *argChunkLoadTimeout)
	pLog = pLog.WithField("chunk-load-retries", *argChunkLoadRetries)
	pLog = pLog.WithField("max-chunks", *argMaxChunks)
	pLog = pLog.WithField("refresh-interval", *argRefreshInterval)
	pLog = pLog.WithField("fuse-options", *argMountOptions)
	pLog = pLog.WithField("UID", uid)
	pLog = pLog.WithField("GID", gid)
	pLog = pLog.WithField("umask", umask)
	pLog.Info("Parameters")
	// Log.Debugf("speed-limit          : %v", *argDownloadSpeedLimit)
	// version missing here

	// create all directories
	if err := os.MkdirAll(*argConfigPath, 0766); nil != err {
		alog.Info(map[string]interface{}{
			"Error": err,
		}, "Could not create configuration directory")
		os.Exit(1)
	}
	chunkPath := filepath.Join(*argTempPath, "chunks")

	// set the global buffer configuration
	chunkSize, err := parseSizeArg(*argChunkSize)
	if nil != err {
		// log.Fatalf("%v", err)
		os.Exit(2)
	}

	// read the configuration
	configPath := filepath.Join(*argConfigPath, "config.json")
	cfg, err := config.Read(configPath)
	if nil != err {
		cfg, err = config.Create(configPath)
		if nil != err {
			// log.Infof("%v", err)
			// log.Fatalf("Could not read configuration")
			os.Exit(3)
		}
	}

	cache, err := drive.NewCache(*argConfigPath, *argLogLevel > 3)
	if nil != err {
		// log.Fatalf("%v", err)
		os.Exit(4)
	}
	defer cache.Close()

	client, err := drive.NewClient(cfg, cache, *argRefreshInterval, *argRootNodeID)
	if nil != err {
		// log.Fatalf("%v", err)
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
		// log.Fatalf("%v", err)
		os.Exit(4)
	}

	// check os signals like SIGINT/TERM
	checkOsSignals(argMountPoint)
	if err := mount.Mount(client, chunkManager, argMountPoint, mountOptions, uid, gid, umask, fuseLogging); nil != err {
		// log.Fatalf("%v", err)
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
					// log.Errorf("%v", err)
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
		// log.Infof("%v", err)
		return 0, fmt.Errorf("Could not parse numeric value %v", input)
	}
	if value < 0 {
		return 0, fmt.Errorf("Numeric value must not be negative %v", input)
	}
	value *= multiplier
	return int64(value), nil
}
