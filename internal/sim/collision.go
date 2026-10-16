package sim

// HitBounds uses the original EEX+5 ship probe and the current pose's extent.
// Synthetic stage fixtures retain the smaller development hit box.
func (p Player) HitBounds() Rect {
	if p.OriginalMotion != nil {
		d, frame := p.OriginalMotion.Data, p.SpriteFrame()
		if d != nil && frame < len(d.CollisionY) && frame < len(d.CollisionHeight) {
			return Rect{p.X - 9, p.Y - 7 + float64(d.CollisionY[frame]), 16, float64(d.CollisionHeight[frame])}
		}
	}
	return Rect{p.X - 6, p.Y - 3, 12, 6}
}

func (w *World) collide() {
	terrain := w.TerrainInView()
	// Core 0x14D4 resolves Force and ship contact before friendly weapons.
	// Synthetic development scenes retain their established collision order.
	originalPass := w.weaponTables() != nil
	if originalPass && !w.actorPassComplete {
		w.collideOriginalActorWeapons()
	} else if !originalPass {
		w.collideActorContacts(true)
	}
	for index := range w.Bullets {
		bullet := &w.Bullets[index]
		if bullet.Lifetime <= 0 || bullet.Hostile != nil {
			continue
		}
		for _, wall := range terrain {
			if bullet.Original != nil {
				break
			}
			if !bullet.Bounds().Overlaps(wall) {
				continue
			}
			switch bullet.Kind {
			case BlueLaser:
				previous := bullet.Entity
				previous.X -= bullet.VX
				previous.Y -= bullet.VY
				if previous.Bounds().Y+previous.H <= wall.Y || previous.Bounds().Y >= wall.Y+wall.H {
					bullet.VY = -bullet.VY
				} else {
					bullet.VX = -bullet.VX
				}
				bullet.X, bullet.Y = previous.X, previous.Y
				bullet.Bounces++
				if bullet.Bounces > 8 {
					bullet.Lifetime = 0
				}
			case YellowLaser:
				if bullet.VY != 0 {
					if bullet.VY > 0 {
						bullet.Y = wall.Y - bullet.H/2 - 0.01
					} else {
						bullet.Y = wall.Y + wall.H + bullet.H/2 + 0.01
					}
					bullet.VY = 0
					bullet.VX *= 5
				} else {
					bullet.Lifetime = 0
				}
			default:
				bullet.Lifetime = 0
			}
			break
		}
		if bullet.Lifetime <= 0 {
			continue
		}
		if !bullet.Friendly {
			blocked := w.Force.Mode != ForceNone && bullet.Bounds().Overlaps(w.Force.Bounds())
			for _, bit := range w.BitEntities() {
				blocked = blocked || bullet.Bounds().Overlaps(bit.Bounds())
			}
			if blocked {
				bullet.Lifetime = 0
				continue
			}
			if w.Player.Invulnerable == 0 && bullet.Bounds().Overlaps(w.Player.HitBounds()) {
				bullet.Lifetime = 0
				w.killPlayer()
			}
			continue
		}
		if originalPass && bullet.Original != nil {
			continue
		}
		for enemyIndex := range w.Enemies {
			enemy := &w.Enemies[enemyIndex]
			if enemy.Health <= 0 || enemy.PendingOriginalPass() || enemy.NoCollidable || enemy.OriginalCollisionPaused || enemy.NoFriendlyTarget || enemy.bossRole == bossProjectile || !enemy.overlapsHit(bullet.CollisionBounds()) || alreadyHit(*bullet, enemy.ID) {
				continue
			}
			damage := bullet.Damage
			if bullet.Kind == Beam && bullet.PierceBudget > 0 {
				damage = bullet.PierceBudget
				if !enemy.Invincible && damage > enemy.Health {
					damage = enemy.Health
				}
				bullet.PierceBudget -= damage
				if bullet.PierceBudget == 0 {
					bullet.Lifetime = 0
				}
			}
			w.damageEnemy(enemy, damage)
			bullet.HitIDs = append(bullet.HitIDs, enemy.ID)
			if !bullet.Piercing || bullet.Lifetime <= 0 {
				bullet.Lifetime = 0
				break
			}
		}
	}
	w.collideActorContacts(false)
	if !originalPass && w.Player.Alive && w.Player.Invulnerable == 0 && w.ControlsEnabled() {
		for _, wall := range terrain {
			if w.Player.HitBounds().Overlaps(wall) {
				w.killPlayer()
				break
			}
		}
	}
	if w.Player.Alive && !w.actorPassComplete {
		for index := range w.PowerUps {
			power := &w.PowerUps[index]
			if power.Kind != PowerNone && !(power.Original != nil && power.Age == 0) && power.overlapsPlayer(w.Player) {
				w.collect(power.Kind)
				power.Kind = PowerNone
			}
		}
	}
	if !originalPass {
		w.updateOriginalAutomaticFire()
	}
	w.updateOriginalHostileShots()
	w.compactEntities()
}

