package sim

import (
	"strings"

	"github.com/olivierh59500/r-type/internal/source"
)

func newBattleshipActorReference(stages []StageSpec, family string) (*ActorReference, error) {
	w := NewWorld(1)
	if err := w.SetStages(stages); err != nil {
		return nil, err
	}
	if err := w.StartStage(3); err != nil {
		return nil, err
	}
	w.rngOverride = func(*World) uint64 { return 0 }
	if strings.HasSuffix(family, "-jitter") {
		calls := uint64(0)
		w.rngOverride = func(*World) uint64 {
			calls++
			return calls*37 + 11
		}
	}
	w.Scene.Phase = ScenePlaying
	w.Player.X, w.Player.Y, w.Player.Invulnerable = 182, 75, 1<<30
	w.Enemies, w.Effects, w.Bullets = nil, nil, nil
	if strings.HasPrefix(family, "battleship-schedule") {
		if family == "battleship-schedule-full-exit" {
			w.Scene.Palette = append([]uint16(nil), w.Stages[2].Palette...)
		}
		return &ActorReference{World: w, Family: family, Terrain: true}, nil
	}
	var selected []Wave
	for _, wave := range w.Stages[2].Waves {
		matches := wave.BattleshipTurret != nil && strings.HasPrefix(family, "battleship-respawning")
		if wave.Reactor != nil {
			matches = wave.Reactor.Kind == 1 && strings.HasPrefix(family, "battleship-reactor-a") || wave.Reactor.Kind == 2 && strings.HasPrefix(family, "battleship-reactor-b")
		}
		if matches {
			selected = append(selected, wave)
		}
	}
	w.Stages[2].Waves = nil
	w.Scroll = 80
	if family == "battleship-iris" || family == "battleship-mixed" {
		w.Scroll = 352
	}
	spec := w.Stages[2].Boss
	if strings.HasPrefix(family, "battleship-respawning") || strings.HasPrefix(family, "battleship-reactor-") {
		w.Stages[2].Waves = selected
		w.StageTick, w.EventTick = 10000, 10000
		for range selected {
			w.spawnWaves()
		}
		w.Stages[2].Waves = nil
		return &ActorReference{World: w, Family: family, Terrain: true}, nil
	}
	if family != "battleship-iris" {
		for _, record := range spec.Third.Mounted {
			w.StageTick = record.Trigger
			w.spawnBattleshipMounted(spec)
		}
	}
	if family == "battleship-iris" || family == "battleship-mixed" {
		w.StageTick = spec.Third.Iris.Trigger
		w.updateBattleshipIris(spec)
	}
	return &ActorReference{World: w, Family: family, Terrain: true}, nil
}

