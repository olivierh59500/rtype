package sim

import "github.com/olivierh59500/r-type/internal/source"

type ScenePhase uint8

const (
	ScenePlaying ScenePhase = iota
	SceneFadeIn
	SceneDeathHold
	SceneDeathFade
	SceneBlackout
	SceneGameOverMusic
)

// OriginalSceneState keeps the original independent PAL wait and main-loop
// palette counters. Manual movement remains active during checkpoint fade-in.
type OriginalSceneState struct {
	Phase                          ScenePhase
	Palette                        []uint16
	Counter, Timer, Hold, BlackPAL int
	SavedScroll                    float64
	DeathRequested                 bool
	DeathInitialized               bool
	DeathAimHidden                 bool
	GameOverMusicRequestPAL        uint64
	GameOverMusicReady             bool
}

func (w *World) sceneRules() *source.SceneRules {
	tables := w.weaponTables()
	if tables == nil {
		return nil
	}
	return tables.Scenes
}

// PlayfieldPalette exposes the sixteen original colors. Index zero remains
// black; scene routines animate entries one through fifteen. HUD and player
// attached sprites use their independent original palettes.
func (w *World) PlayfieldPalette() []uint16 {
	if w.StageResult != nil {
		return w.StageResult.Palette
	}
	if w.State == Title && w.Menu != nil {
		return w.Menu.Palette
	}
	if len(w.Boss.FinalPalette) == 16 {
		return w.Boss.FinalPalette
	}
	if w.Scene.Phase == SceneFadeIn || w.Scene.Phase == SceneDeathFade || w.Scene.Phase == SceneBlackout {
		return w.Scene.Palette
	}
	if w.Boss.Clearing && w.sceneRules() != nil {
		return w.Scene.Palette
	}
	return w.bossHitPalette()
}

// ControlsEnabled reports whether main-loop weapons, events and collisions
// are enabled. The original PAL joystick still moves the ship during fade-in.
func (w *World) ControlsEnabled() bool {
	return w.sceneRules() == nil || w.Scene.Phase == ScenePlaying && w.Player.Alive && !w.Boss.VictoryInitialized && !w.finalControlsDisabled()
}

func (w *World) SpriteVisible() bool {
	visible := w.Player.Alive || w.sceneRules() != nil && !w.Scene.DeathInitialized
	if visible && w.sceneRules() != nil && (w.Boss.VictoryInitialized || w.finalControlsDisabled() || w.Scene.Phase == SceneFadeIn && !w.InitialPlayerIntroActive()) {
		// IRQ 0x25EA checks the disabled-control flag and bit one of the
		// PAL counter before substituting offscreen hardware X=95.
		counter := w.Tick
		if final := w.Boss.Final; final != nil && final.Phase >= 7 && final.PostPAL < 525 {
			// Core 0x3FC resets the counter after the final-fade header.
			// The following PALs count from zero during the postlude wait.
			counter = uint64(max(0, 525-final.PostPAL))
		}
		return counter&2 == 0
	}
	return visible
}

func (w *World) resetOriginalScene() {
	w.EventTick = 0
	w.Scene = OriginalSceneState{}
	rules := w.sceneRules()
	if rules == nil {
		return
	}
	palette := w.Stages[w.Stage-1].Palette
	if len(palette) != 16 {
		return
	}
	w.Scene = OriginalSceneState{Phase: SceneFadeIn, Counter: rules.FadeInCounter, Palette: make([]uint16, 16)}
	w.Player.Invulnerable = 0
}

func (w *World) chooseOriginalCheckpoint(scroll float64) {
	selected := 0
	for index, checkpoint := range w.Stages[w.Stage-1].Checkpoints {
		if checkpoint.Scroll <= scroll && checkpoint.Scroll >= w.Stages[w.Stage-1].Checkpoints[selected].Scroll {
			selected = index
		}
	}
	w.Checkpoint = selected
}

