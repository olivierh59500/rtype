package sim

import (
	"fmt"
	"github.com/olivierh59500/r-type/internal/source"
)

func (w *World) updateBattleshipReactor(enemy *Enemy) bool {
	data := enemy.Reactor
	if data == nil {
		return false
	}
	if !enemy.bossInitialized {
		enemy.bossInitialized = true
		enemy.BehaviorTicks = data.WaitTicks
		enemy.bossDelay = data.ShotFirst
		enemy.bossMotion = data.DeathReload
		enemy.reactorPatchTick = data.PatchFirstWait
	}
	defer w.updateReactorPatches(enemy)
	if data.Kind == 2 {
		enemy.X -= w.Scroll - enemy.CameraX
		enemy.Y -= w.ScrollY - enemy.CameraY
		enemy.CameraX, enemy.CameraY = w.Scroll, w.ScrollY
	}
	if enemy.DamageTaken >= data.DamageLimit || enemy.BehaviorPhase == 2 {
		enemy.BehaviorPhase = 2
		enemy.Invincible = true
		enemy.bossMotion--
		if enemy.bossMotion < 0 {
			enemy.Health = 0
			w.addScore(data.Score)
			// The source flags its original node for the common death
			// animator after the final independently allocated explosion.
			w.bossImmediateDeath(enemy.Entity, 0)
			return true
		}
		// Original callbacks sample Y before X and use the respective
		// reactor widths (96 or 160) as the horizontal random range.
		y := enemy.Y - enemy.H/2 + float64(int(w.nextRandom()%30)-10)
		x := enemy.X - enemy.W/2 + float64(int(w.nextRandom()%uint64(int(enemy.W)))-15)
		if effect, ok := w.originalDeathEffect(Entity{X: x, Y: y}, 0); ok {
			effect.Priority = 10
			if data.Kind == 2 {
				effect.Priority = 30
			}
			w.Effects = append(w.Effects, effect)
		}
		return true
	}
	if data.Kind == 1 {
		// The source keeps decrementing this word during an undamaged
		// escape, but skips the subtraction once damage exceeds twenty.
		if enemy.DamageTaken <= data.EscapeDamage {
			enemy.BehaviorTicks--
		}
		if enemy.DamageTaken > data.EscapeDamage || enemy.BehaviorTicks < 0 {
			enemy.BehaviorPhase = 1
		}
		if enemy.BehaviorPhase == 1 {
			if enemy.MotionTick >= len(data.MotionX) {
				enemy.BehaviorPhase = 2
				return true
			}
			enemy.X += float64(data.MotionX[enemy.MotionTick])
			enemy.Y += float64(data.MotionY[enemy.MotionTick])
			enemy.MotionTick++
			if enemy.X-enemy.W/2+GameOriginX < 0 || enemy.Y-enemy.H/2+GameOriginY >= 208 {
				enemy.Health = 0
			}
			return true
		}
	}
	enemy.X -= w.Scroll - enemy.CameraX
	enemy.Y -= w.ScrollY - enemy.CameraY
	enemy.CameraX, enemy.CameraY = w.Scroll, w.ScrollY
	if data.Kind == 2 {
		enemy.bossDelay--
		if enemy.bossDelay == 0 {
			shot := enemy.Variant
			enemy.Variant ^= 1
			enemy.bossDelay = 1
			if enemy.Variant == 0 {
				enemy.bossDelay = data.ShotPeriod
			}
			w.spawnReactorProjectile(enemy, shot, 0)
		}
		enemy.BehaviorTicks--
		if enemy.BehaviorTicks < 0 {
			enemy.Y++
			if enemy.X-enemy.W/2+GameOriginX < 0 || enemy.Y-enemy.H/2+GameOriginY >= 207 {
				enemy.Health = 0
			}
		}
		return true
	}
	enemy.bossDelay--
	if enemy.bossDelay <= 0 {
		enemy.bossDelay = data.ShotPeriod
		for shot := range data.ShotOffsetsY {
			w.spawnReactorProjectile(enemy, shot, 16)
		}
	}
	return true
}

