package sim

func (w *World) spawnBossFour(spec *BossSpec) {
	data := spec.Fourth
	if data == nil || w.Boss.Active || int(w.Scroll) < spec.FreezeScroll {
		return
	}
	w.Boss.Active, w.Boss.CameraPaused, w.Boss.BossIntro = true, true, true
	w.Boss.BodiesRemaining = len(data.Bodies)
	w.Boss.Health = len(data.Bodies) * data.WeakpointHealth
	var parents []uint64
	for body, record := range data.Bodies {
		enemy := w.newBossActor(record.Descriptor, spec.FourthArt[body], record.X, record.Y, bossFourBody)
		enemy.Priority = record.Priority
		enemy.bossSegment = body
		enemy.AnimationOverride = true
		enemy.AnimationFrame = 0
		parents = append(parents, enemy.ID)
		w.Enemies = append(w.Enemies, enemy)
		weak := w.newBossActor(data.WeakpointDescriptor, spec.FourthWeakArt, record.X, record.Y, bossFourWeak)
		weak.Priority = 10
		weak.Hidden = true
		weak.bossParent = enemy.ID
		weak.bossSegment = body
		weak.AnimationOverride = true
		weak.AnimationFrame = body
		weak.bossDelay = data.DeathTicks
		for _, box := range data.WeakpointCollision {
			weak.CollisionFrames = append(weak.CollisionFrames, CollisionFrame{X: box.X, Y: box.Y, Width: box.Width, Height: box.Height})
		}
		w.Enemies = append(w.Enemies, weak)
	}
	for turret, parent := range []int{0, 0, 1} {
		enemy := w.newBossActor(data.TurretDescriptor, spec.FourthTurretArt, 1, 1, bossFourTurret)
		enemy.Priority = 11
		enemy.bossParent = parents[parent]
		enemy.bossSegment = turret
		enemy.AnimationOverride = true
		enemy.AimMap, enemy.AimThresholds = spec.AimMap, spec.AimThresholds
		enemy.NoFriendlyTarget = turret > 0
		w.Enemies = append(w.Enemies, enemy)
	}
}

func (w *World) bossParent(id uint64) *Enemy {
	for index := range w.Enemies {
		if w.Enemies[index].ID == id {
			return &w.Enemies[index]
		}
	}
	return nil
}