func (w *World) initializeOriginalDeath() {
	rules := w.sceneRules()
	scene := &w.Scene
	if w.Boss.Final != nil && w.Boss.Final.Phase > 0 {
		// Death fade and the ending map hook both mutate the source's
		// single FE palette. Keep that same shared state in native Go.
		palette := w.PlayfieldPalette()
		if len(palette) != 16 {
			palette = w.Stages[w.Stage-1].Palette
		}
		w.Boss.FinalPalette = append([]uint16(nil), palette...)
		scene.Palette = w.Boss.FinalPalette
	}
	scene.Phase, scene.Hold, scene.Counter = SceneDeathHold, rules.DeathHold, rules.DeathFadeCounter
	scene.SavedScroll = w.Scroll
	scene.DeathInitialized = true
	// Collision routines only set the request flag. Core 0x6DE removes the
	// active ship during initialization, preserving the signed spare counter.
	w.Player.Alive = false
	w.Player.DeathTicks = 75
	w.Lives--
	w.chooseOriginalCheckpoint(scene.SavedScroll)
	w.resetOriginalPowers()
	// Core 0x18FE clears the charge display, retaining its held/charge bytes
	// until checkpoint initialization. Core 0x56A resets upgrades, satellites
	// and missiles; existing spread/cannon/colored shots continue moving.
	w.WeaponImages = nil
	w.pendingForce = false
	anchor := Entity{X: w.Player.X - 14, Y: w.Player.Y - 7}
	if effect, ok := w.originalDeathEffect(anchor, 3); ok {
		w.Effects = append(w.Effects, effect)
	}
	w.Events = append(w.Events, Event{Kind: EventPlayerDestroyed, X: w.Player.X, Y: w.Player.Y})
}

// advanceOriginalScene runs before the actor pass. Its false return preserves
// the source's busy PAL blackout, during which no game-main-loop work occurs.
func (w *World) advanceOriginalScene(mainTick bool) bool {
	rules := w.sceneRules()
	if rules == nil {
		return true
	}
	scene := &w.Scene
	if scene.Phase == SceneGameOverMusic {
		// Core 0x762 blocks only the main program. Ordinary World.Step
		// still advances PAL, hidden Force and input/pause bookkeeping.
		scene.GameOverMusicReady = w.Tick > scene.GameOverMusicRequestPAL
		return false
	}
	if scene.DeathInitialized {
		// Death main 0x72E writes hardware X20 after anchoring its effect.
		// A following PAL IRQ updates EE to -75; by the next main pass at
		// least one IRQ has passed the source's alternating blink gate.
		scene.DeathAimHidden = true
	}
	if scene.Phase == SceneBlackout {
		scene.BlackPAL--
		if scene.BlackPAL <= 0 {
			w.resetCheckpoint()
		}
		return false
	}
	if !mainTick {
		return true
	}
	if scene.DeathRequested && !scene.DeathInitialized {
		w.initializeOriginalDeath()
		return true
	}
	switch scene.Phase {
	case SceneDeathHold:
		scene.Hold--
		if scene.Hold == 0 {
			if w.Lives <= 0 {
				scene.Phase = SceneGameOverMusic
				scene.GameOverMusicRequestPAL = w.Tick
				return false
			}
			scene.Phase = SceneDeathFade
		}
	case SceneFadeIn, SceneDeathFade:
		scene.Timer--
		if scene.Timer < 0 {
			scene.Timer = rules.FadeReload
			if scene.Phase == SceneFadeIn {
				palette := w.Stages[w.Stage-1].Palette
				for color := 1; color < 16; color++ {
					for shift := 0; shift <= 8; shift += 4 {
						mask := uint16(15 << shift)
						if scene.Palette[color]&mask < palette[color]&mask {
							scene.Palette[color] += uint16(1 << shift)
						}
					}
				}
				scene.Counter--
				if scene.Counter < 0 {
					scene.Phase = ScenePlaying
					if w.InitialPlayerIntroActive() {
						// First-spawn 0x6A0 clears the shared route/fade byte.
						scene.Timer = 0
					}
				}
			} else {
				for color := 1; color < 16; color++ {
					for shift := 0; shift <= 8; shift += 4 {
						if scene.Palette[color]&(15<<shift) != 0 {
							scene.Palette[color] -= uint16(1 << shift)
						}
					}
				}
				scene.Counter--
				if scene.Counter == 0 {
					scene.Phase = SceneBlackout
					scene.BlackPAL = rules.DeathBlackPAL
					return false
				}
			}
		}
	}
	return true
}

// FinishGameOverMusicWait resumes the blocked main branch after the native
// sequencer reports completion. It consumes no PAL and uses no fixed duration.
func (w *World) FinishGameOverMusicWait() bool {
	if w.State != Playing || w.Scene.Phase != SceneGameOverMusic {
		return false
	}
	w.beginScoreFlow()
	return true
}
