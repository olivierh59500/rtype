package game

import "github.com/olivierh59500/r-type/internal/sound"

// syncGameOverMusic follows the consumed VBlank request during the blocked
// game-over main branch. Requesting the jingle alone does not initialize it.
// Playback and completion use the native PCM sequencer without a host clock.
func syncGameOverMusic(ready bool, stagePlayers []playback, music *sound.StageMusic, output, ending playback, jingle int, started, focused, playing bool) (bool, error) {
	for _, player := range stagePlayers {
		// Device and recording players use pointer identity. Leave the
		// selected stream running across consecutive active updates.
		if player != output {
			player.Pause()
		}
	}
	if ending != nil {
		ending.Pause()
	}
	if ready && !started {
		if err := music.PlayJingle(jingle); err != nil {
			if output != nil {
				output.Pause()
			}
			return false, err
		}
		started = true
	}
	if output != nil {
		if ready && started && focused && playing {
			output.Play()
		} else {
			output.Pause()
		}
	}
	return started, nil
}
