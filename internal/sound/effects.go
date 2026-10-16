package sound

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"sync"
)

// Effect holds an original signed 8-bit sample and its original Paula settings.
// Address records source provenance; sample playback uses the extracted bytes.
type Effect struct {
	Samples                  []byte
	Address                  uint32
	Period, Volume, Priority int
}

// Effects implements R-Type's two-voice priority arbiter at original RAM 3AE6.
// Play and Stop may run on the game thread while Read runs on the audio thread.
type Effects struct {
	mu        sync.Mutex
	effects   []Effect
	voices    [2]effectVoice
	frames    int
	partial   [4]byte
	partialAt int
}

type effectVoice struct {
	phase    int8
	priority byte
	pending  int
	current  int
	playing  bool
	position float64
}

// NewEffects creates an endless stereo PCM stream with original channel 2 in
// the right speaker and channel 3 in the left speaker.
func NewEffects(effects []Effect) (*Effects, error) {
	p := &Effects{effects: make([]Effect, len(effects)), partialAt: 4}
	for number, effect := range effects {
		if len(effect.Samples) == 0 || len(effect.Samples)%2 != 0 || effect.Period <= 0 || effect.Period > 65535 || effect.Volume < 0 || effect.Volume > 127 || effect.Priority < 0 || effect.Priority > 255 {
			return nil, fmt.Errorf("invalid original sound effect %d", number)
		}
		p.effects[number] = effect
		p.effects[number].Samples = bytes.Clone(effect.Samples)
	}
	for index := range p.voices {
		p.voices[index].pending = -1
		p.voices[index].current = -1
	}
	return p, nil
}

var _ io.Reader = (*Effects)(nil)

// Play queues the numbered original effect when its priority permits it.
// Equal-priority idle channels select channel 3, exactly as in the Amiga game.
func (p *Effects) Play(number int) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.queue(number)
}

func (p *Effects) queue(number int) bool {
	if number < 0 || number >= len(p.effects) {
		return false
	}
	priority := byte(p.effects[number].Priority)
	first, second := p.voices[0].priority, p.voices[1].priority
	channel := 0
	if priority < first {
		if priority < second {
			return false
		}
		channel = 1
	} else if priority >= second && first >= second {
		channel = 1
	}
	voice := &p.voices[channel]
	voice.pending, voice.priority = number, priority
	return true
}

// Stop cancels active samples and pending effects when leaving gameplay.
func (p *Effects) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	for index := range p.voices {
		p.voices[index] = effectVoice{pending: -1, current: -1}
	}
}

func (p *Effects) tick() {
	for index := range p.voices {
		voice := &p.voices[index]
		if voice.pending < 0 {
			continue
		}
		if voice.phase > 0 {
			// The original disables the old channel for one PAL tick before
			// starting a replacement sample (RAM 3B54-3BD4).
			voice.playing, voice.phase = false, -1
			continue
		}
		voice.current, voice.pending = voice.pending, -1
		voice.phase, voice.position, voice.playing = 1, 0, true
		voice.priority = byte(p.effects[voice.current].Priority)
	}
}

// complete represents the sample-completion interrupt at original RAM 3C14.
// Paula's initial DMA fetch primes the original interrupt guard; completing
// the sample then disables this voice and releases its priority.
func (p *Effects) complete(channel int) {
	voice := &p.voices[channel]
	voice.playing, voice.phase, voice.priority = false, 0, 0
}

func (p *Effects) Read(output []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	written := 0
	for written < len(output) {
		if p.partialAt == 4 {
			if p.frames == 0 {
				p.tick()
			}
			left, right := p.mixFrame()
			binary.LittleEndian.PutUint16(p.partial[0:2], uint16(left))
			binary.LittleEndian.PutUint16(p.partial[2:4], uint16(right))
			p.partialAt = 0
			p.frames = (p.frames + 1) % (SampleRate / palTicks)
		}
		n := copy(output[written:], p.partial[p.partialAt:])
		written += n
		p.partialAt += n
	}
	return written, nil
}

func (p *Effects) mixFrame() (int16, int16) {
	values := [2]int16{}
	for index := range p.voices {
		voice := &p.voices[index]
		if !voice.playing {
			continue
		}
		effect := &p.effects[voice.current]
		if voice.position >= float64(len(effect.Samples)) {
			p.complete(index)
			continue
		}
		volume := min(effect.Volume, 64)
		values[index] = int16(int(int8(effect.Samples[int(voice.position)])) * volume * 2)
		voice.position += float64(palAudioClock) / (float64(effect.Period) * SampleRate)
	}
	return values[1], values[0]
}
