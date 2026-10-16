package sound

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/olivierh59500/r-type/internal/source"
)

// StageMusic transcribes the game's compact custom pattern player. Early
// levels use 98-byte patterns; later main scores use 66-byte patterns.
// Its input contains only extracted order, pattern, tuning and sample data.
type StageMusic struct {
	mu                                               sync.Mutex
	data                                             []byte
	spec                                             source.LevelMusic
	voices                                           [4]stageVoice
	order, restart, patterns, stride, step, channels int
	phase, jingle, ended, active                     bool
	started                                          bool
	offMask                                          uint16
	frames                                           int
	partial                                          [4]byte
	partialAt                                        int
	err                                              error
	fading, fadeComplete                             bool
	fadeCount                                        int
	fadeStep                                         int
}

type stageVoice struct {
	begin, length               int
	period, volume              uint16
	playing                     bool
	currentBegin, currentLength int
	position                    float64
}

// NewStageMusic starts the original main score using normalized asset metadata.
func NewStageMusic(data []byte, spec source.LevelMusic) (*StageMusic, error) {
	if len(data) == 0 || spec.MainStride < 6 || spec.BossStride < 6 || spec.SampleTable < 0 || spec.SampleTable+spec.SampleCount*8 > len(data) || spec.Periods < 0 || spec.Periods >= len(data) || spec.Samples < 0 || spec.Samples >= len(data) {
		return nil, errors.New("invalid original stage music data")
	}
	p := &StageMusic{data: bytes.Clone(data), spec: spec, partialAt: 4}
	if err := p.Start(false); err != nil {
		return nil, err
	}
	return p, nil
}

var _ io.Reader = (*StageMusic)(nil)

// Start selects the two-channel main or boss score, preserving original stereo.
func (p *StageMusic) Start(boss bool) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.order, p.restart, p.patterns, p.stride = p.spec.Main, p.spec.Main, p.spec.MainPatterns, p.spec.MainStride
	if boss {
		p.order, p.restart, p.patterns, p.stride = p.spec.Boss1, p.spec.Boss2, p.spec.BossPatterns, p.spec.BossStride
	}
	p.channels, p.jingle, p.ended, p.active, p.step, p.err = 2, false, false, true, 0, nil
	// Main/boss initialization preserves the driver's half-rate phase and
	// enables its two DMA bits without clearing an interrupted jingle's others.
	p.frames, p.partialAt, p.started = 0, 4, false
	p.fading, p.fadeComplete, p.fadeCount = false, false, 0
	p.fadeStep = 1
	p.err = p.row()
	return p.err
}

// StartFull selects the original D0=255 four-channel looping mode used by
// high-score name entry and the ending. It is distinct from finite jingles.
func (p *StageMusic) StartFull() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	for channel := range p.voices {
		p.voices[channel].playing = false
	}
	p.order, p.restart, p.patterns, p.stride = p.spec.Main, p.spec.Main, p.spec.MainPatterns, p.spec.MainStride
	p.channels, p.jingle, p.ended, p.active, p.step, p.err = 4, false, false, true, 0, nil
	p.frames, p.partialAt, p.started = 0, 4, false
	p.fading, p.fadeComplete, p.fadeCount = false, false, 0
	p.fadeStep = 2
	p.err = p.row()
	return p.err
}

// PlayJingle selects one of the extracted four-channel completion themes.
func (p *StageMusic) PlayJingle(index int) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if index < 0 || index >= len(p.spec.Jingles) {
		return errors.New("stage jingle index is outside original score")
	}
	for channel := range p.voices {
		p.voices[channel].playing = false
	}
	p.order, p.restart = p.spec.Jingles[index], p.spec.Jingles[index]
	p.patterns, p.stride = p.spec.BossPatterns, p.spec.BossStride
	p.channels, p.jingle, p.ended, p.active, p.step, p.err = 4, true, false, true, 0, nil
	p.frames, p.partialAt, p.started = 0, 4, false
	p.fading, p.fadeComplete, p.fadeCount = false, false, 0
	p.fadeStep = 1
	p.err = p.row()
	return p.err
}

// Stop releases the original music channels when leaving a stage.
func (p *StageMusic) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.active = false
	for channel := range p.voices {
		p.voices[channel].playing = false
	}
}

// JingleDone reports the original completion signal without consuming it.
// The game thread can switch back to the main score after an introduction.
func (p *StageMusic) JingleDone() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.jingle && p.ended
}

// FadeOut requests the original 63-interrupt volume fade before boss music.
// Repeated requests preserve progress, as the original D3=1 command does.
func (p *StageMusic) FadeOut() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.active {
		p.fadeComplete = true
		return
	}
	p.fading = true
}

// FadeDone exposes the original zero return value after all music DMA stops.
func (p *StageMusic) FadeDone() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.fadeComplete
}

