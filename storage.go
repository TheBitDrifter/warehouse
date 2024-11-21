package warehouse

import (
	"fmt"

	"github.com/TheBitDrifter/mask"
	"github.com/TheBitDrifter/table"
)

var _ Storage = &storage{}

var mainIndex = table.Factory.NewEntryIndex()

type storage struct {
	locked     bool
	schema     table.Schema
	archetypes *archetypes
	opQueue    opQueue
}

type archetypes struct {
	nextID           archetypeID
	asSlice          []archetype
	idsGroupedByMask map[mask.Mask]archetypeID
}

func newStorage(schema table.Schema) Storage {
	archetypes := &archetypes{
		nextID:           1,
		idsGroupedByMask: make(map[mask.Mask]archetypeID),
	}
	storage := &storage{
		archetypes: archetypes,
		schema:     schema,
	}
	return storage
}

func (sto *storage) NewEntities(n int, components ...Component) ([]Entity, error) {
	if sto.locked {
		return nil, LockedStorageError{}
	}
	var entityMask mask.Mask
	for _, component := range components {
		sto.schema.Register(component)
		bit := sto.schema.RowIndexFor(component)
		entityMask.Mark(bit)
	}
	var entityArchetype archetype
	id, archetypeFound := sto.archetypes.idsGroupedByMask[entityMask]

	if archetypeFound {
		found := sto.archetypes.asSlice[id-1]
		entityArchetype = found
	}
	if !archetypeFound {
		created, err := newArchetype(sto.schema, mainIndex, sto.archetypes.nextID, components...)
		if err != nil {
			return nil, err
		}
		entityArchetype = created

		sto.archetypes.asSlice = append(sto.archetypes.asSlice, entityArchetype)
		sto.archetypes.nextID++
	}
	entries, err := entityArchetype.table.NewEntries(n)
	if err != nil {
		return nil, err
	}
	entities := make([]Entity, len(entries))
	for i, entry := range entries {
		entities[i] = &entity{
			Entry: entry,
			sto:   sto,
		}
	}
	return entities, nil
}

func (sto *storage) RowIndexFor(c Component) uint32 {
	return sto.schema.RowIndexFor(c)
}

func (sto *storage) Locked() bool {
	return sto.locked
}

func (sto *storage) Lock() {
	sto.locked = true
}

func (sto *storage) Unlock() error {
	err := sto.processOperationQueue()
	if err != nil {
		return err
	}
	sto.locked = false
	return nil
}

func (s *storage) EnqueueNewEntities(amount int, components ...Component) error {
	if !s.locked {
		_, err := s.NewEntities(amount, components...)
		if err != nil {
			return fmt.Errorf("failed to create entities directly: %w", err)
		}
		return nil
	}

	s.opQueue.createOps = append(s.opQueue.createOps, operation{
		typ:    opCreate,
		amount: amount,
		comps:  components,
	})
	return nil
}

func (s *storage) DestroyEntities(entities ...Entity) error {
	if s.locked {
		return LockedStorageError{}
	}
	tableGroups := make(map[table.Table][]int)
	for _, entity := range entities {
		if entity == nil {
			continue
		}
		tableGroups[entity.Table()] = append(tableGroups[entity.Table()], int(entity.ID()))
	}
	for tbl, ids := range tableGroups {
		if _, err := tbl.DeleteEntries(ids...); err != nil {
			return fmt.Errorf("failed to delete entries: %w", err)
		}
	}
	return nil
}

func (s *storage) EnqueueDestroyEntities(entities ...Entity) error {
	if !s.locked {
		return s.DestroyEntities(entities...)
	}

	tableGroups := make(map[table.Table][]table.EntryID)
	for _, entity := range entities {
		if entity == nil {
			continue
		}
		tableGroups[entity.Table()] = append(tableGroups[entity.Table()], entity.ID())
	}

	for tbl, ids := range tableGroups {
		s.opQueue.EnqueueDestroy(tbl, ids)
	}
	return nil
}
