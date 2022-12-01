package entity

import (
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/df-mc/dragonfly/server/world/sound"
	"github.com/go-gl/mathgl/mgl64"
	"math/rand"
	"time"
)

// Lightning is a lethal element to thunderstorms. Lightning momentarily increases the skylight's brightness to slightly greater than full daylight.
type Lightning struct {
	pos mgl64.Vec3

	state    int
	liveTime int

	damage     float64
	blockFire  bool
	entityFire bool
}

// NewLightning creates a lightning entity. The lightning entity will be positioned at the position passed.
func NewLightning(pos mgl64.Vec3) *Lightning {
	li := &Lightning{
		pos:        pos,
		state:      2,
		liveTime:   rand.Intn(3) + 1,
		damage:     5,
		blockFire:  true,
		entityFire: true,
	}
	return li
}

// NewLightningWithDamage creates a lightning entity but lets you specify damage and whether blocks and
// entities should be set on fire.
func NewLightningWithDamage(pos mgl64.Vec3, dmg float64, blockFire, entityFire bool) *Lightning {
	li := NewLightning(pos)
	li.damage = dmg
	li.blockFire = blockFire
	li.entityFire = entityFire
	return li
}

// Type returns LightningType.
func (*Lightning) Type() world.EntityType {
	return LightningType{}
}

// Position returns the current position of the lightning entity.
func (li *Lightning) Position() mgl64.Vec3 {
	return li.pos
}

// World returns the world that the lightning entity is currently in, or nil if it is not added to a world.
func (li *Lightning) World() *world.World {
	w, _ := world.OfEntity(li)
	return w
}

// Close closes the lighting.
func (li *Lightning) Close() error {
	li.World().RemoveEntity(li)
	return nil
}

// Rotation ...
func (li *Lightning) Rotation() (c cube.Rotation) {
	return cube.Rotation{}
}

// New strikes the Lightning at a specific position in a new world.
func (li *Lightning) New(pos mgl64.Vec3) world.Entity {
	return NewLightning(pos)
}

// Tick ...
func (li *Lightning) Tick(w *world.World, _ int64) {
	f := fire().(interface {
		Start(w *world.World, pos cube.Pos)
	})

	if li.state == 2 { // Init phase
		w.PlaySound(li.pos, sound.Thunder{})
		w.PlaySound(li.pos, sound.Explosion{})

		bb := li.Type().BBox(li).GrowVec3(mgl64.Vec3{3, 6, 3}).Translate(li.pos.Add(mgl64.Vec3{0, 3}))
		for _, e := range w.EntitiesWithin(bb, nil) {
			// Only damage entities that weren't already dead.
			if l, ok := e.(Living); ok && l.Health() > 0 {
				if li.damage > 0 {
					l.Hurt(li.damage, LightningDamageSource{})
				}
				if f, ok := e.(Flammable); ok && li.entityFire && f.OnFireDuration() < time.Second*8 {
					f.SetOnFire(time.Second * 8)
				}
			}
		}
		if li.blockFire && w.Difficulty().FireSpreadIncrease() >= 10 {
			f.Start(w, cube.PosFromVec3(li.pos))
		}
	}

	if li.state--; li.state < 0 {
		if li.liveTime == 0 {
			_ = li.Close()
		} else if li.state < -rand.Intn(10) {
			li.liveTime--
			li.state = 1

			if li.blockFire && w.Difficulty().FireSpreadIncrease() >= 10 {
				f.Start(w, cube.PosFromVec3(li.pos))
			}
		}
	}
}

// fire returns a fire block.
func fire() world.Block {
	f, ok := world.BlockByName("minecraft:fire", map[string]any{"age": int32(0)})
	if !ok {
		panic("could not find fire block")
	}
	return f
}

// LightningType is a world.EntityType implementation for Lightning.
type LightningType struct{}

func (LightningType) EncodeEntity() string                  { return "minecraft:lightning_bolt" }
func (LightningType) BBox(world.Entity) cube.BBox           { return cube.BBox{} }
func (LightningType) DecodeNBT(map[string]any) world.Entity { return nil }
func (LightningType) EncodeNBT(world.Entity) map[string]any {
	return map[string]any{}
}