func (ref *ActorReference) stepBattleship() {
	if strings.HasPrefix(ref.Family, "battleship-schedule") {
		ref.stepBattleshipSchedule()
		return
	}
	w := ref.World
	w.Events = w.Events[:0]
	w.Tick, w.StageTick = uint64(ref.Frame*2), 1400+ref.Frame
	base := 80
	if ref.Family == "battleship-iris" || ref.Family == "battleship-mixed" {
		base = 352
	}
	x, y := float64(base+ref.Frame/10), float64(120+(ref.Frame/20)%4)
	w.CameraDeltaX, w.CameraDeltaY = x-w.Scroll, y-w.ScrollY
	w.Scroll, w.ScrollY = x, y
	w.captureOriginalTerrain()
	w.advanceEffectsForPass(true)
	if strings.HasPrefix(ref.Family, "battleship-reactor-") && ref.Frame == 200 {
		damage := 0
		switch ref.Family {
		case "battleship-reactor-a-escape":
			damage = 21
		case "battleship-reactor-a-death", "battleship-reactor-a-death-jitter":
			damage = 30
		case "battleship-reactor-b-death", "battleship-reactor-b-death-jitter":
			damage = 55
		}
		for index := range w.Enemies {
			if enemy := &w.Enemies[index]; enemy.Reactor != nil && damage != 0 {
				enemy.DamageTaken = damage
				enemy.Health = enemy.MaxHealth - damage
			}
		}
	}
	if (ref.Family == "battleship-mounted-hit" || ref.Family == "battleship-mixed") && ref.Frame == 40 {
		for index := range w.Enemies {
			if enemy := &w.Enemies[index]; enemy.bossRole == bossThreeMounted {
				enemy.DamageTaken = w.Stages[2].Boss.Third.Mounted[enemy.bossSegment].DamageLimit
				if !enemy.Invincible {
					enemy.Health = enemy.MaxHealth - enemy.DamageTaken
				}
			}
		}
	}
	if ref.Family == "battleship-mixed" {
		for _, hit := range [][2]int{{100, 80}, {250, 196}, {400, 220}, {450, 280}, {500, 340}, {650, 440}} {
			if ref.Frame == hit[0] {
				for index := range w.Enemies {
					if enemy := &w.Enemies[index]; enemy.bossRole == bossThreeIris {
						enemy.DamageTaken = hit[1]
						enemy.Health = enemy.MaxHealth - enemy.DamageTaken
					}
				}
			}
		}
	}
	if ref.Family == "battleship-mounted-hit" && ref.Frame >= 5 && ref.Frame <= 8 || ref.Family == "battleship-mixed" && ref.Frame >= 80 && ref.Frame <= 90 || ref.Family == "battleship-respawning-hit" && (ref.Frame >= 5 && ref.Frame <= 20 || ref.Frame >= 400 && ref.Frame <= 420) || strings.HasPrefix(ref.Family, "battleship-reactor-") && strings.HasSuffix(ref.Family, "-hit") && ref.Frame >= 200 && ref.Frame <= 250 {
		// The fixture writes the single source cannon slot, replacing any
		// previous injected envelope. This is not an additional fire command.
		for index := range w.Bullets {
			if b := &w.Bullets[index]; b.Original != nil && b.Original.Kind == OriginalCannon {
				b.Lifetime = 0
			}
		}
		w.spawnOriginalWeapon(Beam, OriginalWeaponMotion{Kind: OriginalCannon, X: 32, Y: 32, Width: 16, Height: 12, Collision: source.WeaponBounds{Width: 320, Height: 208}}, 5, true)
	}
	w.updateOriginalActorPass()
	w.updateOriginalHostileShots()
	if ref.Family == "battleship-iris" || ref.Family == "battleship-mixed" {
		w.updateBattleshipIris(w.Stages[2].Boss)
	}
	w.syncOriginalActors()
	w.compactEntities()
	ref.Frame++
}

func (ref *ActorReference) stepBattleshipSchedule() {
	w := ref.World
	w.Events = w.Events[:0]
	w.Tick, w.StageTick = uint64(ref.Frame*2), ref.Frame
	w.advanceEffectsForPass(true)
	if ref.Frame >= 43 {
		if w.Boss.Clearing {
			w.updateBossVictory(w.Stages[2].Boss)
		} else {
			w.EventTick++
			w.spawnWaves()
		}
	}
	w.advanceCamera()
	w.captureOriginalTerrain()
	if (ref.Family == "battleship-schedule-hit" || ref.Family == "battleship-schedule-exit" || ref.Family == "battleship-schedule-full-exit") && (ref.Frame == 2720 || ref.Frame == 2724) || ref.Family == "battleship-schedule-shots-hit" && ref.Frame >= 2820 && ref.Frame <= 2860 {
		for index := range w.Bullets {
			if b := &w.Bullets[index]; b.Original != nil && b.Original.Kind == OriginalCannon {
				b.Lifetime = 0
			}
		}
		height := 208
		if ref.Family == "battleship-schedule-shots-hit" {
			height = 26
		}
		w.spawnOriginalWeapon(Beam, OriginalWeaponMotion{Kind: OriginalCannon, X: 32, Y: 32, Width: 16, Height: 12, Collision: source.WeaponBounds{Width: 320, Height: height}}, 5, true)
	}
	w.updateOriginalActorPass()
	w.updateOriginalHostileShots()
	w.updateBossMapHook()
	w.syncOriginalActors()
	w.compactEntities()
	ref.Frame++
}
