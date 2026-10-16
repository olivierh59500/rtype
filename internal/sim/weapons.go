package sim

import "math"

func (w *World) addBullet(kind BulletKind, friendly bool, x, y, vx, vy, width, height float64, damage, lifetime int, piercing bool) {
	if kind == EnemyShot && !friendly && w.hostileShotData() != nil {
		// Recovered common-shot callers previously retained the collision
		// corner raw+(5,4); the original pool stores the raw muzzle instead.
		w.spawnOriginalHostileShot(int(x)+GameOriginX-5, int(y)+GameOriginY-4, int(vx), int(vy))
		return
	}
	w.Bullets = append(w.Bullets, Bullet{Entity: w.newEntity(x, y, vx, vy, width, height), Kind: kind, Friendly: friendly, Damage: damage, Lifetime: lifetime, Piercing: piercing})
}

func (w *World) fireNormal() {
	if w.weaponTables() != nil {
		w.fireOriginalNormal()
		return
	}
	p := w.Player
	w.addBullet(Shot, true, p.X+21, p.Y+1, 16, 0, 16, 4, 1, 90, false)
	w.Events = append(w.Events, Event{Kind: EventShot, X: p.X, Y: p.Y})
	if w.Force.Mode == ForceFront || w.Force.Mode == ForceBack {
		w.fireForceWeapon()
	}
	for _, bit := range w.BitEntities() {
		w.addBullet(Shot, true, bit.X+8, bit.Y, 6, 0, 8, 3, 1, 90, false)
	}
	w.fireMissiles()
}

func (w *World) fireBeam(charge int) {
	if w.weaponTables() != nil {
		w.fireOriginalBeam(charge)
		return
	}
	// Original strength records at core RAM 0x4CA2 use thresholds 6/12/18/24
	// and damage 5/7/9/11 at the original 25 Hz main-loop cadence.
	damage, width, height, offsetX, offsetY := 5, 16.0, 12.0, 21.0, 2.0
	if charge >= 12 {
		damage, width, height, offsetX, offsetY = 7, 32, 12, 19, 2
	}
	if charge >= 18 {
		damage, width, height, offsetX, offsetY = 9, 48, 14, 26, 1
	}
	if charge >= MaxCharge {
		damage, width, height, offsetX, offsetY = 11, 64, 16, 29, 2
	}
	// Core 0x1B02 advances the wave cannon fourteen pixels per update.
	w.addBullet(Beam, true, w.Player.X+offsetX, w.Player.Y+offsetY, 14, 0, width, height, damage, 100, true)
	w.Events = append(w.Events, Event{Kind: EventBeam, X: w.Player.X, Y: w.Player.Y, Value: charge})
	w.fireMissiles()
}

func (w *World) fireForceWeapon() {
	if w.weaponTables() != nil {
		w.fireOriginalForceWeapon()
		return
	}
	f, p := w.Force, w.Player
	direction := 1.0
	if f.Mode == ForceBack {
		direction = -1
	}
	x := f.X + direction*10
	if f.Level < 2 {
		w.addBullet(ForceShot, true, x, f.Y, direction*5, 0, 8, 4, 1, 90, false)
		return
	}
	// Weapon shapes and damage are approximations of the documented families;
	// exact Amiga shot tables and collision envelopes have not been extracted.
	switch p.Weapon {
	case WeaponRed:
		for _, vy := range []float64{-0.7, 0.7} {
			w.addBullet(RedLaser, true, x, f.Y, direction*5, vy, 12, 5, 2, 100, false)
		}
	case WeaponBlue:
		for _, vy := range []float64{-2, 0, 2} {
			w.addBullet(BlueLaser, true, x, f.Y, direction*4, vy, 10, 4, 1, 140, false)
		}
	case WeaponYellow:
		for _, vy := range []float64{-3, 3} {
			w.addBullet(YellowLaser, true, x, f.Y, direction, vy, 8, 8, 2, 120, false)
		}
	default:
		w.addBullet(ForceShot, true, x, f.Y, direction*5, 0, 8, 4, 1, 90, false)
	}
}

