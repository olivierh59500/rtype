package sim

func (w *World) updateRespawningTurret(enemy *Enemy) bool {
	d := enemy.BattleshipTurret
	if d == nil {
		return false
	}
	if !enemy.bossInitialized {
		for _, value := range d.AimMap {
			enemy.AimMap = append(enemy.AimMap, int8(value))
		}
		enemy.AimThresholds = d.AimThresholds
		enemy.bossInitialized = true
		enemy.TargetX = int(enemy.X - enemy.W/2 + GameOriginX)
		enemy.TargetY = int(enemy.Y - enemy.H/2 + GameOriginY)
		enemy.BehaviorTicks = d.InitialDelay
	}
	enemy.TargetX -= int(w.Scroll - enemy.CameraX)
	enemy.TargetY -= int(w.ScrollY - enemy.CameraY)
	enemy.CameraX, enemy.CameraY = w.Scroll, w.ScrollY
	enemy.X = float64(enemy.TargetX) - GameOriginX + enemy.W/2
	enemy.Y = float64(enemy.TargetY) - GameOriginY + enemy.H/2
	if enemy.BehaviorPhase == 1 {
		// The source main counter increments before actor callbacks; the
		// native StageTick records the preceding completed update here.
		if (w.StageTick+1)&1 == 0 {
			enemy.BehaviorTicks = (enemy.BehaviorTicks - 1) & 255
			if enemy.BehaviorTicks == 0 {
				enemy.BehaviorPhase = 2
				enemy.BehaviorTicks = d.RegenerateDelay
				enemy.Variant = 0
				enemy.Invincible = true
			}
		}
		enemy.Hidden, enemy.NoCollidable = true, true
		return true
	}
	if enemy.BehaviorPhase == 2 {
		enemy.BehaviorTicks = (enemy.BehaviorTicks - 1) & 255
		if enemy.BehaviorTicks == 0 {
			enemy.BehaviorPhase = 0
			enemy.BehaviorTicks = d.FirstAfterRegen
			enemy.Health = 4000
			enemy.Invincible = false
			enemy.DamageTaken = 0
		} else {
			enemy.Variant ^= 1
			if enemy.Variant == 0 {
				enemy.Hidden, enemy.NoCollidable = true, true
				return true
			}
		}
	} else {
		if enemy.TargetX < 16 || enemy.TargetX > 370 || enemy.TargetY < 16 || enemy.TargetY >= 220 {
			enemy.Health = 0
			return true
		}
		if enemy.DamageTaken&255 >= d.DamageLimit {
			enemy.BehaviorPhase = 1
			enemy.BehaviorTicks = d.HiddenDelay
			enemy.Hidden, enemy.NoCollidable = true, true
			// Source 0x4AC88 appends an independent priority-twenty death
			// node. Its animator and sound run later in this same live pass.
			if effect, ok := w.originalDeathEffect(enemy.Entity, 0); ok {
				effect.Priority = 20
				w.Effects = append(w.Effects, effect)
			}
			return true
		}
	}
	enemy.Hidden, enemy.NoCollidable = false, false
	direction := w.originalAimDirection(*enemy)
	enemy.AnimationOverride = true
	enemy.AnimationFrame = d.AimAnimation[direction]
	variant := d.AimAnimation[direction] & 7
	offset := d.AimOffsets[variant]
	enemy.X += float64(offset[0])
	enemy.Y += float64(offset[1])
	enemy.BehaviorTicks = (enemy.BehaviorTicks - 1) & 255
	if enemy.BehaviorTicks == 0 {
		x, y := int(enemy.X-enemy.W/2+GameOriginX), int(enemy.Y-enemy.H/2+GameOriginY)
		if x <= 368 && y >= 16 && y <= 223 {
			width, height := float64(d.ProjectileWidth), float64(d.ProjectileHeight)
			shot := Enemy{Entity: w.newEntity(0-GameOriginX+width/2, float64(y)-GameOriginY+height/2, 0, 0, width, height), Priority: 1, BattleshipTurret: d, Behavior: BehaviorBattleshipTurretShot, Kind: EnemyFighter, Health: 1, MaxHealth: 1, Invincible: true, Descriptor: d.ProjectileDescriptor, Sprite: d.ProjectileSprite, Variant: variant, TargetX: x, TargetY: y, Animation: d.ProjectileAnimations[variant], AnimationTicks: d.ProjectileFrameTicks}
			for _, box := range d.ProjectileCollisions {
				shot.CollisionFrames = append(shot.CollisionFrames, CollisionFrame{X: box.X, Y: box.Y, Width: box.Width, Height: box.Height})
			}
			// Original priority one inserts the child after its priority-zero
			// parent, so core 0x10E6 visits it during this same actor pass.
			w.Enemies = append(w.Enemies, shot)
		}
		enemy.BehaviorTicks = 150 + int(w.nextRandom()%100)
	}
	return true
}

func (w *World) updateBattleshipTurretShot(enemy *Enemy) bool {
	d := enemy.BattleshipTurret
	if d == nil {
		return false
	}
	direction := enemy.Variant
	enemy.AnimationOverride = true
	if enemy.MotionTick < 4 {
		// The descriptor begins with three shared launch images before
		// switching to the selected direction's two-image animation.
		enemy.AnimationFrame = enemy.MotionTick
		position := d.ProjectileMotion[direction][enemy.MotionTick]
		enemy.X = float64(enemy.TargetX+position[0]) - GameOriginX + enemy.W/2
		enemy.Y = float64(enemy.TargetY+position[1]) - GameOriginY + enemy.H/2
		enemy.MotionTick++
		if enemy.MotionTick < 4 {
			return true
		}
	}
	frames := d.ProjectileAnimations[direction]
	if len(frames) > 0 {
		enemy.AnimationFrame = frames[max(0, enemy.Age-4)%len(frames)]
	}
	enemy.X += float64(d.ProjectileVX[direction])
	enemy.Y += float64(d.ProjectileVY[direction])
	x, y := enemy.X-enemy.W/2+GameOriginX, enemy.Y-enemy.H/2+GameOriginY
	if x > 368 || y < 16 || y >= 224 {
		enemy.Health = 0
	}
	return true
}
