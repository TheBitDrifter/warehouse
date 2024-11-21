package warehouse

import "fmt"

type LockedStorageError struct{}

func (e LockedStorageError) Error() string {
	return fmt.Sprintf("storage is currently locked")
}

type EntityRelationError struct {
	child, parent Entity
}

func (e EntityRelationError) Error() string {
	return fmt.Sprintf("child (%v) already has parent %v", e.child, e.parent)
}

type ComponentExistsError struct {
	Component Component
}

func (e ComponentExistsError) Error() string {
	return fmt.Sprintf("component already exists on entity: %T", e.Component)
}

type ComponentNotFoundError struct {
	Component Component
}

func (e ComponentNotFoundError) Error() string {
	return fmt.Sprintf("component does not exist on entity: %T", e.Component)
}