func (w *World) fireDetachedForce(input Input) {
	// The original Force burst is dispatched by the initial physical fire
	// edge in 1DE0, not by a repeating autofire cooldown while held.
	if w.weaponTables() != nil {
		return
	}
	if !input.Fire || w.forceCooldown > 0 || (w.Force.Mode != ForceFree && w.Force.Mode != ForceReturning) {
		return
	}
	f := w.Force
	for _, vy := range []float64{-1.25, 1.25} {
		w.addBullet(ForceShot, true, f.X+8, f.Y, 4, vy, 6, 4, 1, 100, false)
	}
	if f.Level >= 3 {
		for _, vy := range []float64{-1.25, 1.25} {
			w.addBullet(ForceShot, true, f.X-8, f.Y, -4, vy, 6, 4, 1, 100, false)
		}
	}
	w.forceCooldown = 12
}

func (w *World) fireMissiles() {
	if w.weaponTables() != nil {
		return
	}
	if !w.Player.Missiles || w.missileCooldown > 0 {
		return
	}
	for _, vy := range []float64{-0.8, 0.8} {
		w.addBullet(Missile, true, w.Player.X, w.Player.Y, 2, vy, 8, 4, 2, 200, false)
	}
	w.missileCooldown = 40
}

func (w *World) enemyAimedShot(enemy Enemy, speed float64) {
	dx, dy := w.Player.X-enemy.X, w.Player.Y-enemy.Y
	length := math.Sqrt(dx*dx + dy*dy)
	if length < 0.001 {
		dx, dy, length = -1, 0, 1
	}
	w.addBullet(EnemyShot, false, enemy.X-enemy.W/2, enemy.Y, dx/length*speed, dy/length*speed, 5, 5, 1, 250, false)
}

func (w *World) updateBullets() {
	for index := range w.Bullets {
		bullet := &w.Bullets[index]
		if bullet.Hostile != nil {
			continue
		}
		if bullet.Original != nil {
			if w.weaponTables() == nil {
				w.updateOriginalBullet(bullet)
			}
			continue
		}
		bullet.Age++
		if bullet.Kind == Missile {
			var target *Enemy
			nearest := 1e12
			for i := range w.Enemies {
				enemy := &w.Enemies[i]
				if enemy.Health <= 0 {
					continue
				}
				dx, dy := enemy.X-bullet.X, enemy.Y-bullet.Y
				distance := dx*dx + dy*dy
				if distance < nearest {
					nearest, target = distance, enemy
				}
			}
			if target != nil {
				dx, dy := target.X-bullet.X, target.Y-bullet.Y
				length := math.Sqrt(dx*dx + dy*dy)
				if length > 0.001 {
					bullet.VX += (dx/length*3 - bullet.VX) * 0.08
					bullet.VY += (dy/length*3 - bullet.VY) * 0.08
				}
			}
		}
		if bullet.Kind == RedLaser && bullet.Age%12 == 0 {
			bullet.VY = -bullet.VY
		}
		bullet.X += bullet.VX
		bullet.Y += bullet.VY
		if bullet.Kind == BlueLaser {
			if bullet.Y < bullet.H/2 {
				bullet.Y = bullet.H / 2
				bullet.VY = -bullet.VY
				bullet.Bounces++
			}
			if bullet.Y > PlayHeight-bullet.H/2 {
				bullet.Y = PlayHeight - bullet.H/2
				bullet.VY = -bullet.VY
				bullet.Bounces++
			}
		}
		if bullet.Kind == YellowLaser && bullet.VY != 0 {
			if bullet.Y < bullet.H/2 || bullet.Y > PlayHeight-bullet.H/2 {
				bullet.Y = clamp(bullet.Y, bullet.H/2, PlayHeight-bullet.H/2)
				bullet.VY = 0
				bullet.VX *= 5
			}
		}
	}
}

// BitEntities returns the protected satellite positions for rendering and
// collision. Their motion currently follows the ship directly; original lag
// behavior remains to be measured.
func (w *World) BitEntities() []Entity {
	if w.weaponTables() != nil {
		return append([]Entity(nil), w.bits[:min(2, w.Player.Bits)]...)
	}
	var bits []Entity
	if w.Player.Bits > 0 {
		bits = append(bits, Entity{X: w.Player.X - 2, Y: w.Player.Y - 20, W: 10, H: 10})
	}
	if w.Player.Bits > 1 {
		bits = append(bits, Entity{X: w.Player.X - 2, Y: w.Player.Y + 20, W: 10, H: 10})
	}
	return bits
}
