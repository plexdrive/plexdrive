package main

import (
	"io"
	"time"
)

// APIClient is the API client interface describing all necessary methods
type APIClient interface {
	// GetObject gets one object by id
	GetObject(id string) (*APIObject, error)

	// GetObjectsByParent gets all files under a parent folder
	GetObjectsByParent(parentID string) ([]*APIObject, error)

	// TODO: add GetObjectsSince()

	// Download opens the file handle
	Download(id string) (io.ReadCloser, error)

	// Open a file handle
	Open(id string) error

	// Release close a file handle
	Release(id string) error
}

// APIObject is a object returned by the API
type APIObject struct {
	InternalID int
	ID         string
	Parents    []string
	Name       string
	IsDir      bool
	Size       uint64
	MTime      time.Time
}
