package warehouse

import (
	"github.com/TheBitDrifter/table"
)

// Component represents a data attribute/state that can be attached to entities
// Components can be used to create queries for entities
type Component interface {
	table.ElementType
}
