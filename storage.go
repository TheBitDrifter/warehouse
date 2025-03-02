package warehouse

import (
	"errors"
	"fmt"

	"github.com/TheBitDrifter/bark"
	"github.com/TheBitDrifter/mask"
	"github.com/TheBitDrifter/table"
)

// Ensure storage implements Storage interface
var _ Storage = &storage{}

var (
	globalEntryIndex = table.Factory.NewEntryIndex()
	globalEntities   = make([]entity, 0)
)

// Storage defines the interface for entity storage and manipulation
type Storage interface {
	Entity(id int) (Entity, error)
	NewEntities(int, ...Component) ([]Entity, error)
	NewOrExistingArchetype(components ...Component) (Archetype, error)
	EnqueueNewEntities(int, ...Component) error
	DestroyEntities(...Entity) error
	EnqueueDestroyEntities(...Entity) error
	RowIndexFor(Component) uint32
	Locked() bool
	AddLock()
	PopLock()
	Register(...Component)
	tableFor(...Component) (table.Table, error)

	TransferEntities(target Storage, entities ...Entity) error
	Enqueue(EntityOperation)
	Archetypes() []ArchetypeImpl
}

// storage implements the Storage interface
type storage struct {
	locked         bool
	locks          int
	schema         table.Schema
	archetypes     *archetypes
	operationQueue EntityOperationsQueue
}

// archetypes manages archetype collections and identification
type archetypes struct {
	nextID           archetypeID
	asSlice          []ArchetypeImpl
	idsGroupedByMask map[mask.Mask]archetypeID
}

// newStorage creates a new Storage implementation with the given schema
func newStorage(schema table.Schema) Storage {
	archetypes := &archetypes{
		nextID:           1,
		idsGroupedByMask: make(map[mask.Mask]archetypeID),
	}
	storage := &storage{
		archetypes:     archetypes,
		schema:         schema,
		operationQueue: &entityOperationsQueue{},
	}
	return storage
}

// Entity retrieves an entity by ID
func (sto *storage) Entity(id int) (Entity, error) {
	return &globalEntities[id-1], nil
}

// NewOrExistingArchetype gets an existing archetype matching the component signature or creates a new one
func (sto *storage) NewOrExistingArchetype(components ...Component) (Archetype, error) {
	var entityMask mask.Mask
	for _, component := range components {
		sto.schema.Register(component)
		bit := sto.schema.RowIndexFor(component)
		entityMask.Mark(bit)
	}
	id, archetypeFound := sto.archetypes.idsGroupedByMask[entityMask]
	if archetypeFound {
		return sto.archetypes.asSlice[id-1], nil
	}

	created, err := newArchetype(sto, globalEntryIndex, sto.archetypes.nextID, components...)
	if err != nil {
		return nil, err
	}
	sto.archetypes.asSlice = append(sto.archetypes.asSlice, created)
	sto.archetypes.idsGroupedByMask[entityMask] = created.id
	sto.archetypes.nextID++
	return &created, nil
}

// NewEntities creates n new entities with the specified components
func (sto *storage) NewEntities(n int, components ...Component) ([]Entity, error) {
	if sto.locked {
		return nil, errors.New("storage is locked")
	}
	var entityMask mask.Mask
	for _, component := range components {
		sto.schema.Register(component)
		bit := sto.schema.RowIndexFor(component)
		entityMask.Mark(bit)
	}
	var entityArchetype Archetype
	id, archetypeFound := sto.archetypes.idsGroupedByMask[entityMask]
	if archetypeFound {
		entityArchetype = sto.archetypes.asSlice[id-1]
	} else {
		created, err := sto.NewOrExistingArchetype(components...)
		entityArchetype = created
		if err != nil {
			return nil, err
		}
	}
	entries, err := entityArchetype.Table().NewEntries(n)
	if err != nil {
		return nil, err
	}
	currentLen := len(globalEntities)
	neededCap := currentLen + n
	if cap(globalEntities) < neededCap {
		newCap := max(neededCap, 2*cap(globalEntities))
		newEntities := make([]entity, currentLen, newCap)
		copy(newEntities, globalEntities)
		globalEntities = newEntities
	}
	globalEntities = globalEntities[:neededCap]

	entities := make([]Entity, n)
	for i, entry := range entries {
		en := &entity{
			Entry:      entry,
			sto:        sto,
			id:         entry.ID(),
			components: components,
		}
		entities[i] = en
		globalEntities[currentLen+i] = *en
	}

	return entities, nil
}

