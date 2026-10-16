package game

import (
	"github.com/olivierh59500/r-type/internal/sim"
	"github.com/olivierh59500/r-type/internal/sound"
)

// finishGameOverMusicWait applies the native driver's completion event to the
// blocked main context. The last completed PAL remains unchanged.
func finishGameOverMusicWait(world *sim.World, music *sound.StageMusic, started bool) bool {
	return started && music.JingleDone() && world.FinishGameOverMusicWait()
}
