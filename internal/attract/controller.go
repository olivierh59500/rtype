// Package attract schedules an authored, normal-input first-stage demo.
// Its controller owns no rendering, assets, save data, or platform input devices;
// Session keeps the demonstration world separate from the player's world.
package attract

import (
	"encoding/binary"

	"github.com/olivierh59500/r-type/internal/pilot"
	"github.com/olivierh59500/r-type/internal/sim"
)

//go:generate go run ../../cmd/attractgen -replay ../../.cache/playcheck/zero-camera-stage1-verified.inputs.json -manifest ../../assets/runtime/manifest.json -out replay_data.go

const DefaultTimeoutPAL = 20 * sim.TickRate

// Frame describes one host update. Started includes recorded input zero.
// The final recorded input is emitted normally; Ended is returned on the next
// call with no input. Human activity takes precedence over either event.
type Frame struct {
	Input                       sim.Input
	Started, Ended, Interrupted bool
}

// Controller counts focused title updates and emits recorded input levels.
// The host supplies human activity before replacing input with the demo and
// remains responsible for restoring its preserved title or starting real play.
type Controller struct {
	timeoutPAL       int
	idle, replayTick int
	active           bool
}

func New(timeoutPAL int) *Controller {
	if timeoutPAL <= 0 {
		timeoutPAL = DefaultTimeoutPAL
	}
	return &Controller{timeoutPAL: timeoutPAL}
}

func (c *Controller) Active() bool    { return c.active }
func (c *Controller) ReplayTick() int { return c.replayTick }
func (c *Controller) IdleTicks() int  { return c.idle }

// Advance consumes one PAL host tick. Hosts should omit calls while unfocused
// or while an explicit replay/autoplay check owns input. Inactive play never
// accumulates a title timeout. Any human activity resets the idle interval.
func (c *Controller) Advance(atTitle, humanActive bool) Frame {
	if humanActive {
		interrupted := c.Cancel()
		return Frame{Interrupted: interrupted}
	}
	if c.active {
		if c.replayTick >= FrameCount() {
			c.Reset()
			return Frame{Ended: true}
		}
		input := InputAt(c.replayTick)
		c.replayTick++
		return Frame{Input: input}
	}
	if !atTitle {
		c.idle = 0
		return Frame{}
	}
	c.idle++
	if c.idle < c.timeoutPAL {
		return Frame{}
	}
	c.idle, c.replayTick, c.active = 0, 1, true
	return Frame{Started: true, Input: InputAt(0)}
}

// Cancel releases all pending replay levels and reports whether a demo was
// active. It never emits input or advances the host's simulation.
func (c *Controller) Cancel() bool {
	active := c.active
	c.Reset()
	return active
}

func (c *Controller) Reset() {
	c.idle, c.replayTick, c.active = 0, 0, false
}

func Metadata() pilot.ReplayMetadata { return replayMetadata }
func FrameCount() int                { return replayFrameCount }
func InputSHA256() string            { return replayInputSHA256 }
func FinalDigest() string            { return replayFinalDigest }

// InputAt exposes only a copied button level. Returned values and metadata
// cannot mutate the generated replay or its source-verification record.
func InputAt(tick int) sim.Input {
	if tick < 0 || tick >= len(replayLevels) {
		return sim.Input{}
	}
	return pilot.Buttons(replayLevels[tick]).Input()
}

var replayLevels = expandRuns(replayRuns)

func expandRuns(encoded []byte) []byte {
	levels := make([]byte, 0, replayFrameCount)
	for cursor := 0; cursor < len(encoded); {
		count, size := binary.Uvarint(encoded[cursor:])
		if size <= 0 || count == 0 || count > uint64(replayFrameCount-len(levels)) || cursor+size >= len(encoded) {
			panic("invalid generated attract input data")
		}
		cursor += size
		buttons := encoded[cursor]
		cursor++
		for remaining := count; remaining > 0; remaining-- {
			levels = append(levels, buttons)
		}
	}
	if len(levels) != replayFrameCount {
		panic("incomplete generated attract input data")
	}
	return levels
}
