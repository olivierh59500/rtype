package sim

import "github.com/olivierh59500/r-type/internal/source"

// OriginalBossSevenState keeps the five independent original lane counters.
// PatchTimer is a signed-byte clock; lane timers are signed words.
type OriginalBossSevenState struct {
	PatchTimer             int
	LaneTimers, LaneFrames []int
	OpenedMask             int
}

func (w *World) bossSevenState(spec *BossSpec) *OriginalBossSevenState {
	if w.Boss.Seven == nil {
		s := &OriginalBossSevenState{}
		for _, lane := range spec.Seven.Lanes {
			s.LaneTimers = append(s.LaneTimers, lane.InitialDelay)
			s.LaneFrames = append(s.LaneFrames, 0)
		}
		w.Boss.Seven = s
		for _, patch := range spec.Seven.InitialPatches {
			w.patchBossSeven(patch)
		}
	}
	return w.Boss.Seven
}

func (w *World) patchBossSeven(patch *source.MapPatch) {
	if patch == nil || patch.Width < 1 {
		return
	}
	for index, value := range patch.Words {
		w.SetTileAt(patch.Column+index%patch.Width, patch.Row+index/patch.Width, value)
	}
}

// bossSevenAnimate translates core 0x117C without the hardware blitter.
// Index advances after choosing art, so an enabled stream index is one ahead.
func bossSevenAnimate(enemy *Enemy, stream []int, automatic bool) {
	enemy.bossMotion--
	if enemy.bossMotion >= 0 || len(stream) == 0 {
		return
	}
	enemy.bossMotion = max(1, enemy.AnimationTicks) - 1
	index := enemy.AnimationStart
	if index < 0 || index >= len(stream) {
		index = 0
	}
	enemy.AnimationFrame = stream[index]
	enemy.AnimationStart = index
	if automatic {
		enemy.AnimationStart++
	}
}

func (w *World) updateBossSevenActors(spec *BossSpec) {
	w.bossSevenState(spec)
	d := spec.Seven
	count := len(w.Enemies)
	for index := 0; index < count; index++ {
		enemy := &w.Enemies[index]
		if enemy.Health <= 0 || !w.selectedActor(enemy) {
			continue
		}
		switch enemy.bossRole {
		case bossSevenController:
			enemy.Age++
			enemy.MaskFlash, enemy.maskFlashPending = enemy.maskFlashPending, false
			automatic := enemy.BehaviorPhase == 2
			switch enemy.BehaviorPhase {
			case 0:
				enemy.bossDelay--
				if enemy.bossDelay < 0 {
					enemy.AnimationStart = 0
					enemy.BehaviorPhase = 1
				}
			case 1:
				enemy.X += float64(d.LeftVX)
				if int(enemy.X-enemy.W/2+GameOriginX) <= d.LeftLimit {
					enemy.BehaviorPhase = 2
					automatic = true
				}
			case 2:
				if enemy.AnimationStart == d.StopAnimationIndex {
					enemy.BehaviorPhase = 3
					automatic = false
					enemy.X += float64(d.RightVX)
				}
			case 3:
				enemy.X += float64(d.RightVX)
				if int(enemy.X-enemy.W/2+GameOriginX) >= d.RightLimit {
					enemy.BehaviorPhase = 0
					enemy.bossDelay = d.ReturnWait
				}
			}
			w.Boss.Health = max(0, d.DamageLimit-enemy.DamageTaken)
			w.Boss.HitFlash = w.Boss.DamageTaken != enemy.DamageTaken
			w.Boss.DamageTaken = enemy.DamageTaken
			if enemy.DamageTaken >= d.DamageLimit {
				enemy.Health = 0
				w.Boss.Defeated = true
				w.addScore(d.Score)
				w.Events = append(w.Events, Event{Kind: EventBossDefeated, Value: w.Stage, X: enemy.X, Y: enemy.Y})
				w.startBossVictory(spec)
				continue
			}
			bossSevenAnimate(enemy, d.Animation, automatic)
		case bossSevenFalling:
			enemy.Age++
			enemy.Y += float64(d.FallingVY)
			if int(enemy.Y-enemy.H/2+GameOriginY) >= d.FallingLimit {
				enemy.Health = 0
				w.bossImmediateDeath(enemy.Entity, 0)
				continue
			}
			variant := d.Variants[enemy.bossSegment]
			bossSevenAnimate(enemy, variant.Animation, variant.Animated)
		case bossSevenGuard:
			enemy.Age++
			enemy.MaskFlash, enemy.maskFlashPending = enemy.maskFlashPending, false
			w.updateBossSevenGuard(enemy, d)
			if enemy.Health > 0 {
				bossSevenAnimate(enemy, d.GuardAnimation, false)
			}
		}
	}
}

