package main

import (
	"bytes"
	"io"
	"io/ioutil"
)

// Buffer is a stream buffer for the download
type Buffer struct {
	reader *bytes.Reader
}

// NewBuffer creates a new buffer
func NewBuffer(reader io.ReadCloser) (*Buffer, error) {
	defer reader.Close()
	buf, err := ioutil.ReadAll(reader)
	if nil != err {
		return nil, err
	}
	return &Buffer{
		reader: bytes.NewReader(buf),
	}, nil
}

// Close closes all handles and clears the cache
func (b *Buffer) Close() error {
	// TODO: close reader
	return nil
}

// ReadAt reads some bytes
func (b *Buffer) Read(p []byte) (n int, err error) {
	return b.reader.Read(p)
}
