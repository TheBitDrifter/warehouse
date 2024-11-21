package warehouse_bench

import (
	"testing"

	"github.com/TheBitDrifter/table"
	"github.com/TheBitDrifter/warehouse"
)

// go test -bench=. ./benchmarks/proto -benchmem -cpuprofile=kain.prof -tags="unsafe c256"

const (
	nPos    = 9000
	nPosVel = 1000
)

type Position struct {
	X float64
	Y float64
}

type Velocity struct {
	X float64
	Y float64
}

func BenchmarkIterWarehouseGet(b *testing.B) {
	b.StopTimer()

	velocity := warehouse.FactoryNewComponent[Velocity]()
	position := warehouse.FactoryNewComponent[Position]()
	schema := table.Factory.NewSchema()
	storage := warehouse.Factory.NewStorage(schema)

	storage.NewEntities(nPosVel, position, velocity)
	storage.NewEntities(nPos, position)

	query := warehouse.Factory.NewQuery()
	query.And(velocity, position)
	cursor := warehouse.Factory.NewCursor(query, storage)

	b.StartTimer()

	for i := 0; i < b.N; i++ {
		for cursor.Next() {
			pos := position.GetFromCursor(cursor)
			vel := velocity.GetFromCursor(cursor)

			pos.X += vel.X
			pos.Y += vel.Y
		}
	}
}
