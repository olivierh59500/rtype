package pilot

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"

	"github.com/olivierh59500/r-type/internal/sim"
)

type Death struct {
	ColliderEvidence []string
	Tick, Stage      int
	Scroll, X, Y     float64
	Cause            string
	Input            Buttons
	NearbyActors     []string
}
type StageProgress struct {
	Stage, EnterTick, ExitTick, Deaths, Pickups, Score int
	MaxScroll                                          float64
	BossDamage                                         int
	BossDefeated                                       bool
	MaxActorLinks                                      int
}
type Report struct {
	InitialControl                      *StartupSnapshot
	Policy                              string
	Seed                                uint64
	Ticks                               int
	Outcome                             string
	Stage, Lives, Score                 int
	Stages                              []StageProgress
	Deaths                              []Death
	InputChanges, Shots, Beams, Pickups int
	FinalDigest                         string
	MaxActorLinks                       int
}
type StartupSnapshot struct {
	Tick                 int
	HardwareX, HardwareY int
	Scroll, ScrollY      float64
}

// Progress is observer telemetry only. It is not fed back into pilot choices.
type Progress struct {
	Ticks, Stage, Lives, Score, Deaths int
	Scroll                             float64
}

// Run drives the unmodified world through normal input levels. The report may
// inspect exact post-step diagnostics, but those values never reach the pilot.
func Run(w *sim.World, p *Pilot, metadata ReplayMetadata, maxTicks, stopAfterStage int, playback *Replay) (Report, *Replay) {
	if playback != nil {
		metadata = playback.Metadata
	}
	replay := &Replay{Version: ReplayVersion, Metadata: metadata}
	report := Report{Policy: metadata.Policy, Seed: metadata.Seed, Outcome: "time_limit", Stages: []StageProgress{{Stage: 1}}}
	var last Buttons
	var bossDamage bossDamageReport
	for tick := 0; tick < maxTicks; tick++ {
		input := sim.Input{}
		if playback != nil {
			input = playback.InputAt(tick)
		} else {
			input = p.Next(w, tick)
		}
		buttons := Encode(input)
		replay.Frames = append(replay.Frames, ReplayFrame{Tick: tick, Buttons: buttons})
		if buttons != last {
			report.InputChanges++
		}
		last = buttons
		beforeAlive, beforeStage := w.Player.Alive, w.Stage
		beforeIntro := w.InitialPlayerIntroActive()
		priorActors := append([]sim.Enemy(nil), w.Enemies...)
		priorBullets := append([]sim.Bullet(nil), w.Bullets...)
		for index := range priorBullets {
			if original := priorBullets[index].Hostile; original != nil {
				copyOf := *original
				priorBullets[index].Hostile = &copyOf
			}
		}
		w.Step(input)
		if beforeIntro && !w.InitialPlayerIntroActive() {
			report.InitialControl = &StartupSnapshot{Tick: tick, HardwareX: int(w.Player.X) + 113, HardwareY: int(w.Player.Y) + 37, Scroll: w.Scroll, ScrollY: w.ScrollY}
		}
		report.Ticks = tick + 1
		if p != nil && p.Progress != nil && (tick+1)%1000 == 0 {
			p.Progress(Progress{Ticks: tick + 1, Stage: w.Stage, Lives: w.Lives, Score: w.Score, Deaths: len(report.Deaths), Scroll: w.Scroll})
		}
		progress := &report.Stages[len(report.Stages)-1]
		if w.Stage == beforeStage {
			progress.MaxScroll = math.Max(progress.MaxScroll, w.Scroll)
			progress.BossDamage = max(progress.BossDamage, bossDamage.observe(w))
			progress.BossDefeated = progress.BossDefeated || w.Boss.Defeated
			progress.MaxActorLinks = max(progress.MaxActorLinks, w.PeakOriginalActorLinks)
			report.MaxActorLinks = max(report.MaxActorLinks, w.PeakOriginalActorLinks)
		}
		for _, event := range w.Events {
			switch event.Kind {
			case sim.EventShot:
				report.Shots++
			case sim.EventBeam:
				report.Beams++
			case sim.EventPowerUp:
				if event.Value >= 0 {
					report.Pickups++
					progress.Pickups++
				}
			}
		}
		if beforeAlive && !w.Player.Alive {
			death := Death{Tick: tick, Stage: beforeStage, Scroll: w.Scroll, X: w.Player.X, Y: w.Player.Y, Input: buttons, Cause: "actor_or_projectile"}
			if w.OriginalSolidAt(int(w.Player.X)+36, int(w.Player.Y)+31) {
				death.Cause = "terrain_probe"
			}
			for _, enemy := range priorActors {
				if enemy.Health > 0 && enemy.HitBounds().Overlaps(w.Player.HitBounds()) {
					death.ColliderEvidence = append(death.ColliderEvidence, fmt.Sprintf("%s hidden=%v collider=%+v", enemy.Sprite, enemy.Hidden, enemy.HitBounds()))
				}
				if enemy.Health > 0 && !enemy.Hidden && rectDistance(enemy.Bounds(), w.Player.Bounds()) < 30 {
					death.NearbyActors = append(death.NearbyActors, fmt.Sprintf("%s at %.0f,%.0f", enemy.Sprite, enemy.X, enemy.Y))
				}
			}
			// Exact previous-pass projectile evidence belongs only to this
			// diagnostic report. It is never included in pilot observations.
			playerProbe := w.Player.HitBounds()
			playerProbe.X -= 2
			playerProbe.W = 20
			for _, bullet := range priorBullets {
				if h := bullet.Hostile; h != nil && h.Phase == 0 && bullet.Lifetime > 0 {
					probe := sim.Rect{X: float64(h.X + 5 - sim.GameOriginX), Y: float64(h.Y + 4 - sim.GameOriginY), W: 6, H: 6}
					if probe.X <= playerProbe.X+playerProbe.W && probe.X+probe.W >= playerProbe.X && probe.Y <= playerProbe.Y+playerProbe.H && probe.Y+probe.H >= playerProbe.Y {
						death.ColliderEvidence = append(death.ColliderEvidence, fmt.Sprintf("prior hostile slot %d raw %d,%d velocity %d,%d probe=%+v", h.Slot, h.X, h.Y, h.VX, h.VY, probe))
					}
				}
			}
			report.Deaths = append(report.Deaths, death)
			progress.Deaths++
		}
		if w.Stage != beforeStage {
			progress.ExitTick = tick
			progress.Score = w.Score
			if beforeStage >= stopAfterStage && stopAfterStage > 0 {
				report.Outcome = "stage_completed"
				break
			}
			report.Stages = append(report.Stages, StageProgress{Stage: w.Stage, EnterTick: tick})
		}
		if outcome, terminal := assessmentTerminal(w); terminal {
			report.Outcome = outcome
			break
		}
		if playback != nil && tick+1 >= len(playback.Frames) {
			report.Outcome = "replay_finished"
			break
		}
	}
	report.Stage, report.Lives, report.Score = w.Stage, w.Lives, w.Score
	state := struct {
		Tick                                      uint64
		Stage, StageTick, EventTick, Lives, Score int
		Scroll, ScrollY                           float64
		Player                                    sim.Player
		EnemyCount, BulletCount                   int
	}{w.Tick, w.Stage, w.StageTick, w.EventTick, w.Lives, w.Score, w.Scroll, w.ScrollY, w.Player, len(w.Enemies), len(w.Bullets)}
	data, _ := json.Marshal(state)
	report.FinalDigest = fmt.Sprintf("%x", sha256.Sum256(data))
	return report, replay
}

// A gameplay assessment ends when the final ship enters the music wait. The
// renderer's audio bridge owns that wait's completion and score-screen flow.
// Brain defeat alone does not complete a run when the ship also dies.
func assessmentTerminal(w *sim.World) (string, bool) {
	if w.Scene.Phase == sim.SceneGameOverMusic && (w.State == sim.Playing || w.State == sim.Paused) {
		return "run_failed", true
	}
	if w.State != sim.ScoreEntry && w.State != sim.ScoreTable && w.State != sim.GameOver && w.State != sim.Complete {
		return "", false
	}
	if w.Stage == 8 && w.Boss.Final != nil && w.Boss.Final.Phase >= 7 {
		return "campaign_completed", true
	}
	return "run_failed", true
}
