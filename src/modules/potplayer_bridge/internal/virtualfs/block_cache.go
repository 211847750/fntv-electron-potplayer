package virtualfs

import "sync"

const (
	mediaBlockSize          int64 = 1 << 20
	mediaBlockCacheCapacity       = 2
)

type mediaCacheBlock struct {
	start    int64
	data     []byte
	lastUsed uint64
}

type mediaBlockCache struct {
	mu     sync.Mutex
	clock  uint64
	blocks []mediaCacheBlock
}

func (c *mediaBlockCache) readAt(dst []byte, offset int64) int {
	if len(dst) == 0 {
		return 0
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	for i := range c.blocks {
		block := &c.blocks[i]
		blockEnd := block.start + int64(len(block.data))
		if offset < block.start || offset >= blockEnd {
			continue
		}

		c.clock++
		block.lastUsed = c.clock
		return copy(dst, block.data[offset-block.start:])
	}
	return 0
}

func (c *mediaBlockCache) store(start int64, data []byte) {
	if len(data) == 0 {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.clock++
	for i := range c.blocks {
		if c.blocks[i].start == start {
			c.blocks[i].data = data
			c.blocks[i].lastUsed = c.clock
			return
		}
	}

	block := mediaCacheBlock{start: start, data: data, lastUsed: c.clock}
	if len(c.blocks) < mediaBlockCacheCapacity {
		c.blocks = append(c.blocks, block)
		return
	}

	lru := 0
	for i := 1; i < len(c.blocks); i++ {
		if c.blocks[i].lastUsed < c.blocks[lru].lastUsed {
			lru = i
		}
	}
	c.blocks[lru] = block
}

func mediaBlockStart(offset int64) int64 {
	return offset / mediaBlockSize * mediaBlockSize
}
