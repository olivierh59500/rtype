package sim

import "github.com/olivierh59500/r-type/internal/source"

// OriginalFinalBossState translates the final-stage map hook at 0x4A420.
// Its two map scripts, damage transition and ending phases have independent
// counters, preserving the original signed-byte timer boundaries.
type OriginalFinalBossState struct {
	Initialized                                    bool
	Phase                                          int
	PrimaryCursor, PrimaryTimer                    int
	PrimaryRepeats                                 []int
	SecondaryCursor, SecondaryTimer                int
	SecondaryActive, ShotToggle                    bool
	CellTimer, SporeReload                         int
	Timer, SubTimer, Steps, MessageCursor          int
	PostPAL                                        int
	ControlsDisabled, MusicRequested, MusicStarted bool
}

func (w *World) finalBossState(spec *BossSpec) *OriginalFinalBossState {
	if w.Boss.Final == nil {
		d := spec.Eighth
		f := &OriginalFinalBossState{Initialized: true, CellTimer: d.CellFirst, SporeReload: d.SporeReload}
		for _, step := range d.Primary {
			f.PrimaryRepeats = append(f.PrimaryRepeats, step.Repeat)
		}
		w.Boss.Final = f
		for _, patch := range d.InitialPatches {
			w.applyFinalMapPatch(patch)
		}
	}
	return w.Boss.Final
}

func (w *World) applyFinalMapPatch(patch *source.MapPatch) {
	if patch == nil {
		return
	}
	for index, word := range patch.Words {
		w.SetTileAt(patch.Column+index%patch.Width, patch.Row+index/patch.Width, word)
	}
}

// updateBossEightActors runs before the original collision pass. A newly
// accumulated brain hit changes its state on the following main update.
func (w *World) updateBossEightActors(spec *BossSpec) {
	f, d := w.finalBossState(spec), spec.Eighth
	count := len(w.Enemies)
	for index := 0; index < count; index++ {
		enemy := &w.Enemies[index]
		if enemy.Health <= 0 || !w.selectedActor(enemy) {
			continue
		}
		enemy.OriginalCollisionPaused = enemy.ContactCooldown > 0
		if !w.actorPassRunning && enemy.ContactCooldown > 0 {
			enemy.ContactCooldown--
		}
		switch enemy.bossRole {
		case bossEightBrain:
			w.Boss.DamageTaken = enemy.DamageTaken
			w.Boss.Health = max(0, d.DamageLimit-enemy.DamageTaken)
			if f.Phase == 0 && enemy.DamageTaken >= d.DamageLimit {
				f.Phase = 1
				f.Timer = 40
				w.Boss.Defeated, w.Boss.Exiting = true, true
				enemy.Health = 0
				w.addScore(d.Score)
				w.Events = append(w.Events, Event{Kind: EventBossDefeated, Value: w.Stage, X: enemy.X, Y: enemy.Y})
			}
		case bossEightCell:
			enemy.Age++
			if enemy.bossBroken {
				if enemy.bossAnimationIndex >= d.CellExplosionStopIndex {
					enemy.Health = 0
					continue
				}
				advanceFinalCellAnimation(enemy)
				continue
			}
			if enemy.DamageTaken >= 1 {
				enemy.bossBroken = true
				enemy.MotionTick = 0
				art := spec.EighthExplosionArt
				enemy.X = enemy.X - enemy.W/2 + float64(art.Width)/2
				enemy.Y = enemy.Y - enemy.H/2 + float64(art.Height)/2
				enemy.W, enemy.H = float64(art.Width), float64(art.Height)
				enemy.Sprite = art.Sprite
				enemy.Descriptor = d.CellExplosionDescriptor
				enemy.Animation = d.CellExplosionAnimation
				enemy.AnimationTicks = d.CellExplosionTicks
				enemy.AnimationOverride = true
				enemy.bossAnimationIndex, enemy.bossAnimationTimer = 0, 0
				enemy.CollisionFrames = art.CollisionFrames
				advanceFinalCellAnimation(enemy)
				w.addScore(200)
				w.Events = append(w.Events, Event{Kind: EventSound, Value: 10, X: enemy.X, Y: enemy.Y})
				continue
			}
			enemy.X += enemy.VX
			enemy.Y += enemy.VY
			x, y := enemy.X-enemy.W/2+GameOriginX, enemy.Y-enemy.H/2+GameOriginY
			if x < 16 || x >= 360 || y < 0 || y > 224 {
				enemy.Health = 0
				continue
			}
			advanceFinalCellAnimation(enemy)
		case bossEightSpore:
			if f.Phase > 0 {
				enemy.Health = 0
				continue
			}
			enemy.Age++
			enemy.bossDelay--
			if enemy.bossDelay < 0 {
				enemy.bossDelay = f.SporeReload
				x, y := int(enemy.X-enemy.W/2)+GameOriginX, int(enemy.Y-enemy.H/2)+GameOriginY
				targetX, targetY := w.originalPlayerAim()
				direction := originalAimDirection(x, y, targetX, targetY, w.weaponTables())
				enemy.Variant = originalTurnDirection(direction, enemy.Variant)
			}
			v := d.SporeVelocity[enemy.Variant&15]
			enemy.X += float64(v[0])
			enemy.Y += float64(v[1])
			enemy.Y = clamp(enemy.Y, enemy.H/2-GameOriginY, 223+enemy.H/2-GameOriginY)
		case bossEightMessage:
			enemy.Age++
			enemy.X += enemy.VX
			if enemy.X-enemy.W/2+GameOriginX >= 360 {
				enemy.Health = 0
			}
		}
	}
}

