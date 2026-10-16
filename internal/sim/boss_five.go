package sim

// spawnBossFive translates the event at RAM 0x4AD00. Its eight arms carry
// separate damage counters and remain positioned relative to the moving core.
func (w *World) spawnBossFive(spec *BossSpec) {
	d := spec.Fifth
	if d == nil || w.Boss.Active || int(w.Scroll) < d.Trigger {
		return
	}
	w.Boss.Active = true
	w.Boss.Health = d.DamageLimit
	w.Boss.MusicSection = 2
	w.Events = append(w.Events, Event{Kind: EventBossStarted, Value: 2})
	controller := w.newBossActor(d.Descriptor, spec.FifthArt, d.X, d.Y, bossFiveController)
	controller.Invincible = false
	controller.Health, controller.MaxHealth = d.CoreHealth, d.CoreHealth
	controller.AnimationOverride, controller.AnimationFrame = true, 0
	controller.Priority = 10
	controller.bossMotion = d.IntroReload
	controller.BehaviorPhase = 0
	controller.AimThresholds = d.AimThresholds
	for _, v := range d.AimMap {
		controller.AimMap = append(controller.AimMap, int8(v))
	}
	w.Boss.ControllerID = controller.ID
	w.Enemies = append(w.Enemies, controller)
	for arm, offset := range d.Offsets {
		part := w.newBossActor(d.PartDescriptor, spec.FifthPartArt, d.X+offset[0], d.Y+offset[1], bossFivePart)
		part.Invincible = false
		part.Health, part.MaxHealth = d.PartHealth, d.PartHealth
		part.Priority = 30
		part.bossSegment = arm
		part.bossParent = controller.ID
		part.TargetX, part.TargetY = offset[0], offset[1]
		part.bossMotion = 1
		if arm&1 != 0 {
			part.bossMotion = -1
		}
		part.AimMap, part.AimThresholds = controller.AimMap, controller.AimThresholds
		w.Enemies = append(w.Enemies, part)
	}
}

func (w *World) updateBossFiveActors(spec *BossSpec) {
	d := spec.Fifth
	if d == nil || !w.Boss.Active {
		return
	}
	controller := w.bossParent(w.Boss.ControllerID)
	if controller != nil && controller.Health > 0 && w.selectedActor(controller) {
		controller.Age++
		controller.MaskFlash, controller.maskFlashPending = controller.maskFlashPending, false
		fatal := false
		if controller.DamageTaken >= d.DamageLimit && !w.Boss.Defeated {
			fatal = true
			w.Boss.Defeated = true
			w.Boss.Health = 0
			w.addScore(d.Score)
			w.startBossVictory(spec)
			for i := range w.Enemies {
				if w.Enemies[i].bossRole == bossFivePart {
					w.Enemies[i].bossBroken = true
				}
			}
		}
		// 0x4AD9C flags death but falls through to one final motion/attack
		// callback before the common animator replaces the core descriptor.
		w.updateBossFiveController(controller, spec)
		if fatal {
			controller.Health = 0
			w.bossImmediateDeath(controller.Entity, d.CoreDeathKind)
		} else {
			w.Boss.Health = d.DamageLimit - controller.DamageTaken
		}
	}
	for index := range w.Enemies {
		part := &w.Enemies[index]
		if part.bossRole != bossFivePart || part.Health <= 0 || !w.selectedActor(part) {
			continue
		}
		part.Age++
		part.MaskFlash, part.maskFlashPending = part.maskFlashPending, false
		if part.bossBroken || part.DamageTaken >= d.PartDamageLimit {
			part.Health = 0
			w.addScore(d.PartScore)
			w.bossImmediateDeath(part.Entity, 0)
			continue
		}
		if controller != nil {
			w.updateBossFivePart(part, *controller, spec)
		}
	}
}

