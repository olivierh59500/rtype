package source

import (
	"encoding/binary"
	"fmt"
)

// BattleshipTurretData preserves the six independently respawning gun actors.
// Their base coordinates follow the camera; aim offsets affect only drawing.
type BattleshipTurretData struct {
	DamageLimit, InitialDelay, HiddenDelay, RegenerateDelay, FirstAfterRegen int
	AimMap                                                                   []int
	AimThresholds, AimAnimation                                              []int
	AimOffsets                                                               [][2]int
	ProjectileDescriptor                                                     uint32
	ProjectileSprite                                                         string
	ProjectileWidth, ProjectileHeight, ProjectileFrameTicks                  int
	ProjectileCollisions                                                     []CollisionFrame
	ProjectileMotion                                                         [][][2]int
	ProjectileAnimations                                                     [][]int
	ProjectileVX, ProjectileVY                                               []int
}

func annotateBattleshipTurret(memory []byte, event *Event, level int) bool {
	if level != 3 || event.SpawnAddress != 0x4ab7c || event.UpdateAddress != 0x4ac2c {
		return false
	}
	u32 := func(a int) int { return int(binary.BigEndian.Uint32(memory[a:])) }
	u16 := func(a int) int { return int(binary.BigEndian.Uint16(memory[a:])) }
	i16 := func(a int) int { return int(int16(binary.BigEndian.Uint16(memory[a:]))) }
	d := &BattleshipTurretData{DamageLimit: int(event.Parameter & 255), InitialDelay: 50, HiddenDelay: 110, RegenerateDelay: 40, FirstAfterRegen: 25, ProjectileDescriptor: 0x4b9ce, ProjectileSprite: "object-3-4b9ce"}
	animation := int(uint32(uint16(event.Count))<<16 | uint32(uint16(event.Aux)))
	if animation < 0x4a2e4 || animation+16 > len(memory) {
		return false
	}
	for i := 0; i < 16; i++ {
		d.AimAnimation = append(d.AimAnimation, int(memory[animation+i]))
		event.AimAnimation = append(event.AimAnimation, int(memory[animation+i]))
	}
	for _, v := range memory[0x4a1c:0x4a30] {
		d.AimMap = append(d.AimMap, int(v))
	}
	for i := 0; i < 5; i++ {
		d.AimThresholds = append(d.AimThresholds, u16(0x4a30+i*2))
	}
	// The mapped aim values select seven directions. The following bytes are
	// trajectory samples, rather than an eighth pair of pointers.
	for i := 0; i < 7; i++ {
		d.AimOffsets = append(d.AimOffsets, [2]int{i16(0x4b9b2 + i*4), i16(0x4b9b4 + i*4)})
		motion, art := u32(0x4bab0+i*8), u32(0x4bab4+i*8)
		var trajectory [][2]int
		for n := 0; n < 4; n++ {
			trajectory = append(trajectory, [2]int{i16(motion + n*4), i16(motion + n*4 + 2)})
		}
		d.ProjectileMotion = append(d.ProjectileMotion, trajectory)
		var frames []int
		for n := 0; n < 64; n++ {
			frame := int(int8(memory[art+n]))
			if frame < 0 {
				break
			}
			frames = append(frames, frame)
		}
		d.ProjectileAnimations = append(d.ProjectileAnimations, frames)
		d.ProjectileVX = append(d.ProjectileVX, i16(0x4bb58+i*4))
		d.ProjectileVY = append(d.ProjectileVY, i16(0x4bb5a+i*4))
	}
	p := int(d.ProjectileDescriptor)
	size := u16(p)
	d.ProjectileWidth, d.ProjectileHeight = (size&63-1)*16, (size>>6)/4
	d.ProjectileFrameTicks = int(memory[p+7]) + 1
	frameCount := 0
	for _, animation := range d.ProjectileAnimations {
		for _, frame := range animation {
			if frame+1 > frameCount {
				frameCount = frame + 1
			}
		}
	}
	var err error
	d.ProjectileCollisions, err = collisionFrames(memory, u32(p+24), frameCount)
	if err != nil {
		panic(fmt.Sprintf("validated battleship projectile: %v", err))
	}
	event.CollisionFrames, _ = collisionFrames(memory, u32(int(event.Descriptor)+24), 7)
	event.Health = 8000
	event.BattleshipTurret = d
	event.Behavior = "battleship_respawning_turret"
	return true
}