func (w *World) updateBossEight(spec *BossSpec) {
	f, d := w.finalBossState(spec), spec.Eighth
	scroll := int(w.Scroll)
	switch f.Phase {
	case 0:
		f.CellTimer--
		if f.CellTimer == 0 {
			f.CellTimer = d.CellReload
			w.spawnFinalCell(spec)
		}
		if scroll < d.AnimationScroll {
			return
		}
		if !w.Boss.ControllerActive && scroll >= d.BrainScroll {
			w.Boss.Active, w.Boss.ControllerActive = true, true
			w.Boss.MusicSection = 2
			w.Boss.Health = d.DamageLimit
			art := BossArt{Width: 1, Height: 1, CollisionFrames: []CollisionFrame{{X: d.BrainCollision.X, Y: d.BrainCollision.Y, Width: d.BrainCollision.Width, Height: d.BrainCollision.Height}}}
			enemy := w.newBossActor(d.BrainDescriptor, art, d.BrainX, d.BrainY, bossEightBrain)
			enemy.Hidden = true
			enemy.Invincible = false
			enemy.Health = d.BrainHP
			enemy.MaxHealth = d.BrainHP
			if enemy.Health == 0 {
				enemy.Health = 2000
				enemy.MaxHealth = 2000
			}
			w.Boss.ControllerID = enemy.ID
			w.Enemies = append(w.Enemies, enemy)
			w.Events = append(w.Events, Event{Kind: EventBossStarted, Value: 2})
		}
		if scroll >= d.FreezeScroll {
			w.Boss.CameraPaused = true
		}
		f.PrimaryTimer--
		if f.PrimaryTimer < 0 {
			f.PrimaryTimer = 4
			w.stepFinalPrimary(spec)
		}
		if f.SecondaryActive {
			f.SecondaryTimer--
			if f.SecondaryTimer < 0 {
				f.SecondaryTimer = 4
				w.stepFinalSecondary(spec)
			}
		}
	case 1:
		f.Timer--
		if f.Timer < 0 {
			f.Phase = 2
			f.Steps = 15
			f.SubTimer = 0
			w.Boss.CameraPaused = false
			return
		}
		if f.Timer == 20 {
			w.applyFinalMapPatch(d.DestroyedPatch)
		}
		w.spawnFinalExplosion(spec)
	case 2:
		if scroll < 1695 {
			return
		}
		f.SubTimer--
		if f.SubTimer < 0 {
			f.SubTimer = 2
			w.finalPaletteDown(d)
			f.Steps--
			if f.Steps == 0 {
				f.Phase = 3
				f.Steps = 15
				w.Scroll = 1985
			}
		}
	case 3:
		if scroll < 1990 {
			return
		}
		f.SubTimer--
		if f.SubTimer < 0 {
			f.SubTimer = 3
			w.finalPaletteUp(d)
			f.Steps--
			if f.Steps == 0 {
				f.Phase = 4
				f.Timer = 1
				f.MessageCursor = 0
			}
		}
	case 4:
		f.Timer--
		if f.Timer != 0 {
			return
		}
		for f.MessageCursor < len(d.Messages) {
			message := d.Messages[f.MessageCursor]
			f.MessageCursor++
			enemy := w.newBossActor(d.MessageDescriptor, spec.EighthMessageArt, 16, message.Y, bossEightMessage)
			enemy.VX = float64(message.VX)
			enemy.AnimationOverride = true
			enemy.AnimationFrame = message.Frame
			enemy.NoCollidable = true
			w.Enemies = append(w.Enemies, enemy)
			if message.Delay != 0 {
				f.Timer = message.Delay
				return
			}
		}
		f.Phase = 5
		f.Timer = 144
	case 5:
		f.Timer--
		if f.Timer == 0 {
			f.Phase = 6
			f.Steps = 15
			f.SubTimer = 0
		}
	case 6:
		f.SubTimer--
		if f.SubTimer < 0 {
			f.SubTimer = 2
			w.finalPaletteDown(d)
			f.Steps--
			if f.Steps == 0 {
				f.Phase = 7
				f.PostPAL = 525
				w.Boss.CameraPaused = true
			}
		}
	case 7:
		// The original returns from the main loop, shows the score table,
		// waits 525 PAL interrupts, and returns to the title flow.
	}
}

