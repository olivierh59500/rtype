package source

import (
	"encoding/binary"
	"fmt"
)

// BossSevenLane preserves one independent ceiling opening and spawn clock.
type BossSevenLane struct {
	SpawnX       int         `json:"spawn_x"`
	InitialDelay int         `json:"initial_delay"`
	Patches      []*MapPatch `json:"patches"`
}

// BossSevenVariant identifies the original descending object and art phase.
type BossSevenVariant struct {
	Score          int    `json:"score"`
	Descriptor     uint32 `json:"descriptor"`
	Animation      []int  `json:"animation"`
	AnimationTicks int    `json:"animation_ticks"`
	InitialIndex   int    `json:"initial_index"`
	Animated       bool   `json:"animated"`
}

// BossSevenData contains declarative data for the resource-stage arena.
// The controller's stop value is an animation stream index, not an art frame.
type BossSevenData struct {
	Trigger, FreezeScroll, SpawnX, SpawnY, DamageLimit, Score                                      int
	Descriptor                                                                                     uint32
	ControllerHealth, GuardHealth                                                                  int
	InitialWait, ReturnWait, LeftVX, LeftLimit, RightVX, RightLimit                                int
	StopAnimationIndex, AnimationTicks                                                             int
	Animation                                                                                      []int
	InitialPatches                                                                                 []*MapPatch
	Lanes                                                                                          []BossSevenLane
	Variants                                                                                       []BossSevenVariant
	PatchReload, OpeningWaitBase, OpeningWaitRange                                                 int
	SpawnWaitBase, SpawnWaitRange, SpawnYTop, FallingVY, FallingLimit                              int
	GuardDescriptor                                                                                uint32
	GuardXBase, GuardXRange, GuardY, GuardRiseVY, GuardTopY                                        int
	GuardDamageLimit, GuardReturnWait, GuardInitialFire, GuardFireBase, GuardFireRange, GuardScore int
	GuardAnimation                                                                                 []int
	GuardMuzzle                                                                                    [][2]int
	AimMap, AimThresholds, ShotVX, ShotVY                                                          []int
	DeathSprite, VictorySprite                                                                     string
	DeathWidth, DeathHeight, DeathTicks, DeathSound                                                int
	VictoryWidth, VictoryHeight, VictoryTicks, VictorySound                                        int
	VictoryAnimation, DeathAnimation                                                               []int
}

