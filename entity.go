package warehouse

import (
	"fmt"

	"github.com/TheBitDrifter/mask"
	"github.com/TheBitDrifter/table"
	iter_util "github.com/TheBitDrifter/util/iter"
)

var _ Entity = &entity{}

type entity struct {
	sto *storage
	table.Entry
	relationships relationships
}

type relationships struct {
	parent    Entity
	onDestroy EntityDestroyCallback
}

func (e *entity) SetParent(parent Entity, callback EntityDestroyCallback) error {
	if e.relationships.parent != nil {
		return EntityRelationError{e, e.relationships.parent}
	}
	e.relationships.parent = parent
	err := parent.SetDestroyCallback(callback)
	if err != nil {
		return err
	}
	return nil
}

func (e *entity) SetDestroyCallback(callback EntityDestroyCallback) error {
	e.relationships.onDestroy = callback
	return nil
}

func (e *entity) AddComponent(c Component) error {
	if e.sto.locked {
		return LockedStorageError{}
	}
	originTable := e.Table()
	if originTable.Contains(c) {
		return ComponentExistsError{Component: c}
	}

	originMask := originTable.(mask.Maskable).Mask()
	destMask := originMask
	destMask.Mark(e.sto.schema.RowIndexFor(c))

	destArchetype, err := e.getOrCreateArchetype(destMask, c)
	if err != nil {
		return fmt.Errorf("failed to get/create archetype: %w", err)
	}

	if err := originTable.TransferEntries(destArchetype.table, e.Index()); err != nil {
		return fmt.Errorf("failed to transfer entity: %w", err)
	}
	return nil
}

func (e *entity) RemoveComponent(c Component) error {
	if e.sto.locked {
		return LockedStorageError{}
	}
	originTable := e.Table()
	if !originTable.Contains(c) {
		return ComponentNotFoundError{Component: c}
	}

	originMask := originTable.(mask.Maskable).Mask()
	destMask := originMask
	destMask.Unmark(e.sto.schema.RowIndexFor(c))

	destArchetype, err := e.getOrCreateArchetypeWithout(destMask, c)
	if err != nil {
		return fmt.Errorf("failed to get/create archetype: %w", err)
	}

	if err := originTable.TransferEntries(destArchetype.table, e.Index()); err != nil {
		return fmt.Errorf("failed to transfer entity: %w", err)
	}
	return nil
}

func (e *entity) EnqueueAddComponent(c Component) error {
	if !e.sto.locked {
		return e.AddComponent(c)
	}
	e.sto.opQueue.EnqueueComponentOp(opAddComponent, e.Table(), e.ID(), c)
	return nil
}

func (e *entity) EnqueueRemoveComponent(c Component) error {
	if !e.sto.locked {
		return e.RemoveComponent(c)
	}
	e.sto.opQueue.EnqueueComponentOp(opRemoveComponent, e.Table(), e.ID(), c)
	return nil
}

func (e *entity) getOrCreateArchetype(mask mask.Mask, newComp Component) (archetype, error) {
	if id, found := e.sto.archetypes.idsGroupedByMask[mask]; found {
		return e.sto.archetypes.asSlice[id-1], nil
	}

	// Create new archetype with all components including the new one
	originalComps := iter_util.Collect(e.Table().ElementTypes())
	newComps := make([]Component, len(originalComps)+1)
	for i, ogComp := range originalComps {
		newComps[i] = ogComp
	}
	newComps[len(newComps)-1] = newComp

	created, err := newArchetype(e.sto.schema, mainIndex, e.sto.archetypes.nextID, newComps...)
	if err != nil {
		return archetype{}, err
	}

	e.sto.archetypes.asSlice = append(e.sto.archetypes.asSlice, created)
	e.sto.archetypes.idsGroupedByMask[mask] = e.sto.archetypes.nextID
	e.sto.archetypes.nextID++

	return created, nil
}

// Helper for finding or creating archetypes when removing components
func (e *entity) getOrCreateArchetypeWithout(mask mask.Mask, removeComp Component) (archetype, error) {
	if id, found := e.sto.archetypes.idsGroupedByMask[mask]; found {
		return e.sto.archetypes.asSlice[id-1], nil
	}

	// Create new archetype with all components except the removed one
	originalComps := iter_util.Collect(e.Table().ElementTypes())
	newComps := make([]Component, 0, len(originalComps)-1)
	for _, comp := range originalComps {
		if comp != removeComp {
			newComps = append(newComps, comp)
		}
	}

	created, err := newArchetype(e.sto.schema, mainIndex, e.sto.archetypes.nextID, newComps...)
	if err != nil {
		return archetype{}, err
	}

	e.sto.archetypes.asSlice = append(e.sto.archetypes.asSlice, created)
	e.sto.archetypes.idsGroupedByMask[mask] = e.sto.archetypes.nextID
	e.sto.archetypes.nextID++

	return created, nil
}
