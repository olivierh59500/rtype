package sim

import "github.com/olivierh59500/r-type/internal/source"

func newArenaActorReference(stages []StageSpec, family string) (*ActorReference, error) {
	w := NewWorld(1)
	if err := w.SetStages(stages); err != nil {
		return nil, err
	}
	if err := w.StartStage(7); err != nil {
		return nil, err
	}
	w.Scene.Phase = ScenePlaying
	w.rngOverride = func(*World) uint64 { return 0 }
	w.Player.X, w.Player.Y, w.Player.Invulnerable = 162, 55, 1<<30
	w.Enemies, w.Effects, w.Bullets = nil, nil, nil
	w.Stages[6].Waves = nil
	spec := w.Stages[6].Boss
	w.bossSevenState(spec)
	if family == "boss-seven-falling" {
		for index, variant := range spec.Seven.Variants {
			enemy := w.newBossActor(variant.Descriptor, spec.SevenVariantArt[index], spec.Seven.Lanes[index].SpawnX, spec.Seven.SpawnYTop, bossSevenFalling)
			enemy.bossSegment = index
			enemy.AnimationStart, enemy.AnimationTicks, enemy.AnimationOverride = variant.InitialIndex, variant.AnimationTicks, true
			enemy.Invincible = false
			enemy.Score = variant.Score
			w.Enemies = append(w.Enemies, enemy)
		}
		w.Scroll = 0
	} else {
		w.Scroll = float64(spec.Seven.Trigger)
		w.updateBossSeven(spec)
		w.Scroll = float64(spec.Seven.FreezeScroll)
	}
	return &ActorReference{World: w, Family: family, Terrain: true}, nil
}

func (ref *ActorReference) stepArena() {
	w := ref.World
	w.Events = w.Events[:0]
	w.StageTick = ref.Frame
	w.Tick = uint64(ref.Frame * 2)
	w.captureOriginalTerrain()
	w.advanceEffectsForPass(true)
	if ref.Family == "boss-seven-hit" && (ref.Frame >= 50 && ref.Frame <= 65 || ref.Frame >= 300 && ref.Frame <= 325) {
		for index := range w.Bullets {
			if b := &w.Bullets[index]; b.Original != nil && b.Original.Kind == OriginalCannon {
				b.Lifetime = 0
			}
		}
		w.spawnOriginalWeapon(Beam, OriginalWeaponMotion{Kind: OriginalCannon, X: 32, Y: 32, Width: 16, Height: 12, Collision: source.WeaponBounds{Width: 320, Height: 208}}, 5, true)
	}
	if ref.Family == "boss-seven-death" && ref.Frame == 400 {
		for index := range w.Enemies {
			if e := &w.Enemies[index]; e.bossRole == bossSevenController {
				e.DamageTaken = 50
				e.Health = e.MaxHealth - 50
			}
		}
	}
	w.updateOriginalActorPass()
	w.updateOriginalHostileShots()
	if ref.Family != "boss-seven-falling" {
		w.updateBossSeven(w.Stages[6].Boss)
	}
	w.syncOriginalActors()
	w.compactEntities()
	ref.Frame++
}