func extractBossSeven(memory []byte, events []Event) (*Boss, error) {
	if len(memory) < 0x4c1ac {
		return nil, fmt.Errorf("stage-seven original ranges are unavailable")
	}
	u16 := func(a int) int { return int(binary.BigEndian.Uint16(memory[a:])) }
	i16 := func(a int) int { return int(int16(binary.BigEndian.Uint16(memory[a:]))) }
	u32 := func(a int) int { return int(binary.BigEndian.Uint32(memory[a:])) }
	animation := func(a int) []int {
		var out []int
		for i := 0; i < 128 && a+i < len(memory); i++ {
			v := int(int8(memory[a+i]))
			if v < 0 {
				break
			}
			out = append(out, v)
		}
		return out
	}
	d := &BossSevenData{Trigger: u16(0x4b1e6), FreezeScroll: u16(0x4b1f6), Descriptor: uint32(u32(0x4b2e8)), SpawnX: u16(0x4b2e0), SpawnY: int(memory[0x4b2dd]), DamageLimit: u16(0x4b354), Score: 500,
		InitialWait: u16(0x4b2f2), ReturnWait: u16(0x4b346), LeftVX: -2, LeftLimit: u16(0x4b318), RightVX: 2, RightLimit: u16(0x4b33e), StopAnimationIndex: u16(0x4b32e), PatchReload: int(memory[0x4b21d]), OpeningWaitBase: u16(0x4b24c), OpeningWaitRange: int(memory[0x4b245]), SpawnWaitBase: u16(0x4b26c), SpawnWaitRange: int(memory[0x4b265]), SpawnYTop: int(memory[0x4b28b]), FallingVY: 4, FallingLimit: u16(0x4b2ca),
		GuardDescriptor: uint32(u32(0x4b6ac)), GuardXBase: u16(0x4b71e), GuardXRange: int(memory[0x4b717]), GuardY: u16(0x4b6a4), GuardRiseVY: -2, GuardTopY: u16(0x4b786), GuardDamageLimit: u16(0x4b6c8), GuardReturnWait: int(memory[0x4b6f5]), GuardInitialFire: int(memory[0x4b6bf]), GuardFireBase: u16(0x4b76c), GuardFireRange: int(memory[0x4b765]), GuardScore: 500,
		DeathSprite: "object-7-0493a", VictorySprite: "object-7-049a8", DeathSound: 10, VictorySound: 12}
	if d.Trigger != 2110 || d.FreezeScroll != 2111 || d.Descriptor != 0x4bc70 || d.DamageLimit != 50 || d.GuardDescriptor != 0x4c0ea {
		return nil, fmt.Errorf("unsupported original stage-seven boss profile")
	}
	d.ControllerHealth, d.GuardHealth = u16(0x4b2e4), u16(0x4b6a8)
	d.Animation = animation(u32(int(d.Descriptor) + 20))
	d.AnimationTicks = int(memory[int(d.Descriptor)+7]) + 1
	d.GuardAnimation = animation(u32(int(d.GuardDescriptor) + 20))
	mapAddress, rowBytes := u32(0x4a30c), u16(0x41b4+6*10+8)
	patch := func(a, target int) (*MapPatch, error) {
		if a < 0 || a+2 > len(memory) || target < mapAddress {
			return nil, fmt.Errorf("invalid stage-seven patch")
		}
		width, height := int(memory[a])+1, int(memory[a+1])+1
		if width > 32 || height > 32 || a+2+width*height*2 > len(memory) {
			return nil, fmt.Errorf("unbounded stage-seven patch")
		}
		return &MapPatch{Column: (target - mapAddress) % rowBytes / 2, Row: (target - mapAddress) / rowBytes, Width: width, Height: height, Words: words(memory[a+2 : a+2+width*height*2])}, nil
	}
	for lane := 0; lane < 5; lane++ {
		a := 0x4bd44 + lane*12
		item := BossSevenLane{SpawnX: i16(a + 2), InitialDelay: 5 + lane*3}
		for frame := 0; frame < 4; frame++ {
			p, err := patch(u32(0x4bd80+frame*4), u32(a+4))
			if err != nil {
				return nil, err
			}
			item.Patches = append(item.Patches, p)
		}
		d.InitialPatches = append(d.InitialPatches, item.Patches[0])
		d.Lanes = append(d.Lanes, item)
		descriptor := u32(0x4bcce + lane*8)
		d.Variants = append(d.Variants, BossSevenVariant{Score: bcdScore(uint32(u32(descriptor + 36))), Descriptor: uint32(descriptor), Animation: animation(u32(descriptor + 20)), AnimationTicks: int(memory[descriptor+7]) + 1, InitialIndex: u16(0x4bcce + lane*8 + 6), Animated: descriptor == 0x4b8f6})
	}
	// Init 0x4B3AE installs each pulse's closed counterpart before scrolling.
	for _, event := range events {
		if event.Descriptor != 0x4be14 {
			continue
		}
		variant := int(event.Parameter & 0xffff)
		if variant < 0 || variant >= 6 {
			return nil, fmt.Errorf("invalid initial city pulse variant")
		}
		p, err := patch(u32(0x4bde4+0x18+variant*4), mapAddress+int(int16(event.Parameter>>16)))
		if err != nil {
			return nil, err
		}
		d.InitialPatches = append(d.InitialPatches, p)
	}
	for direction := 0; direction < 16; direction++ {
		d.GuardMuzzle = append(d.GuardMuzzle, [2]int{i16(0x4c16c + direction*4), i16(0x4c16e + direction*4)})
		d.ShotVX = append(d.ShotVX, i16(0x4b1a+direction*4))
		d.ShotVY = append(d.ShotVY, i16(0x4b1c+direction*4))
	}
	for _, v := range memory[0x4a1c:0x4a30] {
		d.AimMap = append(d.AimMap, int(v))
	}
	for i := 0; i < 5; i++ {
		d.AimThresholds = append(d.AimThresholds, u16(0x4a30+i*2))
	}
	for _, effect := range []struct {
		a       int
		victory bool
	}{{0x493a, false}, {0x49a8, true}} {
		size := u16(effect.a)
		frames := animation(u32(effect.a + 20))
		if effect.victory {
			d.VictoryWidth, d.VictoryHeight, d.VictoryTicks, d.VictoryAnimation = (size&63-1)*16, (size>>6)/4, len(frames), frames
		} else {
			d.DeathWidth, d.DeathHeight, d.DeathTicks, d.DeathAnimation = (size&63-1)*16, (size>>6)/4, len(frames), frames
		}
	}
	return &Boss{Kind: "stage7_resource_arena", Seven: d, VictoryCounter: 36, VictoryReload: 2, VictoryTicks: 106}, nil
}
