package source

import (
	"encoding/binary"
	"fmt"
)

// MiniTailData describes the independent stage-two miniature tail formations.
// These actors have no connection to the large Gomander boss's linked tail.
type MiniTailData struct {
	Count, SpawnX, SpawnYBase, SpawnYRange, DelayStep, OscillationReload int
	RetargetChance, RetargetThreshold, RetargetBase, RetargetRange       int
	Velocity                                                             [][2]int
	Animations                                                           [][]int
}

func annotateMiniTail(memory []byte, event *Event, level int) bool {
	if level != 2 || event.SpawnAddress != 0x4a638 || event.UpdateAddress != 0x4a6ae {
		return false
	}
	d := &MiniTailData{Count: int(event.Parameter>>16) + 1, SpawnX: 372, SpawnYBase: 80, SpawnYRange: 105, DelayStep: 20, OscillationReload: 6, RetargetChance: 100, RetargetThreshold: 70, RetargetBase: 25, RetargetRange: 33}
	for index := 0; index < 3; index++ {
		a := 0x4af98 + index*4
		d.Velocity = append(d.Velocity, [2]int{int(int16(binary.BigEndian.Uint16(memory[a:]))), int(int16(binary.BigEndian.Uint16(memory[a+2:])))})
	}
	d.Animations = append(d.Animations, append([]int(nil), event.Animation...))
	for _, a := range []int{0x4afd0, 0x4afd4} {
		var frames []int
		for index := 0; index < 32; index++ {
			value := int(int8(memory[a+index]))
			if value < 0 {
				break
			}
			frames = append(frames, value)
		}
		d.Animations = append(d.Animations, frames)
	}
	event.MiniTail = d
	event.Behavior = "mini_tail"
	event.Health = 1
	return true
}

func annotateDoubleSpitter(memory []byte, event *Event, level int) bool {
	if level != 7 || event.SpawnAddress != 0x4b60c || event.UpdateAddress != 0x4b640 {
		return false
	}
	u32 := func(a int) int { return int(binary.BigEndian.Uint32(memory[a:])) }
	d := &SpitterData{VX: -3, Health: 1, InitialCooldownRange: 30, FirePeriod: 50, ProjectileYBase: 10, ProjectileDescriptor: 0x4c00a, ProjectileVX: -7, ProjectileMinX: 16, SpawnCount: 2, SpawnX: event.X, SpawnYBase: event.Y, SpawnDelayStep: 24, SpawnAnimationCycle: 2}
	a := int(d.ProjectileDescriptor)
	size := int(binary.BigEndian.Uint16(memory[a:]))
	d.ProjectileWidth, d.ProjectileHeight = (size&63-1)*16, (size>>6)/4
	d.ProjectileSprite = fmt.Sprintf("object-%d-%05x", level, d.ProjectileDescriptor)
	animation := u32(a + 20)
	frames := 1
	for index := 0; index < 32; index++ {
		value := int(int8(memory[animation+index]))
		if value < 0 {
			break
		}
		d.ProjectileAnimation = append(d.ProjectileAnimation, value)
		frames = max(frames, value+1)
	}
	d.ProjectileAnimationTicks = int(memory[a+7]) + 1
	d.ProjectileCollision, _ = collisionFrames(memory, u32(a+24), frames)
	event.Spitter = d
	event.Behavior = "spitter"
	event.Health = 1
	return true
}
