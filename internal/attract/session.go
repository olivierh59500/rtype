package attract

import (
	"fmt"

	"github.com/olivierh59500/r-type/internal/pilot"
	"github.com/olivierh59500/r-type/internal/sim"
	"github.com/olivierh59500/r-type/internal/source"
)

// Session keeps the player's world outside the demonstration. Scores, records,
// mutable stage data and the first-ship latch must never leak from a replay.
type Session struct {
	player *sim.World
}

func (s *Session) Active() bool { return s.player != nil }

// Begin creates an independent title world with the recorded seed and the
// player's configured rules. The first replayed Start enters through World.Step.
func (s *Session) Begin(player *sim.World) (*sim.World, error) {
	if s.Active() || player == nil || player.State != sim.Title {
		return nil, fmt.Errorf("attract demonstration requires an idle title world")
	}
	demo := sim.NewWorld(pilot.DefaultSeed)
	if err := demo.SetStages(player.Stages); err != nil {
		return nil, err
	}
	if err := demo.SetInitialLives(player.InitialLives); err != nil {
		return nil, err
	}
	if err := demo.SetExtraLifeThresholds(player.ExtraLifeThresholds); err != nil {
		return nil, err
	}
	demo.HighScore = player.HighScore
	demo.Records = append([]source.HighScoreEntry(nil), player.Records...)
	demo.RecordChoices, demo.RecordPrompt = player.RecordChoices, player.RecordPrompt
	s.player = player
	return demo, nil
}

// Restore returns the exact preserved player session, then releases it.
func (s *Session) Restore() *sim.World {
	player := s.player
	s.player = nil
	return player
}
