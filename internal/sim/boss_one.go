package sim

// spawnBossIfDue translates the original stage-one event spawners. The boss
// body is original map art; only its sixteen tail segments and hidden weak
// point are entities. The source scroll clock determines both trigger points.
func (w *World) spawnBossIfDue() {
	spec := w.Stages[w.Stage-1].Boss
	if spec != nil && spec.Fifth != nil {
		w.spawnBossFive(spec)
		return
	}
	if spec != nil && spec.Fourth != nil {
		w.spawnBossFour(spec)
		return
	}
	if spec != nil && spec.Kind == "stage2_gomander" {
		w.spawnBossTwoComponents(spec)
		return
	}
	if spec == nil || spec.Kind != "stage1_dobkeratops" {
		return
	}
	if !w.Boss.Active && int(w.Scroll) >= spec.TailTrigger {
		w.Boss.Active = true
		w.Boss.MusicSection = 2
		w.Events = append(w.Events, Event{Kind: EventBossStarted, Value: 2})
		for segment := range spec.TailFrames[0] {
			enemy := w.newBossActor(spec.TailDescriptor, spec.TailArt, 400, 5, bossTail)
			enemy.bossSegment = segment
			enemy.bossDelay = segment
			enemy.AnimationOverride = true
			enemy.AnimationFrame = 5
			w.Enemies = append(w.Enemies, enemy)
		}
	}
	if !w.Boss.ControllerActive && int(w.Scroll) >= spec.ControllerTrigger {
		w.Boss.ControllerActive = true
		enemy := w.newBossActor(spec.ControllerDescriptor, spec.ControllerArt, spec.ProjectileStartX, spec.ProjectileStartY, bossController)
		enemy.Hidden = true
		enemy.CollisionFrames = []CollisionFrame{spec.ControllerCollision}
		// The original 0x4B72A timer reload is 0x3F, firing after 64 calls.
		enemy.bossDelay = 0x3f
		w.Boss.ControllerID = enemy.ID
		w.Enemies = append(w.Enemies, enemy)
	}
}

func (w *World) newBossActor(descriptor uint32, art BossArt, x, y int, role bossRole) Enemy {
	width, height := float64(art.Width), float64(art.Height)
	return Enemy{Entity: w.newEntity(float64(x)-GameOriginX+width/2, float64(y)-GameOriginY+height/2, 0, 0, width, height), Kind: EnemyBoss, Health: 1, MaxHealth: 1, Invincible: true, Sprite: art.Sprite, Descriptor: descriptor, Animation: art.Animation, AnimationTicks: art.AnimationTicks, CollisionFrames: art.CollisionFrames, bossRole: role}
}

