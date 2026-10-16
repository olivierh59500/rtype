package sim

import "math"

func (w *World) updateRecoveredBehavior(enemy *Enemy) bool {
	switch enemy.Behavior {
	case BehaviorInitialArrow:
		return w.updateInitialArrow(enemy)
	case BehaviorHuntingEmitter, BehaviorHunterShot:
		return w.updateHuntingEmitter(enemy)
	case BehaviorShieldVehicle:
		return w.updateShieldVehicle(enemy)
	case BehaviorSpiralSentinel:
		return w.updateSpiralSentinel(enemy)
	case BehaviorGroundWalker:
		return w.updateGroundWalker(enemy)
	case BehaviorWalkerHoming:
		return w.updateWalkerHoming(enemy)
	case BehaviorWalkerCannon:
		return w.updateWalkerCannon(enemy)
	case BehaviorMiniTail:
		return w.updateOriginalMiniTail(enemy)
	case BehaviorRespawningTurret:
		return w.updateRespawningTurret(enemy)
	case BehaviorBattleshipTurretShot:
		return w.updateBattleshipTurretShot(enemy)
	case BehaviorFireWorm:
		return w.updateOriginalFireWorm(enemy)
	case BehaviorGroundGunner:
		return w.updateGroundGunner(enemy)
	case BehaviorArcProjectile:
		return w.updateArcProjectile(enemy)
	case BehaviorBattleshipReactor:
		return w.updateBattleshipReactor(enemy)
	case BehaviorFixedProjectile:
		return w.updateFixedProjectile(enemy)
	case BehaviorPowerCarrier:
		return w.updatePowerCarrier(enemy)
	case BehaviorTerrainStamp:
		return w.updateTerrainStamp(enemy)
	case BehaviorGroundHopper:
		return w.updateGroundHopper(enemy)
	case BehaviorRouteCrawler:
		return w.updateRouteCrawler(enemy)
	case BehaviorStalker:
		return w.updateStalker(enemy)
	case BehaviorTilePulse:
		return w.updateTilePulse(enemy)
	case BehaviorQuarterTurner:
		return w.updateQuarterTurner(enemy)
	case BehaviorSpitter, BehaviorSpitterProjectile:
		return w.updateSpitter(enemy)
	case BehaviorWallCrawler:
		return w.updateWallCrawler(enemy)
	case BehaviorMountedTurret:
		if len(enemy.AimMap) < 20 || len(enemy.AimThresholds) < 5 {
			return false
		}
		enemy.X -= w.Scroll - enemy.CameraX
		enemy.Y -= w.ScrollY - enemy.CameraY
		enemy.CameraX, enemy.CameraY = w.Scroll, w.ScrollY
		// Source 0x4ABC6 retires a mounted wave turret before aiming,
		// firing or drawing when its raw origin leaves this rectangle.
		x, y := enemy.X-enemy.W/2+GameOriginX, enemy.Y-enemy.H/2+GameOriginY
		if x < 16 || x > 370 || y < 9 || y >= 224 {
			enemy.Health = 0
			return true
		}
		enemy.Variant = w.originalAimDirection(*enemy)
		if enemy.ShootCooldown > 0 {
			enemy.ShootCooldown--
		}
		if enemy.ShootCooldown == 0 {
			minimum, maximum := enemy.ShootMin, enemy.ShootMax
			if minimum < 1 {
				minimum = 31
			}
			if maximum < minimum {
				maximum = minimum
			}
			enemy.ShootCooldown = minimum + int(w.nextRandom()%uint64(maximum-minimum+1))
			direction := enemy.Variant
			x, y := enemy.X, enemy.Y
			offsetIndex := direction
			if len(enemy.AimAnimation) > direction {
				offsetIndex = enemy.AimAnimation[direction]
			}
			if len(enemy.AimOffsetsX) > offsetIndex && len(enemy.AimOffsetsY) > offsetIndex {
				x = enemy.X - enemy.W/2 + float64(enemy.AimOffsetsX[offsetIndex]) + 5
				y = enemy.Y - enemy.H/2 + float64(enemy.AimOffsetsY[offsetIndex]) + 4
			}
			if len(enemy.ShotVX) >= 16 && len(enemy.ShotVY) >= 16 {
				// Core 0x130A uses a quantized direction from the actual muzzle.
				muzzle := *enemy
				muzzle.X, muzzle.Y = x-5+enemy.W/2, y-4+enemy.H/2
				shotDirection := w.originalAimDirection(muzzle)
				w.addBullet(EnemyShot, false, x, y, float64(enemy.ShotVX[shotDirection]), float64(enemy.ShotVY[shotDirection]), 12, 12, 1, 250, false)
			} else {
				w.enemyAimedShot(*enemy, 3)
			}
		}
		return true
	case BehaviorApproachLunge:
		// Stage-two 0x4A578 approaches at one pixel per update, then launches
		// with velocity selected by its original orientation table.
		if enemy.BehaviorPhase == 0 {
			enemy.X--
			difference := enemy.X - enemy.W/2 - (w.Player.X - 14)
			near := difference <= 80
			if enemy.Variant == 1 || enemy.Variant == 2 {
				near = difference < -80
			}
			if near {
				enemy.BehaviorPhase = 1
			}
			return true
		}
		vx, vy := enemy.LungeVX, enemy.LungeVY
		originalY := enemy.Y - enemy.H/2 + GameOriginY
		if originalY > 160 || originalY <= 45 {
			// ASR rounds negative odd words toward minus infinity.
			vx, vy = math.Floor(vx/2), math.Floor(vy/2)
			if enemy.Variant == 2 {
				vx = 0
			}
		}
		enemy.X += vx
		enemy.Y += vy
		return true
	case BehaviorHoming:
		if len(enemy.MotionX) < 16 || len(enemy.MotionY) < 16 || len(enemy.AimMap) < 20 || len(enemy.AimThresholds) < 5 {
			return false
		}
		// Stage-five 0x4ABA0 steers once every five updates, for 100 turns.
		period := enemy.TurnTicks
		if period < 1 {
			period = 5
		}
		if enemy.BehaviorTicks%period == 0 && (enemy.HomingTurns == 0 || enemy.BehaviorPhase < enemy.HomingTurns) {
			target := w.originalAimDirection(*enemy)
			difference := target - enemy.Variant
			if (difference >= 0 && difference <= 8) || difference < -8 {
				enemy.Variant++
			} else {
				enemy.Variant--
			}
			enemy.Variant = (enemy.Variant + 16) % 16
			enemy.BehaviorPhase++
		}
		enemy.BehaviorTicks++
		enemy.X += float64(enemy.MotionX[enemy.Variant])
		enemy.Y += float64(enemy.MotionY[enemy.Variant])
		if enemy.HomingClampY {
			enemy.Y = clamp(enemy.Y, -GameOriginY+enemy.H/2, 223-GameOriginY+enemy.H/2)
		}
		return true
	}
	return false
}

// originalAimDirection translates the integer ratio classifier at core 0x145C.
// Its thresholds and direction map are extracted data, not executable code.
func (w *World) originalAimDirection(enemy Enemy) int {
	targetX, targetY := w.originalPlayerAim()
	dx := int(enemy.X-enemy.W/2) + GameOriginX - targetX
	dy := int(enemy.Y-enemy.H/2) + GameOriginY - targetY
	quadrant := 0
	if dy < 0 {
		dy = -dy
		quadrant += 5
	}
	if dx < 0 {
		dx = -dx
		quadrant += 10
	}
	// Equal horizontal coordinates branch directly to index zero in 0x145C.
	index := 0
	if dx != 0 {
		ratio := dy * 512 / dx
		index = 0
		for index < 5 && ratio <= enemy.AimThresholds[index] {
			index++
		}
		if index == 5 {
			index = 4
		}
	}
	return int(enemy.AimMap[quadrant+index])
}
