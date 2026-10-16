// Package sound renders the original R-Type TFMX title score with native Go
// sequencing and sample mixing. No 68000 instructions execute at runtime.
package sound

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const (
	// SampleRate is the signed 16-bit little-endian stereo output frequency.
	SampleRate    = 48000
	palTicks      = 50
	palAudioClock = 3546895
)

// TFMX implements the early four-channel player shipped with R-Type. Its
// pattern and instrument scripts are music data, independent of machine code.
// Read must have one owner, as with Ebitengine's audio stream readers.
type TFMX struct {
	data, samples                      []byte
	periods                            [256]uint16
	tracks                             [8]musicTrack
	voices                             [4]musicVoice
	first, last, row, speed, countdown int
	active                             bool
	frames                             int
	partial                            [4]byte
	partialAt                          int
	err                                error
}

type musicTrack struct {
	pattern, transpose byte
	offset, step, wait int
	loop               byte
	cleared            bool
}

type musicVoice struct {
	state                                                      int
	macro, step, wait                                          int
	loop                                                       byte
	note, volume, noteVolume                                   byte
	detune                                                     int8
	period, outputPeriod                                       uint16
	begin, length                                              int
	playing                                                    bool
	currentBegin, currentLength                                int
	position                                                   float64
	vibratoTime, vibratoCount                                  byte
	vibratoSpeed                                               int8
	vibratoOffset                                              int16
	portaCount, portaWait                                      byte
	portaSpeed, portaPeriod                                    uint16
	envelopeCount, envelopeWait, envelopeTarget, envelopeSpeed byte
}

// NewTFMX creates an endless PCM stream from locally extracted music, samples
// and the original driver's 48-entry big-endian tuning table. The subsong
// argument is the original zero-based song slot, not a filtered song index.
func NewTFMX(mdat, samples, periods []byte, subsong int) (*TFMX, error) {
	if len(mdat) < 0x800 || !bytes.HasPrefix(mdat, []byte("TFMX-SONG")) {
		return nil, errors.New("invalid R-Type TFMX title module")
	}
	if len(samples) == 0 || len(periods) != 96 {
		return nil, errors.New("original samples and 48-entry tuning table are required")
	}
	if subsong < 0 || subsong >= 32 {
		return nil, errors.New("TFMX song slot must be between 0 and 31")
	}
	// This native transcription intentionally targets the uncompressed v1
	// layout used by the supplied disk, with fixed table offsets.
	if binary.BigEndian.Uint32(mdat[0x1d0:]) != 0 || binary.BigEndian.Uint32(mdat[0x1d4:]) != 0 || binary.BigEndian.Uint32(mdat[0x1d8:]) != 0 {
		return nil, errors.New("compressed or later TFMX layouts are unsupported")
	}
	p := &TFMX{data: bytes.Clone(mdat), samples: bytes.Clone(samples), active: true, partialAt: 4}
	for note := 0; note < 48; note++ {
		p.periods[note] = binary.BigEndian.Uint16(periods[note*2:])
	}
	p.first = int(binary.BigEndian.Uint16(mdat[0x100+subsong*2:]))
	p.last = int(binary.BigEndian.Uint16(mdat[0x140+subsong*2:]))
	p.speed = int(binary.BigEndian.Uint16(mdat[0x180+subsong*2:]))
	if p.first > p.last || p.last >= 512 || 0x800+p.last*16+16 > len(mdat) {
		return nil, errors.New("TFMX song has invalid track bounds")
	}
	p.row = p.first
	if err := p.loadRow(); err != nil {
		return nil, err
	}
	return p, nil
}

var _ io.Reader = (*TFMX)(nil)

