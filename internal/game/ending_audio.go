package game

import (
	"github.com/olivierh59500/r-type/internal/sim"
	"github.com/olivierh59500/r-type/internal/sound"
)

// playback is implemented by both the device player and the deterministic
// export clock. This interface itself never opens a host audio device.
type playback interface {
	Play()
	Pause()
	SetVolume(float64)
}

// syncEndingMusic follows the consumed VBlank music request, independently of
// the main-loop fade phase. The old score has stopped at brain defeat; the
// ending score starts only when the native VBlank state selects its driver.
func syncEndingMusic(final *sim.OriginalFinalBossState, stagePlayers []playback, music *sound.StageMusic, output playback, started, focused, playing bool) (bool, int, error) {
	for _, player := range stagePlayers {
		player.Pause()
	}
	selected := final != nil && final.MusicStarted
	jingle := 0
	if selected {
		jingle = 4
	}
	if output == nil {
		return started, jingle, nil
	}
	if selected && !started {
		if err := music.StartFull(); err != nil {
			return started, jingle, err
		}
		started = true
	}
	if selected && focused && playing {
		output.Play()
	} else {
		output.Pause()
	}
	return started, jingle, nil
}
