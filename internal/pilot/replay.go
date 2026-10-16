// Package pilot provides a visible-state controller and normal-input play checks.
package pilot

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/olivierh59500/r-type/internal/sim"
)

const ReplayVersion = 1
const DefaultSeed uint64 = sim.OriginalInitialRandomState

// Buttons is a portable input-level encoding; it does not encode CPU registers.
type Buttons uint16

const (
	Up Buttons = 1 << iota
	Down
	Left
	Right
	Fire
	Force
	Start
	Pause
)

func Encode(in sim.Input) Buttons {
	var result Buttons
	for _, pair := range []struct {
		on  bool
		bit Buttons
	}{{in.Up, Up}, {in.Down, Down}, {in.Left, Left}, {in.Right, Right}, {in.Fire, Fire}, {in.Force, Force}, {in.Start, Start}, {in.Pause, Pause}} {
		if pair.on {
			result |= pair.bit
		}
	}
	return result
}
func (b Buttons) Input() sim.Input {
	return sim.Input{Up: b&Up != 0, Down: b&Down != 0, Left: b&Left != 0, Right: b&Right != 0, Fire: b&Fire != 0, Force: b&Force != 0, Start: b&Start != 0, Pause: b&Pause != 0}
}

type ReplayMetadata struct {
	Memorized                bool   `json:"memorized,omitempty"`
	ArenaScan                bool   `json:"arena_scan,omitempty"`
	AssetManifestSHA256      string `json:"asset_manifest_sha256,omitempty"`
	HorizonPAL, BeamWidth    int
	ADFSHA256                string `json:"adf_sha256"`
	Seed                     uint64 `json:"seed"`
	TickRate                 int    `json:"tick_rate"`
	MainTickDivider          int    `json:"main_tick_divider"`
	InitialState             string `json:"initial_state"`
	InitialStage             int    `json:"initial_stage"`
	Policy                   string `json:"policy"`
	ReactionPAL, DecisionPAL int
}
type ReplayFrame struct {
	Tick    int     `json:"tick"`
	Buttons Buttons `json:"buttons"`
}

// Replay stores every host input tick, including the initial Title-to-game edge.
type Replay struct {
	Version  int            `json:"version"`
	Metadata ReplayMetadata `json:"metadata"`
	Frames   []ReplayFrame  `json:"frames"`
}

func (r *Replay) InputAt(tick int) sim.Input {
	if tick < 0 || tick >= len(r.Frames) {
		return sim.Input{}
	}
	return r.Frames[tick].Buttons.Input()
}
func WriteReplay(writer io.Writer, replay *Replay) error {
	return json.NewEncoder(writer).Encode(replay)
}

// ValidateSource rejects a different original disk or extracted data revision.
// Legacy logs without a manifest hash still require the original ADF hash.
func (r *Replay) ValidateSource(adfSHA256, manifestSHA256 string) error {
	if r.Metadata.ADFSHA256 != adfSHA256 {
		return fmt.Errorf("replay source ADF hash differs from loaded original assets")
	}
	if recorded := r.Metadata.AssetManifestSHA256; recorded != "" && recorded != manifestSHA256 {
		return fmt.Errorf("replay asset manifest hash differs from loaded extracted data")
	}
	return nil
}

func ReadReplay(reader io.Reader) (*Replay, error) {
	var replay Replay
	decoder := json.NewDecoder(reader)
	if err := decoder.Decode(&replay); err != nil {
		return nil, err
	}
	if replay.Version != ReplayVersion || replay.Metadata.TickRate != sim.TickRate || replay.Metadata.MainTickDivider != 2 || replay.Metadata.InitialState != "Title" || replay.Metadata.InitialStage != 1 {
		return nil, fmt.Errorf("unsupported replay origin or timing")
	}
	for index, frame := range replay.Frames {
		if frame.Tick != index || frame.Buttons&^Buttons(255) != 0 {
			return nil, fmt.Errorf("invalid input at replay tick %d", index)
		}
	}
	return &replay, nil
}