// Read returns stereo PCM and preserves complete frames across arbitrary reads.
// Runtime validation errors are reported instead of silently playing bad data.
func (p *TFMX) Read(output []byte) (int, error) {
	written := 0
	for written < len(output) {
		if p.partialAt == len(p.partial) {
			if p.err != nil {
				return written, p.err
			}
			if p.frames == 0 {
				if p.err = p.tick(); p.err != nil {
					return written, p.err
				}
			}
			left, right := p.mixFrame()
			binary.LittleEndian.PutUint16(p.partial[0:2], uint16(left))
			binary.LittleEndian.PutUint16(p.partial[2:4], uint16(right))
			p.partialAt = 0
			p.frames = (p.frames + 1) % (SampleRate / palTicks)
		}
		n := copy(output[written:], p.partial[p.partialAt:])
		p.partialAt += n
		written += n
	}
	return written, nil
}

func (p *TFMX) command(offset int) ([4]byte, error) {
	var command [4]byte
	if offset < 0 || offset+4 > len(p.data) {
		return command, fmt.Errorf("TFMX command offset 0x%x is outside music data", offset)
	}
	copy(command[:], p.data[offset:offset+4])
	return command, nil
}

func (p *TFMX) scriptOffset(table int, index byte) (int, error) {
	command, err := p.command(table + int(index)*4)
	if err != nil {
		return 0, err
	}
	offset := int(binary.BigEndian.Uint32(command[:]))
	if offset < 0x800 || offset+4 > len(p.data) {
		return 0, fmt.Errorf("TFMX script %d has invalid offset 0x%x", index, offset)
	}
	return offset, nil
}

// R-Type predates the later EFFE track-command marker. Its FA00, FA01 and
// FA02 rows mean stop, jump and speed, respectively (driver offsets 0x148-0x314).
func (p *TFMX) loadRow() error {
	for budget := 0; budget < 512; budget++ {
		offset := 0x800 + p.row*16
		if offset < 0x800 || offset+16 > len(p.data) {
			return fmt.Errorf("TFMX track row %d is outside music data", p.row)
		}
		row := p.data[offset : offset+16]
		switch binary.BigEndian.Uint16(row) {
		case 0xfa00:
			p.active = false
			return nil
		case 0xfa01:
			p.row = int(binary.BigEndian.Uint16(row[2:])) & 511
			continue
		case 0xfa02:
			p.speed = int(binary.BigEndian.Uint16(row[2:]))
			p.countdown = p.speed
			p.row = (p.row + 1) & 511
			continue
		}
		for index := range p.tracks {
			track := &p.tracks[index]
			track.pattern, track.transpose = row[index*2], row[index*2+1]
			track.step, track.wait, track.loop, track.cleared = 0, 0, 255, false
			if track.pattern < 0x80 {
				var err error
				track.offset, err = p.scriptOffset(0x400, track.pattern)
				if err != nil {
					return err
				}
			}
		}
		return nil
	}
	return errors.New("TFMX track commands do not reach a pattern row")
}

func (p *TFMX) tick() error {
	if !p.active {
		return nil
	}
	// The original player sequences patterns before updating effects and macros.
	if p.countdown > 0 {
		p.countdown--
	} else {
		p.countdown = p.speed
		if err := p.patterns(); err != nil {
			return err
		}
	}
	for index := range p.voices {
		voice := &p.voices[index]
		if voice.state <= 0 {
			voice.modulate()
		}
		if err := p.instrument(voice); err != nil {
			return err
		}
	}
	return nil
}

func (p *TFMX) patterns() error {
	// A pattern-end command advances all eight tracks immediately. Restarting
	// traversal reproduces the original player's shared-row synchronization.
	for budget := 0; budget < 512; budget++ {
		advanced := false
		for index := range p.tracks {
			track := &p.tracks[index]
			if track.pattern == 0xff {
				continue
			}
			if track.pattern == 0xfe {
				if !track.cleared {
					voice := &p.voices[track.transpose&3]
					voice.state, voice.playing = 0, false
					track.cleared = true
				}
				continue
			}
			if track.wait > 0 {
				track.wait--
				continue
			}
			var err error
			advanced, err = p.pattern(track, index)
			if err != nil {
				return err
			}
			if advanced || !p.active {
				break
			}
		}
		if !advanced || !p.active {
			return nil
		}
	}
	return errors.New("TFMX patterns advance rows without yielding")
}

