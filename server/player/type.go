package player

import (
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
)

// Type is a world.EntityType implementation for Player.
type Type struct{}

func (Type) EncodeEntity() string { return "minecraft:player" }
func (Type) BBox(e world.Entity) cube.BBox {
	p := e.(*Player)
	s := p.Scale()
	switch {
	// TODO: Shrink BBox for sneaking once implemented in Bedrock Edition. This is already a thing in Java Edition.
	case p.Gliding(), p.Swimming():
		return cube.Box(-0.3*s, 0, -0.3*s, 0.3*s, 0.6*s, 0.3*s)
	default:
		return cube.Box(-0.3*s, 0, -0.3*s, 0.3*s, 1.8*s, 0.3*s)
	}
}