func (w *World) collideOriginalPlayerTerrain() {
	if w.Player.Alive && w.Player.Invulnerable == 0 && w.ControlsEnabled() && w.weaponTables() != nil && w.weaponTables().Player != nil && w.originalPlayerTerrainHit() {
		w.killPlayer()
	}
}

func (w *World) collideActorContacts(original bool) {
	for index := range w.Enemies {
		enemy := &w.Enemies[index]
		if (enemy.Descriptor != 0) != original || enemy.Health <= 0 || enemy.PendingOriginalPass() || enemy.NoCollidable || enemy.OriginalCollisionPaused {
			continue
		}
		w.collideActorContact(enemy, original)
	}
}

func (w *World) collideActorContact(enemy *Enemy, original bool) {
	counter := enemy.bossRole == bossController || enemy.bossRole == bossTwoPart || enemy.bossRole == bossFourWeak || enemy.bossRole == bossEightCell || enemy.bossRole == bossThreeMounted && enemy.Invincible
	counter = counter || enemy.Behavior == BehaviorRouteCrawler || enemy.Behavior == BehaviorShieldVehicle || (enemy.Behavior == BehaviorSpiralSentinel && enemy.SpiralSentinel != nil && enemy.bossSegment == enemy.SpiralSentinel.Head)
	counter = counter || enemy.Reactor != nil && enemy.Invincible
	counter = counter || enemy.bossRole == bossThreeCore
	if (!enemy.Invincible || counter) && w.Force.Mode != ForceNone && enemy.ContactCooldown == 0 && enemy.overlapsHit(w.Force.Bounds()) {
		damage := 3
		if original || counter {
			damage = 1
		}
		w.damageEnemy(enemy, damage)
		if !original {
			enemy.ContactCooldown = 10
		}
		if counter {
			enemy.ContactCooldown = 0
		}
		if enemy.Health <= 0 {
			return
		}
	}
	if w.Player.Alive && w.Player.Invulnerable == 0 && enemy.overlapsHit(w.Player.HitBounds()) {
		w.killPlayer()
	}
}

func alreadyHit(bullet Bullet, id uint64) bool {
	for _, hit := range bullet.HitIDs {
		if hit == id {
			return true
		}
	}
	return false
}

func (w *World) killEnemy(enemy Enemy) {
	if enemy.Behavior == BehaviorHuntingEmitter && enemy.HuntingEmitter != nil {
		w.addScore(enemy.Score)
		w.specialDeath(enemy, enemy.HuntingEmitter.Death)
		return
	}
	if enemy.Behavior == BehaviorWalkerHoming || enemy.Behavior == BehaviorWalkerCannon {
		w.finishWalkerProjectile(enemy)
		return
	}
	if enemy.Behavior == BehaviorArcProjectile {
		w.finishArcProjectile(enemy)
		return
	}
	w.addScore(enemy.Score)
	deathKind := 0
	if enemy.bossRole == bossThreeCoreShot {
		deathKind = 1
	}
	if enemy.Behavior == BehaviorGroundWalker {
		deathKind = 2
	}
	effect, originalEffect := w.originalDeathEffect(enemy.Entity, deathKind)
	if enemy.Descriptor == 0 || !originalEffect {
		effect = Effect{Entity: w.newEntity(enemy.X, enemy.Y, 0, 0, enemy.W, enemy.H), Lifetime: 24}
		originalEffect = false
	}
	if originalEffect {
		effect.ActorOwner, effect.Priority = enemy.ID, enemy.Priority
		effect.MaskFlash = enemy.maskFlashPending
	}
	w.Effects = append(w.Effects, effect)
	if enemy.Behavior == BehaviorPowerCarrier {
		// Core 0x2D38 uses effect eight while releasing the original power item.
		w.Events = append(w.Events, Event{Kind: EventPowerUp, X: enemy.X, Y: enemy.Y, Value: -1})
	} else {
		value := int(enemy.Kind)
		if originalEffect {
			value = -1
		}
		w.Events = append(w.Events, Event{Kind: EventEnemyDestroyed, X: enemy.X, Y: enemy.Y, Value: value})
	}
	if enemy.Drop != PowerNone {
		velocity := -0.55
		if enemy.Behavior == BehaviorPowerCarrier {
			velocity = -1
		}
		power := PowerUp{Entity: w.newEntity(enemy.X, enemy.Y, velocity, 0, 14, 14), Kind: enemy.Drop}
		if enemy.Behavior == BehaviorPowerCarrier && w.weaponTables() != nil && w.weaponTables().Pickup != nil {
			data := w.weaponTables().Pickup
			power.X = enemy.X - enemy.W/2 + float64(data.Width)/2
			power.Y = enemy.Y - enemy.H/2 + float64(data.Height)/2
			power.W, power.H = float64(data.Width), float64(data.Height)
			power.Original = data
		}
		w.PowerUps = append(w.PowerUps, power)
	}
}

