package source

import (
	"bytes"
	"encoding/binary"
)

// annotateEvent follows original data references for behavior families whose
// layouts have been independently verified in the disassembly. Opcode prefixes
// only identify an offline data schema; no instruction is executed or emulated.
func annotateEvent(memory []byte, event *Event, start, end, level int) {
	if annotateSpecial(memory, event, level) {
		return
	}
	if annotateMiniTail(memory, event, level) || annotateDoubleSpitter(memory, event, level) {
		return
	}
	if annotateFireWorm(memory, event, level) {
		return
	}
	if annotateCarrier(memory, event) {
		return
	}
	u32 := func(address int) int { return int(binary.BigEndian.Uint32(memory[address:])) }
	u16 := func(address int) int { return int(binary.BigEndian.Uint16(memory[address:])) }
	i16 := func(address int) int { return int(int16(binary.BigEndian.Uint16(memory[address:]))) }
	spawn, update := int(event.SpawnAddress), int(event.UpdateAddress)
	if spawn < start || spawn+64 > end || update < start || update+64 > end {
		return
	}
	if annotateFamilies(memory, event, start, end, level) {
		return
	}
	if annotateGroundGunner(memory, event, start, end, level) {
		return
	}
	event.Variant = int(event.Parameter >> 16)
	formationPrefix := []byte{0x26, 0x69, 0, 6, 0x32, 0x1b, 0x6b, 0x1c}
	formationUpdate := []byte{0x4a, 0x28, 0, 0x32, 0x67, 6, 0x53, 0x28, 0, 0x32, 0x4e, 0x75, 0x43, 0xf9}
	if bytes.HasPrefix(memory[spawn:], formationPrefix) && bytes.HasPrefix(memory[update:], formationUpdate) && bytes.Equal(memory[update+34:update+36], []byte{0xb0, 0x7c}) {
		motionAddress, length := u32(update+14), u16(update+36)
		if length <= 0 || length > 256 || motionAddress < 0x4d300 || motionAddress+length > len(memory) {
			return
		}
		event.Behavior, event.MotionLoop = "formation_sine", true
		parameter := int(event.Parameter)
		for index := 0; index < 64 && parameter >= start && parameter+4 <= end; index++ {
			y := i16(parameter)
			if y < 0 {
				break
			}
			event.Formation = append(event.Formation, Formation{Y: y, Delay: int(memory[parameter+2]), Animation: int(memory[parameter+3])})
			parameter += 4
		}
		for _, value := range memory[motionAddress : motionAddress+length] {
			event.MotionY = append(event.MotionY, int(int8(value)))
		}
		return
	}
	trajectoryPrefix := []byte{0x3e, 0x29, 0, 0x0a, 0x42, 0x46, 0x30, 0x29, 0, 2, 0x32, 0x29, 0, 4, 0x74, 1}
	trajectory := bytes.HasPrefix(memory[spawn:], trajectoryPrefix)
	if level == 7 && spawn == 0x4a98e {
		// This verified variant chooses seven or ten ticks between group members.
		trajectory = true
		event.SpawnDelay = 7
		if event.Count > 9 {
			event.SpawnDelay = 10
		}
	}
	if trajectory {
		parameter := int(event.Parameter)
		if parameter >= start && parameter+14 <= end {
			length := u16(parameter) + 1
			xAddress, yAddress, animationAddress := u32(parameter+2), u32(parameter+6), u32(parameter+10)
			if length < 4096 && xAddress > 0 && xAddress+length <= len(memory) && yAddress > 0 && yAddress+length <= len(memory) && animationAddress > 0 && animationAddress+length <= len(memory) {
				event.Behavior = "motion"
				if event.SpawnDelay == 0 {
					event.SpawnDelay = 7
				}
				for index := 0; index < length; index++ {
					event.MotionX = append(event.MotionX, int(int8(memory[xAddress+index])))
					event.MotionY = append(event.MotionY, int(int8(memory[yAddress+index])))
					event.MotionAnimation = append(event.MotionAnimation, int(memory[animationAddress+index]))
				}
			}
		}
		return
	}
	if level == 2 && spawn == 0x4a538 && event.Variant < 4 {
		event.Behavior = "approach_lunge"
		event.LungeVX, event.LungeVY = i16(0x4af14+event.Variant*4), i16(0x4af16+event.Variant*4)
		animation := u32(int(event.Descriptor)+20) + event.Variant*3
		event.Animation = nil
		for index := 0; index < 32 && animation+index < end; index++ {
			frame := int(int8(memory[animation+index]))
			if frame < 0 {
				break
			}
			event.Animation = append(event.Animation, frame)
		}
		return
	}
	if level == 3 && spawn == 0x4ab40 {
		event.Behavior, event.RelativeScroll = "mounted_turret", true
		event.Health = int(int16(uint16(event.Parameter)))
		event.ShootMin, event.ShootMax = 31, 80
		animation := u32(int(event.Descriptor) + 20)
		if event.Parameter>>16 != 0 {
			animation = 0x4b892
		}
		if animation >= start && animation+16 <= end {
			for _, frame := range memory[animation : animation+16] {
				event.AimAnimation = append(event.AimAnimation, int(frame))
			}
		}
		for index := 0; index < 8; index++ {
			event.AimOffsetsX = append(event.AimOffsetsX, i16(0x4b8e2+index*4))
			event.AimOffsetsY = append(event.AimOffsetsY, i16(0x4b8e4+index*4))
		}
		// These are two fixed sixteen-direction banks, not a sentinel-
		// terminated animation. Scanning onward consumes collision words.
		event.Animation = append([]int(nil), event.AimAnimation...)
		for index := 0; index < 16; index++ {
			event.ShotVX = append(event.ShotVX, i16(0x4b1a+index*4))
			event.ShotVY = append(event.ShotVY, i16(0x4b1c+index*4))
		}
		for _, value := range memory[0x4a1c:0x4a30] {
			event.AimMap = append(event.AimMap, int(value))
		}
		for index := 0; index < 5; index++ {
			event.AimThresholds = append(event.AimThresholds, u16(0x4a30+index*2))
		}
		return
	}
	if (level == 5 && spawn == 0x4ab76) || (level == 8 && spawn == 0x4a8be) {
		event.Behavior = "homing"
		velocity := 0x4b394
		event.TurnTicks, event.HomingTurns = 5, 100
		if level == 8 {
			velocity = 0x4a972
			event.TurnTicks, event.HomingTurns = u16(0x4b064)+1, 0
			event.Health = -1
		}
		for index := 0; index < 16; index++ {
			event.MotionX = append(event.MotionX, i16(velocity+index*4))
			event.MotionY = append(event.MotionY, i16(velocity+2+index*4))
		}
		for _, value := range memory[0x4a1c:0x4a30] {
			event.AimMap = append(event.AimMap, int(value))
		}
		for index := 0; index < 5; index++ {
			event.AimThresholds = append(event.AimThresholds, u16(0x4a30+index*2))
		}
		return
	}
	if level == 6 && spawn == 0x4ac70 && event.Variant < 4 {
		event.Behavior, event.MotionLoop, event.RelativeScroll = "motion_single", true, true
		period, direction := int(event.Parameter&0xffff)+1, event.Variant
		if period > 256 {
			return
		}
		initial := direction
		for phase := 0; phase < 4; phase++ {
			direction = u16(0x4aede + direction*8 + 6)
			if direction > 3 {
				break
			}
			animationAddress := u32(0x4aede + direction*8)
			var animation []int
			for index := 0; index < 32 && animationAddress >= start && animationAddress+index < end; index++ {
				frame := int(int8(memory[animationAddress+index]))
				if frame < 0 {
					break
				}
				animation = append(animation, frame)
			}
			for tick := 0; tick < period; tick++ {
				event.MotionX = append(event.MotionX, i16(0x4aece+direction*4))
				event.MotionY = append(event.MotionY, i16(0x4aed0+direction*4))
				frame := 0
				if len(animation) > 0 {
					frame = animation[(tick/max(1, event.AnimationTicks))%len(animation)]
				}
				event.MotionAnimation = append(event.MotionAnimation, frame)
			}
			if direction == initial {
				break
			}
		}
	}
}