func (w *World) updateBossFourActors(spec *BossSpec) {
	data := spec.Fourth
	if data == nil || !w.Boss.Active {
		return
	}
	// Original body priorities are 5,6,7, so body one updates before zero/two.
	var spawned []Enemy
	for _, body := range []int{1, 0, 2} {
		for index := range w.Enemies {
			enemy := &w.Enemies[index]
			if enemy.bossRole != bossFourBody || enemy.bossSegment != body || enemy.Health <= 0 || !w.selectedActor(enemy) {
				continue
			}
			enemy.Age++
			enemy.MaskFlash, enemy.maskFlashPending = enemy.maskFlashPending, false
			if !w.actorPassRunning && enemy.ContactCooldown > 0 {
				enemy.ContactCooldown--
			}
			if enemy.BehaviorPhase == 2 {
				enemy.Health = 0
				w.Boss.BodiesRemaining--
				w.bossImmediateDeath(enemy.Entity, 0)
				continue
			}
			record := data.Bodies[body]
			if body == 2 {
				if enemy.bossDelay > 0 {
					enemy.bossDelay--
				} else {
					x, y := int(enemy.X-enemy.W/2+GameOriginX), int(enemy.Y-enemy.H/2+GameOriginY)
					playerX, playerY := int(w.Player.X-14+GameOriginX), int(w.Player.Y-7+GameOriginY)
					difference := y + 34 - playerY
					if difference < 0 {
						difference = -difference
					}
					if x <= playerX && difference <= 28 {
						shot := w.newBossActor(data.ProjectileDescriptor, spec.FourthProjectileArt, x+60, y+20+int(w.nextRandom()%40), bossFourShot)
						shot.NoFriendlyTarget = true
						spawned = append(spawned, shot)
						enemy.bossDelay = 14
					}
				}
			}
			if w.Boss.BossIntro {
				enemy.X += float64(data.IntroVX)
				if enemy.X-enemy.W/2+GameOriginX >= float64(record.MotionX[0]+data.MotionOffsetX) {
					w.Boss.BossIntro = false
					enemy.X -= float64(data.IntroVX)
				}
			} else {
				phase := enemy.MotionTick
				enemy.X = float64(record.MotionX[phase]+data.MotionOffsetX) - GameOriginX + enemy.W/2
				enemy.Y = float64(record.MotionY[phase]) - GameOriginY + enemy.H/2
				enemy.MotionTick = (phase + 1) % len(record.MotionX)
			}
		}
	}
	for index := range w.Enemies {
		enemy := &w.Enemies[index]
		if enemy.Health <= 0 || !w.selectedActor(enemy) {
			continue
		}
		if enemy.bossRole == bossFourWeak {
			parent := w.bossParent(enemy.bossParent)
			if parent == nil && !enemy.bossBroken {
				enemy.Health = 0
				continue
			}
			enemy.Age++
			if parent != nil && !enemy.bossBroken {
				enemy.X = parent.X - parent.W/2 + enemy.W/2
				enemy.Y = parent.Y - parent.H/2 + enemy.H/2
			}
			if enemy.bossBroken {
				enemy.bossDelay--
				if enemy.bossDelay == 0 {
					enemy.Health = 0
					w.addScore(data.BodyScore)
					w.bossImmediateDeath(enemy.Entity, 0)
				}
				// The weak-point callback allocates one independent explosion
				// at priority twenty, including its final score-awarding visit.
				y := enemy.Y - enemy.H/2 + float64(w.nextRandom()%60)
				x := enemy.X - enemy.W/2 + float64(w.nextRandom()%80)
				if effect, ok := w.originalDeathEffect(Entity{X: x, Y: y}, 0); ok {
					effect.Priority = 20
					w.Effects = append(w.Effects, effect)
				}
			} else if parent.ContactCooldown == 0 && enemy.DamageTaken != enemy.BehaviorTicks {
				enemy.BehaviorTicks = enemy.DamageTaken
				if enemy.DamageTaken >= data.WeakpointHealth {
					enemy.bossBroken = true
					parent.BehaviorPhase = 2
					enemy.AnimationFrame = 3
				} else {
					parent.ContactCooldown = 3
					parent.maskFlashPending = true
				}
			}
		}
	}
	for index := range w.Enemies {
		enemy := &w.Enemies[index]
		if enemy.bossRole != bossFourTurret || enemy.Health <= 0 || !w.selectedActor(enemy) {
			continue
		}
		parent := w.bossParent(enemy.bossParent)
		if parent == nil || parent.BehaviorPhase == 2 {
			enemy.Health = 0
			w.bossImmediateDeath(enemy.Entity, 0)
			continue
		}
		turret := enemy.bossSegment
		enemy.X = parent.X - parent.W/2 + float64(data.TurretOffsets[turret*2]) + enemy.W/2
		enemy.Y = parent.Y - parent.H/2 + float64(data.TurretOffsets[turret*2+1]) + enemy.H/2
		direction := w.originalAimDirection(*enemy)
		enemy.AnimationFrame = data.TurretFrames[direction]
		if direction >= 8 {
			enemy.bossDelay--
		}
		if direction >= 8 && enemy.bossDelay < 0 {
			enemy.bossDelay = 35 + int(w.nextRandom()%70)
			// The art aims from the turret, but 0x130A independently aims
			// the projectile again from the selected muzzle's coordinates.
			muzzle := *enemy
			muzzle.X += float64(data.TurretShotX[direction-8])
			muzzle.Y += float64(data.TurretShotY[direction-8])
			shotDirection := w.originalAimDirection(muzzle)
			w.addBullet(EnemyShot, false, muzzle.X-muzzle.W/2+5, muzzle.Y-muzzle.H/2+4, float64(data.ShotVX[shotDirection]), float64(data.ShotVY[shotDirection]), 12, 12, 1, 250, false)
		}
	}
	for index := range w.Enemies {
		enemy := &w.Enemies[index]
		if enemy.bossRole == bossFourShot && enemy.Health > 0 && w.selectedActor(enemy) {
			enemy.Age++
			enemy.X += float64(data.ProjectileVX)
			if enemy.X-enemy.W/2+GameOriginX >= float64(data.ProjectileMaxX) {
				enemy.Health = 0
			}
		}
	}
	w.Enemies = append(w.Enemies, spawned...)
}

func (w *World) updateBossFourMap(spec *BossSpec) {
	if w.Boss.Active && w.Boss.BodiesRemaining == 0 {
		w.Boss.Defeated = true
		w.startBossVictory(spec)
	}
}
