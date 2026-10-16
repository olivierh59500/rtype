package game

import (
	"fmt"
	"io"
	"io/fs"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/olivierh59500/r-type/internal/sound"
)

// NewRecordingFS uses the actual game renderer and sound sequencers with a
// deterministic PCM clock. Recording does not listen to hardware input or use
// a speaker, microphone, emulator or desktop screen capture.
func NewRecordingFS(files fs.FS) (*Game, error) { return newFS(files, true) }

// SetMuted changes only this game's output gain. Players keep reading their
// sequencers, preserving music-completion and gameplay transition timing.
func (g *Game) SetMuted(muted bool) {
	titleVolume, musicVolume := .7, .6
	if muted {
		titleVolume, musicVolume = 0, 0
	}
	for _, player := range []playback{g.titleAudio, g.effectsAudio} {
		if player != nil {
			player.SetVolume(titleVolume)
		}
	}
	for _, player := range g.stageAudio {
		if player != nil {
			player.SetVolume(musicVolume)
		}
	}
	for _, player := range []playback{g.menuAudio, g.endingAudio} {
		if player != nil {
			player.SetVolume(musicVolume)
		}
	}
}

func (g *Game) newAudioPlayer(stream io.Reader, volume float64) (playback, error) {
	if g.recordAudio != nil {
		player := g.recordAudio.NewPlayer(stream)
		player.SetVolume(volume)
		return player, nil
	}
	context := audio.CurrentContext()
	if context == nil {
		context = audio.NewContext(sound.SampleRate)
	}
	player, err := context.NewPlayer(stream)
	if err != nil {
		return nil, err
	}
	player.SetVolume(volume)
	return player, nil
}

// ReadRecordingAudio emits exactly one native PAL block after Game.Update.
func (g *Game) ReadRecordingAudio() ([]byte, error) {
	if g.recordAudio == nil {
		return nil, fmt.Errorf("game was not created for deterministic recording")
	}
	return g.recordAudio.ReadTick()
}

// DrawRecordingTouchPreview captures the shared Android layout without
// cancelling a replay or changing its world, input, clocks or sound state.
func (g *Game) DrawRecordingTouchPreview(screen *ebiten.Image) {
	previous := g.touch
	g.touch = true
	g.Draw(screen)
	g.touch = previous
}
