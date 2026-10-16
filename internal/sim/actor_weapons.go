package sim

import "sort"

// collideOriginalActorWeapons retains 0x14D4's actor ownership and weapon-pool
// order. A collision death unwinds the rest of that actor's weapon checks, then
// its common firing hook still runs before the next actor is visited.
func (w *World) collideOriginalActorWeapons() {
	weapons := w.originalFriendlyWeaponOrder()
	for _, index := range w.orderedEnemyIndices() {
		w.collideOriginalActorWeapon(&w.Enemies[index], weapons)
	}
}

func (w *World) originalFriendlyWeaponOrder() []int {
	var weapons []int
	for index, b := range w.Bullets {
		if b.Friendly && b.Original != nil && b.Lifetime > 0 {
			weapons = append(weapons, index)
		}
	}
	sort.SliceStable(weapons, func(a, b int) bool {
		return originalCollisionOrder(w.Bullets[weapons[a]].Original) < originalCollisionOrder(w.Bullets[weapons[b]].Original)
	})
	return weapons
}

func (w *World) collideOriginalActorWeapon(enemy *Enemy, weapons []int) {
	rawX := int(enemy.X-enemy.W/2) + GameOriginX
	inOriginalRange := enemy.Descriptor == 0 || uint16(rawX) < 350
	if enemy.Health > 0 && !enemy.PendingOriginalPass() && !enemy.NoCollidable && !enemy.OriginalCollisionPaused && inOriginalRange {
		if enemy.Descriptor != 0 {
			w.collideActorContact(enemy, true)
		}
		if !enemy.NoFriendlyTarget && enemy.bossRole != bossProjectile {
			bitsDone := false
			for _, weapon := range weapons {
				if weapon >= len(w.Bullets) || w.Bullets[weapon].Original == nil {
					continue
				}
				if !bitsDone && originalCollisionOrder(w.Bullets[weapon].Original) >= 100 {
					w.collideOriginalBits(enemy)
					bitsDone = true
				}
				if enemy.Health <= 0 {
					break
				}
				bullet := &w.Bullets[weapon]
				if bullet.Lifetime <= 0 || !enemy.overlapsHit(bullet.CollisionBounds()) || alreadyHit(*bullet, enemy.ID) {
					continue
				}
				damage := bullet.Damage
				keep := false
				switch bullet.Original.Kind {
				case OriginalCannon:
					// 0x1580 writes the excess into the dying actor's scratch
					// word, leaving the cannon's own strength unchanged.
					if !enemy.Invincible && damage > enemy.Health {
						damage = enemy.Health
						keep = true
					}
				case OriginalWaveSmall, OriginalWaveLarge:
					// Red waves remain active through positive-HP targets
					// they can destroy, including exact-strength matches.
					keep = !enemy.Invincible && enemy.Health <= damage
				}
				if !keep {
					bullet.Lifetime = 0
				}
				w.damageEnemy(enemy, damage)
				bullet.HitIDs = append(bullet.HitIDs, enemy.ID)
			}
			if !bitsDone && enemy.Health > 0 {
				w.collideOriginalBits(enemy)
			}
		}
	}
	w.updateOriginalAutomaticFireActor(enemy)
}

// collideOriginalBits follows 0x15DE/0x1606 between missiles and colored shots.
// Its inclusive seven-pixel extent begins at the satellite's original probe,
// independently of the wider hardware sprite's presentation rectangle.
func (w *World) collideOriginalBits(enemy *Enemy) {
	for index := 0; index < w.Player.Bits && index < 2 && enemy.Health > 0; index++ {
		bit := w.bits[index]
		w.OriginalSolidHit(int(bit.X)+28, int(bit.Y)+28)
		if enemy.overlapsHit(Rect{X: bit.X - 8, Y: bit.Y - 8, W: 7, H: 7}) {
			w.damageEnemy(enemy, 1)
		}
	}
}

// originalCollisionOrder follows missiles (0x31BC), ricochet/ground slots
// (0x17E8), then descending shared slots (0x153C). Lower keys run first.
func originalCollisionOrder(m *OriginalWeaponMotion) int {
	switch m.Kind {
	case OriginalMissile:
		return m.Slot
	case OriginalRicochet, OriginalGroundWave:
		return 100 + m.Slot
	case OriginalCannon:
		return 200
	case OriginalWaveLarge:
		return 201
	case OriginalWaveSmall:
		return 230 - m.Group
	case OriginalSpread:
		return 300 - m.Slot
	}
	return 400
}
