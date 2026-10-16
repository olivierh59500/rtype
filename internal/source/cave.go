package source

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io/fs"
)

// CaveData contains stage-two map creatures and their emitted actors. All
// coordinates, frames and movement tables are normalized original data.
type CaveData struct {
	Version                  int
	ADFSHA256                string
	Creatures                []CaveCreatureData
	BodyDescriptor           uint32
	BodyHP, DamageLimit      int
	BodyDeathKind, BodyScore int
	BodyCollisions           []CollisionFrame
	ShotDescriptor           uint32
	ShotHP, AnimationTicks   int
	ShotScore                int
	ShotAnimation            []int
	BounceAnimation          []int
	ShotCollisions           []CollisionFrame
	Curves                   []CaveCurve
	Directions               [][2]int
	Opposite                 []int
	ShotDelays               [2]int
	CurveTimers              [2]int
}

// CaveCreatureData describes one authored four-by-three animated map region.
type CaveCreatureData struct {
	SpawnScroll, OpenScroll, CloseScroll int
	Y, InitialFrame, Column, Row         int
	Frames                               [][]uint16
}

// CaveCurve retains the signed movement pairs, including the final Y sentinel.
type CaveCurve struct{ X, Y []int8 }

// ReadCave loads the original stage-two companion recreated by the extractor.
func ReadCave(files fs.FS) (*CaveData, error) {
	content, err := fs.ReadFile(files, "cave.json")
	if err != nil {
		return nil, fmt.Errorf("original cave data is missing; recreate runtime assets with cmd/extract: %w", err)
	}
	var data CaveData
	if err := json.Unmarshal(content, &data); err != nil {
		return nil, err
	}
	if err := data.Validate(); err != nil {
		return nil, err
	}
	return &data, nil
}

// Validate rejects incomplete movement, animation and collision operands before
// they can enter either host's native simulation.
func (data *CaveData) Validate() error {
	if data == nil || data.Version != 1 || data.ADFSHA256 != ADFSHA256 || len(data.Creatures) != 2 || len(data.BodyCollisions) != 10 || len(data.ShotCollisions) != 9 || len(data.Curves) != 2 || len(data.Directions) != 32 || len(data.Opposite) != 16 || data.BodyHP <= 0 || data.DamageLimit <= 0 || data.ShotHP <= 0 || data.AnimationTicks <= 0 || data.BodyDeathKind != 2 || data.BodyDescriptor != 0x4b3cc || data.ShotDescriptor != 0x4b610 || data.BodyScore < 0 || data.ShotScore < 0 {
		return fmt.Errorf("unsupported original cave data")
	}
	for _, creature := range data.Creatures {
		if creature.SpawnScroll < 0 || creature.SpawnScroll >= creature.OpenScroll || creature.OpenScroll >= creature.CloseScroll || creature.InitialFrame < 0 || creature.InitialFrame >= len(data.BodyCollisions)-4 || creature.Column < 0 || creature.Row < 0 || len(creature.Frames) != 6 {
			return fmt.Errorf("invalid original cave creature")
		}
		for _, frame := range creature.Frames {
			if len(frame) != 12 {
				return fmt.Errorf("invalid original cave map frame")
			}
		}
	}
	for _, curve := range data.Curves {
		if len(curve.X) == 0 || len(curve.X) != len(curve.Y) || curve.Y[len(curve.Y)-1] != 127 {
			return fmt.Errorf("invalid original cave movement curve")
		}
	}
	for _, stream := range [][]int{data.ShotAnimation, data.BounceAnimation} {
		if len(stream) == 0 || stream[0] < 0 || stream[len(stream)-1] != -1 {
			return fmt.Errorf("unterminated original cave animation")
		}
		for _, frame := range stream {
			if frame < -1 || frame >= len(data.ShotCollisions) {
				return fmt.Errorf("invalid original cave animation frame")
			}
		}
	}
	if len(data.BounceAnimation) <= 5 || data.BounceAnimation[5] < 0 {
		return fmt.Errorf("original cave bounce entry is missing")
	}
	for index := range data.ShotDelays {
		if data.ShotDelays[index] <= 0 || data.CurveTimers[index] <= 0 {
			return fmt.Errorf("invalid original cave launch timing")
		}
	}
	for _, direction := range data.Opposite {
		if direction < 0 || direction >= len(data.Opposite) {
			return fmt.Errorf("invalid original cave direction map")
		}
	}
	return nil
}

