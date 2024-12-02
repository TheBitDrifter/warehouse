package warehouse

import (
	"iter"

	"github.com/TheBitDrifter/table"
)

type Storage interface {
	Entity(id int) (Entity, error)
	NewEntities(int, ...Component) ([]Entity, error)
	EnqueueNewEntities(int, ...Component) error
	DestroyEntities(...Entity) error
	EnqueueDestroyEntities(...Entity) error
	RowIndexFor(Component) uint32
	Locked() bool
	Lock()
	Unlock()
}

type EntityDestroyCallback func(Entity)

type Entity interface {
	table.Entry
	SetParent(parent Entity, callback EntityDestroyCallback) error
	SetDestroyCallback(EntityDestroyCallback) error
	AddComponent(Component) error
	RemoveComponent(Component) error
	EnqueueAddComponent(Component) error
	EnqueueRemoveComponent(Component) error
}

type Component interface {
	table.ElementType
}

type Archetype interface {
	ID() uint32
	Table() table.Table
}

type Query interface {
	QueryNode
	And(items ...interface{}) QueryNode
	Or(items ...interface{}) QueryNode
	Not(items ...interface{}) QueryNode
}

type QueryNode interface {
	Evaluate(archetype Archetype, storage Storage) bool
}

type iCursor interface {
	Entities() iter.Seq2[int, table.Table]
	Next() bool
}

type Cache[T any] interface {
	GetIndex(string) (int, bool)
	GetItem(int) *T
	GetItem32(uint32) *T
	Register(string, T) (int, error)
}

// Warning: internal Dependencies abound!
type Cursor struct {
	// The query to filter entities
	query QueryNode

	// The storage to iterate over
	storage Storage

	// Current iteration state
	currentArchetype archetype
	storageIndex     int
	entityIndex      int
	remaining        int

	// Initialization state
	initialized     bool
	matchedStorages []archetype
}

type AccessibleComponent[T any] struct {
	Component
	table.Accessor[T] // concrete.
}

type CacheLocation struct {
	Key   string
	Index uint32
}

type SimpleCache[T any] struct {
	items       []T
	itemIndices map[string]int
	maxCapacity int
}
