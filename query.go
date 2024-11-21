package warehouse

import (
	"github.com/TheBitDrifter/mask"
)

type Operation int

const (
	OpAnd Operation = iota
	OpOr
	OpNot
)

type compositeNode struct {
	op         Operation
	children   []QueryNode
	components []Component
}

type leafNode struct {
	components []Component
}

type query struct {
	root QueryNode
}

func newQuery() Query {
	return &query{}
}

func newCompositeNode(op Operation, components []Component) *compositeNode {
	return &compositeNode{
		op:         op,
		children:   make([]QueryNode, 0),
		components: components,
	}
}

func newLeafNode(components []Component) *leafNode {
	return &leafNode{components: components}
}

func (n *compositeNode) Evaluate(archetype Archetype, storage Storage) bool {
	// Build mask at evaluation time
	var nodeMask mask.Mask
	for _, comp := range n.components {
		bit := storage.RowIndexFor(comp)
		nodeMask.Mark(bit)
	}

	archeMask := archetype.Table().(mask.Maskable).Mask()

	switch n.op {
	case OpAnd:
		if !archeMask.ContainsAll(nodeMask) {
			return false
		}
		for _, child := range n.children {
			if !child.Evaluate(archetype, storage) {
				return false
			}
		}
		return true

	case OpOr:
		if archeMask.ContainsAny(nodeMask) {
			return true
		}
		for _, child := range n.children {
			if child.Evaluate(archetype, storage) {
				return true
			}
		}
		return false

	case OpNot:
		if len(n.children) == 0 {
			return archeMask.ContainsNone(nodeMask)
		}
		for _, child := range n.children {
			if child.Evaluate(archetype, storage) {
				return false
			}
		}
		return !archeMask.ContainsAny(nodeMask)
	}
	return false
}

func (n *leafNode) Evaluate(archetype Archetype, storage Storage) bool {
	var nodeMask mask.Mask
	for _, comp := range n.components {
		bit := storage.RowIndexFor(comp)
		nodeMask.Mark(bit)
	}

	archeMask := archetype.Table().(mask.Maskable).Mask()
	return archeMask.ContainsAll(nodeMask)
}

func (q *query) And(items ...interface{}) QueryNode {
	components, children := q.processItems(items...)
	node := newCompositeNode(OpAnd, components)
	node.children = children
	if q.root == nil {
		q.root = node
	}
	return node
}

func (q *query) Or(items ...interface{}) QueryNode {
	components, children := q.processItems(items...)
	node := newCompositeNode(OpOr, components)
	node.children = children
	if q.root == nil {
		q.root = node
	}
	return node
}

func (q *query) Not(items ...interface{}) QueryNode {
	components, children := q.processItems(items...)
	node := newCompositeNode(OpNot, components)
	node.children = children
	if q.root == nil {
		q.root = node
	}
	return node
}

func (q *query) processItems(items ...interface{}) ([]Component, []QueryNode) {
	components := make([]Component, 0)
	children := make([]QueryNode, 0)

	for _, item := range items {
		switch v := item.(type) {
		case Component:
			components = append(components, v)
		case []Component:
			components = append(components, v...)
		case QueryNode:
			children = append(children, v)
		}
	}

	return components, children
}

func (q *query) Evaluate(archetype Archetype, storage Storage) bool {
	if q.root == nil {
		return false
	}
	return q.root.Evaluate(archetype, storage)
}
