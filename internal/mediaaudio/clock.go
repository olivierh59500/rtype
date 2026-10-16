// Package mediaaudio clocks native PCM streams for deterministic media export.
package mediaaudio

import (
	"encoding/binary"
	"fmt"
	"io"
	"math"
)

// Clock advances exactly one stereo audio block per captured game update.
// It never opens an audio device or records sound from other applications.
type Clock struct {
	players []*Player
	frames  int
	pcm     []byte
	scratch []byte
	mix     []float64
}

type Player struct {
	stream  io.Reader
	playing bool
	volume  float64
}

func New(sampleRate, tickRate int) (*Clock, error) {
	if sampleRate <= 0 || tickRate <= 0 || sampleRate%tickRate != 0 {
		return nil, fmt.Errorf("audio sample rate must be a positive multiple of the capture tick rate")
	}
	frames := sampleRate / tickRate
	return &Clock{frames: frames, pcm: make([]byte, frames*4), scratch: make([]byte, frames*4), mix: make([]float64, frames*2)}, nil
}

func (c *Clock) NewPlayer(stream io.Reader) *Player {
	p := &Player{stream: stream, volume: 1}
	c.players = append(c.players, p)
	return p
}

func (p *Player) Play()                   { p.playing = true }
func (p *Player) Pause()                  { p.playing = false }
func (p *Player) SetVolume(value float64) { p.volume = value }

// ReadTick returns signed 16-bit little-endian stereo PCM. The returned buffer
// is reused on the next call, so callers must write or copy it before advancing.
func (c *Clock) ReadTick() ([]byte, error) {
	clear(c.mix)
	for _, player := range c.players {
		if !player.playing {
			continue
		}
		if _, err := io.ReadFull(player.stream, c.scratch); err != nil {
			return nil, fmt.Errorf("capture audio: %w", err)
		}
		for index := range c.mix {
			value := int16(binary.LittleEndian.Uint16(c.scratch[index*2:]))
			c.mix[index] += float64(value) * player.volume
		}
	}
	for index, value := range c.mix {
		value = math.Max(-32768, math.Min(32767, math.Round(value)))
		binary.LittleEndian.PutUint16(c.pcm[index*2:], uint16(int16(value)))
	}
	return c.pcm, nil
}
