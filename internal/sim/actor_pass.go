package sim

import "fmt"

// updateOriginalActorPass follows the live insertion order rather than a
// storage snapshot: motion, animator pose, contacts and common fire belong to
// one actor before its successor is read. Newly appended children can therefore
// run immediately while children prepended before the cursor wait until later.
func (w *World) updateOriginalActorPass() {
	w.syncOriginalActors()
	w.ActorImages = w.ActorImages[:0]
	w.ActorImagesValid = true
	for index := range w.Effects {
		w.Effects[index].actorAdvanced = false
	}
	combat := w.ControlsEnabled()
	w.actorPassRunning, w.actorCombatEnabled = true, combat
	defer func() { w.actorPassRunning = false }()
	weapons := w.originalFriendlyWeaponOrder()
	previous := uint64(0)
	for id, visits := w.actorList.Next(0), 0; id != 0 && visits < 10000; visits++ {
		effectsAtEntry := len(w.Effects)
		index := w.originalEnemyIndex(id)
		if index >= 0 {
			w.actorUpdateID = id
			if w.Enemies[index].bossRole != bossNone {
				w.updateBossActors()
			} else {
				enemy := w.Enemies[index]
				w.updateEnemy(&enemy)
				w.Enemies[index] = enemy
			}
			w.actorUpdateID = 0
			w.syncOriginalActors()
			// A motion routine can append and relocate the enemy storage.
			index = w.originalEnemyIndex(id)
			if index >= 0 {
				enemy := &w.Enemies[index]
				enemy.automaticFireEligible = enemy.Health > 0
				if enemy.Health > 0 && (!enemy.Hidden || enemy.bossRole == bossTwoPart || enemy.Behavior == BehaviorRespawningTurret || enemy.Behavior == BehaviorTilePulse) && !enemy.PendingOriginalPass() {
					w.recordOriginalActor(enemy)
				}
				if combat {
					w.advanceOriginalHurtGate(enemy)
					w.collideOriginalActorWeapon(enemy, weapons)
				} else {
					w.updateOriginalAutomaticFireActor(enemy)
				}
			}
		} else {
			for powerIndex := range w.PowerUps {
				power := &w.PowerUps[powerIndex]
				if power.ID != id {
					continue
				}
				w.updatePowerUp(power)
				if power.Kind != PowerNone {
					w.ActorImages = append(w.ActorImages, OriginalActorImage{Entity: power.Entity, Sprite: "object-0-05288", Frame: power.Frame()})
					if combat && power.overlapsPlayer(w.Player) {
						w.collect(power.Kind)
						power.Kind = PowerNone
					}
				}
			}
		}
		// Replacement effects keep their actor's node. Independently allocated
		// effects and drops enter the same list through their native birth IDs.
		for effectIndex := range w.Effects {
			effect := &w.Effects[effectIndex]
			owner := effect.ID
			if effect.ActorOwner != 0 {
				owner = effect.ActorOwner
			}
			if owner != id || effect.Sprite == "" {
				continue
			}
			// Contact deaths flag the current actor after its body was drawn.
			// Its replacement animator begins when this node is visited again.
			if effectIndex >= effectsAtEntry && effect.ActorOwner == id && effect.Delay > 0 {
				continue
			}
			if !effect.actorAdvanced {
				effect.actorAdvanced = true
				if !w.advanceEffect(effect) {
					effect.Lifetime = 0
					continue
				}
			}
			if effect.Delay == 0 && effect.Lifetime > 0 {
				// Core 0x111E retains flag 0x23 bit zero. The first accepted
				// explosion blit consumes it; an offscreen rejection keeps it.
				image := OriginalActorImage{Entity: effect.Entity, Sprite: effect.Sprite, Frame: effect.Frame(), MaskFlash: effect.MaskFlash}
				w.ActorImages = append(w.ActorImages, image)
				if image.BlitterVisible(int(w.Scroll)) {
					effect.MaskFlash = false
				}
			}
		}
		w.syncOriginalActors()
		present := false
		for _, node := range w.actorList.Nodes {
			if node.ID == id {
				present = true
				break
			}
		}
		if present {
			previous = id
			id = w.actorList.Next(id)
		} else {
			id = w.actorList.Next(previous)
		}
	}
	w.actorUpdateID = 0
	w.actorPassComplete = true
}

// advanceOriginalHurtGate belongs to 0x14D4 after motion and the current blit.
// A paused contact pass retains its weapons; it does not consume a shot before
// deciding whether the actor can receive damage.
func (w *World) advanceOriginalHurtGate(enemy *Enemy) {
	enemy.OriginalCollisionPaused = enemy.ContactCooldown > 0
	if !enemy.OriginalCollisionPaused || enemy.Health <= 0 {
		return
	}
	enemy.ContactCooldown--
	if enemy.ContactCooldown&1 != 0 {
		enemy.maskFlashPending = true
		w.Events = append(w.Events, Event{Kind: EventSound, Value: 6, X: enemy.X, Y: enemy.Y})
	}
}

func (w *World) originalEnemyIndex(id uint64) int {
	for index, enemy := range w.Enemies {
		if enemy.ID == id {
			return index
		}
	}
	return -1
}

func (w *World) recordOriginalActor(enemy *Enemy) {
	sprite := enemy.Sprite
	if sprite == "" {
		sprite = []string{"fighter", "drifter", "turret", "carrier", "boss"}[enemy.Kind]
	}
	pose := enemy.Entity
	if enemy.Behavior == BehaviorRespawningTurret && enemy.Hidden {
		// Keep the original rejected/offscreen blitter attempt separately
		// from the fixed anchor needed when the turret regenerates.
		pose.X = -1 - GameOriginX + pose.W/2
	}
	w.ActorImages = append(w.ActorImages, OriginalActorImage{Entity: pose, Sprite: sprite, Frame: enemy.Frame(), MaskFlash: enemy.MaskFlash})
}

// ActorImageName remains data-only: original source addresses never identify
// executable behavior in the renderer or the live Go update path.
func (image OriginalActorImage) ActorImageName() string {
	return fmt.Sprintf("%s-%d", image.Sprite, image.Frame)
}