// updateBossActors preserves the original linked-list order: controller first,
// then the head driving the shared tail clock, then its remaining segments.
// Projectiles emitted by that controller are first updated next main tick.
func (w *World) updateBossActors() {
	spec := w.Stages[w.Stage-1].Boss
	if spec != nil && spec.Seven != nil {
		w.updateBossSevenActors(spec)
		return
	}
	if spec != nil && spec.Fifth != nil {
		w.updateBossFiveActors(spec)
		return
	}
	if spec != nil && spec.Eighth != nil {
		w.updateBossEightActors(spec)
		return
	}
	if spec != nil && spec.Fourth != nil {
		w.updateBossFourActors(spec)
		return
	}
	if spec != nil && spec.Kind == "stage3_battleship" {
		w.updateBattleshipMounted(spec)
		w.updateBattleshipIrisActor(spec)
		w.updateBattleshipCoreActors(spec)
		return
	}
	if spec != nil && spec.Kind == "stage2_gomander" {
		w.updateBossTwoActors(spec)
		return
	}
	if spec == nil || spec.Kind != "stage1_dobkeratops" || !w.Boss.Active {
		return
	}
	count := len(w.Enemies)
	for index := 0; index < count; index++ {
		if w.Enemies[index].bossRole != bossController || w.Enemies[index].Health <= 0 || !w.selectedActor(&w.Enemies[index]) {
			continue
		}
		enemy := w.Enemies[index]
		enemy.Age++
		w.updateBossController(&enemy, spec)
		w.Enemies[index] = enemy
	}
	tailClock := w.actorUpdateID == 0
	for _, e := range w.Enemies {
		if e.ID == w.actorUpdateID && e.bossRole == bossTail && e.bossSegment == len(spec.TailFrames[0])-1 {
			tailClock = true
		}
	}
	if !w.Boss.Defeated && tailClock {
		if !w.Boss.CameraPaused {
			w.Boss.TailX--
		}
		w.Boss.TailTicks--
		if w.Boss.TailTicks < 0 {
			w.Boss.TailTicks = spec.TailPeriod - 1
			w.Boss.TailFrame = (w.Boss.TailFrame + 1) % len(spec.TailFrames)
		}
	}
	for index := 0; index < count; index++ {
		enemy := &w.Enemies[index]
		if enemy.Health <= 0 || !w.selectedActor(enemy) {
			continue
		}
		switch enemy.bossRole {
		case bossTail:
			enemy.Age++
			enemy.MaskFlash = w.Boss.HitFlash && !w.Boss.Defeated
			if w.Boss.Defeated {
				if enemy.bossDelay < 0 {
					enemy.Health = 0
					w.bossImmediateDeath(enemy.Entity, 0)
				} else {
					enemy.bossDelay--
				}
				continue
			}
			position := spec.TailFrames[w.Boss.TailFrame][enemy.bossSegment]
			enemy.X = float64(w.Boss.TailX+position[0]) - GameOriginX + enemy.W/2
			enemy.Y = float64(position[1]+spec.TailYOffset) - GameOriginY + enemy.H/2
			if enemy.bossSegment == len(spec.TailFrames[0])-1 {
				enemy.AimMap, enemy.AimThresholds = spec.AimMap, spec.AimThresholds
				enemy.AnimationFrame = (w.originalAimDirection(*enemy) & 7) >> 1
			}
		case bossProjectile:
			enemy.Age++
			w.updateBossProjectile(enemy, spec)
		}
	}
}

func (w *World) updateBossController(enemy *Enemy, spec *BossSpec) {
	if !w.Boss.CameraPaused {
		enemy.X--
	}
	if w.Boss.Defeated {
		enemy.bossMotion--
		if enemy.bossMotion < 0 {
			enemy.bossMotion = spec.ControllerDeathReload
			kinds := spec.ControllerDeathKinds
			if len(kinds) > 0 {
				kind := kinds[int(w.nextRandom()%uint64(len(kinds)))]
				y := enemy.Y - enemy.H/2 + float64(spec.ControllerDeathYOffset+int(w.nextRandom()%uint64(max(1, spec.ControllerDeathYRange))))
				x := enemy.X - enemy.W/2 + float64(spec.ControllerDeathXOffset+int(w.nextRandom()%uint64(max(1, spec.ControllerDeathXRange))))
				w.bossDeferredDeath(Entity{X: x, Y: y}, kind)
			}
		}
		return
	}
	enemy.bossDelay--
	if enemy.bossDelay < 0 {
		enemy.bossDelay = 0x3f
		w.spawnBossVolley(*enemy, spec)
		w.Boss.EyeActive = true
	}
	w.Boss.HitFlash = enemy.DamageTaken != w.Boss.DamageTaken
	w.Boss.DamageTaken = enemy.DamageTaken
	w.Boss.Health = spec.ControllerHealth - enemy.DamageTaken
	if w.Boss.Health < 0 {
		w.Boss.Health = 0
	}
	if enemy.DamageTaken >= spec.DamageThreshold && w.Boss.ArenaTicks > spec.DamageArenaTimer {
		w.Boss.ArenaTicks = spec.DamageArenaTimer
	}
	if enemy.DamageTaken >= spec.ControllerHealth {
		w.Boss.Defeated = true
		w.startBossVictory(spec)
		w.addScore(spec.Score)
		w.Events = append(w.Events, Event{Kind: EventBossDefeated, Value: w.Stage, X: enemy.X, Y: enemy.Y})
	}
}

