package sim

import "fmt"

// SetInitialLives configures playable ships, including the active ship.
func (w *World) SetInitialLives(lives int) error {
	if w.State == Playing || w.State == Paused {
		return fmt.Errorf("cannot change initial lives during a game")
	}
	if lives < 1 || lives > 6 {
		return fmt.Errorf("initial lives must be between one and six")
	}
	w.InitialLives = lives
	w.Lives, w.DisplayLives = lives, max(0, lives-1)
	return nil
}

// OriginalLifeCounter translates signed RAM 0x4226. Core 0x55A consumes a
// spare at spawn, while core 0x752 tests negativity before the next spawn.
// A dead ship has already left Lives but has not yet consumed that counter.
func (w *World) OriginalLifeCounter() int {
	if w.Player.Alive {
		return w.Lives - 2
	}
	return w.Lives - 1
}

// redrawLives follows 0xB96: a nonnegative counter draws counter+1 spare icons.
// Death alone does not redraw the strip; spawn and an awarded spare do.
func (w *World) redrawLives() {
	w.DisplayLives = max(0, w.OriginalLifeCounter()+1)
}

// SetExtraLifeThresholds installs the extracted BCD score table as native Go
// integers. It may be changed between runs, and is copied before storage.
func (w *World) SetExtraLifeThresholds(thresholds []int) error {
	if w.State == Playing || w.State == Paused {
		return fmt.Errorf("cannot replace extra-life thresholds during a game")
	}
	for index, threshold := range thresholds {
		if threshold <= 0 || (index > 0 && threshold <= thresholds[index-1]) {
			return fmt.Errorf("extra-life thresholds must be positive and strictly increasing")
		}
	}
	w.ExtraLifeThresholds = append([]int(nil), thresholds...)
	w.extraLifeCursor, w.lastLifeScore = 0, 0
	return nil
}

// updateExtraLives translates the score-strip update at core 0xAF4. Only one
// threshold can be consumed when the score changes; a score-stable frame does
// not revisit another threshold crossed by the same large score increase.
func (w *World) updateExtraLives() {
	if w.Score == w.lastLifeScore {
		return
	}
	w.lastLifeScore = w.Score
	if w.extraLifeCursor >= len(w.ExtraLifeThresholds) || w.Score < w.ExtraLifeThresholds[w.extraLifeCursor] {
		return
	}
	w.extraLifeCursor++
	if w.OriginalLifeCounter() < 4 {
		w.Lives++
		w.redrawLives()
		w.Events = append(w.Events, Event{Kind: EventExtraLife, Value: w.Lives})
	}
}
