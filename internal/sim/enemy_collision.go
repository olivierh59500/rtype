package sim

// PendingOriginalPass marks children inserted before their parent's original
// linked-list cursor. Their first animator/contact/blitter pass is next tick.
// Map-only hidden weak points and separate boss controllers own their own gates.
func (e Enemy) PendingOriginalPass() bool {
	return e.Descriptor != 0 && e.Age == 0 && e.bossRole == bossNone
}

// HitBounds resolves the original inclusive envelope for the currently drawn
// animation frame. Synthetic development entities retain their visual bounds.
func (e Enemy) HitBounds() Rect {
	frame := e.Frame()
	if frame >= 0 && frame < len(e.CollisionFrames) {
		box := e.CollisionFrames[frame]
		return Rect{X: e.X - e.W/2 + float64(box.X), Y: e.Y - e.H/2 + float64(box.Y), W: float64(box.Width), H: float64(box.Height)}
	}
	return e.Bounds()
}

func (e Enemy) overlapsHit(other Rect) bool {
	box := e.HitBounds()
	if len(e.CollisionFrames) > 0 {
		return box.X <= other.X+other.W && box.X+box.W >= other.X && box.Y <= other.Y+other.H && box.Y+box.H >= other.Y
	}
	return box.Overlaps(other)
}

func (w *World) damageEnemy(enemy *Enemy, damage int) {
	if damage <= 0 {
		return
	}
	if enemy.bossRole == bossCaveCreature {
		enemy.DamageTaken += damage
		enemy.ContactCooldown = 3
		enemy.maskFlashPending = true
		w.Events = append(w.Events, Event{Kind: EventSound, Value: 6, X: enemy.X, Y: enemy.Y})
		return
	}
	if enemy.Behavior == BehaviorShieldVehicle || (enemy.Behavior == BehaviorSpiralSentinel && enemy.SpiralSentinel != nil && enemy.bossSegment == enemy.SpiralSentinel.Head) {
		enemy.DamageTaken += damage
		return
	}
	if enemy.bossRole == bossThreeMounted && enemy.Invincible {
		enemy.DamageTaken += damage
		return
	}
	if enemy.bossRole == bossController && !w.Boss.Defeated {
		if enemy.ContactCooldown > 0 {
			return
		}
		enemy.DamageTaken += damage
		return
	}
	if enemy.bossRole == bossTwoPart {
		enemy.DamageTaken += damage
		return
	}
	if enemy.bossRole == bossEightCell {
		enemy.DamageTaken += damage
		return
	}
	if enemy.bossRole == bossFourWeak {
		enemy.DamageTaken += damage
		return
	}
	if enemy.Behavior == BehaviorRouteCrawler && enemy.Invincible {
		// Negative shield HP still consumes weapons and records their damage.
		// The next motion callback decides whether to accept or discard it.
		enemy.DamageTaken += damage
		return
	}
	if enemy.Reactor != nil && enemy.Invincible {
		// A dying reactor has source HP -1. It still consumes weapons and
		// records their damage, without restarting the positive-HP hurt gate.
		enemy.DamageTaken += damage
		return
	}
	if enemy.bossRole == bossThreeCore {
		enemy.DamageTaken += damage
		return
	}
	if enemy.Invincible {
		return
	}
	enemy.DamageTaken += damage
	enemy.Health -= damage
	if enemy.Health <= 0 {
		w.killEnemy(*enemy)
	} else if enemy.Descriptor != 0 {
		// Core 0x1674 reloads actor byte 0x28 to three for positive-HP
		// survivors. Negative-HP damage counters returned earlier at 0x166E.
		enemy.ContactCooldown = 3
		enemy.maskFlashPending = true
		w.Events = append(w.Events, Event{Kind: EventSound, Value: 6, X: enemy.X, Y: enemy.Y})
	}
}
