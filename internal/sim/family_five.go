package sim

import "github.com/olivierh59500/r-type/internal/source"

// OriginalFireWormState preserves one linked original segment's phase, route
// offset, damage-detachment limit and neighboring actors by stable native IDs.
type OriginalFireWormState struct {
	Data                                    *source.FireWormData
	Number, Phase, Path, Delay, Limit, Jump int
	Previous, Next                          uint64
}

func (w *World) spawnOriginalWorm(wave Wave) {
	d := wave.FireWorm
	x, y := int(wave.X-wave.W/2)+GameOriginX, int(wave.Y-wave.H/2)+GameOriginY
	var parts []Enemy
	for number := 0; number < d.BodyCount+2; number++ {
		art, descriptor, hp := wave.WormHeadArt, d.HeadDescriptor, 1000
		if number > 0 {
			art, descriptor = wave.WormBodyArt, d.BodyDescriptor
		}
		if number == d.BodyCount+1 {
			art, descriptor, hp = wave.WormTailArt, d.TailDescriptor, 2000
		}
		yOffset := 0
		if number == d.BodyCount+1 {
			yOffset = 2
		}
		enemy := Enemy{Entity: w.newEntity(float64(x)-GameOriginX+float64(art.Width)/2, float64(y+yOffset)-GameOriginY+float64(art.Height)/2, 0, 0, float64(art.Width), float64(art.Height)), Kind: EnemyFighter, Health: hp, MaxHealth: hp, Sprite: art.Sprite, Descriptor: descriptor, CollisionFrames: art.CollisionFrames, Behavior: BehaviorFireWorm, AnimationOverride: true, FireWorm: &OriginalFireWormState{Data: d, Number: number, Phase: -1, Path: d.StartOffset, Delay: number * 4}}
		if number == 0 {
			enemy.Priority = wave.Priority
		}
		if number > 0 {
			previous := &parts[len(parts)-1]
			enemy.FireWorm.Previous = previous.ID
			previous.FireWorm.Next = enemy.ID
		}
		parts = append(parts, enemy)
	}
	// Core 0x12A6 prepends each actor: the tail updates before the head.
	for index := len(parts) - 1; index >= 0; index-- {
		w.Enemies = append(w.Enemies, parts[index])
	}
}

func (w *World) wormActor(id uint64, current *Enemy) *Enemy {
	if current != nil && current.ID == id {
		return current
	}
	for index := range w.Enemies {
		if w.Enemies[index].ID == id {
			return &w.Enemies[index]
		}
	}
	return nil
}

func (w *World) updateOriginalFireWorm(enemy *Enemy) bool {
	f := enemy.FireWorm
	if f == nil {
		return false
	}
	switch f.Phase {
	case -1:
		f.Delay--
		if f.Delay < 0 {
			f.Phase = 0
			f.Limit = f.Delay
		}
		return true
	case 2:
		if f.Number >= 0 && f.Number < len(f.Data.Scatter) {
			v := f.Data.Scatter[f.Number]
			enemy.X += float64(v[0])
			enemy.Y += float64(v[1])
		}
		w.wormBounds(enemy)
		return true
	case 1:
		// Original 0x4AB2E branches unconditionally to this limit test;
		// its preceding damage comparison does not alter the route.
		if f.Limit <= f.Path {
			f.Path = f.Jump
			f.Limit = 32767
			f.Phase = 0
		}
	case 0:
		if enemy.DamageTaken >= 10 {
			if f.Number == 0 {
				w.addScore(1100)
				enemy.Health = 0
				if len(f.Data.Scatter) > 0 {
					v := f.Data.Scatter[0]
					enemy.X += float64(v[0])
					enemy.Y += float64(v[1])
				}
				w.bossImmediateDeath(enemy.Entity, 0)
				for part := enemy; part != nil; part = w.wormActor(part.FireWorm.Next, enemy) {
					state := part.FireWorm
					state.Phase = 2
					part.Invincible = false
					part.Health = 3
					part.MaxHealth = 3
					part.DamageTaken = 0
					part.ContactCooldown = 0
					part.Animation = state.Data.ScatterAnimation
					part.AnimationTicks = max(1, state.Data.ScatterAnimationTicks)
					part.AnimationStart = part.AnimationFrame
					part.AnimationOverride = false
					part.Age = 0
					if state.Next == 0 {
						break
					}
				}
				enemy.Health = 0
				return true
			}
			if previous := w.wormActor(f.Previous, enemy); previous != nil {
				previous.FireWorm.Next = 0
			}
			f.Previous = 0
			frame := enemy.AnimationFrame
			jump := 0
			if frame >= 0 && frame < len(f.Data.BreakOffsets) {
				jump = f.Data.BreakOffsets[frame]
			}
			limit := f.Path
			for part := enemy; part != nil; part = w.wormActor(part.FireWorm.Next, enemy) {
				state := part.FireWorm
				state.Jump = jump
				state.Limit = limit
				part.DamageTaken = 0
				if state.Phase < 0 {
					f.Path = jump
				}
				state.Phase = 1
				if state.Next == 0 {
					break
				}
			}
			return true
		}
	}
	for attempt := 0; attempt < 2; attempt++ {
		index := f.Path
		if index < 0 || index >= len(f.Data.Path) {
			enemy.Health = 0
			return true
		}
		step := f.Data.Path[index]
		f.Path += 3
		if step.DX == 127 {
			f.Path -= 6
			continue
		}
		enemy.X += float64(step.DX)
		enemy.Y += float64(step.DY)
		enemy.AnimationFrame = step.Frame
		w.wormBounds(enemy)
		return true
	}
	enemy.Health = 0
	return true
}

func (w *World) wormBounds(enemy *Enemy) {
	x, y := enemy.X-enemy.W/2+GameOriginX, enemy.Y-enemy.H/2+GameOriginY
	if x < 16 || x > 368 || y < 16 || y > 230 {
		enemy.Health = 0
	}
}
