package source

import "encoding/binary"

// FireWormStep retains one original signed route triple. DX 127 marks the
// original repeat of the preceding triple, rather than terminating the actor.
type FireWormStep struct{ DX, DY, Frame int }

// FireWormData contains the stage-five segmented worm's original route and
// damage-detachment tables. All actor linkage and movement are native Go.
type FireWormData struct {
	BodyCount, StartOffset                         int
	HeadDescriptor, BodyDescriptor, TailDescriptor uint32
	Path                                           []FireWormStep
	BreakOffsets                                   []int
	Scatter                                        [][2]int
	ScatterAnimation                               []int
	ScatterAnimationTicks                          int
}

func annotateFireWorm(memory []byte, event *Event, level int) bool {
	if level != 5 || event.SpawnAddress != 0x4a928 || event.UpdateAddress != 0x4a9f0 {
		return false
	}
	u16 := func(a int) int { return int(binary.BigEndian.Uint16(memory[a:])) }
	i16 := func(a int) int { return int(int16(binary.BigEndian.Uint16(memory[a:]))) }
	d := &FireWormData{BodyCount: int(int16(uint16(event.Parameter >> 16))), StartOffset: int(event.Parameter & 65535), HeadDescriptor: 0x4b0d0, BodyDescriptor: 0x4b18a, TailDescriptor: 0x4b232}
	if d.BodyCount < 0 || d.BodyCount > 15 {
		return false
	}
	// Routes begin at byte offsets that need not be divisible by three.
	// Index every possible triple so native cursors retain exact byte units.
	for a := 0x67fc8; a+3 <= 0x688b8; a++ {
		d.Path = append(d.Path, FireWormStep{DX: int(int8(memory[a])), DY: int(int8(memory[a+1])), Frame: int(memory[a+2] & 15)})
	}
	for index := 0; index < 16; index++ {
		d.BreakOffsets = append(d.BreakOffsets, u16(0x4b0b0+index*2))
	}
	for index := 0; index < 17; index++ {
		d.Scatter = append(d.Scatter, [2]int{i16(0x4b08c + index*4), i16(0x4b08e + index*4)})
	}
	// All three original descriptors share this sixteen-pose stream and
	// period. Head destruction enables the ordinary animator on its followers.
	animation := int(binary.BigEndian.Uint32(memory[int(d.HeadDescriptor)+20:]))
	d.ScatterAnimationTicks = int(memory[int(d.HeadDescriptor)+7]) + 1
	for index := 0; index < 32; index++ {
		frame := int(int8(memory[animation+index]))
		if frame < 0 {
			break
		}
		d.ScatterAnimation = append(d.ScatterAnimation, frame)
	}
	event.FireWorm = d
	event.Behavior = "fire_worm"
	event.Health = 1000
	return true
}
