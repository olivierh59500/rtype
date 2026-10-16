package sim

import "github.com/olivierh59500/r-type/internal/source"

// OriginalMiniTailState preserves the independent oscillator and one-time
// horizontal-position retarget at original stage-two RAM 0x4A6AE.
type OriginalMiniTailState struct {
	Data                                        *source.MiniTailData
	Mode, Direction, Oscillation, RetargetTimer int
	Toggle                                      bool
}

func (w *World) spawnOriginalMiniTails(wave Wave) {
	d := wave.MiniTail
	var actors []Enemy
	for number := 0; number < d.Count; number++ {
		y := d.SpawnYBase
		if number%5 == 4 {
			y = int(w.Player.X) + 18
		} else {
			y += int(w.nextRandom() % uint64(d.SpawnYRange))
		}
		state := &OriginalMiniTailState{Data: d, Direction: 1, Oscillation: int(w.nextRandom() % 7), RetargetTimer: 32767}
		if int(w.nextRandom()%uint64(d.RetargetChance)) >= d.RetargetThreshold {
			state.RetargetTimer = d.RetargetBase + int(w.nextRandom()%uint64(d.RetargetRange))
		}
		enemy := Enemy{Priority: wave.Priority, Entity: w.newEntity(float64(d.SpawnX)-GameOriginX+wave.W/2, float64(y)-GameOriginY+wave.H/2, 0, 0, wave.W, wave.H), Kind: EnemyDrifter, Health: 1, MaxHealth: 1, Score: wave.Score, Descriptor: wave.Descriptor, Sprite: wave.Sprite, CollisionFrames: wave.CollisionFrames, Behavior: BehaviorMiniTail, MoveDelay: number * d.DelayStep, Animation: wave.Animation, AnimationTicks: wave.AnimationTicks, MiniTail: state}
		actors = append(actors, enemy)
	}
	for index := len(actors) - 1; index >= 0; index-- {
		w.Enemies = append(w.Enemies, actors[index])
	}
}

func (w *World) updateOriginalMiniTail(enemy *Enemy) bool {
	f := enemy.MiniTail
	if f == nil {
		return false
	}
	if enemy.MoveDelay >= 0 {
		enemy.MoveDelay--
		return true
	}
	if f.Mode == 0 {
		f.Toggle = !f.Toggle
		if f.Toggle {
			enemy.Y += float64(f.Direction)
			f.Oscillation--
			if f.Oscillation < 0 {
				f.Direction = -f.Direction
				f.Oscillation = f.Data.OscillationReload
			}
		}
	}
	if f.Mode < 0 || f.Mode >= len(f.Data.Velocity) {
		enemy.Health = 0
		return true
	}
	v := f.Data.Velocity[f.Mode]
	enemy.X += float64(v[0])
	enemy.Y += float64(v[1])
	x, y := int(enemy.X-enemy.W/2)+GameOriginX, int(enemy.Y-enemy.H/2)+GameOriginY
	if x < 16 || x > 371 || y < 0 || y > 196 {
		enemy.Health = 0
		return true
	}
	f.RetargetTimer--
	if f.RetargetTimer < 0 {
		f.Mode = 2
		if x+20 > int(w.Player.X)+18 {
			f.Mode = 1
		}
		f.RetargetTimer = 32767
		if f.Mode < len(f.Data.Animations) {
			enemy.Animation = f.Data.Animations[f.Mode]
			enemy.Age = 1
			enemy.AnimationStart = 0
		}
	}
	return true
}