func (p *StageMusic) span(offset, length int) ([]byte, error) {
	if offset < 0 || length < 0 || offset+length > len(p.data) {
		return nil, fmt.Errorf("stage music offset 0x%x is outside original data", offset)
	}
	return p.data[offset : offset+length], nil
}

func (p *StageMusic) tick() error {
	if !p.active {
		return nil
	}
	defer func() {
		if p.fading {
			if p.fadeCount == 62 {
				p.active, p.fading, p.fadeComplete = false, false, true
				for channel := range p.voices {
					p.voices[channel].playing = false
				}
			} else {
				p.fadeCount += max(1, p.fadeStep)
			}
		}
	}()
	if p.jingle && p.ended {
		p.active = false
		for channel := range p.voices {
			p.voices[channel].playing = false
		}
		return nil
	}
	if !p.phase {
		p.phase = true
		for channel := 0; channel < p.channels; channel++ {
			if p.offMask&(1<<channel) != 0 {
				p.voices[channel].playing = false
			}
		}
		return nil
	}
	p.phase = false
	p.step += 4
	marker, err := p.span(p.patterns+p.step, 1)
	if err != nil {
		return err
	}
	if marker[0] == 255 {
		p.step = 0
		p.order += p.channels * 2
		entry, err := p.span(p.order, 1)
		if err != nil {
			return err
		}
		if entry[0] == 255 {
			p.ended = true
			if p.jingle {
				p.active = false
				for channel := range p.voices {
					p.voices[channel].playing = false
				}
				return nil
			}
			p.order = p.restart
		}
	}
	return p.row()
}

func (p *StageMusic) row() error {
	p.offMask = 0
	for channel := 0; channel < p.channels; channel++ {
		order, err := p.span(p.order+channel*2, 2)
		if err != nil {
			return err
		}
		entry, err := p.span(p.patterns+int(order[0])*p.stride+p.step, 4)
		if err != nil {
			return err
		}
		sample, err := p.span(p.spec.SampleTable+int(entry[0])*8, 8)
		if err != nil {
			return err
		}
		voice := &p.voices[channel]
		voice.begin = p.spec.Samples + int(binary.BigEndian.Uint32(sample))
		voice.length = int(binary.BigEndian.Uint16(sample[6:])) * 2
		if _, err := p.span(voice.begin, max(voice.length, 1)); err != nil {
			return err
		}
		if entry[1]&128 != 0 {
			delta := uint16(entry[1] & 127)
			if entry[3]&128 != 0 {
				voice.period += delta
			} else {
				voice.period -= delta
			}
		} else {
			tuning, err := p.span(p.spec.Periods+(int(entry[1])+int(order[1]))*2, 2)
			if err != nil {
				return err
			}
			voice.period = binary.BigEndian.Uint16(tuning)
		}
		voice.volume = uint16(max(int(entry[2])-p.fadeCount, 1))
		if entry[3]&1 == 0 {
			p.offMask |= 1 << channel
		}
		if !voice.playing {
			voice.currentBegin, voice.currentLength, voice.position = voice.begin, voice.length, 0
		}
		voice.playing = true
	}
	return nil
}

// Read supplies endless signed 16-bit stereo PCM; completed jingles turn silent.
func (p *StageMusic) Read(output []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	written := 0
	for written < len(output) {
		if p.partialAt == 4 {
			if p.err != nil {
				return written, p.err
			}
			if p.frames == 0 && p.started {
				if p.err = p.tick(); p.err != nil {
					return written, p.err
				}
			}
			p.started = true
			left, right := p.mixFrame()
			binary.LittleEndian.PutUint16(p.partial[:2], uint16(left))
			binary.LittleEndian.PutUint16(p.partial[2:], uint16(right))
			p.partialAt = 0
			p.frames = (p.frames + 1) % (SampleRate / palTicks)
		}
		n := copy(output[written:], p.partial[p.partialAt:])
		written += n
		p.partialAt += n
	}
	return written, nil
}

func (p *StageMusic) mixFrame() (int16, int16) {
	left, right := 0, 0
	for channel := range p.voices {
		voice := &p.voices[channel]
		if !voice.playing || voice.period == 0 {
			continue
		}
		if voice.currentLength <= 0 {
			voice.currentLength = 131072
		}
		for voice.position >= float64(voice.currentLength) {
			voice.position -= float64(voice.currentLength)
			voice.currentBegin, voice.currentLength = voice.begin, voice.length
			if voice.currentLength <= 0 {
				voice.currentLength = 131072
			}
		}
		offset := voice.currentBegin + int(voice.position)
		sample := 0
		if offset >= 0 && offset < len(p.data) {
			sample = int(int8(p.data[offset]))
		}
		volume := voice.volume & 127
		if volume >= 64 {
			volume = 64
		}
		value := sample * int(volume) * 2
		if channel == 0 || channel == 3 {
			left += value
		} else {
			right += value
		}
		voice.position += float64(palAudioClock) / (float64(voice.period) * SampleRate)
	}
	return saturate(left), saturate(right)
}
