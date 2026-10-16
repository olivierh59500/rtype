package sim

// bossImmediateDeath replaces a live actor in the same original actor pass.
// The common animator draws its first frame and starts its sound immediately.
func (w *World) bossImmediateDeath(actor Entity, kind int) {
	if effect, ok := w.originalDeathEffect(actor, kind); ok {
		sound := effect.Sound
		effect.Delay, effect.Age, effect.Sound = 0, 1, 0
		effect.actorAdvanced = w.actorPassRunning
		effect.ActorOwner = actor.ID
		for _, enemy := range w.Enemies {
			if enemy.ID == actor.ID {
				effect.Priority = enemy.Priority
				effect.MaskFlash = enemy.maskFlashPending
				if enemy.bossRole != bossNone {
					effect.MaskFlash = effect.MaskFlash || enemy.MaskFlash
				}
				break
			}
		}
		w.Effects = append(w.Effects, effect)
		w.Events = append(w.Events, Event{Kind: EventSound, Value: sound, X: actor.X, Y: actor.Y})
		return
	}
	// Synthetic test worlds have no extracted common-effect descriptors.
	w.Effects = append(w.Effects, Effect{Entity: actor, Lifetime: 24})
}

func (w *World) bossDeferredDeath(anchor Entity, kind int) {
	if effect, ok := w.originalDeathEffect(anchor, kind); ok {
		w.Effects = append(w.Effects, effect)
	}
}

// bossHitPalette preserves the organ's four explicit copper color writes.
// The tail boss uses a blitter mask flash and never changes the playfield palette.
func (w *World) bossHitPalette() []uint16 {
	if !w.Boss.HitFlash || w.Boss.Defeated || w.Stage < 1 || w.Stage > len(w.Stages) {
		return nil
	}
	stage := w.Stages[w.Stage-1]
	if stage.Boss == nil || stage.Boss.Second == nil || len(stage.Palette) != 16 {
		return nil
	}
	d := stage.Boss.Second
	palette := append([]uint16(nil), stage.Palette...)
	for index, color := range d.FlashIndices {
		if color >= 0 && color < 16 && index < len(d.FlashValues) {
			palette[color] = d.FlashValues[index]
		}
	}
	return palette
}
