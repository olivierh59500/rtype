package sim

import "github.com/olivierh59500/r-type/internal/source"

// OriginalBattleshipIrisState translates the independent map iris at 0x4B20E.
// Actor damage is copied before collisions, preserving the original hook delay.
type OriginalBattleshipIrisState struct {
	Mode, OpeningCursor, OpeningTimer, DamageCursor, Damage, CollisionPhase int
	FlashKind, FlashFrame, FlashTimer                                       int
	Explosions, RangeX, OffsetX, X, Y                                       int
}

func (w *World) battleshipIrisState(spec *BossSpec) *OriginalBattleshipIrisState {
	if w.Boss.Iris == nil {
		d := spec.Third.Iris
		w.Boss.Iris = &OriginalBattleshipIrisState{Mode: 1, CollisionPhase: d.InitialCollisionPhase}
		w.applyBattleshipPatch(d.InitialPatch)
	}
	return w.Boss.Iris
}

func (w *World) updateBattleshipIrisActor(spec *BossSpec) {
	if spec.Third.Iris == nil {
		return
	}
	s := w.battleshipIrisState(spec)
	d := spec.Third.Iris
	for index := range w.Enemies {
		enemy := &w.Enemies[index]
		if enemy.bossRole != bossThreeIris || enemy.Health <= 0 || !w.selectedActor(enemy) {
			continue
		}
		enemy.Age++
		enemy.X -= w.Scroll - enemy.CameraX
		enemy.Y -= w.ScrollY - enemy.CameraY
		enemy.CameraX, enemy.CameraY = w.Scroll, w.ScrollY
		s.X, s.Y = int(enemy.X-enemy.W/2+GameOriginX), int(enemy.Y-enemy.H/2+GameOriginY)
		s.Damage = enemy.DamageTaken
		if s.CollisionPhase >= 0 && s.Mode < 0 {
			enemy.Health = 0
			continue
		}
		frame := enemy.AnimationStart
		if s.CollisionPhase >= 0 {
			frame = s.CollisionPhase
		}
		if frame < 0 || frame >= len(d.Animation) {
			frame = 0
		}
		if len(d.Animation) > 0 {
			enemy.AnimationFrame = d.Animation[frame]
			enemy.AnimationStart = frame + 1
		}
	}
}

func (w *World) updateBattleshipIris(spec *BossSpec) {
	if spec.Third.Iris == nil {
		return
	}
	s := w.battleshipIrisState(spec)
	d := spec.Third.Iris
	if s.Mode == 1 {
		if w.StageTick+1 < d.Trigger {
			return
		}
		enemy := w.newBossActor(d.Descriptor, BossArt{Width: 0, Height: 1}, d.X, d.Y, bossThreeIris)
		enemy.Hidden, enemy.AnimationOverride = true, true
		enemy.Invincible = false
		enemy.Health, enemy.MaxHealth = d.Health, d.Health
		enemy.CameraX, enemy.CameraY = w.Scroll, w.ScrollY
		for _, box := range d.Collision {
			enemy.CollisionFrames = append(enemy.CollisionFrames, CollisionFrame{X: box.X, Y: box.Y, Width: box.Width, Height: box.Height})
		}
		w.Enemies = append(w.Enemies, enemy)
		s.Mode = 0
		return
	}
	if s.Mode < 0 {
		w.irisDebris(d, s)
		return
	}
	if s.Damage < d.DamageStart {
		s.OpeningTimer--
		if s.OpeningTimer >= 0 {
			return
		}
		if s.OpeningCursor >= len(d.Opening) {
			s.Mode = -1
			return
		}
		frame := d.Opening[s.OpeningCursor]
		s.OpeningTimer = frame.Reload
		if frame.Stop {
			s.Mode = -1
			return
		}
		w.applyBattleshipPatch(frame.Patch)
		s.OpeningCursor++
		return
	}
	// Several satisfied damage gates can be consumed in one original update.
	for transitions := 0; transitions <= len(d.DamageSteps); transitions++ {
		if s.FlashKind > 0 && s.FlashKind <= len(d.Flash) {
			s.FlashTimer--
			if s.FlashTimer < 0 {
				frames := d.Flash[s.FlashKind-1]
				if len(frames) > 0 {
					frame := frames[s.FlashFrame%len(frames)]
					s.FlashTimer = frame.Reload
					w.applyBattleshipPatch(frame.Patch)
					s.FlashFrame = (s.FlashFrame + 1) % len(frames)
				}
			}
		}
		if s.DamageCursor >= len(d.DamageSteps) {
			s.Mode = -1
			return
		}
		step := d.DamageSteps[s.DamageCursor]
		if step.Damage > s.Damage {
			w.irisDebris(d, s)
			return
		}
		s.Explosions, s.RangeX, s.OffsetX = step.Explosions, step.RangeX, step.OffsetX
		s.FlashKind = step.Flash
		if step.Patch != nil {
			w.applyBattleshipPatch(step.Patch)
		}
		s.DamageCursor++
		s.CollisionPhase++
		s.FlashTimer, s.FlashFrame = 0, 0
	}
}

func (w *World) irisDebris(d *source.BattleshipIrisData, s *OriginalBattleshipIrisState) {
	if s.Explosions <= 0 {
		return
	}
	s.Explosions--
	if s.Explosions == 0 {
		w.addScore(d.ExplosionScore)
	}
	y := s.Y + d.ExplosionYOffset + int(w.nextRandom()%uint64(max(1, d.ExplosionYRange)))
	x := s.X + s.OffsetX + int(w.nextRandom()%uint64(max(1, s.RangeX)))
	if effect, ok := w.originalDeathEffect(Entity{X: float64(x) - GameOriginX, Y: float64(y) - GameOriginY}, 0); ok {
		// The map hook allocates after the actor pass. Both the first blit
		// and its sound therefore belong to the following main update.
		effect.Priority = 5
		w.Effects = append(w.Effects, effect)
	}
}