func (w *World) stepFinalPrimary(spec *BossSpec) {
	f, d := w.Boss.Final, spec.Eighth
	for count := 0; count < 64 && f.PrimaryCursor < len(d.Primary); count++ {
		index := f.PrimaryCursor
		step := d.Primary[index]
		if step.Patch != nil {
			w.applyFinalMapPatch(step.Patch)
			f.PrimaryCursor++
			return
		}
		if step.Repeat > 0 {
			f.PrimaryRepeats[index]--
			if f.PrimaryRepeats[index] != 0 {
				f.PrimaryCursor = step.Target
				continue
			}
			f.PrimaryRepeats[index] = step.Repeat
			f.PrimaryCursor++
			f.SecondaryActive = true
			f.SecondaryTimer = 0
			f.SecondaryCursor = 0
		} else {
			f.PrimaryCursor = step.Target
		}
	}
}

func (w *World) stepFinalSecondary(spec *BossSpec) {
	f, d := w.Boss.Final, spec.Eighth
	if f.SecondaryCursor >= len(d.Secondary) {
		f.SecondaryActive = false
		return
	}
	step := d.Secondary[f.SecondaryCursor]
	f.SecondaryCursor++
	if step.Shot {
		f.ShotToggle = !f.ShotToggle
		if !f.ShotToggle {
			w.spawnFinalSpore(spec)
			if f.SporeReload > 0 {
				f.SporeReload--
			}
		}
	}
	if step.Stop {
		f.SecondaryActive = false
		return
	}
	w.applyFinalMapPatch(step.Patch)
}

func (w *World) spawnFinalCell(spec *BossSpec) {
	d := spec.Eighth
	y := 0
	if w.nextRandom()%10 >= 5 {
		y = 224
	}
	x := int(w.nextRandom()%300) + 45
	targetX, targetY := w.originalPlayerAim()
	direction := originalAimDirection(x, y, targetX, targetY, w.weaponTables())
	if direction == 4 || direction == 12 {
		return
	}
	enemy := w.newBossActor(d.CellDescriptor, spec.EighthCellArt, x, y, bossEightCell)
	// Source HP is -1: contacts accumulate damage without the ordinary hurt gate.
	enemy.Invincible = true
	enemy.VX = float64(d.CellVelocity[direction][0])
	enemy.VY = float64(d.CellVelocity[direction][1])
	enemy.Animation = d.CellAnimation[direction]
	enemy.AnimationTicks = d.CellAnimationTicks
	enemy.AnimationOverride = true
	w.Enemies = append(w.Enemies, enemy)
}

func (w *World) spawnFinalSpore(spec *BossSpec) {
	w.spawnFinalSporeAt(spec, 300, 120, 12, 20)
}

// spawnFinalSporeAt shares the original movement callback while preserving
// the distinct ordinary-event and arena constructors' initial conditions.
func (w *World) spawnFinalSporeAt(spec *BossSpec, x, y, direction, wait int) {
	enemy := w.newBossActor(spec.Eighth.SporeDescriptor, spec.EighthSporeArt, x, y, bossEightSpore)
	enemy.NoFriendlyTarget = true
	enemy.Priority = 10
	enemy.Variant = direction
	enemy.bossDelay = wait
	enemy.Animation = spec.Eighth.SporeAnimation
	enemy.AnimationTicks = spec.Eighth.SporeAnimationTicks
	w.Enemies = append(w.Enemies, enemy)
}

