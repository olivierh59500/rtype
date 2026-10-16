package sim

import (
	"fmt"
	"github.com/olivierh59500/r-type/internal/source"
)

// RecordEntryState replaces the original joystick character selector with
// native indices; the original forty-seven characters retain their meanings.
type RecordEntryState struct {
	Row, Choice, Repeat, Direction int
	Name                           string
}

func (w *World) SetRecords(data *source.RecordsData) error {
	if data == nil {
		return nil
	}
	if len(data.Entries) != 10 || len(data.Choices) != 47 {
		return fmt.Errorf("invalid original score table")
	}
	for index, entry := range data.Entries {
		if entry.Score < 0 || len(entry.Name) > 16 || (index > 0 && entry.Score > data.Entries[index-1].Score) {
			return fmt.Errorf("invalid original score entry%d", index)
		}
	}
	w.Records = append([]source.HighScoreEntry(nil), data.Entries...)
	w.RecordChoices, w.RecordPrompt = data.Choices, data.Prompt
	return nil
}

// beginScoreFlow inserts equal scores ahead of the previous entry, exactly as
// core 0x3684's unsigned comparison. Existing row positions remain fixed.
func (w *World) beginScoreFlow() {
	if len(w.Records) == 0 {
		w.State = GameOver
		return
	}
	w.RecordEntry = nil
	w.State = ScoreTable
	w.RecordTimer = 525
	row := -1
	for index, entry := range w.Records {
		if w.Score >= entry.Score {
			row = index
			break
		}
	}
	if row < 0 {
		return
	}
	for index := len(w.Records) - 1; index > row; index-- {
		w.Records[index].Name = w.Records[index-1].Name
		w.Records[index].Score = w.Records[index-1].Score
	}
	w.Records[row].Name = ""
	w.Records[row].Score = w.Score
	w.RecordEntry = &RecordEntryState{Row: row}
	w.State = ScoreEntry
}

func (w *World) finishRecordEntry() {
	if w.RecordEntry != nil {
		w.Records[w.RecordEntry.Row].Name = w.RecordEntry.Name
		w.RecordRevision++
	}
	w.RecordEntry = nil
	w.RecordTimer = 525
	w.State = ScoreTable
}

// FinishRecordEntry lets a host's Back action retain an unfinished local name.
func (w *World) FinishRecordEntry() {
	if w.State == ScoreEntry {
		w.finishRecordEntry()
	}
}

func (w *World) stepScoreFlow(input Input) bool {
	if w.State != ScoreEntry && w.State != ScoreTable {
		return false
	}
	fire := input.Fire && !w.previous.Fire
	start := input.Start && !w.previous.Start
	if w.State == ScoreTable {
		w.RecordTimer--
		if w.RecordTimer <= 0 || fire || start {
			w.State = Title
			w.Boss = BossState{}
		}
		return true
	}
	s := w.RecordEntry
	if s == nil {
		w.State = ScoreTable
		return true
	}
	if start {
		w.finishRecordEntry()
		return true
	}
	direction := 0
	if input.Up || input.Right {
		direction = 1
	} else if input.Down || input.Left {
		direction = -1
	}
	if direction == 0 {
		s.Repeat, s.Direction = 0, 0
	} else {
		s.Repeat--
		if direction != s.Direction || s.Repeat < 0 {
			s.Choice = (s.Choice + direction + 47) % 47
			s.Direction, s.Repeat = direction, 5
		}
	}
	if !fire {
		return true
	}
	switch s.Choice {
	case 44:
		if len(s.Name) > 0 {
			s.Name = s.Name[:len(s.Name)-1]
			w.Records[s.Row].Name = s.Name
		}
	case 46:
		w.finishRecordEntry()
	default:
		if len(s.Name) < 16 {
			character := w.RecordChoices[s.Choice]
			if s.Choice == 45 {
				character = ' '
			}
			s.Name += string(character)
			w.Records[s.Row].Name = s.Name
			if len(s.Name) == 16 {
				w.finishRecordEntry()
			}
		}
	}
	return true
}