func (w *World) spawnBossVolley(controller Enemy, spec *BossSpec) {
	y := int(w.Player.Y - w.Player.H/2 + GameOriginY)
	direction := 0
	if y < spec.ProjectileAimLow {
		direction = -1
	} else if y >= spec.ProjectileAimHigh {
		direction = 1
	}
	for projectile, delay := range spec.ProjectileDelays {
		enemy := w.newBossActor(spec.ProjectileDescriptor, spec.ProjectileArt, spec.ProjectileStartX, spec.ProjectileStartY, bossProjectile)
		enemy.bossDelay = delay
		enemy.bossMotion = direction
		enemy.bossTargetX = controller.X - controller.W/2 + float64(spec.ProjectileTeleportX) + enemy.W/2
		enemy.AnimationStart = projectile * 2 % 8
		w.Enemies = append(w.Enemies, enemy)
	}
}

func (w *World) updateBossProjectile(enemy *Enemy, spec *BossSpec) {
	deferredDeath := w.Boss.Defeated
	if enemy.bossDelay > 0 {
		enemy.bossDelay--
		if enemy.bossDelay == 0 {
			enemy.X = enemy.bossTargetX
		}
		if deferredDeath {
			enemy.Health = 0
			w.bossImmediateDeath(enemy.Entity, 0)
		}
		return
	}
	if enemy.bossMotion == 0 {
		enemy.X += float64(spec.ProjectileStraightVX)
	} else {
		phase := enemy.MotionTick
		if phase >= len(spec.ProjectileMotionX) {
			phase = len(spec.ProjectileMotionX) - 1
		}
		enemy.X += float64(spec.ProjectileMotionX[phase])
		vertical := spec.ProjectileMotionY[phase]
		if enemy.bossMotion > 0 {
			vertical = -vertical
		}
		enemy.Y += float64(vertical)
		if enemy.MotionTick+1 < len(spec.ProjectileMotionX) {
			enemy.MotionTick++
		}
	}
	if enemy.X-enemy.W/2+GameOriginX <= 16 {
		enemy.Health = 0
		return
	}
	if deferredDeath {
		enemy.Health = 0
		w.bossImmediateDeath(enemy.Entity, 0)
	}
}

// updateBossMapHook translates 0x4B4E0 after the enemy update. The first mouth
// counter starts at zero and wraps as an unsigned byte before its first cycle.
func (w *World) updateBossMapHook() {
	spec := w.Stages[w.Stage-1].Boss
	if spec != nil && spec.Seven != nil {
		w.updateBossSeven(spec)
		return
	}
	if spec != nil && spec.Fifth != nil {
		w.updateBossFiveMap(spec)
		return
	}
	if spec != nil && spec.Eighth != nil {
		w.updateBossEight(spec)
		return
	}
	if spec != nil && spec.Fourth != nil {
		w.updateBossFourMap(spec)
		return
	}
	if spec != nil && spec.Kind == "stage6_crawler_queue" {
		w.updateBossSix(spec)
		return
	}
	if spec != nil && spec.Kind == "stage3_battleship" {
		w.spawnBattleshipMounted(spec)
		w.Boss.ArenaTicks--
		if w.Boss.ArenaTicks < 0 {
			w.battleshipCoreState().Defeated = true
			w.Boss.Defeated = true
			w.startBossVictory(spec)
		}
		w.updateBattleshipCoreMap(spec)
		w.updateBattleshipIris(spec)
		return
	}
	if spec != nil && spec.Kind == "stage2_gomander" {
		w.updateBossTwoMap(spec)
		return
	}
	if spec == nil || spec.Kind != "stage1_dobkeratops" || int(w.Scroll) < spec.HookTrigger {
		return
	}
	if w.Boss.EyeActive {
		w.Boss.EyeTicks--
		if w.Boss.EyeTicks < 0 {
			w.Boss.EyeTicks = spec.EyePeriod - 1
			w.patchTiles(spec.EyeColumn, spec.EyeRow, spec.EyeFrames[w.Boss.EyeFrame])
			w.Boss.EyeFrame++
			if w.Boss.EyeFrame >= len(spec.EyeFrames) {
				w.Boss.EyeActive = false
				w.Boss.EyeFrame, w.Boss.EyeTicks = 0, 0
			}
		}
	}
	w.Boss.MouthTicks--
	if w.Boss.MouthTicks == 0 {
		w.Boss.MouthTicks = uint8(spec.MouthPeriod)
		w.patchTiles(spec.MouthColumn, spec.MouthRow, spec.MouthFrames[w.Boss.MouthFrame])
		w.Boss.MouthFrame = (w.Boss.MouthFrame + 1) % len(spec.MouthFrames)
	}
	if w.Boss.Escaping {
		if int(w.Scroll) >= spec.EscapeScroll {
			w.Boss.CameraPaused = true
			w.startBossVictory(spec)
		}
	} else if int(w.Scroll) >= spec.FreezeScroll {
		w.Boss.CameraPaused = true
		w.Boss.ArenaTicks--
		if w.Boss.ArenaTicks < 0 {
			w.Boss.Escaping = true
			w.Boss.CameraPaused = false
		}
	}
}

