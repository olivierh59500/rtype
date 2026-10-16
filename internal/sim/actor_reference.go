package sim

import (
	"fmt"

	"github.com/olivierh59500/r-type/internal/source"
)

// ActorReference is an explicit development fixture for the independent
// original actor-pass trace. It uses native rules with explicit terrain and
// controlled RNG (zero except in named jitter cases). Designated cannon/damage
// injections exercise specific transitions;
// this fixture is separate from the normal-input expert demonstration.
type ActorReference struct {
	World   *World
	Frame   int
	Family  string
	Terrain bool
}

// NewActorFamilyReference reconstructs a documented development scenario with
// extracted spawn records. No original executable code is used by the fixture.
func NewActorFamilyReference(stages []StageSpec, family string) (*ActorReference, error) {
	level, descriptor, members, playerX, playerY := 1, uint32(0x4b99a), 4, 32.0, 25.0
	switch family {
	case "formation":
	case "spiral":
		descriptor, members, playerX, playerY = 0x4bf04, 1, 182, 75
	case "hunter", "hunter-hit":
		descriptor, members, playerX, playerY = 0x4bd02, 1, 182, 75
	case "vehicle":
		level, descriptor, members, playerX, playerY = 7, 0x4bfda, 1, 182, 75
	case "boss-two", "boss-two-hit", "boss-two-death", "boss-two-hit-fine", "boss-two-long":
		level, descriptor, members, playerX, playerY = 2, 0x4b1cc, 0, 182, 75
	case "boss-two-controller":
		level, descriptor, members, playerX, playerY = 2, 0x4b074, 0, 182, 75
	case "boss-one-tail", "boss-one-tail-death":
		descriptor, members, playerX, playerY = 0x4c1ca, 0, 182, 75
	case "worm", "worm-head", "worm-body":
		level, descriptor, members, playerX, playerY = 5, 0x4b0d0, 1, 182, 75
	case "boss-four", "boss-four-hit", "boss-four-death":
		level, descriptor, members, playerX, playerY = 4, 0x4bace, 0, 182, 75
	case "boss-five", "boss-five-hit", "boss-five-arm-death", "boss-five-death", "boss-five-intro-death":
		level, descriptor, members, playerX, playerY = 5, 0x4b5f8, 0, 87, 81
	case "battleship-mounted", "battleship-mounted-hit", "battleship-iris", "battleship-mixed", "battleship-respawning", "battleship-respawning-hit":
		return newBattleshipActorReference(stages, family)
	case "battleship-reactor-a", "battleship-reactor-a-hit", "battleship-reactor-a-escape", "battleship-reactor-a-death", "battleship-reactor-a-death-jitter", "battleship-reactor-b", "battleship-reactor-b-hit", "battleship-reactor-b-death", "battleship-reactor-b-death-jitter":
		return newBattleshipActorReference(stages, family)
	case "battleship-schedule", "battleship-schedule-jitter", "battleship-schedule-hit", "battleship-schedule-exit", "battleship-schedule-full-exit", "battleship-schedule-shots-hit":
		return newBattleshipActorReference(stages, family)
	case "boss-seven-arena", "boss-seven-hit", "boss-seven-death", "boss-seven-falling":
		return newArenaActorReference(stages, family)
	case "city-pulse", "city-pulse-offset":
		return newCityPulseReference(stages, family)
	case "quarter-turner-up", "quarter-turner-right", "quarter-turner-down", "quarter-turner-boundary", "quarter-turner-moving", "quarter-turner-hidden-target":
		return newQuarterTurnerReference(stages, family)
	case "final-cells", "final-cells-hit", "final-spore", "final-spore-wave", "final-arena", "final-arena-hit":
		return newFinalActorReference(stages, family)
	case "crawler-queue", "crawler-queue-hit", "crawler-wave", "crawler-wave-hit":
		return newCrawlerActorReference(stages, family)
	default:
		return nil, fmt.Errorf("unknown native compound actor fixture %q", family)
	}
	w := NewWorld(1)
	if err := w.SetStages(stages); err != nil {
		return nil, err
	}
	if err := w.StartStage(level); err != nil {
		return nil, err
	}
	w.rngOverride = func(*World) uint64 { return 0 }
	w.Scene.Phase = ScenePlaying
	for index := range w.Tiles {
		w.Tiles[index] = 0
	}
	w.Player.X, w.Player.Y, w.Player.Invulnerable = playerX, playerY, 1<<30
	w.Enemies, w.Effects, w.Bullets = nil, nil, nil
	var waves []Wave
	group := uint32(0)
	for _, wave := range w.Stages[level-1].Waves {
		if wave.Descriptor == descriptor {
			if group == 0 {
				group = wave.OriginalAddress
			}
			if wave.OriginalAddress == group {
				waves = append(waves, wave)
			}
		}
	}
	if len(waves) != members {
		return nil, fmt.Errorf("original %s actor reference is unavailable", family)
	}
	w.Stages[level-1].Waves = waves
	if members == 0 {
		spec := w.Stages[level-1].Boss
		if family == "boss-two-controller" {
			w.Scroll = float64(spec.FreezeScroll)
			w.updateBossMapHook()
			w.Boss.OrganFrame = 3
		} else if spec.Fourth != nil {
			w.Scroll = float64(spec.FreezeScroll)
			w.spawnBossIfDue()
		} else if spec.Fifth != nil {
			w.Scroll = float64(spec.Fifth.Trigger)
			w.spawnBossIfDue()
		} else {
			w.Scroll = float64(spec.TailTrigger)
			if spec.Second != nil {
				w.Scroll = float64(spec.Second.Trigger)
			}
			w.spawnBossIfDue()
		}
	} else {
		w.Scroll = float64(waves[0].Trigger)
		w.spawnCursor = 0
		w.spawnWaves()
	}
	w.Scroll = 0
	if family == "boss-two-hit-fine" {
		w.Scroll = 10
	}
	return &ActorReference{World: w, Family: family}, nil
}

