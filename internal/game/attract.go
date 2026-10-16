package game

import (
	"crypto/sha256"
	"fmt"

	"github.com/olivierh59500/r-type/internal/attract"
	"github.com/olivierh59500/r-type/internal/sim"
	"github.com/olivierh59500/r-type/internal/source"
)

// SetAttractDelay selects focused, idle title ticks before the expert demo.
// Zero disables it for original-reference captures. A different source-data
// revision keeps the original menu rather than replaying incompatible inputs.
func (g *Game) SetAttractDelay(ticks int) {
	g.stopAttract()
	g.attract = nil
	metadata := attract.Metadata()
	if ticks <= 0 || metadata.ADFSHA256 != source.ADFSHA256 || metadata.AssetManifestSHA256 != fmt.Sprintf("%x", sha256.Sum256(g.bundle.Manifest)) {
		return
	}
	g.attract = attract.New(ticks)
}

// AttractDemo reports a demonstration, which can never own the human session.
func (g *Game) AttractDemo() bool { return g.attractSession.Active() }

// SetHostFocused complements mobile foreground state with actual window focus.
// Platform callbacks publish this change to the game's update thread.
func (g *Game) SetHostFocused(focused bool) {
	if g.hostFocused == focused {
		return
	}
	g.hostFocused = focused
	if !focused {
		g.Pause()
	}
}

func (g *Game) attractInput(in sim.Input, focused bool) (sim.Input, error) {
	if g.attract == nil || g.AutoPlay || g.inputReplay != nil {
		return in, nil
	}
	if !focused {
		g.stopAttract()
		g.attract.Reset()
		return in, nil
	}
	frame := g.attract.Advance(g.AtTitle(), g.input.humanActive)
	if frame.Started {
		world, err := g.attractSession.Begin(g.World)
		if err != nil {
			g.attract.Reset()
			return sim.Input{}, err
		}
		g.World = world
		g.resetAttractAudio()
		g.input.reset()
	}
	if frame.Interrupted {
		g.stopAttract()
		// Consume the triggering gesture. In particular Pause and Force must
		// not become a command in the new run while the finger remains down.
		return sim.Input{Start: true}, nil
	}
	if frame.Ended {
		g.stopAttract()
		return sim.Input{}, nil
	}
	if g.AttractDemo() {
		return frame.Input, nil
	}
	return in, nil
}

func (g *Game) stopAttract() {
	player := g.attractSession.Restore()
	if player == nil {
		return
	}
	g.World = player
	if g.attract != nil {
		g.attract.Reset()
	}
	g.resetAttractAudio()
	g.input.reset()
	g.World.CancelInput()
}

// Audio readers retain song state independently of the World. Reset their
// host roles when swapping sessions so a human cannot inherit a boss theme,
// a charge effect or a stage-completion/game-over wait from the demo.
func (g *Game) resetAttractAudio() {
	for _, player := range g.stageAudio {
		player.Pause()
	}
	for _, music := range g.stageMusic {
		music.Stop()
	}
	if g.titleAudio != nil {
		g.titleAudio.Pause()
	}
	if g.effectsAudio != nil {
		g.effectsAudio.Pause()
		g.effects.Stop()
	}
	if g.endingAudio != nil {
		g.endingAudio.Pause()
		g.endingMusic.Stop()
	}
	if g.menuAudio != nil {
		g.menuAudio.Pause()
		g.menuMusic.Stop()
	}
	g.musicStage, g.musicJingle, g.audioCharge, g.audioStageTick = 0, 0, 0, 0
	g.bossMusic, g.bossFade, g.stageDeath, g.gameOverWait = false, false, false, false
	g.endingStarted, g.menuStarted, g.menuFading = false, false, false
}