func (w *World) updateBossVictory(spec *BossSpec) {
	if w.Boss.Clearing {
		if !w.Boss.VictoryInitialized {
			// Source 0x7AA owns this initialization, independently of the
			// earlier F9 request raised by an encounter's actor or map hook.
			w.Boss.VictoryInitialized = true
			w.Boss.victoryCounter, w.Boss.victoryTimer = spec.VictoryCounter, 0
		}
		w.Boss.VictoryTicks++
		w.Boss.victoryTimer--
		if w.Boss.victoryTimer < 0 {
			w.Boss.victoryTimer = spec.VictoryReload
			w.Boss.victoryCounter--
			if w.sceneRules() != nil && w.Boss.victoryCounter <= 15 {
				for color := 1; color < len(w.Scene.Palette); color++ {
					for shift := 0; shift <= 8; shift += 4 {
						if w.Scene.Palette[color]&(15<<shift) != 0 {
							w.Scene.Palette[color] -= uint16(1 << shift)
						}
					}
				}
			}
			if w.Boss.victoryCounter == 0 {
				w.Boss.Exiting = true
				w.Boss.ReadyToAdvance = false
				w.Boss.MusicSection = 0
			}
		}
	}
}

func (w *World) startBossVictory(spec *BossSpec) {
	if w.Boss.Clearing || !w.Player.Alive {
		return
	}
	w.Boss.Clearing = true
	w.Boss.victoryStartTick = w.StageTick
}

// updateBossExit translates the default ship-exit route in core 0x2348. Its
// hardware target is (270,150), one pixel per PAL axis, independently of25Hz AI.
func (w *World) updateBossExit() {
	if w.Boss.Final != nil && w.Boss.Final.Phase > 0 {
		w.updateFinalPlayerExit()
		return
	}
	x, y := 157.0, 113.0
	// Core 0x23BC compares the coordinates before either axis moves. An
	// arriving move therefore sets the ready flag on the following PAL.
	if w.Player.X == x && w.Player.Y == y {
		w.Boss.ReadyToAdvance = true
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
	if motion := w.Player.OriginalMotion; motion != nil {
		// Vertical exit movement uses the normal PAL banking path; once Y
		// is reached, 0x2234 recenters the tilt independently of X movement.
		if w.Player.VY < 0 {
			motion.Tilt = max(0, motion.Tilt-1)
		} else if w.Player.VY > 0 {
			motion.Tilt = min(16, motion.Tilt+1)
		} else if motion.Tilt < 8 {
			motion.Tilt++
		} else if motion.Tilt > 8 {
			motion.Tilt--
		}
	}
}