// Step preserves the original fixture's one main actor pass per sample, without
// changing native motion, collision, node insertion or presentation policies.
func (ref *ActorReference) Step() {
	if ref.Family == "quarter-turner-up" || ref.Family == "quarter-turner-right" || ref.Family == "quarter-turner-down" || ref.Family == "quarter-turner-boundary" || ref.Family == "quarter-turner-moving" || ref.Family == "quarter-turner-hidden-target" {
		ref.stepQuarterTurner()
		return
	}
	if ref.Family == "crawler-queue" || ref.Family == "crawler-queue-hit" || ref.Family == "crawler-wave" || ref.Family == "crawler-wave-hit" {
		ref.stepCrawler()
		return
	}
	if ref.Family == "final-cells" || ref.Family == "final-cells-hit" || ref.Family == "final-spore" || ref.Family == "final-spore-wave" || ref.Family == "final-arena" || ref.Family == "final-arena-hit" {
		ref.stepFinal()
		return
	}
	if ref.Family == "boss-seven-arena" || ref.Family == "boss-seven-hit" || ref.Family == "boss-seven-death" || ref.Family == "boss-seven-falling" {
		ref.stepArena()
		return
	}
	if ref.Family == "city-pulse" || ref.Family == "city-pulse-offset" {
		ref.stepCityPulse()
		return
	}
	if ref.Terrain {
		ref.stepBattleship()
		return
	}
	w := ref.World
	w.Events = w.Events[:0]
	w.StageTick = ref.Frame
	w.Tick = uint64(ref.Frame * 2)
	w.advanceEffectsForPass(true)
	if (ref.Family == "formation" || ref.Family == "hunter-hit") && ref.Frame == 50 || ref.Family == "boss-two-controller" && (ref.Frame == 1 || ref.Frame == 2) || ref.Family == "boss-four-hit" && ref.Frame >= 81 && ref.Frame <= 84 || ref.Family == "boss-five-hit" && ref.Frame >= 80 && ref.Frame <= 110 {
		for index := range w.Bullets {
			if b := &w.Bullets[index]; b.Original != nil && b.Original.Kind == OriginalCannon {
				b.Lifetime = 0
			}
		}
		w.spawnOriginalWeapon(Beam, OriginalWeaponMotion{Kind: OriginalCannon, X: 32, Y: 32, Width: 16, Height: 12, Collision: source.WeaponBounds{Width: 320, Height: 208}}, 5, true)
	}
	if ref.Family == "spiral" && ref.Frame == 400 {
		for index := range w.Enemies {
			if e := &w.Enemies[index]; e.Spiral != nil && e.bossSegment == e.Spiral.Data.Head {
				e.DamageTaken = 6
			}
		}
	}
	if ref.Family == "vehicle" && (ref.Frame == 350 || ref.Frame == 450) {
		for index := range w.Enemies {
			if e := &w.Enemies[index]; e.ShieldVehicle != nil {
				e.DamageTaken = 70
			}
		}
	}
	if (ref.Family == "boss-two-hit" || ref.Family == "boss-two-hit-fine") && ref.Frame == 80 {
		for index := range w.Enemies {
			if e := &w.Enemies[index]; e.bossRole == bossTwoPart && e.bossSegment == 2 {
				e.DamageTaken = 1
			}
		}
	}
	if ref.Family == "boss-two-death" && ref.Frame == 300 {
		w.Boss.Defeated = true
	}
	if ref.Family == "boss-one-tail-death" && ref.Frame == 40 {
		w.Boss.Defeated = true
	}
	if ref.Family == "worm-head" && ref.Frame == 40 || ref.Family == "worm-body" && ref.Frame == 80 {
		part := 0
		if ref.Family == "worm-body" {
			part = 3
		}
		for index := range w.Enemies {
			if e := &w.Enemies[index]; e.FireWorm != nil && e.FireWorm.Number == part {
				e.DamageTaken = 10
			}
		}
	}
	if ref.Family == "boss-four-hit" && ref.Frame == 80 {
		for index := range w.Enemies {
			if e := &w.Enemies[index]; e.bossRole == bossFourWeak {
				e.DamageTaken = 1
			}
		}
	}
	if ref.Family == "boss-four-death" && (ref.Frame == 100 || ref.Frame == 180 || ref.Frame == 260) {
		body := (ref.Frame - 100) / 80
		for index := range w.Enemies {
			if e := &w.Enemies[index]; e.bossRole == bossFourWeak && e.bossSegment == body {
				e.DamageTaken = 30
			}
		}
	}
	if ref.Family == "boss-five-arm-death" && ref.Frame == 120 {
		for index := range w.Enemies {
			if enemy := &w.Enemies[index]; enemy.bossRole == bossFivePart && enemy.bossSegment == 3 {
				enemy.DamageTaken = 20
				enemy.Health = enemy.MaxHealth - 20
			}
		}
	}
	if ref.Family == "boss-five-death" && ref.Frame == 160 || ref.Family == "boss-five-intro-death" && ref.Frame == 10 {
		for index := range w.Enemies {
			if enemy := &w.Enemies[index]; enemy.bossRole == bossFiveController {
				enemy.DamageTaken = 31
				enemy.Health = enemy.MaxHealth - 31
			}
		}
	}
	w.updateOriginalActorPass()
	w.updateOriginalHostileShots()
	w.compactEntities()
	ref.Frame++
}
