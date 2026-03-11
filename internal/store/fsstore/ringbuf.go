package fsstore

// ringBuffer is a fixed-size circular buffer of strings for event deduplication.
type ringBuffer struct {
	buf   []string
	index map[string]struct{}
	pos   int
	size  int
}

func newRingBuffer(size int) *ringBuffer {
	return &ringBuffer{
		buf:   make([]string, size),
		index: make(map[string]struct{}, size),
		size:  size,
	}
}

// Add inserts a value, evicting the oldest if the buffer is full.
func (r *ringBuffer) Add(val string) {
	// Evict the value being overwritten
	if old := r.buf[r.pos]; old != "" {
		delete(r.index, old)
	}
	r.buf[r.pos] = val
	r.index[val] = struct{}{}
	r.pos = (r.pos + 1) % r.size
}

// Contains returns true if the value is in the buffer.
func (r *ringBuffer) Contains(val string) bool {
	_, ok := r.index[val]
	return ok
}