func (w *World) killPlayer() {
	if !w.Player.Alive || w.Boss.VictoryInitialized && !(w.actorPassRunning && w.actorCombatEnabled) {
		return
	}
	if w.sceneRules() != nil {
		// Core 0x13C2/0x179C/0x184A only store the pending request. Repeated
		// contacts cannot spend another ship or erase a charge before 0x6AE.
		w.Scene.DeathRequested = true
		return
	}
	w.Player.Alive = false
	w.Player.Charge = 0
	w.Player.DeathTicks = 75
	w.Lives--
	w.Effects = append(w.Effects, Effect{Entity: w.newEntity(w.Player.X, w.Player.Y, 0, 0, 32, 32), Lifetime: 36})
	w.Events = append(w.Events, Event{Kind: EventPlayerDestroyed, X: w.Player.X, Y: w.Player.Y})
}

func (w *World) collect(kind PowerUpKind) {
	p := &w.Player
	if w.weaponTables() != nil && w.weaponTables().Pickup != nil {
		// Core 0x1706 adds packed BCD $0400 for every collected item.
		w.addScore(400)
	}
	switch kind {
	case PowerRed, PowerBlue, PowerYellow:
		p.Weapon = Weapon(int(kind) - int(PowerRed) + int(WeaponRed))
		if p.WeaponLevel < 3 {
			p.WeaponLevel++
		}
		if w.Force.Mode == ForceNone {
			w.Force = Force{Entity: w.newEntity(p.X+23, p.Y, 0, 0, 16, 16), Mode: ForceFront}
		}
		w.Force.Level = p.WeaponLevel
	case PowerSpeed:
		if p.SpeedLevel < MaxSpeedLevel {
			p.SpeedLevel++
		}
	case PowerMissiles:
		p.Missiles = true
	case PowerBit:
		if p.Bits < 2 {
			p.Bits++
		}
	}
	w.Events = append(w.Events, Event{Kind: EventPowerUp, X: p.X, Y: p.Y, Value: int(kind)})
}

func (w *World) compactEntities() {
	enemies := w.Enemies[:0]
	for _, enemy := range w.Enemies {
		// Spiral members begin at raw X606; a hunter and its emitted shots can
		// also travel beyond the right viewport before their return. These
		// source routines own retirement and must survive generic view culling.
		if (enemy.Behavior == BehaviorRespawningTurret || enemy.Behavior == BehaviorSpiralSentinel || enemy.Behavior == BehaviorHuntingEmitter || enemy.Behavior == BehaviorHunterShot) && enemy.Health > 0 {
			enemies = append(enemies, enemy)
			continue
		}
		if enemy.bossRole != bossNone {
			if enemy.Health > 0 {
				enemies = append(enemies, enemy)
			}
			continue
		}
		if enemy.Health > 0 && enemy.X+enemy.W/2 >= -32 && enemy.X-enemy.W/2 <= Width+160 && enemy.Y+enemy.H/2 >= -64 && enemy.Y-enemy.H/2 <= PlayHeight+64 {
			enemies = append(enemies, enemy)
		}
	}
	w.Enemies = enemies
	bullets := w.Bullets[:0]
	for _, bullet := range w.Bullets {
		if bullet.Hostile != nil {
			if bullet.Lifetime > 0 {
				bullets = append(bullets, bullet)
			}
			continue
		}
		if bullet.Lifetime > 0 && bullet.Age < bullet.Lifetime && bullet.X > -48 && bullet.X < Width+48 && bullet.Y > -16 && bullet.Y < PlayHeight+16 {
			bullets = append(bullets, bullet)
		}
	}
	w.Bullets = bullets
	powers := w.PowerUps[:0]
	for _, power := range w.PowerUps {
		if power.Kind != PowerNone && power.X > -16 && power.Age < 1500 {
			powers = append(powers, power)
		}
	}
	w.PowerUps = powers
}