// RowIndexFor returns the bit index for a component in the schema
func (sto *storage) RowIndexFor(c Component) uint32 {
	return sto.schema.RowIndexFor(c)
}

// Locked checks if the storage is currently locked
func (sto *storage) Locked() bool {
	return sto.locks != 0
}

// AddLock increments the lock counter and sets the locked state
func (sto *storage) AddLock() {
	sto.locks++
	sto.locked = true
}

// PopLock decreases the lock counter and processes queued operations if unlocked
func (sto *storage) PopLock() {
	sto.locked = false
	if sto.locks == 0 {
		return
	}
	sto.locks--
	if !sto.Locked() {
		err := sto.operationQueue.ProcessAll(sto)
		if err != nil {
			panic(bark.AddTrace(err))
		}
	}
}

// EnqueueNewEntities either creates entities immediately or queues creation if storage is locked
func (s *storage) EnqueueNewEntities(count int, components ...Component) error {
	if !s.locked {
		_, err := s.NewEntities(count, components...)
		if err != nil {
			return fmt.Errorf("failed to create entities directly: %w", err)
		}
		return nil
	}
	s.operationQueue.Enqueue(
		NewEntityOperation{
			count:      count,
			components: components,
		},
	)
	return nil
}

// DestroyEntities removes entities from storage
func (s *storage) DestroyEntities(entities ...Entity) error {
	if s.locked {
		return errors.New("storage is locked")
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
		index := en.ID() - 1
		if int(index) < len(globalEntities) {
			globalEntities[index] = entity{}
		}
	}
	return nil
}

// EnqueueDestroyEntities either destroys entities immediately or queues destruction if storage is locked
func (s *storage) EnqueueDestroyEntities(entities ...Entity) error {
	if !s.locked {
		return s.DestroyEntities(entities...)
	}
	for _, en := range entities {
		s.operationQueue.Enqueue(
			DestroyEntityOperation{
				entity:   en,
				recycled: en.Recycled(),
			})
	}
	return nil
}

// TransferEntities moves entities from this storage to the target storage
func (s *storage) TransferEntities(target Storage, entities ...Entity) error {
	if s.locked {
		return errors.New("storage is locked")
	}
	for _, en := range entities {
		comps := en.Components()
		target.Register(comps...)
		targetTbl, err := target.tableFor(comps...)
		if err != nil {
			return err
		}

		err = en.Table().TransferEntries(targetTbl, en.Index())
		if err != nil {
			return err
		}
		en.SetStorage(target)
	}
	return nil
}

// Register adds components to the storage schema
func (s *storage) Register(comps ...Component) {
	ets := make([]table.ElementType, len(comps))
	for i, c := range comps {
		ets[i] = c
	}
	s.schema.Register(ets...)
}

// Enqueue adds an operation to the queue
func (s *storage) Enqueue(op EntityOperation) {
	s.operationQueue.Enqueue(op)
}

// Archetypes returns all archetypes in this storage
func (s *storage) Archetypes() []ArchetypeImpl {
	return s.archetypes.asSlice
}

// tableFor gets or creates a table for the given component set
func (s *storage) tableFor(comps ...Component) (table.Table, error) {
	archeMask := mask.Mask{}
	for _, c := range comps {
		bit := s.RowIndexFor(c)
		archeMask.Mark(bit)
	}

	id, ok := s.archetypes.idsGroupedByMask[archeMask]
	decrement := 1
	if !ok {
		decrement++
		created, err := newArchetype(s, globalEntryIndex, s.archetypes.nextID, comps...)
		if err != nil {
			return nil, err
		}
		s.archetypes.asSlice = append(s.archetypes.asSlice, created)
		s.archetypes.nextID++
		id = s.archetypes.nextID
	}
	arche := s.archetypes.asSlice[id-archetypeID(decrement)]
	return arche.table, nil
}
