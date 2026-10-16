package sim

import (
	"fmt"
	"strings"

	"github.com/olivierh59500/r-type/internal/source"
)

type crawlerOracleFrame struct {
	Timer, Cursor int
	Finished      bool
	Actors        []crawlerOracleActor
}

type crawlerOracleActor struct {
	X, Y, HP, Damage, Accepted, Direction, RouteOffset, Cooldown int
}

// newCrawlerActorReference keeps the real native terrain and either the entire
// boss queue or the first authored ordinary crawler event. Only explicit
// target/cannon inputs belong to this controlled fixture.
func newCrawlerActorReference(stages []StageSpec, family string) (*ActorReference, error) {
	w := NewWorld(1)
	if err := w.SetStages(stages); err != nil {
		return nil, err
	}
	if err := w.StartStage(6); err != nil {
		return nil, err
	}
	w.Scene.Phase = ScenePlaying
	w.rngOverride = func(*World) uint64 { return 0 }
	w.Player.X, w.Player.Y, w.Player.Invulnerable = 162, 55, 1<<30
	w.Enemies, w.Effects, w.Bullets = nil, nil, nil
	if strings.HasPrefix(family, "crawler-queue") {
		w.Stages[5].Waves = nil
		w.Scroll = 2300
	} else {
		var selected *Wave
		for _, wave := range w.Stages[5].Waves {
			if wave.Descriptor == 0x4adbc {
				selected = &wave
				break
			}
		}
		if selected == nil {
			return nil, fmt.Errorf("original ordinary crawler event is absent")
		}
		w.Stages[5].Waves = []Wave{*selected}
		w.Scroll = float64(selected.Trigger)
		w.spawnWaves()
		for index := range w.Enemies {
			w.Enemies[index].CameraX = w.Scroll
		}
	}
	return &ActorReference{World: w, Family: family, Terrain: true}, nil
}

func (ref *ActorReference) stepCrawler() {
	w := ref.World
	w.Events = w.Events[:0]
	w.StageTick, w.Tick = ref.Frame, uint64(ref.Frame*2)
	queue := strings.HasPrefix(ref.Family, "crawler-queue")
	if queue {
		w.Scroll = float64(2300 + min(ref.Frame, 4))
	}
	// Alternating two legal target X coordinates exercises both shield sides.
	w.Player.X = float64(80 + 200*((ref.Frame/60)%2) - 18)
	w.captureOriginalTerrain()
	w.advanceEffectsForPass(true)
	if strings.HasSuffix(ref.Family, "-hit") && ref.Frame >= 30 && ref.Frame <= 900 && ref.Frame%7 == 0 {
		for index := range w.Bullets {
			if b := &w.Bullets[index]; b.Original != nil && b.Original.Kind == OriginalCannon {
				b.Lifetime = 0
			}
		}
		w.spawnOriginalWeapon(Beam, OriginalWeaponMotion{Kind: OriginalCannon, X: 32, Y: 32, Width: 16, Height: 12, Collision: source.WeaponBounds{Width: 320, Height: 208}}, 5, true)
	}
	w.updateOriginalActorPass()
	w.updateOriginalHostileShots()
	if queue {
		w.updateBossSix(w.Stages[5].Boss)
	}
	w.syncOriginalActors()
	w.compactEntities()
	ref.Frame++
}

// crawlerActorStates follows source list order and separates the pending
// collision word from damage already accepted by the motion callback.
func crawlerActorStates(w *World) *crawlerOracleFrame {
	s := &crawlerOracleFrame{Timer: w.Boss.QueueTimer, Cursor: w.Boss.QueueCursor, Finished: w.Boss.Clearing}
	for _, node := range w.actorList.Nodes {
		index := w.originalEnemyIndex(node.ID)
		if index < 0 {
			continue
		}
		e := &w.Enemies[index]
		if e.Descriptor != 0x4adbc || e.Health <= 0 {
			continue
		}
		// The source HP word is a fixed contact threshold. Native Health
		// tracks remaining health separately from that source word.
		hp := e.MaxHealth
		if e.Invincible {
			hp = -1
		}
		s.Actors = append(s.Actors, crawlerOracleActor{X: int(e.X-e.W/2) + GameOriginX, Y: int(e.Y-e.H/2) + GameOriginY, HP: hp, Damage: e.DamageTaken, Accepted: e.crawlerDamage, Direction: e.Variant, RouteOffset: e.MotionTick, Cooldown: e.ContactCooldown})
	}
	return s
}
