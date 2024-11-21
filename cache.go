package warehouse

import "fmt"

var _ Cache[any] = &SimpleCache[any]{}

func (c *SimpleCache[T]) GetIndex(key string) (int, bool) {
	index, ok := c.itemIndices[key]
	return index, ok
}

func (c *SimpleCache[T]) GetItem(index int) *T {
	item := &c.items[index]
	return item
}

func (c *SimpleCache[T]) GetItem32(index uint32) *T {
	item := &c.items[index]
	return item
}

func (c *SimpleCache[T]) Register(key string, item T) (int, error) {
	// TODO: PROPER ERROR
	if len(c.itemIndices) >= c.maxCapacity {
		return -1, fmt.Errorf("cache at maximum capacity (%d)", c.maxCapacity)
	}

	idx := len(c.items)
	c.itemIndices[key] = idx
	c.items = append(c.items, item) // Use append instead of direct assignment

	return idx, nil
}

func (c *SimpleCache[T]) Clear() {
	c.items = make([]T, c.maxCapacity)
	c.itemIndices = make(map[string]int)
}
