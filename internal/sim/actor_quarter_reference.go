package sim

import "fmt"

// newQuarterTurnerReference uses an authored event and the complete original
// stage-four map. The ship target and camera are stationary in these fixtures.
func newQuarterTurnerReference(stages []StageSpec, family string) (*ActorReference, error) {
	direction := 2
	if family == "quarter-turner-up" || family == "quarter-turner-boundary" {
		direction = 0
	} else if family == "quarter-turner-right" || family == "quarter-turner-hidden-target" {
		direction = 1
	}
	w := NewWorld(1)
	if err := w.SetStages(stages); err != nil {
		return nil, err
	}
	if err := w.StartStage(4); err != nil {
		return nil, err
	}
	w.Scene.Phase = ScenePlaying
	if family == "quarter-turner-hidden-target" {
		// Match the original EE word after the death fade hides the ship.
		w.Scene.DeathAimHidden = true
	}
	w.Player.X, w.Player.Y, w.Player.Invulnerable = 162, 55, 1<<30
	if family == "quarter-turner-boundary" {
		w.Player.Y = 155
	}
	w.rngOverride = func(*World) uint64 { return 0 }
	w.Enemies, w.Effects, w.Bullets = nil, nil, nil
	for _, wave := range w.Stages[3].Waves {
		if wave.QuarterTurner == nil || wave.Variant != direction {
			continue
		}
		w.Stages[3].Waves = []Wave{wave}
		w.Scroll = float64(wave.Trigger)
		w.spawnCursor = 0
		w.spawnWaves()
		if family == "quarter-turner-boundary" {
			// Match the explicitly controlled edge probe in the source fixture.
			enemy := &w.Enemies[0]
			enemy.X, enemy.Y = -GameOriginX+enemy.W/2, 100-GameOriginY+enemy.H/2
			enemy.BehaviorTicks = 1
		}
		return &ActorReference{World: w, Family: family, Terrain: true}, nil
	}
	return nil, fmt.Errorf("original quarter-turner event %q is absent", family)
}

func (ref *ActorReference) stepQuarterTurner() {
	w := ref.World
	if ref.Family == "quarter-turner-moving" {
		// Explicit triangular target paths exercise changing direction gates.
		xPhase, yPhase := ref.Frame%160, ref.Frame%120
		// The player words use different anchors from actor blitter coordinates.
		w.Player.X = float64(100+2*min(xPhase, 160-xPhase)) - 18
		w.Player.Y = float64(50+2*min(yPhase, 120-yPhase)) - 25
	}
	w.Events = w.Events[:0]
	w.StageTick, w.Tick = ref.Frame, uint64(ref.Frame*2)
	w.captureOriginalTerrain()
	w.advanceEffectsForPass(true)
	w.updateOriginalActorPass()
	w.updateOriginalHostileShots()
	w.syncOriginalActors()
	w.compactEntities()
	ref.Frame++
}
