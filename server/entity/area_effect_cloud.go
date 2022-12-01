package entity

import (
	"github.com/df-mc/dragonfly/server/block/cube"
	"github.com/df-mc/dragonfly/server/entity/effect"
	"github.com/df-mc/dragonfly/server/internal/nbtconv"
	"github.com/df-mc/dragonfly/server/item/potion"
	"github.com/df-mc/dragonfly/server/world"
	"github.com/go-gl/mathgl/mgl64"
	"time"
)

// AreaEffectCloud is the cloud that is created when: lingering potions are thrown; creepers with potion effects explode;
// dragon fireballs hit the ground.
type AreaEffectCloud struct {
	duration           int64
	reapplicationDelay int64
	durationOnUse      int64

	radius       float64
	radiusOnUse  float64
	radiusGrowth float64

	close bool

	targets map[world.Entity]int64

	t   potion.Potion
	age int64

	transform
}

// NewAreaEffectCloud ...
func NewAreaEffectCloud(pos mgl64.Vec3, t potion.Potion) *AreaEffectCloud {
	return NewAreaEffectCloudWith(pos, t, time.Second*30, time.Second*2, 0, 3.0, -0.5, -0.005)
}

// NewAreaEffectCloudWith ...
func NewAreaEffectCloudWith(pos mgl64.Vec3, t potion.Potion, duration, reapplicationDelay, durationOnUse time.Duration, radius, radiusOnUse, radiusGrowth float64) *AreaEffectCloud {
	a := &AreaEffectCloud{
		duration:           duration.Milliseconds() / 50,
		reapplicationDelay: reapplicationDelay.Milliseconds() / 50,
		durationOnUse:      durationOnUse.Milliseconds() / 50,

		radius:       radius,
		radiusOnUse:  radiusOnUse,
		radiusGrowth: radiusGrowth,

		targets: make(map[world.Entity]int64),
		t:       t,
	}
	a.transform = newTransform(a, pos)
	return a
}

// Type returns AreaEffectCloudType.
func (a *AreaEffectCloud) Type() world.EntityType {
	return AreaEffectCloudType{}
}

// Duration returns the duration of the cloud.
func (a *AreaEffectCloud) Duration() time.Duration {
	a.mu.Lock()
	defer a.mu.Unlock()
	return time.Duration(a.duration) * time.Millisecond * 50
}

// Radius returns information about the cloud's, radius, change rate, and growth rate.
func (a *AreaEffectCloud) Radius() (radius, radiusOnUse, radiusGrowth float64) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.radius, a.radiusOnUse, a.radiusGrowth
}

// Effects returns the effects the area effect cloud provides.
func (a *AreaEffectCloud) Effects() []effect.Effect {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.t.Effects()
}

// Tick ...
func (a *AreaEffectCloud) Tick(w *world.World, _ int64) {
	if a.close {
		_ = a.Close()
		return
	}

	a.age++
	if a.age < 10 {
		// The cloud lives for at least half a second before it may begin spreading effects and growing/shrinking.
		return
	}

	a.mu.Lock()
	if a.age >= a.duration+10 {
		// We've outlived our duration, close the entity the next tick.
		a.close = true
		a.mu.Unlock()
		return
	}

	if a.radiusGrowth != 0 {
		a.radius += a.radiusGrowth
		if a.radius < 0.5 {
			a.close = true
			a.mu.Unlock()
			return
		}
		a.mu.Unlock()

		for _, v := range w.Viewers(a.pos) {
			v.ViewEntityState(a)
		}

		a.mu.Lock()
	}

	if a.age%5 != 0 {
		// Area effect clouds only trigger updates every five ticks.
		a.mu.Unlock()
		return
	}

	for target, expiration := range a.targets {
		if a.age >= expiration {
			delete(a.targets, target)
		}
	}

	a.mu.Unlock()
	entities := w.EntitiesWithin(a.Type().BBox(a).Translate(a.pos), func(entity world.Entity) bool {
		_, target := a.targets[entity]
		_, living := entity.(Living)
		return !living || target || entity == a
	})
	a.mu.Lock()

	var update bool
	for _, e := range entities {
		delta := e.Position().Sub(a.pos)
		delta[1] = 0
		if delta.Len() <= a.radius {
			l := e.(Living)
			for _, eff := range a.t.Effects() {
				if lasting, ok := eff.Type().(effect.LastingType); ok {
					l.AddEffect(effect.New(lasting, eff.Level(), eff.Duration()/4))
					continue
				}
				l.AddEffect(eff)
			}

			a.targets[e] = a.age + a.reapplicationDelay
			radiusUpdate := a.useRadius()
			durationUpdate := a.useDuration()
			if radiusUpdate || durationUpdate {
				update = true
			}
		}
	}
	pos := a.pos
	a.mu.Unlock()

	if update {
		for _, v := range w.Viewers(pos) {
			v.ViewEntityState(a)
		}
	}
}

// useDuration grows duration by the durationOnUse factor. If duration goes under zero, it will close the entity.
// useDuration should always be called when the mutex is locked.
func (a *AreaEffectCloud) useDuration() bool {
	if a.durationOnUse == 0 {
		// No change in duration on use, so don't do anything.
		return false
	}

	a.duration += a.durationOnUse
	if a.duration <= 0 {
		a.close = true
	}
	return true
}

// useRadius grows radius by the radiusOnUse factor. If radius goes under 1/2, it will close the entity. useRadius
// should always be called when the mutex is locked.
func (a *AreaEffectCloud) useRadius() bool {
	if a.radiusOnUse == 0 {
		// No change in radius on use, so don't do anything.
		return false
	}

	a.radius += a.radiusOnUse
	if a.radius <= 0.5 {
		a.close = true
	}
	return true
}

// AreaEffectCloudType is a world.EntityType implementation for AreaEffectCloud.
type AreaEffectCloudType struct{}

func (AreaEffectCloudType) EncodeEntity() string { return "minecraft:area_effect_cloud" }
func (AreaEffectCloudType) BBox(e world.Entity) cube.BBox {
	r, _, _ := e.(*AreaEffectCloud).Radius()
	return cube.Box(-r, 0, -r, r, 0.5, r)
}

func (AreaEffectCloudType) DecodeNBT(m map[string]any) world.Entity {
	return NewAreaEffectCloudWith(
		nbtconv.Vec3(m, "Pos"),
		potion.From(nbtconv.Int32(m, "PotionId")),
		nbtconv.TickDuration[int32](m, "Duration"),
		nbtconv.TickDuration[int32](m, "ReapplicationDelay"),
		nbtconv.TickDuration[int32](m, "DurationOnUse"),
		float64(nbtconv.Float32(m, "Radius")),
		float64(nbtconv.Float32(m, "RadiusOnUse")),
		float64(nbtconv.Float32(m, "RadiusPerTick")),
	)
}

func (AreaEffectCloudType) EncodeNBT(e world.Entity) map[string]any {
	a := e.(*AreaEffectCloud)
	return map[string]any{
		"Pos":                nbtconv.Vec3ToFloat32Slice(a.Position()),
		"ReapplicationDelay": int32(a.reapplicationDelay),
		"RadiusPerTick":      float32(a.radiusGrowth),
		"RadiusOnUse":        float32(a.radiusOnUse),
		"DurationOnUse":      int32(a.durationOnUse),
		"Radius":             float32(a.radius),
		"Duration":           int32(a.duration),
		"PotionId":           int32(a.t.Uint8()),
	}
}
