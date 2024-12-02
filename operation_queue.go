package warehouse

import (
	"fmt"
)

type operation struct {
	typ      operationType
	amount   int
	comps    []Component
	entities []Entity
	sto      Storage
}

type operationType int

const (
	opCreate operationType = iota
	opDestroy
	opAddComponent
	opRemoveComponent
)

type opKey struct {
	entity Entity
}

type opQueue struct {
	createOps      []operation
	componentOps   []operation
	destroyOps     []operation
	pendingDestroy map[opKey]struct{}
	pendingMods    map[opKey]int
}

func newOpQueue() opQueue {
	return opQueue{
		pendingDestroy: make(map[opKey]struct{}),
		pendingMods:    make(map[opKey]int),
	}
}

func (q *opQueue) enqueueOp(op operation) {
	switch op.typ {
	case opCreate:
		q.createOps = append(q.createOps, op)
	case opDestroy:
		q.destroyOps = append(q.destroyOps, op)
	case opAddComponent, opRemoveComponent:
		q.componentOps = append(q.componentOps, op)
	}
}

func (s *storage) processOperationQueue() error {
	if len(s.opQueue.createOps) == 0 &&
		len(s.opQueue.componentOps) == 0 &&
		len(s.opQueue.destroyOps) == 0 {
		return nil
	}

	// Process creates first
	for _, op := range s.opQueue.createOps {
		if _, err := s.NewEntities(op.amount, op.comps...); err != nil {
			return fmt.Errorf("failed to process queued entity creation: %w", err)
		}
	}

	// Process component modifications
	for _, op := range s.opQueue.componentOps {
		entity := op.entities[0]

		// Verify entry hasn't been recycled
		if entity.ID() == 0 {
			continue
		}
		switch op.typ {
		case opAddComponent:
			if err := entity.AddComponent(op.comps[0]); err != nil {
				return fmt.Errorf("failed to add queued component: %w", err)
			}
		case opRemoveComponent:
			if err := entity.RemoveComponent(op.comps[0]); err != nil {
				return fmt.Errorf("failed to remove queued component: %w", err)
			}
		}
	}

	// Process destroys last
	for _, op := range s.opQueue.destroyOps {
		var entities []Entity
		for _, entity := range op.entities {
			entities = append(entities, entity)
		}

		if len(entities) > 0 {
			if err := op.sto.DestroyEntities(entities...); err != nil {
				return fmt.Errorf("failed to delete queued entries: %w", err)
			}
		}
	}

	// Clear all queues
	s.opQueue.createOps = s.opQueue.createOps[:0]
	s.opQueue.componentOps = s.opQueue.componentOps[:0]
	s.opQueue.destroyOps = s.opQueue.destroyOps[:0]
	clear(s.opQueue.pendingDestroy)
	clear(s.opQueue.pendingMods)
	return nil
}

func (q *opQueue) EnqueueDestroy(sto Storage, entries []Entity) {
	// Filter out already queued entities
	var newEntities []Entity
	for _, entity := range entries {
		key := opKey{entity: entity}
		if _, exists := q.pendingDestroy[key]; !exists {
			newEntities = append(newEntities, entity)
			q.pendingDestroy[key] = struct{}{}

			// Remove any pending component operations for this entity
			if idx, hasMods := q.pendingMods[key]; hasMods {
				// Mark operation as no-op by setting type to invalid
				q.componentOps[idx].typ = -1
				delete(q.pendingMods, key)
			}
		}
	}

	if len(newEntities) > 0 {
		q.destroyOps = append(q.destroyOps, operation{
			typ:      opDestroy,
			entities: newEntities,
			sto:      sto,
		})
	}
}

func (q *opQueue) EnqueueComponentOp(typ operationType, sto Storage, entity Entity, comp Component) {
	key := opKey{entity: entity}

	// If entity is pending destroy, ignore component operations
	if _, isDestroyed := q.pendingDestroy[key]; isDestroyed {
		return
	}

	// If entity already has pending component operations, update existing operation
	if existingIdx, exists := q.pendingMods[key]; exists {
		existingOp := &q.componentOps[existingIdx]
		existingOp.comps = []Component{comp}
		existingOp.typ = typ
		return
	}

	// Add new operation
	q.pendingMods[key] = len(q.componentOps)
	q.componentOps = append(q.componentOps, operation{
		typ:      typ,
		entities: []Entity{entity},
		sto:      sto,
		comps:    []Component{comp},
	})
}
