package sim

import "github.com/olivierh59500/r-type/internal/source"

// OriginalGroundWalkerState preserves the original signed word movement
// timers, independent byte cannon clocks, and automatic/fixed animation index.
type OriginalGroundWalkerState struct {
	Phase, Wait, Steps, Burst, Side int
	Index, Timer, Frame             int
	Jump, Auto                      bool
}

func (w *World) updateGroundWalker(enemy *Enemy) bool {
	d := enemy.GroundWalker
	if d == nil {
		return false
	}
	if enemy.Walker == nil {
		enemy.Walker = &OriginalGroundWalkerState{Steps: 100, Auto: true}
	}
	s := enemy.Walker
	w.fireGroundWalker(enemy)
	if enemy.X-enemy.W/2+GameOriginX < 0 {
		enemy.Health = 0
		return true
	}
	if s.Wait >= 0 {
		s.Wait--
		if s.Phase == 1 {
			enemy.X--
		}
	} else {
		x, y := int(enemy.X-enemy.W/2+GameOriginX), int(enemy.Y-enemy.H/2+GameOriginY)
		switch s.Phase {
		case 0:
			if w.OriginalSolidAt(x-6, y+34) || s.Steps < 0 {
				s.Phase, s.Wait, s.Steps = 1, 50, 27
				s.Index, s.Timer, s.Auto = 0, 0, false
			} else {
				s.Steps--
				enemy.X -= 3
			}
		case 1:
			s.Jump, s.Auto = true, true
			if s.Steps >= 0 {
				s.Steps--
				enemy.Y -= 2
			} else {
				s.Phase, s.Wait, s.Steps = 2, 50, 65
			}
		case 2:
			if s.Steps >= 0 {
				s.Steps--
				enemy.X += 2
			} else {
				s.Phase, s.Wait, s.Steps = 3, 20, 40
			}
		case 3:
			if w.OriginalSolidAt(x+22, y+44) {
				s.Phase = 4
				s.Jump = false
				s.Index, s.Timer, s.Auto = 0, 0, false
			} else {
				enemy.Y += 2
				enemy.X++
			}
		default:
			enemy.X--
		}
	}
	s.Timer--
	if s.Timer < 0 {
		s.Timer = max(1, d.FrameTicks) - 1
		stream := d.Stand
		if s.Jump {
			stream = d.Jump
		}
		if s.Index < 0 || s.Index >= len(stream) {
			s.Index = 0
		}
		if len(stream) > 0 {
			s.Frame = stream[s.Index]
		}
		if s.Auto {
			s.Index++
		}
	}
	enemy.AnimationOverride, enemy.AnimationFrame = true, s.Frame
	return true
}

func (w *World) walkerShot(parent Enemy, art source.OriginalObjectData, x, y int, behavior EnemyBehavior) Enemy {
	enemy := Enemy{Entity: w.newEntity(float64(x)-GameOriginX+float64(art.Width)/2, float64(y)-GameOriginY+float64(art.Height)/2, 0, 0, float64(art.Width), float64(art.Height)), GroundWalker: parent.GroundWalker, Behavior: behavior, Kind: EnemyFighter, Health: 1, MaxHealth: 1, Descriptor: art.Descriptor, Sprite: art.Sprite, Animation: art.Animation, AnimationTicks: art.FrameTicks}
	for _, box := range art.Collision {
		enemy.CollisionFrames = append(enemy.CollisionFrames, CollisionFrame{X: box.X, Y: box.Y, Width: box.Width, Height: box.Height})
	}
	return enemy
}

