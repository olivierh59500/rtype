package source

import (
	"encoding/binary"
	"fmt"
)

// BossEightPatch is a normalized original tile-animation script command.
// Target indexes another command; Repeat preserves its byte-sized loop count.
type BossEightPatch struct {
	Patch     *MapPatch `json:"patch,omitempty"`
	Target    int       `json:"target"`
	Repeat    int       `json:"repeat"`
	Secondary bool      `json:"secondary"`
	Shot      bool      `json:"shot"`
	Stop      bool      `json:"stop"`
}

// BossEightMessage preserves one original ending-fleet ship and its delay.
type BossEightMessage struct {
	Y, Frame, VX, Delay int
}

// BossEightData contains the final Bydo arena, map animations and fleet data.
// Original instruction addresses serve only as extraction provenance.
type BossEightData struct {
	AnimationScroll, BrainScroll, FreezeScroll                                                   int
	BrainDescriptor, CellDescriptor, CellExplosionDescriptor, SporeDescriptor, MessageDescriptor uint32
	BrainX, BrainY, BrainHP, DamageLimit, Score                                                  int
	BrainCollision                                                                               CollisionFrame
	InitialPatches                                                                               []*MapPatch
	Primary, Secondary                                                                           []BossEightPatch
	DestroyedPatch                                                                               *MapPatch
	CellVelocity, SporeVelocity                                                                  [][2]int
	CellAnimation                                                                                [][]int
	CellExplosionAnimation                                                                       []int
	CellExplosionTicks                                                                           int
	CellAnimationTicks, CellExplosionStopIndex, SporeAnimationTicks                              int
	SporeAnimation                                                                               []int
	CellFirst, CellReload, SporeReload                                                           int
	Palette, EndingPalette                                                                       []uint16
	Messages                                                                                     []BossEightMessage
}

func extractBossEight(memory []byte) (*Boss, error) {
	if len(memory) < 0x4b068 {
		return nil, fmt.Errorf("stage-eight original data is unavailable")
	}
	u16 := func(a int) int { return int(binary.BigEndian.Uint16(memory[a:])) }
	i16 := func(a int) int { return int(int16(binary.BigEndian.Uint16(memory[a:]))) }
	u32 := func(a int) int { return int(binary.BigEndian.Uint32(memory[a:])) }
	d := &BossEightData{AnimationScroll: u16(0x4a434), BrainScroll: u16(0x4a43a), FreezeScroll: u16(0x4a464), BrainDescriptor: uint32(u32(0x4a44a)), BrainX: u16(0x4a440), BrainY: int(memory[0x4a443]), DamageLimit: u16(0x4a53e), Score: 15000, CellDescriptor: 0x4aa78, CellExplosionDescriptor: 0x4ac04, SporeDescriptor: 0x4b020, MessageDescriptor: 0x4aa4c, CellFirst: 100, CellReload: 26, SporeReload: 6}
	d.BrainHP = u16(0x4a446)
	if d.BrainScroll != 1613 || d.FreezeScroll != 1615 || d.DamageLimit != 60 || d.BrainDescriptor != 0x4afe0 {
		return nil, fmt.Errorf("unsupported original final boss profile")
	}
	d.BrainCollision = CollisionFrame{X: i16(0x4b008), Y: i16(0x4b00a), Width: u16(0x4b00c), Height: u16(0x4b00e)}
	mapAddress, rowBytes := u32(0x4a30c), u16(0x41b4+7*10+8)
	patch := func(a int) (*MapPatch, error) { return originalMapPatch(memory, a, mapAddress, rowBytes) }
	for _, a := range []int{0x4ad80, 0x4ae3e} {
		p, e := patch(a)
		if e != nil {
			return nil, e
		}
		d.InitialPatches = append(d.InitialPatches, p)
	}
	var err error
	d.DestroyedPatch, err = patch(0x4aefc)
	if err != nil {
		return nil, err
	}
	program := func(start, end int) ([]BossEightPatch, error) {
		var out []BossEightPatch
		positions := map[int]int{}
		targets := map[int]int{}
		for a := start; a < end; {
			positions[a] = len(out)
			word := uint32(u32(a))
			a += 4
			step := BossEightPatch{Target: -1}
			if word&0x80000000 == 0 {
				step.Shot = word&0x40000000 != 0
				p, e := patch(int(word & 0x3fffffff))
				if e != nil {
					return nil, e
				}
				step.Patch = p
			} else if word&0x40000000 != 0 {
				targets[len(out)] = u32(a)
				a += 4
			} else if byte(word) == 0 {
				step.Stop = true
			} else {
				step.Repeat = int(byte(word >> 8))
				step.Secondary = true
				targets[len(out)] = u32(a)
				a += 4
			}
			out = append(out, step)
		}
		for index, address := range targets {
			target, ok := positions[address]
			if !ok {
				return nil, fmt.Errorf("final map script target %#x is outside its declarative program", address)
			}
			out[index].Target = target
		}
		return out, nil
	}
	d.Primary, err = program(0x4ac7e, 0x4acde)
	if err != nil {
		return nil, err
	}
	d.Secondary, err = program(0x4acde, 0x4ad0e)
	if err != nil {
		return nil, err
	}
	// Setup sets bit thirty on the fifth secondary pointer before playback.
	if len(d.Secondary) > 4 {
		d.Secondary[4].Shot = true
	}
	animation := func(a int) []int {
		var out []int
		for n := 0; n < 64; n++ {
			v := int(int8(memory[a+n]))
			if v < 0 {
				break
			}
			out = append(out, v)
		}
		return out
	}
	for index := 0; index < 16; index++ {
		d.CellVelocity = append(d.CellVelocity, [2]int{i16(0x4ab84 + index*4), i16(0x4ab86 + index*4)})
		pointer := u32(0x4abc4 + index*4)
		if pointer > 0 {
			d.CellAnimation = append(d.CellAnimation, animation(pointer))
		} else {
			d.CellAnimation = append(d.CellAnimation, nil)
		}
		d.SporeVelocity = append(d.SporeVelocity, [2]int{i16(0x4a972 + index*4), i16(0x4a974 + index*4)})
	}
	d.CellExplosionAnimation = animation(0x4ac2c)
	// Descriptor delays are predecremented; cell retirement tests the next
	// animation index, rather than waiting for the explosion stream sentinel.
	d.CellAnimationTicks = int(memory[d.CellDescriptor+7]) + 1
	d.CellExplosionTicks = int(memory[d.CellExplosionDescriptor+7]) + 1
	d.CellExplosionStopIndex = u16(0x4a684)
	d.SporeAnimationTicks = int(memory[d.SporeDescriptor+7]) + 1
	d.SporeAnimation = animation(u32(int(d.SporeDescriptor) + 20))
	d.Palette = append([]uint16{0}, words(memory[0x4a3b0:0x4a3ce])...)
	d.EndingPalette = words(memory[0x4afc2:0x4afe0])
	for a := 0x4a9b2; a+8 <= 0x4aa4c; a += 8 {
		if i16(a) < 0 {
			break
		}
		d.Messages = append(d.Messages, BossEightMessage{Y: u16(a), Frame: u16(a + 2), VX: i16(a + 4), Delay: u16(a + 6)})
	}
	return &Boss{Kind: "stage8_bydo", Eighth: d, ControllerHealth: d.DamageLimit, Score: d.Score}, nil
}
