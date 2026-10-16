package source

import (
	"encoding/binary"
	"fmt"
)

// IrisPatchFrame preserves one original map replacement and timer reload.
type IrisPatchFrame struct {
	Patch  *MapPatch `json:"patch,omitempty"`
	Reload int       `json:"reload"`
	Stop   bool      `json:"stop,omitempty"`
}

// IrisDamageStep preserves a damage gate, patch or blink choice, and debris.
type IrisDamageStep struct {
	Damage, Explosions, RangeX, OffsetX int
	Patch                               *MapPatch
	Flash                               int
}

// BattleshipIrisData is the original map-backed weak point, independent of
// the battleship's unconditional stage timer and its mounted guns.
type BattleshipIrisData struct {
	Trigger, X, Y, DamageStart, InitialCollisionPhase               int
	Health                                                          int
	Descriptor                                                      uint32
	Animation                                                       []int
	Collision                                                       []CollisionFrame
	InitialPatch                                                    *MapPatch
	Opening                                                         []IrisPatchFrame
	DamageSteps                                                     []IrisDamageStep
	Flash                                                           [][]IrisPatchFrame
	ExplosionYRange, ExplosionYOffset, ExplosionScore               int
	ExplosionSprite                                                 string
	ExplosionWidth, ExplosionHeight, ExplosionTicks, ExplosionSound int
	ExplosionAnimation                                              []int
}

func extractBattleshipIris(memory []byte) (*BattleshipIrisData, error) {
	if len(memory) < 0x4c38a {
		return nil, fmt.Errorf("battleship iris header is unavailable")
	}
	u16 := func(a int) int { return int(binary.BigEndian.Uint16(memory[a:])) }
	i16 := func(a int) int { return int(int16(binary.BigEndian.Uint16(memory[a:]))) }
	u32 := func(a int) int { return int(binary.BigEndian.Uint32(memory[a:])) }
	d := &BattleshipIrisData{Trigger: u16(0x4b21c), X: u16(0x4b228), Y: u16(0x4b22c), Health: u16(0x4b230), Descriptor: uint32(u32(0x4b234)), DamageStart: u16(0x4b258), InitialCollisionPhase: i16(0x4b1f0), ExplosionYRange: int(memory[0x4b30f]), ExplosionYOffset: -u16(0x4b31e), ExplosionScore: 600, ExplosionSprite: "object-3-0493a", ExplosionSound: 10}
	if d.Trigger != 1400 || d.Descriptor != 0x4c33c || d.DamageStart != 80 {
		return nil, fmt.Errorf("unsupported original battleship iris profile")
	}
	mapAddress, rowBytes := u32(0x4a30c), u16(0x41b4+2*10+8)
	patch := func(a int) (*MapPatch, error) { return originalMapPatch(memory, a, mapAddress, rowBytes) }
	var err error
	d.InitialPatch, err = patch(0x4c012)
	if err != nil {
		return nil, err
	}
	for a := 0x4be0a; a+8 <= 0x4c012; a += 8 {
		reload := int(int32(binary.BigEndian.Uint32(memory[a+4:])))
		frame := IrisPatchFrame{Reload: reload, Stop: reload < 0}
		if !frame.Stop {
			frame.Patch, err = patch(u32(a))
			if err != nil {
				return nil, err
			}
		}
		d.Opening = append(d.Opening, frame)
		if frame.Stop {
			break
		}
	}
	if len(d.Opening) == 0 || !d.Opening[len(d.Opening)-1].Stop {
		return nil, fmt.Errorf("iris opening has no end marker")
	}
	for i := 0; i < 16; i++ {
		a := 0x4bd6a + i*12
		threshold := int(int32(binary.BigEndian.Uint32(memory[a:])))
		if threshold < 0 {
			break
		}
		step := IrisDamageStep{Damage: threshold, Explosions: u16(a + 4), RangeX: u16(a + 6), OffsetX: i16(0x4bdbe + (i+1)*2)}
		pointer := int(int32(binary.BigEndian.Uint32(memory[a+8:])))
		if pointer < 0 {
			step.Flash = -pointer
		} else {
			step.Patch, err = patch(pointer)
			if err != nil {
				return nil, err
			}
		}
		d.DamageSteps = append(d.DamageSteps, step)
	}
	for _, start := range []int{0x4bdce, 0x4bdec} {
		var frames []IrisPatchFrame
		for i := 0; i < 5; i++ {
			p, e := patch(u32(start + i*6))
			if e != nil {
				return nil, e
			}
			frames = append(frames, IrisPatchFrame{Patch: p, Reload: u16(start + i*6 + 4)})
		}
		d.Flash = append(d.Flash, frames)
	}
	anim := u32(int(d.Descriptor) + 20)
	for i := 0; i < 32; i++ {
		v := int(int8(memory[anim+i]))
		if v < 0 {
			break
		}
		d.Animation = append(d.Animation, v)
	}
	d.Collision, err = collisionFrames(memory, u32(int(d.Descriptor)+24), 4)
	if err != nil {
		return nil, err
	}
	size := u16(0x493a)
	d.ExplosionWidth, d.ExplosionHeight = (size&63-1)*16, (size>>6)/4
	anim = u32(0x493a + 20)
	for i := 0; i < 64; i++ {
		v := int(int8(memory[anim+i]))
		if v < 0 {
			break
		}
		d.ExplosionAnimation = append(d.ExplosionAnimation, v)
	}
	d.ExplosionTicks = len(d.ExplosionAnimation)
	return d, nil
}