func (w *World) spawnReactorProjectile(enemy *Enemy, shot, xOffset int) {
	data := enemy.Reactor
	width, height := float64(data.ShotWidth), float64(data.ShotHeight)
	x, y := enemy.X-enemy.W/2+float64(xOffset), enemy.Y-enemy.H/2+float64(data.ShotOffsetsY[shot])
	projectile := Enemy{Entity: w.newEntity(x+width/2, y+height/2, float64(data.ShotVX[shot]), float64(data.ShotVY[shot]), width, height), Kind: EnemyFighter, Health: 1, MaxHealth: 1, Invincible: true, Behavior: BehaviorFixedProjectile, Descriptor: data.ShotDescriptor, Sprite: fmt.Sprintf("object-3-%05x", data.ShotDescriptor), Animation: data.ShotAnimation, AnimationTicks: data.ShotAnimationTicks}
	// Source 0x4AF14 gives fans priority twenty and 0x4B196 gives rays
	// priority ten. Original head-or-tail insertion against the complete
	// live list decides whether a newborn moves in its birth pass.
	projectile.Priority = 20
	if data.Kind == 2 {
		projectile.Priority = 10
	}
	projectile.NoFriendlyTarget = data.Kind == 2
	for _, box := range data.ShotCollision {
		projectile.CollisionFrames = append(projectile.CollisionFrames, CollisionFrame{X: box.X, Y: box.Y, Width: box.Width, Height: box.Height})
	}
	projectile.FixedMinXInclusive = data.Kind == 2
	projectile.FixedNoYCutoff = data.Kind == 2
	w.Enemies = append(w.Enemies, projectile)
}

func (w *World) updateReactorPatches(enemy *Enemy) {
	data := enemy.Reactor
	if data == nil || len(data.PatchPairs) == 0 {
		return
	}
	apply := func(patch *source.MapPatch) {
		if patch == nil {
			return
		}
		for index, word := range patch.Words {
			w.SetTileAt(patch.Column+index%patch.Width, patch.Row+index/patch.Width, word)
		}
	}
	if enemy.BehaviorPhase != 0 {
		for _, patch := range data.ClearPatches {
			apply(patch)
		}
		return
	}
	switch enemy.reactorPatchPhase {
	case 0:
		enemy.reactorPatchTick--
		if enemy.reactorPatchTick < 0 {
			enemy.reactorPatchPhase = 1
			enemy.reactorPatchPair = 0
		}
	case 1:
		pair := data.PatchPairs[enemy.reactorPatchPair]
		apply(pair.First)
		apply(pair.Second)
		enemy.reactorPatchPair++
		if enemy.reactorPatchPair >= len(data.PatchPairs) {
			enemy.reactorPatchPhase = 2
			enemy.reactorPatchTick = data.PatchCycleWait
			// The final pointer pair falls through directly into 0x4B022.
			w.updateReactorPatches(enemy)
		}
	case 2:
		enemy.reactorPatchTick--
		if enemy.reactorPatchTick < 0 {
			enemy.reactorPatchTick = data.PatchCycleWait
			enemy.reactorPatchPhase = 0
			apply(data.ClearPatches[len(data.ClearPatches)-1])
		} else {
			enemy.reactorBlink ^= 1
			apply(data.BlinkPatches[enemy.reactorBlink])
		}
	}
}

func (w *World) updateFixedProjectile(enemy *Enemy) bool {
	enemy.X += enemy.VX
	enemy.Y += enemy.VY
	x, y := enemy.X-enemy.W/2+GameOriginX, enemy.Y-enemy.H/2+GameOriginY
	if x < 16 || (enemy.FixedMinXInclusive && x == 16) || (!enemy.FixedNoYCutoff && (y < 16 || y >= 224)) {
		enemy.Health = 0
	}
	return true
}
