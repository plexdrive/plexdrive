package main

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"time"
)

// CleanChunkDir check frequently the temporary directory and
// cleans old stuff
func CleanChunkDir(chunkDir string) {
	for _ = range time.Tick(1 * time.Minute) {
		filepath.Walk(chunkDir, func(path string, f os.FileInfo, err error) error {
			if path == chunkDir {
				return nil
			}

			now := time.Now()
			if !f.IsDir() {
				if now.Sub(f.ModTime()) > 10*time.Minute {
					if err := os.Remove(path); nil != err {
						log.Printf("Could not delete temp file %v", path)
					}
				}
			} else {
				if empty, err := isEmptyDir(path); nil == err && empty {
					if err := os.RemoveAll(path); nil != err {
						log.Printf("Could not delete temp dir %v", path)
					}
				}
			}
			return nil
		})
	}
}

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
