package source

import (
	"encoding/binary"
	"fmt"
)

// SpecialEffectData describes an original common death family.
type SpecialEffectData struct {
	Sprite                      string
	Width, Height, Ticks, Sound int
	Animation                   []int
}
type HuntingEmitterData struct {
	NearLow, NearHigh, NearRange, TargetYRange, TargetYOffset, MaxTargetY                                  int
	PositiveVX, NegativeVX, VY, MaxTargetX, RestRange, RepeatBase, RepeatRange, EscapeReload               int
	ProjectileDescriptor                                                                                   uint32
	ProjectileSprite                                                                                       string
	ProjectileWidth, ProjectileHeight, ProjectileTicks, ProjectileVX, ProjectileOffsetX, ProjectileOffsetY int
	ProjectileAnimation                                                                                    []int
	ProjectileCollision                                                                                    []CollisionFrame
	Death                                                                                                  SpecialEffectData
}
type ShieldVehicleData struct {
	TravelReload, DamageLimit, DebrisReload, Score           int
	DebrisXRange, DebrisYRange, DebrisOffsetX, DebrisOffsetY int
	Death                                                    SpecialEffectData
}
type SpiralSentinelData struct {
	Count, BaseX, BaseY, SpawnX, SpawnY, Head, DelayStep, BreakInitial, BreakStep, StopX, DamageLimit, BrokenIndex int
	MotionX, MotionY                                                                                               []int8
	Body, HeadStream                                                                                               []int
	ShotNear, ShotMin, ShotRange                                                                                   int
	AimMap, AimThresholds, ShotVX, ShotVY                                                                          []int
	Death                                                                                                          SpecialEffectData
}

func specialEffect(memory []byte, level, address, sound int) SpecialEffectData {
	u16 := func(a int) int { return int(binary.BigEndian.Uint16(memory[a:])) }
	p := int(binary.BigEndian.Uint32(memory[address+20:]))
	d := SpecialEffectData{Sprite: fmt.Sprintf("object-%d-%05x", level, address), Width: (u16(address)&63 - 1) * 16, Height: (u16(address) >> 6) / 4, Sound: sound}
	for i := 0; i < 64; i++ {
		v := int(int8(memory[p+i]))
		if v < 0 {
			break
		}
		d.Animation = append(d.Animation, v)
	}
	d.Ticks = len(d.Animation)
	return d
}

func annotateSpecial(memory []byte, event *Event, level int) bool {
	u16 := func(a int) int { return int(binary.BigEndian.Uint16(memory[a:])) }
	i16 := func(a int) int { return int(int16(binary.BigEndian.Uint16(memory[a:]))) }
	u32 := func(a int) int { return int(binary.BigEndian.Uint32(memory[a:])) }
	if (level == 1 && event.SpawnAddress == 0x4abf0) || (level == 7 && event.SpawnAddress == 0x4acd8) {
		update := int(event.UpdateAddress)
		d := &HuntingEmitterData{NearLow: 15, NearHigh: 5, NearRange: 50, TargetYRange: 59, TargetYOffset: -30, MaxTargetY: 176, PositiveVX: 2, NegativeVX: -2, VY: 2, MaxTargetX: 290, RestRange: 25, RepeatBase: 1, RepeatRange: 2, EscapeReload: 125, ProjectileDescriptor: uint32(u32(update + 0xd6)), ProjectileVX: -8, ProjectileOffsetX: -40, ProjectileOffsetY: 10, Death: specialEffect(memory, level, 0x49a8, 12)}
		p := int(d.ProjectileDescriptor)
		d.ProjectileSprite = fmt.Sprintf("object-%d-%05x", level, p)
		size := u16(p)
		d.ProjectileWidth, d.ProjectileHeight = (size&63-1)*16, (size>>6)/4
		d.ProjectileTicks = int(memory[p+7]) + 1
		anim := u32(p + 20)
		for i := 0; i < 32; i++ {
			v := int(int8(memory[anim+i]))
			if v < 0 {
				break
			}
			d.ProjectileAnimation = append(d.ProjectileAnimation, v)
		}
		d.ProjectileCollision, _ = collisionFrames(memory, u32(p+24), 2)
		event.CollisionFrames, _ = collisionFrames(memory, u32(int(event.Descriptor)+24), 3)
		event.Animation = []int{0, 1, 2}
		event.Behavior, event.HuntingEmitter, event.Health = "hunting_emitter", d, 30
		return true
	}
	if level == 7 && event.SpawnAddress == 0x4b534 {
		event.ShieldVehicle = &ShieldVehicleData{TravelReload: u16(0x4b54c), DamageLimit: u16(0x4b57c), DebrisReload: 16, Score: 1500, DebrisXRange: u16(0x4b5c6), DebrisYRange: int(memory[0x4b5b5]), DebrisOffsetX: -15, DebrisOffsetY: -10, Death: specialEffect(memory, level, 0x493a, 10)}
		event.Health, event.Behavior = -1, "shield_vehicle"
		return true
	}
	if level == 1 && event.SpawnAddress == 0x4b080 {
		d := &SpiralSentinelData{Count: 12, BaseX: u16(0x4b082), BaseY: 115, SpawnX: u16(0x4b096), SpawnY: u16(0x4b09a), Head: 8, DelayStep: u16(0x4b0ba), BreakInitial: int(memory[0x4b08f]), BreakStep: 2, StopX: i16(0x4b1ce), DamageLimit: u16(0x4b1e0), BrokenIndex: u16(0x4b1b8), ShotNear: 70, ShotMin: 25, ShotRange: 175, Death: specialEffect(memory, level, 0x493a, 10)}
		for i := 0; i < 0x127; i++ {
			y := int8(memory[0x793f8+i])
			if y == 127 {
				break
			}
			d.MotionX = append(d.MotionX, int8(memory[0x792d1+i]))
			d.MotionY = append(d.MotionY, y)
		}
		for _, item := range []struct {
			address int
			out     *[]int
		}{{0x4bf2c, &d.Body}, {0x4bf37, &d.HeadStream}} {
			for i := 0; i < 11; i++ {
				*item.out = append(*item.out, int(int8(memory[item.address+i])))
			}
		}
		for _, v := range memory[0x4a1c:0x4a30] {
			d.AimMap = append(d.AimMap, int(v))
		}
		for i := 0; i < 5; i++ {
			d.AimThresholds = append(d.AimThresholds, u16(0x4a30+i*2))
		}
		for i := 0; i < 16; i++ {
			d.ShotVX = append(d.ShotVX, i16(0x4b1a+i*4))
			d.ShotVY = append(d.ShotVY, i16(0x4b1c+i*4))
		}
		event.CollisionFrames, _ = collisionFrames(memory, u32(int(event.Descriptor)+24), 17)
		event.Behavior, event.SpiralSentinel, event.Health = "spiral_sentinel", d, -1
		return true
	}
	return false
}
