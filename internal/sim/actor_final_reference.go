package sim

import "github.com/olivierh59500/r-type/internal/source"

// newFinalActorReference mirrors controlled original actor-pass scenarios.
// The arena camera is held at 1615; these fixtures do not play the full ending.
func newFinalActorReference(stages []StageSpec, family string) (*ActorReference, error) {
	w := NewWorld(1)
	if err := w.SetStages(stages); err != nil {
		return nil, err
	}
	if err := w.StartStage(8); err != nil {
		return nil, err
	}
	w.Scene.Phase = ScenePlaying
	w.rngOverride = func(*World) uint64 { return 0 }
	w.Player.X, w.Player.Y, w.Player.Invulnerable = 162, 55, 1<<30
	w.Enemies, w.Effects, w.Bullets = nil, nil, nil
	w.Stages[7].Waves = nil
	spec := w.Stages[7].Boss
	d := spec.Eighth
	if family == "final-spore" {
		w.spawnFinalSpore(spec)
	} else if family == "final-spore-wave" {
		for _, wave := range stages[7].Waves {
			if wave.Descriptor == d.SporeDescriptor {
				w.Scroll = float64(wave.Trigger)
				w.spawnCursor = 0
				w.Stages[7].Waves = []Wave{wave}
				w.spawnWaves()
				w.Stages[7].Waves = nil
				break
			}
		}
	} else if family == "final-cells" || family == "final-cells-hit" {
		for direction, velocity := range d.CellVelocity {
			if direction == 4 || direction == 12 {
				continue
			}
			e := w.newBossActor(d.CellDescriptor, spec.EighthCellArt, 180, 112, bossEightCell)
			e.Invincible = true
			e.VX, e.VY = float64(velocity[0]), float64(velocity[1])
			e.Animation, e.AnimationTicks = d.CellAnimation[direction], d.CellAnimationTicks
			e.AnimationOverride = true
			w.Enemies = append(w.Enemies, e)
		}
	} else {
		w.Scroll = float64(d.BrainScroll)
		w.updateBossEight(spec)
		w.Scroll = float64(d.FreezeScroll)
	}
	return &ActorReference{World: w, Family: family, Terrain: true}, nil
}

func (ref *ActorReference) stepFinal() {
	w := ref.World
	w.Events = w.Events[:0]
	w.StageTick, w.Tick = ref.Frame, uint64(ref.Frame*2)
	w.captureOriginalTerrain()
	w.advanceEffectsForPass(true)
	// Named transitions seed counters or overwrite the one physical cannon
	// slot before the normal native actor pass, exactly as the source fixture.
	if ref.Family == "final-cells" && ref.Frame == 20 {
		for index := range w.Enemies {
			if e := &w.Enemies[index]; e.bossRole == bossEightCell {
				e.DamageTaken = 1
			}
		}
	}
	if ref.Family == "final-cells-hit" && ref.Frame >= 5 && ref.Frame <= 30 || ref.Family == "final-arena-hit" && ref.Frame >= 100 && ref.Frame <= 160 {
		for index := range w.Bullets {
			if b := &w.Bullets[index]; b.Original != nil && b.Original.Kind == OriginalCannon {
				b.Lifetime = 0
			}
		}
		w.spawnOriginalWeapon(Beam, OriginalWeaponMotion{Kind: OriginalCannon, X: 32, Y: 32, Width: 16, Height: 12, Collision: source.WeaponBounds{Width: 320, Height: 208}}, 5, true)
	}
	w.updateOriginalActorPass()
	w.updateOriginalHostileShots()
	if ref.Family == "final-arena" || ref.Family == "final-arena-hit" {
		w.updateBossEight(w.Stages[7].Boss)
	}
	w.syncOriginalActors()
	w.compactEntities()
	ref.Frame++
}