func (w *World) fireGroundWalker(enemy *Enemy) {
	s, d := enemy.Walker, enemy.GroundWalker
	x, y := int(enemy.X-enemy.W/2+GameOriginX), int(enemy.Y-enemy.H/2+GameOriginY)
	s.Burst = (s.Burst - 1) & 255
	if s.Burst&128 != 0 {
		s.Burst = 115
		for _, direction := range d.InitialDirections {
			shot := w.walkerShot(*enemy, d.Homing, x+15, y+10, BehaviorWalkerHoming)
			shot.Variant = direction
			shot.BehaviorTicks = 5
			shot.HomingTurns = 22
			shot.AnimationOverride = true
			shot.AnimationFrame = d.Homing.Animation[direction]
			for _, value := range d.AimMap {
				shot.AimMap = append(shot.AimMap, int8(value))
			}
			shot.AimThresholds = d.AimThresholds
			w.Enemies = append(w.Enemies, shot)
		}
	}
	if s.Side > 0 {
		s.Side--
	}
	difference := y + 8 - int(w.Player.Y-7+GameOriginY)
	if difference < 0 {
		difference = -difference
	}
	if difference <= 18 && s.Side == 0 {
		s.Side = 18
		shot := w.walkerShot(*enemy, d.Side, x, y+8, BehaviorWalkerCannon)
		shot.AnimationOverride = true
		shot.AnimationFrame = d.Side.Animation[0]
		w.Enemies = append(w.Enemies, shot)
	}
}

func (w *World) updateWalkerHoming(enemy *Enemy) bool {
	d := enemy.GroundWalker
	if d == nil {
		return false
	}
	enemy.BehaviorTicks--
	if enemy.BehaviorTicks < 0 {
		enemy.BehaviorTicks = 5
		if enemy.HomingTurns > 0 {
			enemy.HomingTurns--
			x, y := int(enemy.X-enemy.W/2+GameOriginX), int(enemy.Y-enemy.H/2+GameOriginY)
			if w.OriginalSolidAt(x+8, y+8) {
				enemy.Health = 0
				w.finishWalkerProjectile(*enemy)
				return true
			}
			target := w.originalAimDirection(*enemy)
			difference := target - enemy.Variant
			if (difference >= 0 && difference <= 8) || difference < -8 {
				enemy.Variant++
			} else {
				enemy.Variant--
			}
			enemy.Variant = (enemy.Variant + 16) & 15
		}
	}
	x, y := int(enemy.X-enemy.W/2+GameOriginX)+d.VelocityX[enemy.Variant], int(enemy.Y-enemy.H/2+GameOriginY)+d.VelocityY[enemy.Variant]
	if x < 16 || x > 368 || y < 16 || y >= 224 {
		enemy.Health = 0
		return true
	}
	enemy.X = float64(x) - GameOriginX + enemy.W/2
	enemy.Y = float64(y) - GameOriginY + enemy.H/2
	enemy.AnimationOverride = true
	enemy.AnimationFrame = d.Homing.Animation[enemy.Variant]
	return true
}

func (w *World) updateWalkerCannon(enemy *Enemy) bool {
	if enemy.GroundWalker == nil {
		return false
	}
	x, y := int(enemy.X-enemy.W/2+GameOriginX), int(enemy.Y-enemy.H/2+GameOriginY)
	if w.OriginalSolidAt(x-16, y-4) {
		enemy.Health = 0
		w.finishWalkerProjectile(*enemy)
		return true
	}
	enemy.X -= 6
	if enemy.X-enemy.W/2+GameOriginX <= 16 {
		enemy.Health = 0
	}
	return true
}

func (w *World) finishWalkerProjectile(enemy Enemy) {
	d := enemy.GroundWalker
	if d == nil {
		return
	}
	art := d.Spark
	w.Effects = append(w.Effects, Effect{ActorOwner: enemy.ID, Priority: enemy.Priority, Entity: w.newEntity(enemy.X-enemy.W/2+float64(art.Width)/2, enemy.Y-enemy.H/2+float64(art.Height)/2, 0, 0, float64(art.Width), float64(art.Height)), Sprite: art.Sprite, Animation: art.Animation, FrameTicks: art.FrameTicks, Lifetime: d.SparkLifetime})
	w.Events = append(w.Events, Event{Kind: EventSound, Value: d.SparkSound, X: enemy.X, Y: enemy.Y})
}
