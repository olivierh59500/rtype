package sim

import (
	"fmt"

	"github.com/olivierh59500/r-type/internal/source"
)

// OriginalBattleshipCoreState owns the independent hull cycle and central core.
// Phase zero cycles the idle patch bank; phases one/three traverse the authored
// closing/opening banks, separated by the phase-two hold.
type OriginalBattleshipCoreState struct {
	Phase, PatchTick, PatchCursor, CycleWait, FireWait int
	X, Y, Damage                                       int
	Spawned, Defeated                                  bool
}

func (w *World) battleshipCoreState() *OriginalBattleshipCoreState {
	if w.Boss.Battleship == nil {
		w.Boss.Battleship = &OriginalBattleshipCoreState{}
	}
	return w.Boss.Battleship
}

func (w *World) updateBattleshipCoreActors(spec *BossSpec) {
	d := spec.Third.Core
	if d == nil {
		return
	}
	s := w.battleshipCoreState()
	for index := range w.Enemies {
		e := &w.Enemies[index]
		if e.Health <= 0 || !w.selectedActor(e) {
			continue
		}
		switch e.bossRole {
		case bossThreeCore:
			e.Age++
			// Core 0xF2C probes the post-camera axis counters and uses the
			// current command's delta, rather than the displacement just drawn.
			dx, dy := w.originalCameraCompensation()
			e.X -= float64(dx)
			e.Y -= float64(dy)
			s.X, s.Y = int(e.X-e.W/2)+GameOriginX, int(e.Y-e.H/2)+GameOriginY
			if e.DamageTaken != s.Damage {
				s.Damage = e.DamageTaken
				w.Boss.DamageTaken = s.Damage
				e.ContactCooldown = 3
				if e.DamageTaken >= d.DamageLimit {
					s.Defeated = true
					w.Boss.Defeated = true
					w.startBossVictory(spec)
				}
			}
		case bossThreeCoreShot:
			w.updateBattleshipCoreShot(e, d)
		}
	}
}

func (w *World) updateBattleshipCoreShot(e *Enemy, d *source.BattleshipCoreData) {
	e.Age++
	x, y := e.X-e.W/2+GameOriginX, e.Y-e.H/2+GameOriginY
	if w.OriginalSolidAt(int(x)+8, int(y)+8) {
		e.Health = 0
		w.bossImmediateDeath(e.Entity, 1)
		return
	}
	step := min(e.MotionTick, len(d.ShotMotionY)-1)
	x += e.VX
	y += float64(d.ShotMotionY[step])
	e.MotionTick = step + 1
	if x < 0 || x > 368 || y < 0 || y > 224 {
		e.Health = 0
		return
	}
	e.X, e.Y = x-GameOriginX+e.W/2, y-GameOriginY+e.H/2
}

func (w *World) spawnBattleshipCoreShot(d *source.BattleshipCoreData, s *OriginalBattleshipCoreState) {
	art := BossArt{Sprite: fmt.Sprintf("object-3-%05x", d.ShotDescriptor), Width: d.ShotWidth, Height: d.ShotHeight, Animation: d.ShotAnimation, AnimationTicks: d.ShotAnimationTicks}
	for _, box := range d.ShotCollision {
		art.CollisionFrames = append(art.CollisionFrames, CollisionFrame{X: box.X, Y: box.Y, Width: box.Width, Height: box.Height})
	}
	e := w.newBossActor(d.ShotDescriptor, art, s.X+d.ShotOffsetX, s.Y+d.ShotOffsetY, bossThreeCoreShot)
	e.Health, e.MaxHealth, e.Invincible = 1, 1, false
	e.MotionTick = int(w.nextRandom() % uint64(d.ShotStartRange))
	for e.VX == 0 {
		e.VX = float64(2 - int(w.nextRandom()%uint64(d.ShotVXRange)))
	}
	w.Enemies = append(w.Enemies, e)
}

func (w *World) updateBattleshipCoreMap(spec *BossSpec) {
	d := spec.Third.Core
	if d == nil {
		return
	}
	s := w.battleshipCoreState()
	main := w.StageTick + 1
	if s.Defeated {
		w.battleshipCoreDebris(s)
		return
	}
	switch s.Phase {
	case 0:
		if main >= d.AnimationTrigger {
			if main >= d.ShotTrigger {
				s.FireWait--
				if s.FireWait < 0 {
					s.FireWait = d.FireReload
					w.spawnBattleshipCoreShot(d, s)
				}
			}
			s.PatchTick--
			if s.PatchTick < 0 {
				s.PatchTick = d.IdleReload
				s.PatchCursor %= len(d.IdlePatches)
				w.applyBattleshipPatch(d.IdlePatches[s.PatchCursor])
				s.PatchCursor++
			}
		}
		if main == d.Trigger {
			e := w.newBossActor(d.Descriptor, BossArt{Width: 0, Height: 1}, d.X, d.Y, bossThreeCore)
			e.Hidden = true
			for _, box := range d.Collision {
				e.CollisionFrames = append(e.CollisionFrames, CollisionFrame{X: box.X, Y: box.Y, Width: box.Width, Height: box.Height})
			}
			w.Enemies = append(w.Enemies, e)
			s.Spawned = true
			w.Boss.MusicSection = 2
			w.Events = append(w.Events, Event{Kind: EventBossStarted, Value: 2})
		}
		if main >= d.Trigger {
			s.CycleWait--
			if s.CycleWait < 0 {
				s.Phase, s.PatchCursor, s.PatchTick, s.CycleWait = 1, 0, 0, d.CycleCount
			}
		}
	case 1, 3:
		s.PatchTick--
		if s.PatchTick < 0 {
			s.PatchTick = d.PatchReload
			w.applyBattleshipPatch(d.CyclePatches[s.PatchCursor])
			s.PatchCursor++
			s.CycleWait--
			if s.CycleWait == 0 {
				if s.Phase == 1 {
					s.Phase, s.PatchTick = 2, d.CycleHold
				} else {
					s.Phase, s.PatchTick, s.PatchCursor, s.CycleWait = 0, 0, 0, d.CycleWait
				}
			}
		}
	case 2:
		s.PatchTick--
		if s.PatchTick < 0 {
			s.Phase, s.CycleWait = 3, d.CycleCount
		}
	}
}

func (w *World) battleshipCoreDebris(s *OriginalBattleshipCoreState) {
	var x, y int
	for {
		y = s.Y + int(w.nextRandom()%150) - 80
		if y >= 0 && y <= 224 {
			break
		}
	}
	for {
		x = s.X + int(w.nextRandom()%160)
		if x >= 0 && x <= 350 {
			break
		}
	}
	if effect, ok := w.originalDeathEffect(Entity{X: float64(x - GameOriginX), Y: float64(y - GameOriginY)}, 0); ok {
		effect.Priority = 10
		w.Effects = append(w.Effects, effect)
	}
}
