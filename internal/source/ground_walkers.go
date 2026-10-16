package source

import (
	"encoding/binary"
	"fmt"
)

// OriginalObjectData is declarative original art and per-frame collision data.
type OriginalObjectData struct {
	Descriptor                uint32
	Sprite                    string
	Width, Height, FrameTicks int
	Animation                 []int
	Collision                 []CollisionFrame
}
type GroundWalkerData struct {
	Stand, Jump                                                    []int
	FrameTicks                                                     int
	Side, Homing, Spark                                            OriginalObjectData
	SparkLifetime, SparkSound                                      int
	InitialDirections, VelocityX, VelocityY, AimMap, AimThresholds []int
}

func annotateGroundWalker(memory []byte, event *Event, level int) bool {
	if level != 1 || event.SpawnAddress != 0x4b1fc || event.UpdateAddress != 0x4b22e {
		return false
	}
	u16 := func(a int) int { return int(binary.BigEndian.Uint16(memory[a:])) }
	i16 := func(a int) int { return int(int16(binary.BigEndian.Uint16(memory[a:]))) }
	u32 := func(a int) int { return int(binary.BigEndian.Uint32(memory[a:])) }
	stream := func(a int) []int {
		var frames []int
		for i := 0; i < 64 && a+i < len(memory); i++ {
			frame := int(int8(memory[a+i]))
			if frame < 0 {
				break
			}
			frames = append(frames, frame)
		}
		return frames
	}
	object := func(a, frames int) OriginalObjectData {
		size := u16(a)
		item := OriginalObjectData{Descriptor: uint32(a), Sprite: fmt.Sprintf("object-%d-%05x", level, a), Width: (size&63 - 1) * 16, Height: (size >> 6) / 4, FrameTicks: int(memory[a+7]) + 1, Animation: stream(u32(a + 20))}
		item.Collision, _ = collisionFrames(memory, u32(a+24), frames)
		return item
	}
	d := &GroundWalkerData{Stand: stream(0x4bff2), Jump: stream(0x4bffb), FrameTicks: event.AnimationTicks, Side: object(0x4c036, 1), Homing: object(0x4c068, 16), Spark: object(u32(0x4922), 13), SparkLifetime: int(memory[0x4929]) + 1, SparkSound: 11}
	for i := 0; i < 4; i++ {
		d.InitialDirections = append(d.InitialDirections, u16(0x4c122+i*2))
	}
	for i := 0; i < 16; i++ {
		d.VelocityX = append(d.VelocityX, i16(0x4c12a+i*4))
		d.VelocityY = append(d.VelocityY, i16(0x4c12c+i*4))
	}
	for _, v := range memory[0x4a1c:0x4a30] {
		d.AimMap = append(d.AimMap, int(v))
	}
	for i := 0; i < 5; i++ {
		d.AimThresholds = append(d.AimThresholds, u16(0x4a30+i*2))
	}
	event.GroundWalker = d
	event.Behavior = "ground_walker"
	return true
}
