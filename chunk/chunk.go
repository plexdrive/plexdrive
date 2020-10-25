package chunk

import (
	"container/list"
	"hash/crc32"
)

// Chunk of memory
type Chunk struct {
	clean bool
	*chunkHeader
	bytes []byte
	item  *list.Element
}

type chunkHeader struct {
	id       RequestID
	size     uint32
	checksum uint32
}

func (c *Chunk) valid(id RequestID) bool {
	if c.id != id {
		return false
	}
	if !c.clean {
		c.clean = c.checksum == c.calculateChecksum()
	}
	return c.clean
}

func (c *Chunk) update(id RequestID, bytes []byte) {
	c.id = id
	c.size = uint32(copy(c.bytes, bytes))
	c.checksum = c.calculateChecksum()
	c.clean = true
}

func (c *Chunk) calculateChecksum() uint32 {
	size := c.size
	if nil == c.bytes || 0 == size {
		return 0
	}
	maxSize := uint32(len(c.bytes))
	if size > maxSize {
		// corrupt size or truncated chunk, fix size
		c.size = maxSize
		return crc32.Checksum(c.bytes, crc32Table)
	}
	return crc32.Checksum(c.bytes[:size], crc32Table)
}