func (w *World) updateBossSevenGuard(enemy *Enemy, d *source.BossSevenData) {
	if enemy.BehaviorPhase == 0 {
		if enemy.bossDelay > 0 {
			enemy.bossDelay--
			return
		}
		enemy.Y += float64(d.GuardRiseVY)
		if int(enemy.Y-enemy.H/2+GameOriginY) <= d.GuardTopY {
			enemy.DamageTaken = 0
			enemy.Health = enemy.MaxHealth
			enemy.BehaviorPhase = 1
		}
		return
	}
	if enemy.DamageTaken >= d.GuardDamageLimit {
		w.bossSevenEffect(int(enemy.X-enemy.W/2+GameOriginX), int(enemy.Y-enemy.H/2+GameOriginY), d, false)
		enemy.bossDelay = d.GuardReturnWait
		enemy.X = float64(d.GuardXBase+int(w.nextRandom()%uint64(max(1, d.GuardXRange)))) - GameOriginX + enemy.W/2
		enemy.Y = float64(d.GuardY) - GameOriginY + enemy.H/2
		enemy.BehaviorPhase = 0
		w.addScore(d.GuardScore)
		return
	}
	if w.Boss.Defeated {
		enemy.Health = 0
		return
	}
	direction := w.originalAimDirection(*enemy)
	enemy.AnimationStart = direction
	enemy.ShootCooldown--
	if enemy.ShootCooldown != 0 || direction >= len(d.GuardMuzzle) {
		return
	}
	muzzle := d.GuardMuzzle[direction]
	x, y := enemy.X-enemy.W/2+float64(muzzle[0]), enemy.Y-enemy.H/2+float64(muzzle[1])
	aim := *enemy
	aim.X, aim.Y = x+enemy.W/2, y+enemy.H/2
	shotDirection := w.originalAimDirection(aim)
	if shotDirection < len(d.ShotVX) && shotDirection < len(d.ShotVY) {
		w.addBullet(EnemyShot, false, x+5, y+4, float64(d.ShotVX[shotDirection]), float64(d.ShotVY[shotDirection]), 12, 12, 1, 250, false)
	}
	enemy.ShootCooldown = d.GuardFireBase + int(w.nextRandom()%uint64(max(1, d.GuardFireRange)))
}

func (w *World) updateBossSeven(spec *BossSpec) {
	s, d := w.bossSevenState(spec), spec.Seven
	if w.Boss.Defeated {
		y := 40 + int(w.nextRandom()%164)
		x := 40 + int(w.nextRandom()%280)
		w.bossSevenEffect(x, y, d, true)
		return
	}
	if !w.Boss.Active && int(w.Scroll) >= d.Trigger {
		w.Boss.Active, w.Boss.ControllerActive = true, true
		w.Boss.Health = d.DamageLimit
		w.Boss.MusicSection = 2
		w.Events = append(w.Events, Event{Kind: EventBossStarted, Value: 2})
		guard := w.newBossActor(d.GuardDescriptor, spec.SevenGuardArt, d.GuardXBase+int(w.nextRandom()%uint64(max(1, d.GuardXRange))), d.GuardY, bossSevenGuard)
		guard.Invincible = false
		guard.Health, guard.MaxHealth = d.GuardHealth, d.GuardHealth
		guard.AnimationOverride, guard.AnimationTicks = true, 1
		guard.ShootCooldown = d.GuardInitialFire
		for _, value := range d.AimMap {
			guard.AimMap = append(guard.AimMap, int8(value))
		}
		guard.AimThresholds = d.AimThresholds
		w.Enemies = append(w.Enemies, guard)
		controller := w.newBossActor(d.Descriptor, spec.SevenArt, d.SpawnX, d.SpawnY, bossSevenController)
		controller.Invincible = false
		controller.Health, controller.MaxHealth = d.ControllerHealth, d.ControllerHealth
		controller.bossDelay, controller.AnimationTicks, controller.AnimationOverride = d.InitialWait, d.AnimationTicks, true
		w.Boss.ControllerID = controller.ID
		w.Enemies = append(w.Enemies, controller)
	}
	if int(w.Scroll) < d.FreezeScroll {
		return
	}
	w.Boss.CameraPaused = true
	if s.OpenedMask != (1<<len(d.Lanes))-1 {
		s.PatchTimer--
		if s.PatchTimer >= 0 {
			return
		}
		s.PatchTimer = d.PatchReload
		for lane, data := range d.Lanes {
			if s.LaneTimers[lane] != 0 {
				s.LaneTimers[lane]--
				continue
			}
			frame := s.LaneFrames[lane]
			if frame >= len(data.Patches) {
				s.OpenedMask |= 1 << (len(d.Lanes) - lane - 1)
				s.LaneTimers[lane] = d.OpeningWaitBase + int(w.nextRandom()%uint64(max(1, d.OpeningWaitRange)))
			} else {
				w.patchBossSeven(data.Patches[frame])
				s.LaneFrames[lane]++
			}
		}
		return
	}
	for lane, data := range d.Lanes {
		s.LaneTimers[lane]--
		if s.LaneTimers[lane] >= 0 || len(d.Variants) == 0 {
			continue
		}
		s.LaneTimers[lane] = d.SpawnWaitBase + int(w.nextRandom()%uint64(max(1, d.SpawnWaitRange)))
		variantIndex := int(w.nextRandom() % uint64(len(d.Variants)))
		variant := d.Variants[variantIndex]
		if variantIndex >= len(spec.SevenVariantArt) {
			continue
		}
		enemy := w.newBossActor(variant.Descriptor, spec.SevenVariantArt[variantIndex], data.SpawnX, d.SpawnYTop, bossSevenFalling)
		enemy.bossSegment = variantIndex
		enemy.AnimationStart, enemy.AnimationTicks, enemy.AnimationOverride = variant.InitialIndex, variant.AnimationTicks, true
		enemy.Invincible = false
		enemy.Score = variant.Score
		w.Enemies = append(w.Enemies, enemy)
	}
}

func (w *World) bossSevenEffect(x, y int, d *source.BossSevenData, victory bool) {
	kind := 0
	if victory {
		kind = 2
	}
	if effect, ok := w.originalDeathEffect(Entity{X: float64(x) - GameOriginX, Y: float64(y) - GameOriginY}, kind); ok {
		// Both original allocations use priority zero. A guard's child
		// prepends before its live cursor; victory children arrive after
		// the pass. Their first animation and sound start next main update.
		w.Effects = append(w.Effects, effect)
	}
}
