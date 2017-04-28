package main

import (
	"time"
)

// APIClient is the API client interface describing all necessary methods
type APIClient interface {
	// GetObject gets one object by id
	GetObject(id string) (*APIObject, error)

	// GetObjectsByParent gets all files under a parent folder
	GetObjectsByParent(parentID string) ([]*APIObject, error)

	// Open a file handle
	Open(object *APIObject, chunkSize int64) (*Buffer, error)
}

// APIObject is a object returned by the API
type APIObject struct {
	InternalID  int
	ID          string
	Parents     []string
	Name        string
	IsDir       bool
	Size        uint64
	MTime       time.Time
	DownloadURL string
}
