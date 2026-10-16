package sim

import "github.com/olivierh59500/r-type/internal/source"

// OriginalCaveRecord retains one map creature's callback phase and byte counters.
type OriginalCaveRecord struct {
	Phase, Frame, Timer, Steps int
	Broken                     bool
}

// OriginalCaveState belongs to the stage map and survives actor replacement.
type OriginalCaveState struct{ Records [2]OriginalCaveRecord }

// OriginalCaveShotState retains curve, bounce and aiming state for one projectile.
type OriginalCaveShotState struct {
	Bouncing, Up, Homing bool
	Curve, Index         int
	VX, VY               int
	Timer, Direction     int
	LastDamage, AimWait  int
}

func (w *World) initializeCave(data *source.CaveData, previous *OriginalCaveState, retain bool) {
	state := &OriginalCaveState{}
	for index, creature := range data.Creatures {
		state.Records[index].Frame = creature.InitialFrame
		// The actual initializer clears the broken flag and selects the
		// waiting handler/frame, leaving its two inactive byte counters alone.
		if retain && previous != nil {
			state.Records[index].Timer = previous.Records[index].Timer
			state.Records[index].Steps = previous.Records[index].Steps
		}
	}
	w.Boss.Cave = state
}

func (w *World) cavePatch(index, frame int) {
	data := w.Stages[w.Stage-1].Cave.Creatures[index]
	for word, value := range data.Frames[frame] {
		w.SetTileAt(data.Column+word%4, data.Row+word/4, value)
	}
}

// updateCaveMap translates the two 0x4AB90 callbacks inside map hook 0x4A784.
// Map steps follow the actor pass, so collision uses the preceding map frame.
func (w *World) updateCaveMap() {
	data := w.Stages[w.Stage-1].Cave
	state := w.Boss.Cave
	if data == nil || state == nil {
		return
	}
	for index, creature := range data.Creatures {
		record := &state.Records[index]
		if record.Broken {
			record.Phase = 5
			w.cavePatch(index, 5)
			continue
		}
		switch record.Phase {
		case 0:
			if int(w.Scroll) == creature.SpawnScroll {
				art := BossArt{Width: 0, Height: 1}
				for _, box := range data.BodyCollisions {
					art.CollisionFrames = append(art.CollisionFrames, CollisionFrame{X: box.X, Y: box.Y, Width: box.Width, Height: box.Height})
				}
				enemy := w.newBossActor(data.BodyDescriptor, art, 363, creature.Y, bossCaveCreature)
				enemy.Hidden, enemy.Invincible, enemy.AnimationOverride = true, false, true
				enemy.Health, enemy.MaxHealth = data.BodyHP, data.BodyHP
				enemy.bossSegment, enemy.AnimationFrame = index, creature.InitialFrame
				enemy.bossDelay, enemy.bossMotion = data.ShotDelays[0], data.ShotDelays[1]
				w.Enemies = append(w.Enemies, enemy)
				record.Phase = 1
			}
		case 1:
			if int(w.Scroll) == creature.OpenScroll {
				record.Timer, record.Steps, record.Phase = 0, 4, 2
			}
		case 2, 4:
			record.Timer--
			if record.Timer < 0 {
				record.Timer = 5
				if record.Phase == 2 {
					record.Frame++
				} else {
					record.Frame--
				}
				w.cavePatch(index, record.Frame-creature.InitialFrame)
				record.Steps--
				if record.Steps == 0 {
					if record.Phase == 2 {
						record.Phase = 3
					} else {
						record.Phase = 5
					}
				}
			}
		case 3:
			if int(w.Scroll) == creature.CloseScroll {
				record.Timer, record.Steps, record.Phase = 0, 4, 4
			}
		}
	}
}

// updateCaveActors translates the invisible bodies and their ordinary actors;
// the shared actor pass still owns allocation, contacts and replacement effects.
func (w *World) updateCaveActors() {
	data := w.Stages[w.Stage-1].Cave
	if data == nil {
		return
	}
	count := len(w.Enemies)
	for index := 0; index < count; index++ {
		if !w.selectedActor(&w.Enemies[index]) || w.Enemies[index].Health <= 0 {
			continue
		}
		// A shot allocation can grow the slice. Retain the current actor by
		// value until its callback returns instead of writing a stale pointer.
		enemy := w.Enemies[index]
		switch enemy.bossRole {
		case bossCaveCreature:
			enemy.Age++
			if enemy.bossDelay != 0 {
				enemy.bossDelay--
				if enemy.bossDelay == 0 {
					w.spawnCaveShot(enemy)
				}
			}
			if enemy.bossMotion != 0 {
				enemy.bossMotion--
				if enemy.bossMotion == 0 {
					w.spawnCaveShot(enemy)
				}
			}
			enemy.X--
			if int(enemy.X-enemy.W/2)+GameOriginX <= 16 {
				enemy.Health = 0
				break
			}
			record := &w.Boss.Cave.Records[enemy.bossSegment]
			enemy.AnimationFrame = record.Frame
			if enemy.DamageTaken >= data.DamageLimit {
				record.Broken = true
				if record.Frame >= 5 {
					enemy.Y -= 43
				}
				enemy.Health = 0
				w.addScore(data.BodyScore)
				w.bossImmediateDeath(enemy.Entity, data.BodyDeathKind)
			}
		case bossCaveShot:
			enemy.Age++
			w.updateCaveShot(&enemy)
		default:
			continue
		}
		w.Enemies[index] = enemy
	}
}

