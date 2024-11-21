package warehouse

import "github.com/TheBitDrifter/table"

type archetypeID uint32

type archetype struct {
	id    archetypeID
	table table.Table
}

func newArchetype(schema table.Schema, entryIndex table.EntryIndex, id archetypeID, components ...Component) (archetype, error) {
	elementTypes := make([]table.ElementType, len(components))
	for i, comp := range components {
		elementTypes[i] = comp
	}
	tbl, err := table.NewTableBuilder().
		WithSchema(schema).
		WithEntryIndex(entryIndex).
		WithElementTypes(elementTypes...).
		WithEvents(Config.tableEvents).
		Build()
	if err != nil {
		return archetype{}, err
	}
	return archetype{
		table: tbl,
		id:    id,
	}, nil
}

func (a archetype) ID() uint32 {
	return uint32(a.id)
}

func (a archetype) Table() table.Table {
	return a.table
}
