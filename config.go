package warehouse

import "github.com/TheBitDrifter/table"

var Config config = config{}

type config struct {
	tableEvents table.TableEvents
}

func (c *config) SetTableEvents(te table.TableEvents) {
	c.tableEvents = te
}
