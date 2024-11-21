package warehouse

import (
	"iter"

	"github.com/TheBitDrifter/table"
)

var _ iCursor = &Cursor{}

func newCursor(query QueryNode, storage Storage) *Cursor {
	return &Cursor{
		query:   query,
		storage: storage,
	}
}

func (c *Cursor) Next() bool {
	if c.entityIndex < c.remaining {
		c.entityIndex++
		return true
	}
	return c.advance()
}

func (c *Cursor) advance() bool {
	if !c.initialized {
		c.initialize()
	}
	for c.storageIndex < len(c.matchedStorages) {
		c.currentArchetype = c.matchedStorages[c.storageIndex]
		c.remaining = c.currentArchetype.table.Length()

		if c.entityIndex < c.remaining {
			c.entityIndex++
			return true
		}
		c.storageIndex++
		c.entityIndex = 0
	}
	c.Reset()
	return false
}

func (c *Cursor) Entities() iter.Seq2[int, table.Table] {
	return func(yield func(int, table.Table) bool) {
		c.initialize()

		for c.storageIndex < len(c.matchedStorages) {
			c.currentArchetype = c.matchedStorages[c.storageIndex]
			c.remaining = c.currentArchetype.table.Length()

			for c.entityIndex < c.remaining {
				if !yield(c.entityIndex, c.currentArchetype.table) {
					c.Reset()
					return
				}
				c.entityIndex++
			}
			c.entityIndex = 0
			c.storageIndex++
		}
		c.Reset()
	}
}

func (c *Cursor) initialize() {
	if c.initialized {
		return
	}
	c.matchedStorages = make([]archetype, 0)

	// Find all matching archetypes
	for _, arch := range c.storage.(*storage).archetypes.asSlice {
		if c.query.Evaluate(arch, c.storage) {
			c.matchedStorages = append(c.matchedStorages, arch)
		}
	}
	if len(c.matchedStorages) > 0 {
		c.storageIndex = 0
		c.currentArchetype = c.matchedStorages[0]
		c.remaining = c.currentArchetype.table.Length()
	}
	c.initialized = true
}

func (c *Cursor) Reset() {
	c.storageIndex = 0
	c.entityIndex = 0
	c.remaining = 0
	c.matchedStorages = nil
	c.initialized = false
	c.storage.Unlock()
}

func (c *Cursor) CurrentEntity() (int, table.Table) {
	return c.entityIndex, c.currentArchetype.table
}

func (c *Cursor) RemainingInArchetype() int {
	return c.remaining - c.entityIndex
}

func (c *Cursor) TotalMatched() int {
	if !c.initialized {
		c.initialize()
	}
	total := 0
	for _, arch := range c.matchedStorages {
		total += arch.table.Length()
	}
	return total
}
