package sim

import "fmt"

// newCityPulseReference uses the actual ordinary-event constructor and full
// original city map. Only the stationary target and zero RNG are controlled.
func newCityPulseReference(stages []StageSpec, family string) (*ActorReference, error) {
	w := NewWorld(1)
	if err := w.SetStages(stages); err != nil {
		return nil, err
	}
	if err := w.StartStage(7); err != nil {
		return nil, err
	}
	w.Scene.Phase = ScenePlaying
	w.Player.X, w.Player.Y, w.Player.Invulnerable = 162, 55, 1<<30
	w.rngOverride = func(*World) uint64 { return 0 }
	w.Enemies, w.Effects, w.Bullets = nil, nil, nil
	for _, wave := range w.Stages[6].Waves {
		if wave.TilePulse == nil || (wave.TilePulse.EffectYOffset > 16) != (family == "city-pulse-offset") {
			continue
		}
		w.Stages[6].Waves = []Wave{wave}
		w.Scroll = float64(wave.Trigger)
		w.spawnCursor = 0
		w.spawnWaves()
		return &ActorReference{World: w, Family: family, Terrain: true}, nil
	}
	return nil, fmt.Errorf("original city pulse event %q is absent", family)
}

func (ref *ActorReference) stepCityPulse() {
	w := ref.World
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
