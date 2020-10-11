package chunk

import (
	"encoding/binary"
	"hash/crc32"
)

// Chunk of memory
type Chunk struct {
	clean  bool
	header []byte
	bytes  []byte
}

func (c *Chunk) ID() uint64 {
	return binary.LittleEndian.Uint64(c.header[0:])
}

func (c *Chunk) Size() uint32 {
	return binary.LittleEndian.Uint32(c.header[8:])
}

func (c *Chunk) Checksum() uint32 {
	return binary.LittleEndian.Uint32(c.header[12:])
}

func (c *Chunk) Valid(id uint64) bool {
	if !c.clean {
		c.clean = c.Checksum() == c.calculateChecksum()
	}
	return c.clean
}

func (c *Chunk) Update(id uint64, bytes []byte) {
	binary.LittleEndian.PutUint64(c.header[0:], id)
	size := uint32(copy(c.bytes, bytes))
	binary.LittleEndian.PutUint32(c.header[8:], size)
	checksum := c.calculateChecksum()
	binary.LittleEndian.PutUint32(c.header[12:], checksum)
	c.clean = true
}

func (c *Chunk) calculateChecksum() uint32 {
	size := c.Size()
	if nil == c.bytes || 0 == size {
		return 0
	}
	if maxSize := uint32(len(c.bytes)); size > maxSize {
		// corrupt size or truncated chunk, fix size
		binary.LittleEndian.PutUint32(c.header[8:], maxSize)
		return crc32.Checksum(c.bytes, crc32Table)
	}
	return crc32.Checksum(c.bytes[:size], crc32Table)
}