func (w *World) updateBossFiveController(enemy *Enemy, spec *BossSpec) {
	d := spec.Fifth
	if enemy.BehaviorPhase == 0 {
		enemy.bossMotion--
		if enemy.bossMotion >= 0 {
			enemy.X += float64(d.IntroVX)
			return
		}
		enemy.BehaviorPhase = 1
	}
	// A boundary bounce branches back through the timer test in the original;
	// it may choose a fresh direction during that same update.
	for {
		enemy.bossMotion--
		if enemy.bossMotion < 0 {
			enemy.Variant = w.originalAimDirection(*enemy)
			enemy.VX = float64(d.VelocityX[enemy.Variant])
			enemy.VY = float64(d.VelocityY[enemy.Variant])
			enemy.bossMotion = 20 + int(w.nextRandom()%100)
			continue
		}
		x, y := int(enemy.X-enemy.W/2+GameOriginX+enemy.VX), int(enemy.Y-enemy.H/2+GameOriginY+enemy.VY)
		if x >= d.MoveMinX && x <= d.MoveMaxX && y >= d.MoveMinY && y <= d.MoveMaxY {
			enemy.X += enemy.VX
			enemy.Y += enemy.VY
			break
		}
		enemy.bossMotion = int(uint16(enemy.bossMotion) >> 1)
		enemy.Variant = d.BounceDirections[enemy.Variant]
		enemy.VX = float64(d.VelocityX[enemy.Variant])
		enemy.VY = float64(d.VelocityY[enemy.Variant])
	}
	enemy.bossDelay--
	if enemy.bossDelay >= 0 {
		return
	}
	enemy.bossDelay = 35
	x, y := int(enemy.X-enemy.W/2+GameOriginX)+24, int(enemy.Y-enemy.H/2+GameOriginY)+24
	quadrant := 0
	if x < int(w.Player.X-14+GameOriginX) {
		quadrant++
	}
	if y < int(w.Player.Y-7+GameOriginY) {
		quadrant += 2
	}
	for _, arm := range d.AttackOrder[quadrant] {
		for index := range w.Enemies {
			part := &w.Enemies[index]
			if part.bossRole != bossFivePart || part.bossSegment != arm || part.Health <= 0 || part.Variant != 0 {
				continue
			}
			part.Variant = w.originalAimDirection(*part) + 1
			part.BehaviorPhase = 0
			part.BehaviorTicks = 1
			return
		}
	}
}

func (w *World) updateBossFivePart(part *Enemy, controller Enemy, spec *BossSpec) {
	d := spec.Fifth
	if part.Variant == 0 {
		x := int(controller.X-controller.W/2+GameOriginX) + part.TargetX + part.bossMotion
		y := int(controller.Y-controller.H/2+GameOriginY) + part.TargetY - part.bossMotion
		part.bossMotion = -part.bossMotion
		part.X = float64(x) - GameOriginX + part.W/2
		part.Y = float64(y) - GameOriginY + part.H/2
		return
	}
	direction := part.Variant - 1
	dx, dy := d.PartVelocityX[direction], d.PartVelocityY[direction]
	if part.BehaviorPhase == 1 {
		part.TargetX -= dx
		part.TargetY -= dy
		part.BehaviorTicks = (part.BehaviorTicks - 1) & 255
		if part.BehaviorTicks == 0 {
			part.Variant = 0
			part.TargetX, part.TargetY = d.Offsets[part.bossSegment][0], d.Offsets[part.bossSegment][1]
		}
		part.X = controller.X - controller.W/2 + float64(part.TargetX) + part.W/2
		part.Y = controller.Y - controller.H/2 + float64(part.TargetY) + part.H/2
		return
	}
	part.TargetX += dx
	part.TargetY += dy
	part.BehaviorTicks = (part.BehaviorTicks + 1) & 255
	x, y := int(controller.X-controller.W/2+GameOriginX)+part.TargetX, int(controller.Y-controller.H/2+GameOriginY)+part.TargetY
	if x < d.PartMinX || x > d.PartMaxX || y < d.PartMinY || y > d.PartMaxY {
		part.BehaviorPhase = 1
		w.updateBossFivePart(part, controller, spec)
		return
	}
	part.X = float64(x) - GameOriginX + part.W/2
	part.Y = float64(y) - GameOriginY + part.H/2
}

func (w *World) updateBossFiveMap(spec *BossSpec) {
	if int(w.Scroll) >= spec.Fifth.LoopEnd {
		w.Scroll = float64(spec.Fifth.LoopStart)
	}
}