func extractCave(memory []byte, rowBytes, graphicsEnd int) (*CaveData, error) {
	if len(memory) < 0x80000 || rowBytes <= 0 || rowBytes&1 != 0 || graphicsEnd > len(memory) {
		return nil, fmt.Errorf("invalid original cave extraction context")
	}
	u16 := func(address int) int { return int(binary.BigEndian.Uint16(memory[address:])) }
	u32 := func(address int) int { return int(binary.BigEndian.Uint32(memory[address:])) }
	d := &CaveData{Version: 1, ADFSHA256: ADFSHA256, BodyDescriptor: uint32(u32(0x4abcc)), ShotDescriptor: uint32(u32(0x4ad2a)), BodyHP: u16(0x4abc8), DamageLimit: u16(0x4ace2), ShotHP: int(memory[0x4ad27]), ShotDelays: [2]int{u16(0x4abec), u16(0x4abf2)}}
	if d.BodyDescriptor != 0x4b3cc || d.ShotDescriptor != 0x4b610 {
		return nil, fmt.Errorf("unsupported cave creature descriptors")
	}
	var err error
	d.BodyCollisions, err = collisionFrames(memory, u32(int(d.BodyDescriptor)+24), 10)
	if err != nil {
		return nil, err
	}
	d.ShotCollisions, err = collisionFrames(memory, u32(int(d.ShotDescriptor)+24), 9)
	if err != nil {
		return nil, err
	}
	d.AnimationTicks = int(memory[int(d.ShotDescriptor)+7]) + 1
	d.BodyDeathKind = int(memory[0x4abdd])
	for power, bcd := 1, uint32(u32(int(d.ShotDescriptor)+36)); bcd != 0; power, bcd = power*10, bcd>>4 {
		d.ShotScore += int(bcd&15) * power
	}
	for power, bcd := 1, uint32(u32(0x4ad04)); bcd != 0; power, bcd = power*10, bcd>>4 {
		d.BodyScore += int(bcd&15) * power
	}
	d.CurveTimers = [2]int{int(memory[0x4ad5d]), int(memory[0x4ad55])}
	// The bounce entry starts at index five, after the first terminator.
	// Retain both sequences and their sentinels instead of trimming at FF.
	for _, value := range memory[0x4b638:0x4b63b] {
		d.ShotAnimation = append(d.ShotAnimation, int(int8(value)))
	}
	for _, value := range memory[0x4b63b:0x4b645] {
		d.BounceAnimation = append(d.BounceAnimation, int(int8(value)))
	}
	mapBase := u32(0x4a30c)
	for _, item := range []struct{ record, frames int }{{0x4b38c, 0x4b450}, {0x4b3ac, 0x4b4e0}} {
		a := item.record
		offset := u32(a+0x14) - mapBase
		if offset < 0 || offset&1 != 0 {
			return nil, fmt.Errorf("invalid cave map rectangle")
		}
		creature := CaveCreatureData{SpawnScroll: u16(a + 8), OpenScroll: u16(a + 10), CloseScroll: u16(a + 12), Y: u16(a + 14), InitialFrame: u16(a + 16), Column: (offset % rowBytes) / 2, Row: offset / rowBytes}
		for frame := 0; frame < 6; frame++ {
			var words []uint16
			for word := 0; word < 12; word++ {
				words = append(words, uint16(u16(item.frames+frame*24+word*2)))
			}
			creature.Frames = append(creature.Frames, words)
		}
		d.Creatures = append(d.Creatures, creature)
	}
	for _, address := range []int{0x72496, 0x7252f} {
		length := 0x4e
		if address == 0x7252f {
			length = 0x26
		}
		curve := CaveCurve{}
		terminated := false
		for index := 0; index < length; index++ {
			if address+length+index >= graphicsEnd {
				return nil, fmt.Errorf("cave curve exceeds original movement data")
			}
			curve.X = append(curve.X, int8(memory[address+index]))
			curve.Y = append(curve.Y, int8(memory[address+length+index]))
			if memory[address+length+index] == 127 {
				terminated = true
				break
			}
		}
		if !terminated {
			return nil, fmt.Errorf("cave curve has no authored terminator")
		}
		d.Curves = append(d.Curves, curve)
	}
	for index := 0; index < 32; index++ {
		d.Directions = append(d.Directions, [2]int{int(int16(u16(0x4b590 + index*4))), int(int16(u16(0x4b592 + index*4)))})
	}
	for _, value := range memory[0x4b580:0x4b590] {
		d.Opposite = append(d.Opposite, int(value))
	}
	return d, nil
}
