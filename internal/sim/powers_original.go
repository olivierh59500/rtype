package sim

// resetOriginalPowers follows the ownership and projectile stores of core
// 0x56A. Charge/held input, speed and existing non-missile shots survive it.
// Force animation is retained independently of visible Force ownership.
func (w *World) resetOriginalPowers() {
	w.resetOriginalForceHardware()
	w.Force = Force{}
	w.Player.Weapon = WeaponNormal
	w.Player.WeaponLevel, w.Player.Bits = 0, 0
	w.Player.Missiles = false
	w.originalMissileCooldown = 0
	w.bitInitialized = false
	retained := w.Bullets[:0]
	for _, bullet := range w.Bullets {
		if bullet.Original == nil || bullet.Original.Kind != OriginalMissile {
			retained = append(retained, bullet)
		}
	}
	w.Bullets = retained
}
