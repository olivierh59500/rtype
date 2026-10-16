package sim

import "github.com/olivierh59500/r-type/internal/source"

func (w *World) applyBattleshipPatch(patch *source.MapPatch) {
	if patch == nil {
		return
	}
	for i, word := range patch.Words {
		w.SetTileAt(patch.Column+i%patch.Width, patch.Row+i/patch.Width, word)
	}
}

func (w *World) spawnBattleshipMounted(spec *BossSpec) {
	d := spec.Third
	cursor := w.Boss.MountedCursor
	// RAM 0x4B472 increments before checking one record after the actor pass.
	if cursor >= len(d.Mounted) || w.StageTick+1 < d.Mounted[cursor].Trigger {
		return
	}
	record := d.Mounted[cursor]
	w.Boss.MountedCursor++
	enemy := w.newBossActor(d.MountedDescriptor, BossArt{Width: 0, Height: 1}, record.X, record.Y, bossThreeMounted)
	enemy.Hidden = true
	enemy.Invincible = false
	enemy.Health, enemy.MaxHealth = d.MountedHealth, d.MountedHealth
	enemy.bossSegment = cursor
	enemy.CameraX, enemy.CameraY = w.Scroll, w.ScrollY
	enemy.bossDelay = 0
	for _, box := range d.MountedCollision {
		enemy.CollisionFrames = append(enemy.CollisionFrames, CollisionFrame{X: box.X, Y: box.Y, Width: box.Width, Height: box.Height})
	}
	for _, value := range d.AimMap {
		enemy.AimMap = append(enemy.AimMap, int8(value))
	}
	enemy.AimThresholds = d.AimThresholds
	w.Enemies = append(w.Enemies, enemy)
	w.applyBattleshipPatch(record.InitialPatch)
}

func (w *World) updateBattleshipMounted(spec *BossSpec) {
	d := spec.Third
	for index := range w.Enemies {
		enemy := &w.Enemies[index]
		if enemy.bossRole != bossThreeMounted || enemy.Health <= 0 || !w.selectedActor(enemy) {
			continue
		}
		enemy.Age++
		enemy.OriginalCollisionPaused = enemy.ContactCooldown > 0
		if !w.actorPassRunning && enemy.ContactCooldown > 0 {
			enemy.ContactCooldown--
		}
		enemy.X -= w.Scroll - enemy.CameraX
		enemy.Y -= w.ScrollY - enemy.CameraY
		enemy.CameraX, enemy.CameraY = w.Scroll, w.ScrollY
		record := d.Mounted[enemy.bossSegment]
		enemy.bossDelay--
		if enemy.bossDelay < 0 {
			enemy.bossDelay = 30 + int(w.nextRandom()%50)
			offset := d.ShotOffsets[record.ShotVariant]
			muzzle := *enemy
			muzzle.X += float64(offset[0])
			muzzle.Y += float64(offset[1])
			direction := w.originalAimDirection(muzzle)
			w.addBullet(EnemyShot, false, muzzle.X-muzzle.W/2+5, muzzle.Y-muzzle.H/2+4, float64(d.ShotVX[direction]), float64(d.ShotVY[direction]), 12, 12, 1, 250, false)
		}
		if enemy.DamageTaken < record.DamageLimit {
			continue
		}
		if enemy.BehaviorTicks < record.DeathTicks {
			enemy.BehaviorTicks++
			enemy.Invincible = true
			x := enemy.X - enemy.W/2 + float64(w.nextRandom()%50)
			y := enemy.Y - enemy.H/2
			if effect, ok := w.originalDeathEffect(Entity{X: x, Y: y}, 0); ok {
				// The zero-priority allocation prepends before this actor's
				// live cursor, so its animator starts next main update.
				w.Effects = append(w.Effects, effect)
			}
		} else {
			enemy.Health = 0
			w.applyBattleshipPatch(record.DeathPatch)
			w.bossImmediateDeath(enemy.Entity, 0)
		}
	}
}
