package main

import (
	"io"
	"os"
	"path/filepath"
	"time"

	. "github.com/claudetech/loggo/default"
)

// CleanChunkDir check frequently the temporary directory and
// cleans old stuff
func CleanChunkDir(chunkDir string, clearInterval, chunkAge time.Duration, chunkSize, maxTempSize int64) {
	if maxTempSize > 0 {
		Log.Info("Using clear-by-size method for chunk cleaning")
		clearBySize(chunkDir, clearInterval)
	} else {
		Log.Info("Using clear-by-interval method for chunk cleaning")
		clearByInterval(chunkDir, clearInterval, chunkAge)
	}
}

// clearBySize clears the chunk dir temporarily and deletes only the oldest files
func clearBySize(chunkDir string, clearInterval time.Duration) {
	for _ = range time.Tick(clearInterval) {
		deleteEmptyDirs(chunkDir)
	}
}

// clearByInterval clears the chunk dir temporarily regardless of the size
func clearByInterval(chunkDir string, clearInterval, chunkAge time.Duration) {
	for _ = range time.Tick(clearInterval) {
		Log.Debugf("Cleaning chunk directory %v", chunkDir)

		filepath.Walk(chunkDir, func(path string, f os.FileInfo, err error) error {
			if path == chunkDir {
				return nil
			}

			now := time.Now()
			if !f.IsDir() {
				if now.Sub(f.ModTime()) > chunkAge {
					if err := os.Remove(path); nil != err {
						Log.Warningf("Could not delete temp file %v", path)
					}
				}
			} else {
				if empty, err := isEmptyDir(path); nil == err && empty {
					if err := os.RemoveAll(path); nil != err {
						Log.Warningf("Could not delete temp dir %v", path)
					}
				}
			}
			return err
		})
	}
}

// deleteEmptyDirs deletes empty directories
func deleteEmptyDirs(dir string) error {
	err := filepath.Walk(dir, func(path string, f os.FileInfo, err error) error {
		if f.IsDir() && path != dir {
			if empty, err := isEmptyDir(path); nil == err && empty {
				Log.Debugf("Cleaning empty directory %v", path)
				if err := os.RemoveAll(path); nil != err {
					Log.Warningf("Could not delete temp dir %v", path)
				}
			}
		}
		return err
	})

	return err
}

// isEmptyDir checks if the directory is empty
func isEmptyDir(name string) (bool, error) {
	f, err := os.Open(name)
	if err != nil {
		return false, err
	}
	defer f.Close()

	_, err = f.Readdirnames(1)
	if err == io.EOF {
		return true, nil
	}
	return false, err
}
