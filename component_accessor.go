package warehouse

func (c AccessibleComponent[T]) GetFromCursor(cursor *Cursor) *T {
	return c.Get(
		cursor.entityIndex-1,
		cursor.currentArchetype.table,
	)
}

func (c AccessibleComponent[T]) GetFromEntity(entity Entity) *T {
	return c.Get(entity.Index(), entity.Table())
}

func (c AccessibleComponent[T]) Check(cursor *Cursor) bool {
	return c.Accessor.Check(cursor.currentArchetype.table)
}
