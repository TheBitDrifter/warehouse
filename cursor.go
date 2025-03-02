package warehouse

import (
	"iter"

	"github.com/TheBitDrifter/table"
)

// Ensure Cursor implements iCursor interface
var _ iCursor = &Cursor{}

// iCursor defines the interface for iterating over entities in storage
type iCursor interface {
	Entities() iter.Seq2[int, table.Table]
	Next() bool
}

// Cursor provides iteration over filtered entities in storage
type Cursor struct {
	query            QueryNode
	storage          Storage
	currentArchetype ArchetypeImpl
	storageIndex     int
	entityIndex      int
	remaining        int

	initialized     bool
	matchedStorages []ArchetypeImpl
}

// newCursor creates a new cursor for the given query and storage
func newCursor(query QueryNode, storage Storage) *Cursor {
	return &Cursor{
		query:   query,
		storage: storage,
	}
}

// Next advances to the next entity and returns whether one exists
func (c *Cursor) Next() bool {
	if c.entityIndex < c.remaining {
		c.entityIndex++
		return true
	}
	return c.advance()
}

// advance moves to the next available archetype with entities
func (c *Cursor) advance() bool {
	if !c.initialized {
		c.Initialize()
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

// Entities returns an iterator sequence over entities matching the query
func (c *Cursor) Entities() iter.Seq2[int, table.Table] {
	return func(yield func(int, table.Table) bool) {
		c.Initialize()

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

// Initialize sets up the cursor by finding matching archetypes
func (c *Cursor) Initialize() {
	if c.initialized {
		return
	}

	c.storage.AddLock()
	c.matchedStorages = make([]ArchetypeImpl, 0)

	// Find all matching archetypes
	for _, arch := range c.storage.Archetypes() {
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

// Reset clears cursor state and releases the storage lock
func (c *Cursor) Reset() {
	c.storageIndex = 0
	c.entityIndex = 0
	c.remaining = 0
	c.matchedStorages = nil
	c.initialized = false
	c.storage.PopLock()
}

// CurrentEntity returns the entity at the current cursor position
func (c *Cursor) CurrentEntity() (Entity, error) {
	entry, err := c.currentArchetype.table.Entry(c.entityIndex - 1)
	if err != nil {
		return nil, err
	}
	entityID := entry.ID()
	return c.storage.Entity(int(entityID))
}

// EntityAtOffset returns an entity at the specified offset from current position
func (c *Cursor) EntityAtOffset(offset int) (Entity, error) {
	entry, err := c.currentArchetype.table.Entry(c.entityIndex - 1 + offset)
	if err != nil {
		return nil, err
	}
	entityID := entry.ID()
	return c.storage.Entity(int(entityID))
}

// EntityIndex returns the current entity index within the current archetype
func (c *Cursor) EntityIndex() int {
	return c.entityIndex
}

// RemainingInArchetype returns the number of entities left in the current archetype
func (c *Cursor) RemainingInArchetype() int {
	return c.remaining - c.entityIndex
}

// TotalMatched returns the total number of entities matching the query
func (c *Cursor) TotalMatched() int {
	if !c.initialized {
		c.Initialize()
	}

	total := 0
	for _, arch := range c.matchedStorages {
		total += arch.table.Length()
	}

	c.Reset()
	return total
}
