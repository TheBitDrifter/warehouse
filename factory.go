package warehouse

import "github.com/TheBitDrifter/table"

type factory struct{}

var Factory factory

func (f factory) NewStorage(schema table.Schema) Storage {
	return newStorage(schema)
}

func (f factory) NewQuery() Query {
	return newQuery()
}

func (f factory) NewCursor(query QueryNode, storage Storage) *Cursor {
	return newCursor(query, storage)
}

func FactoryNewComponent[T any]() AccessibleComponent[T] {
	iden := table.FactoryNewElementType[T]()
	return AccessibleComponent[T]{
		Component: iden,
		Accessor:  table.FactoryNewAccessor[T](iden),
	}
}

func FactoryNewCache[T any](cap int) Cache[T] {
	return &SimpleCache[T]{
		itemIndices: make(map[string]int),
		maxCapacity: cap,
	}
}