func (w *World) spawnFinalExplosion(spec *BossSpec) {
	y := 30 + int(w.nextRandom()%170)
	x := 200 + int(w.nextRandom()%130)
	anchor := Entity{X: float64(x - GameOriginX), Y: float64(y - GameOriginY)}
	if effect, ok := w.originalDeathEffect(anchor, 2); ok {
		w.Effects = append(w.Effects, effect)
	}
}

// advanceFinalCellAnimation models the source index/timer independently of
// age, because damage resets both and retirement tests the next stream index.
func advanceFinalCellAnimation(enemy *Enemy) {
	enemy.bossAnimationTimer--
	if enemy.bossAnimationTimer >= 0 || len(enemy.Animation) == 0 {
		return
	}
	enemy.bossAnimationTimer = max(1, enemy.AnimationTicks) - 1
	index := enemy.bossAnimationIndex
	if index >= len(enemy.Animation) {
		index = 0
	}
	enemy.AnimationFrame = enemy.Animation[index]
	enemy.bossAnimationIndex = index + 1
}

func (w *World) finalPaletteDown(d *source.BossEightData) {
	if len(w.Boss.FinalPalette) == 0 {
		w.Boss.FinalPalette = append([]uint16(nil), d.Palette...)
	}
	for index := 1; index < len(w.Boss.FinalPalette); index++ {
		word := w.Boss.FinalPalette[index]
		for shift := 0; shift < 12; shift += 4 {
			if word&(15<<shift) != 0 {
				word -= 1 << shift
			}
		}
		w.Boss.FinalPalette[index] = word
	}
}

func (w *World) finalPaletteUp(d *source.BossEightData) {
	for index := 1; index < len(w.Boss.FinalPalette) && index <= len(d.EndingPalette); index++ {
		word, target := w.Boss.FinalPalette[index], d.EndingPalette[index-1]
		for shift := 0; shift < 12; shift += 4 {
			if (word>>shift)&15 < (target>>shift)&15 {
				word += 1 << shift
			}
		}
		w.Boss.FinalPalette[index] = word
	}
}

// advanceFinalPowersPAL precedes the Force controller and main input gate.
// The brain only selects automatic movement. Core 0x2348 resets powers on
// each following PAL until the first fade's music request sets flag 109 bit 7.
// VBlank consumes that request before the following PAL player route.
func (w *World) advanceFinalPowersPAL() {
	f := w.Boss.Final
	if w.sceneRules() == nil || f == nil || f.Phase == 0 {
		return
	}
	if f.MusicRequested {
		f.MusicStarted = true
		return
	}
	// Core 0x219A returns before the automatic route when death is pending.
	// Music consumption above belongs to the earlier VBlank audio gate.
	if w.Scene.DeathRequested {
		return
	}
	w.resetOriginalPowers()
	f.ControlsDisabled = true
	if f.Phase >= 3 {
		f.MusicRequested = true
	}
}

func (w *World) finalControlsDisabled() bool {
	return w.Boss.Final != nil && w.Boss.Final.ControlsDisabled
}

func (w *World) updateFinalPlayerExit() {
	f := w.Boss.Final
	x, y := 87.0, 93.0
	if f.Phase >= 5 {
		x, y = 337, -27
	}
	w.Player.VX, w.Player.VY = 0, 0
	if w.Player.X < x {
		w.Player.VX = 1
	} else if w.Player.X > x {
		w.Player.VX = -1
	}
	if w.Player.Y < y {
		w.Player.VY = 1
	} else if w.Player.Y > y {
		w.Player.VY = -1
	}
	w.Player.X += w.Player.VX
	w.Player.Y += w.Player.VY
	if f.Phase == 7 {
		f.PostPAL--
		if f.PostPAL <= 0 {
			if w.sceneRules() != nil {
				w.beginScoreFlow()
			} else {
				w.State = Complete
			}
			f.Phase = 8
		}
	}
}

// Ending reports the original final-stage automatic ship and ending sequence.
func (w *World) Ending() bool { return w.Boss.Final != nil && w.Boss.Final.Phase > 0 }