func (p *TFMX) pattern(track *musicTrack, index int) (bool, error) {
	for budget := 0; budget < 512; budget++ {
		command, err := p.command(track.offset + track.step*4)
		if err != nil {
			return false, err
		}
		value := int(binary.BigEndian.Uint16(command[2:]))
		switch command[0] {
		case 0xf0:
			if p.row == p.last {
				p.row = p.first
			} else {
				p.row = (p.row + 1) & 511
			}
			return true, p.loadRow()
		case 0xf1:
			if track.loop == 0 {
				track.loop = 255
				track.step++
			} else {
				if track.loop == 255 {
					track.loop = command[1] - 1
				} else {
					track.loop--
				}
				track.step = value
			}
		case 0xf2:
			track.pattern = command[1] & 127
			track.offset, err = p.scriptOffset(0x400, track.pattern)
			if err != nil {
				return false, err
			}
			track.step = value
		case 0xf3:
			track.wait = int(command[1])
			track.step++
			return false, nil
		case 0xf4:
			track.pattern = 255
			return false, nil
		default:
			// The original only mutes note dispatch when a header track-mute
			// word is nonzero; its pattern still advances normally.
			if command[0] < 0xef && binary.BigEndian.Uint16(p.data[0x1c0+index*2:]) == 0 {
				command[0] += track.transpose
				if err := p.note(command); err != nil {
					return false, err
				}
			}
			track.step++
		}
	}
	return false, errors.New("TFMX pattern does not yield")
}

func (p *TFMX) note(command [4]byte) error {
	voice := &p.voices[command[2]&3]
	if command[0] >= 0x30 {
		// This older driver resumes the current macro with its loop released.
		voice.loop, voice.state = 0, -1
		return nil
	}
	offset, err := p.scriptOffset(0x600, command[1])
	if err != nil {
		return err
	}
	voice.note, voice.noteVolume, voice.detune = command[0], command[2]>>4, int8(command[3])
	voice.macro, voice.state = offset, 1
	return nil
}

func (v *musicVoice) modulate() {
	if v.vibratoTime != 0 {
		v.vibratoOffset += int16(v.vibratoSpeed)
		if v.portaSpeed == 0 {
			v.outputPeriod = uint16(int(v.period) + int(v.vibratoOffset))
		}
		v.vibratoCount--
		if v.vibratoCount == 0 {
			v.vibratoCount, v.vibratoSpeed = v.vibratoTime, -v.vibratoSpeed
		}
	}
	if v.portaSpeed != 0 {
		v.portaWait--
		if v.portaWait == 0 {
			v.portaWait = v.portaCount
			current, target := int(v.portaPeriod), int(v.period)
			if current > target {
				current -= int(v.portaSpeed)
				if current <= target {
					current, v.portaSpeed = target, 0
				}
			} else if current < target {
				current += int(v.portaSpeed)
				if current >= target {
					current, v.portaSpeed = target, 0
				}
			}
			v.portaPeriod, v.outputPeriod = uint16(current), uint16(current)
		}
	}
	if v.envelopeCount != 0 {
		if v.envelopeWait > 0 {
			v.envelopeWait--
		} else {
			v.envelopeWait = v.envelopeCount
			volume, target := int(v.volume), int(v.envelopeTarget)
			if volume > target {
				volume -= int(v.envelopeSpeed)
				if volume <= target {
					volume, v.envelopeCount = target, 0
				}
			} else {
				volume += int(v.envelopeSpeed)
				if volume >= target {
					volume, v.envelopeCount = target, 0
				}
			}
			v.volume = byte(volume)
		}
	}
}