func (w *World) spawnCaveShot(parent Enemy) {
	stage, data := w.Stages[w.Stage-1], w.Stages[w.Stage-1].Cave
	x, y := int(parent.X-parent.W/2)+GameOriginX+10, int(parent.Y-parent.H/2)+GameOriginY+8
	if y <= 100 {
		y -= 24
	}
	enemy := w.newBossActor(data.ShotDescriptor, stage.CaveShotArt, x, y, bossCaveShot)
	enemy.Health, enemy.MaxHealth, enemy.Invincible = data.ShotHP, data.ShotHP, false
	enemy.Score = data.ShotScore
	enemy.AnimationTicks, enemy.AnimationOverride = data.AnimationTicks, true
	curve := 0
	if w.nextRandom()%100 <= 70 {
		curve = 1
	}
	enemy.CaveShot = &OriginalCaveShotState{Curve: curve, Up: y <= 90, Timer: data.CurveTimers[curve]}
	w.Enemies = append(w.Enemies, enemy)
}

func (w *World) updateCaveShot(enemy *Enemy) {
	data := w.Stages[w.Stage-1].Cave
	shot := enemy.CaveShot
	x, y := int(enemy.X-enemy.W/2)+GameOriginX, int(enemy.Y-enemy.H/2)+GameOriginY
	if !shot.Bouncing {
		curve := data.Curves[shot.Curve]
		switchToBounce := enemy.DamageTaken != shot.LastDamage
		if !switchToBounce {
			index := shot.Index
			shot.Index++
			if index >= len(curve.Y) || curve.Y[index] == 127 {
				switchToBounce = true
			} else {
				dx, dy := int(curve.X[index]), int(curve.Y[index])
				if !shot.Up {
					dy = -dy
				}
				if x+dx < 32 || x+dx > 302 {
					switchToBounce = true
				} else {
					x, y = x+dx, y+dy
				}
			}
		}
		if switchToBounce {
			shot.Bouncing, shot.LastDamage, shot.Timer = true, enemy.DamageTaken, 15
			shot.Up, shot.Curve, shot.Index, shot.VX, shot.VY = false, 0, 0, 0, 0
			enemy.bossAnimationIndex, enemy.bossAnimationTimer = 5, 0
		}
	} else {
		shot.Timer--
		if shot.Timer < 0 {
			shot.Homing = true
			shot.AimWait--
			if shot.AimWait < 0 {
				shot.AimWait = 4
				targetX, targetY := w.originalPlayerAim()
				shot.Direction = originalAimDirection(x, y, targetX, targetY, w.weaponTables())
			} else {
				shot.Direction = int(w.nextRandom() % 16)
			}
			index := shot.Direction
			if w.nextRandom()%100 <= 90 {
				index += 16
			}
			shot.VX, shot.VY = data.Directions[index][0], data.Directions[index][1]
			shot.Timer = int(w.nextRandom()%100) + 20
		} else if shot.Homing && shot.LastDamage != enemy.DamageTaken {
			shot.LastDamage = enemy.DamageTaken
			shot.VX, shot.VY = 0, 8
			shot.Timer = int(uint16(192-y)>>3) - 2
		}
		for shot.Homing {
			if x+shot.VX >= 48 && x+shot.VX <= 302 && y+shot.VY >= 48 && y+shot.VY <= 192 {
				x, y = x+shot.VX, y+shot.VY
				break
			}
			w.reflectCaveShot(shot, data)
		}
	}
	enemy.X, enemy.Y = float64(x-GameOriginX)+enemy.W/2, float64(y-GameOriginY)+enemy.H/2
	stream := data.ShotAnimation
	if shot.Bouncing {
		stream = data.BounceAnimation
	}
	enemy.bossAnimationTimer--
	if enemy.bossAnimationTimer < 0 {
		enemy.bossAnimationTimer = data.AnimationTicks - 1
		index := enemy.bossAnimationIndex
		if index >= len(stream) || stream[index] < 0 {
			index = 0
		}
		enemy.AnimationFrame = stream[index]
		enemy.bossAnimationIndex = index + 1
	}
}

func (w *World) reflectCaveShot(shot *OriginalCaveShotState, data *source.CaveData) {
	shot.Timer = int(uint16(shot.Timer) >> 1)
	shot.Direction = data.Opposite[shot.Direction]
	shot.VX, shot.VY = data.Directions[shot.Direction][0], data.Directions[shot.Direction][1]
}
