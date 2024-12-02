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
	entities   []entity
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
		opQueue:    newOpQueue(),
	}
	return storage
}

func (sto *storage) Entity(id int) (Entity, error) {
	return &sto.entities[id-1], nil
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
		entityArchetype = sto.archetypes.asSlice[id-1]
	} else {
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
	currentLen := len(sto.entities)
	neededCap := currentLen + n
	if cap(sto.entities) < neededCap {
		// Grow by doubling or adding n, whichever is larger
		newCap := max(neededCap, 2*cap(sto.entities))
		newEntities := make([]entity, currentLen, newCap)
		copy(newEntities, sto.entities)
		sto.entities = newEntities
	}
	sto.entities = sto.entities[:neededCap]

	// Create entities
	entities := make([]Entity, n)
	for i, entry := range entries {
		en := &entity{
			Entry: entry,
			sto:   sto,
			id:    entry.ID(),
		}
		entities[i] = en
		sto.entities[currentLen+i] = *en
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

func (sto *storage) Unlock() {
	sto.locked = false
	err := sto.processOperationQueue()
	if err != nil {
		panic(err)
	}
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
		_, err := tbl.DeleteEntries(ids...)
		if err != nil {
			return fmt.Errorf("failed to delete entries: %w", err)
		}
	}
	for _, en := range entities {
		if en == nil {
			continue
		}
		// Adjust for 0-based indexing since entity IDs start at 1
		index := en.ID() - 1
		if index >= 0 && int(index) < len(s.entities) {
			s.entities[index] = entity{}
		}
	}
	return nil
}

func (s *storage) EnqueueDestroyEntities(entities ...Entity) error {
	if !s.locked {
		return s.DestroyEntities(entities...)
	}

	s.opQueue.EnqueueDestroy(s, entities)

	return nil
}