func (p *TFMX) instrument(voice *musicVoice) error {
	if voice.state == 0 {
		return nil
	}
	if voice.state > 0 {
		voice.step, voice.wait, voice.loop, voice.state = 0, 0, 255, -1
	}
	if voice.wait > 0 {
		voice.wait--
		return nil
	}
	for budget := 0; budget < 512; budget++ {
		command, err := p.command(voice.macro + voice.step*4)
		if err != nil {
			return err
		}
		value := binary.BigEndian.Uint16(command[2:])
		voice.step++
		switch command[0] {
		case 0x00:
			voice.playing = false
			voice.envelopeCount, voice.vibratoTime, voice.portaSpeed = 0, 0, 0
		case 0x01:
			if !voice.playing {
				voice.currentBegin, voice.currentLength, voice.position = voice.begin, voice.length, 0
			}
			voice.playing = true
		case 0x02:
			voice.begin = int(command[1])<<16 | int(value)
		case 0x03:
			voice.length = int(value) * 2
		case 0x04:
			voice.wait = int(value)
			return nil
		case 0x05:
			// The original yields when a finite loop finishes, an R-Type
			// timing detail absent from later TFMX players.
			if voice.loop == 0 {
				voice.loop = 255
				return nil
			}
			if voice.loop == 255 {
				voice.loop = command[1] - 1
			} else {
				voice.loop--
			}
			voice.step = int(value)
		case 0x06:
			voice.macro, err = p.scriptOffset(0x600, command[1])
			if err != nil {
				return err
			}
			voice.step, voice.wait, voice.loop = int(value), 0, 255
		case 0x07, 0x14:
			voice.state = 0
			return nil
		case 0x08, 0x09:
			note := command[1]
			detune := 0
			if command[0] == 8 {
				note += voice.note
				detune = int(voice.detune)
			}
			voice.period = uint16(int(p.periods[note]) + detune + int(int16(value)))
			if voice.portaSpeed == 0 {
				voice.outputPeriod = voice.period
			}
			return nil
		case 0x0a:
			voice.envelopeCount, voice.vibratoTime, voice.portaSpeed = 0, 0, 0
		case 0x0b:
			voice.portaCount, voice.portaWait, voice.portaSpeed = command[1], 1, value
			voice.portaPeriod = voice.period
		case 0x0c:
			voice.vibratoTime = command[1] & 0xfe
			voice.vibratoCount = voice.vibratoTime >> 1
			voice.vibratoSpeed = int8(command[3])
			if voice.portaSpeed == 0 {
				voice.outputPeriod, voice.vibratoOffset = voice.period, 0
			}
		case 0x0d:
			voice.volume = byte(int(voice.noteVolume)*3 + int(value))
		case 0x0e:
			voice.volume = command[3]
		case 0x0f:
			voice.envelopeCount, voice.envelopeWait = command[2], command[2]
			voice.envelopeTarget, voice.envelopeSpeed = command[3], command[1]
		case 0x10:
			return nil
		case 0x11:
			voice.begin += int(int16(value))
		case 0x12:
			voice.length = int(uint16(voice.length/2+int(value))) * 2
		case 0x13:
			voice.playing = false
		default:
			// The original driver advances once and yields for undefined macros.
			return nil
		}
	}
	return errors.New("TFMX instrument does not yield")
}

func (p *TFMX) mixFrame() (int16, int16) {
	left, right := 0, 0
	for index := range p.voices {
		voice := &p.voices[index]
		if !voice.playing || voice.outputPeriod == 0 || !p.active {
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
		if offset >= 0 && offset < len(p.samples) {
			sample = int(int8(p.samples[offset]))
		}
		// Paula uses seven volume bits, with bit 6 selecting full volume.
		volume := int(voice.volume & 127)
		if volume >= 64 {
			volume = 64
		}
		value := sample * volume * 2
		if index == 0 || index == 3 {
			left += value
		} else {
			right += value
		}
		voice.position += float64(palAudioClock) / (float64(voice.outputPeriod) * SampleRate)
	}
	return saturate(left), saturate(right)
}

func saturate(value int) int16 {
	if value < -32768 {
		return -32768
	}
	if value > 32767 {
		return 32767
	}
	return int16(value)
}
