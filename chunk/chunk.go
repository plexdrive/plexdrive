package chunk

import (
	"container/list"
	"encoding/binary"
	"hash/crc32"
)

// Chunk of memory
type Chunk struct {
	clean  bool
	header []byte
	bytes  []byte
	item   *list.Element
}

func (c *Chunk) ID() (id RequestID) {
	copy(id[:], c.header[:24])
	return
}

func (c *Chunk) Size() uint32 {
	return binary.LittleEndian.Uint32(c.header[24:])
}

func (c *Chunk) Checksum() uint32 {
	return binary.LittleEndian.Uint32(c.header[28:])
}

func (c *Chunk) Valid(id uint64) bool {
	if !c.clean {
		c.clean = c.Checksum() == c.calculateChecksum()
	}
	return c.clean
}

func (c *Chunk) Update(id RequestID, bytes []byte) {
	copy(c.header[:24], id[:])
	size := uint32(copy(c.bytes, bytes))
	binary.LittleEndian.PutUint32(c.header[24:], size)
	checksum := c.calculateChecksum()
	binary.LittleEndian.PutUint32(c.header[28:], checksum)
	c.clean = true
}

func (c *Chunk) calculateChecksum() uint32 {
	size := c.Size()
	if nil == c.bytes || 0 == size {
		return 0
	}
	maxSize := uint32(len(c.bytes))
	if size > maxSize {
		// corrupt size or truncated chunk, fix size
		binary.LittleEndian.PutUint32(c.header[24:], maxSize)
		return crc32.Checksum(c.bytes, crc32Table)
	}
	return crc32.Checksum(c.bytes[:size], crc32Table)
}
