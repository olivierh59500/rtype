package source

import (
	"encoding/binary"
	"fmt"
)

// MapPatch is one original variable-size tile replacement rectangle.
type MapPatch struct {
	Column int      `json:"column"`
	Row    int      `json:"row"`
	Width  int      `json:"width"`
	Height int      `json:"height"`
	Words  []uint16 `json:"words"`
}

type ReactorPatchPair struct{ First, Second *MapPatch }

// BossThreeData preserves the battleship's separate controllers and deadline.
// The central core can request departure before that deadline expires.
type BossThreeData struct {
	Core                          *BattleshipCoreData     `json:"core,omitempty"`
	Iris                          *BattleshipIrisData     `json:"iris,omitempty"`
	ExitReload                    int                     `json:"exit_reload"`
	InitialPatches                []*MapPatch             `json:"initial_patches"`
	Mounted                       []BattleshipMountedData `json:"mounted"`
	MountedDescriptor             uint32                  `json:"mounted_descriptor"`
	MountedHealth                 int                     `json:"mounted_health"`
	MountedCollision              []CollisionFrame        `json:"mounted_collision"`
	AimMap                        []int                   `json:"aim_map"`
	AimThresholds, ShotVX, ShotVY []int
	ShotOffsets                   [][2]int `json:"shot_offsets"`
}

// BattleshipCoreData is declarative hull animation, core and attack data.
type BattleshipCoreData struct {
	Descriptor, ShotDescriptor                                            uint32
	AnimationTrigger, Trigger, ShotTrigger, X, Y, DamageLimit             int
	IdleReload, PatchReload, CycleHold, CycleWait, CycleCount, FireReload int
	IdlePatches, CyclePatches                                             []*MapPatch
	Collision, ShotCollision                                              []CollisionFrame
	ShotWidth, ShotHeight, ShotAnimationTicks                             int
	ShotAnimation                                                         []int
	ShotMotionY                                                           []int8
	ShotOffsetX, ShotOffsetY, ShotStartRange, ShotVXRange                 int
}

func extractBattleshipCore(memory []byte, mapAddress, rowBytes int) (*BattleshipCoreData, error) {
	u32 := func(a int) int { return int(binary.BigEndian.Uint32(memory[a:])) }
	d := &BattleshipCoreData{Descriptor: 0x4b7dc, ShotDescriptor: 0x4b80c,
		AnimationTrigger: 1520, Trigger: 2700, ShotTrigger: 2800, X: 122, Y: 98, DamageLimit: 10,
		IdleReload: 3, PatchReload: 2, CycleHold: 25, CycleWait: 125, CycleCount: 11, FireReload: 10,
		ShotOffsetX: -35, ShotOffsetY: -20, ShotStartRange: 7, ShotVXRange: 4}
	for index := 0; index < 4+2*d.CycleCount; index++ {
		patch, err := originalMapPatch(memory, u32(0x4b62e+index*4), mapAddress, rowBytes)
		if err != nil {
			return nil, fmt.Errorf("battleship core patch %d at %#x: %w", index, u32(0x4b62e+index*4), err)
		}
		if index < 4 {
			d.IdlePatches = append(d.IdlePatches, patch)
		} else {
			d.CyclePatches = append(d.CyclePatches, patch)
		}
	}
	for address := 0x6fca6; address < len(memory) && memory[address] != 127; address++ {
		if len(d.ShotMotionY) >= 256 {
			return nil, fmt.Errorf("battleship core shot motion is not bounded")
		}
		d.ShotMotionY = append(d.ShotMotionY, int8(memory[address]))
	}
	a := int(d.ShotDescriptor)
	size := int(binary.BigEndian.Uint16(memory[a:]))
	d.ShotWidth, d.ShotHeight = (size&63-1)*16, (size>>6)/4
	d.ShotAnimationTicks = int(memory[a+7]) + 1
	animation := u32(a + 20)
	frameCount := 0
	for index := 0; index < 64 && memory[animation+index] < 128; index++ {
		frame := int(memory[animation+index])
		d.ShotAnimation = append(d.ShotAnimation, frame)
		frameCount = max(frameCount, frame+1)
	}
	var err error
	d.Collision, err = collisionFrames(memory, u32(int(d.Descriptor)+24), 1)
	if err != nil {
		return nil, err
	}
	d.ShotCollision, err = collisionFrames(memory, u32(a+24), frameCount)
	return d, err
}

// BattleshipMountedData is one original twenty-byte map-mounted gun record.
type BattleshipMountedData struct {
	Trigger, X, Y, DamageLimit, DeathTicks, ShotVariant int
	InitialPatch, DeathPatch                            *MapPatch
}

// ReactorData describes the ship's independent reactor damage and escape rule.
type ReactorData struct {
	ShotWidth, ShotHeight, ShotAnimationTicks int
	ShotAnimation                             []int
	ShotCollision                             []CollisionFrame
	PatchPairs                                []ReactorPatchPair `json:"patch_pairs"`
	BlinkPatches                              []*MapPatch        `json:"blink_patches"`
	ClearPatches                              []*MapPatch        `json:"clear_patches"`
	PatchFirstWait                            int                `json:"patch_first_wait"`
	PatchCycleWait                            int                `json:"patch_cycle_wait"`
	Kind                                      int                `json:"kind"`
	DamageLimit                               int                `json:"damage_limit"`
	EscapeDamage                              int                `json:"escape_damage"`
	WaitTicks                                 int                `json:"wait_ticks"`
	DeathReload                               int                `json:"death_reload"`
	Score                                     int                `json:"score"`
	MotionX, MotionY                          []int8
	ShotOffsetsY, ShotVX, ShotVY              []int
	ShotDescriptor                            uint32 `json:"shot_descriptor"`
	ShotFirst, ShotPeriod                     int
}

func annotateReactor(memory []byte, event *Event, level int) bool {
	if level != 3 {
		return false
	}
	data := &ReactorData{DeathReload: 25, Score: 800}
	switch event.SpawnAddress {
	case 0x4ae26:
		data.Kind, data.DamageLimit, data.EscapeDamage, data.WaitTicks = 1, 30, 20, 1000
		event.Health = 3000
		for index := 0; index < 234; index++ {
			data.MotionX = append(data.MotionX, int8(memory[0x6fad0+index]))
			data.MotionY = append(data.MotionY, int8(memory[0x6fad0+235+index]))
		}
		data.ShotDescriptor = 0x4bcae
		data.PatchFirstWait, data.PatchCycleWait = 100, 75
		mapAddress := int(binary.BigEndian.Uint32(memory[0x4a30c:]))
		rowBytes := int(binary.BigEndian.Uint16(memory[0x41b4+2*10+8:]))
		for address := 0x4bc68; address < 0x4bc90; address += 8 {
			first := int(int32(binary.BigEndian.Uint32(memory[address:])))
			second := int(int32(binary.BigEndian.Uint32(memory[address+4:])))
			pair := ReactorPatchPair{}
			if first > 0 {
				pair.First, _ = originalMapPatch(memory, first, mapAddress, rowBytes)
			}
			if second > 0 {
				pair.Second, _ = originalMapPatch(memory, second, mapAddress, rowBytes)
			}
			data.PatchPairs = append(data.PatchPairs, pair)
			if second < 0 {
				break
			}
		}
		for _, address := range []int{0x4bc0e, 0x4bc2c} {
			patch, _ := originalMapPatch(memory, address, mapAddress, rowBytes)
			data.BlinkPatches = append(data.BlinkPatches, patch)
		}
		for _, address := range []int{0x4bbcc, 0x4bc4a} {
			patch, _ := originalMapPatch(memory, address, mapAddress, rowBytes)
			data.ClearPatches = append(data.ClearPatches, patch)
		}
		data.ShotFirst, data.ShotPeriod = 51, 36
		for index := 0; index < 3; index++ {
			address := 0x4bc9c + index*6
			data.ShotOffsetsY = append(data.ShotOffsetsY, int(int16(binary.BigEndian.Uint16(memory[address:]))))
			data.ShotVX = append(data.ShotVX, int(int16(binary.BigEndian.Uint16(memory[address+2:]))))
			data.ShotVY = append(data.ShotVY, int(int16(binary.BigEndian.Uint16(memory[address+4:]))))
		}
	case 0x4b08c:
		data.Kind, data.DamageLimit, data.WaitTicks = 2, 55, 550
		data.ShotDescriptor = 0x4bcea
		data.ShotFirst, data.ShotPeriod = 25, 25
		data.ShotOffsetsY = []int{19, 27}
		data.ShotVX = []int{-7, -7}
		data.ShotVY = []int{0, 0}
		event.Health = 2000
	default:
		return false
	}
	d := int(data.ShotDescriptor)
	size := int(binary.BigEndian.Uint16(memory[d:]))
	data.ShotWidth, data.ShotHeight = (size&63-1)*16, (size>>6)/4
	data.ShotAnimationTicks = int(memory[d+7]) + 1
	animation := int(binary.BigEndian.Uint32(memory[d+20:]))
	frameCount := 0
	for i := 0; i < 64; i++ {
		frame := int(int8(memory[animation+i]))
		if frame < 0 {
			break
		}
		data.ShotAnimation = append(data.ShotAnimation, frame)
		if frame+1 > frameCount {
			frameCount = frame + 1
		}
	}
	data.ShotCollision, _ = collisionFrames(memory, int(binary.BigEndian.Uint32(memory[d+24:])), frameCount)
	event.Reactor = data
	event.Behavior = "battleship_reactor"
	return true
}

func originalMapPatch(memory []byte, address, mapAddress, rowBytes int) (*MapPatch, error) {
	if address < 0 || address+6 > len(memory) || rowBytes < 2 {
		return nil, fmt.Errorf("map patch header is outside original data")
	}
	target := int(binary.BigEndian.Uint32(memory[address:]))
	width, height := int(memory[address+4])+1, int(memory[address+5])+1
	if width > 64 || height > 64 || address+6+width*height*2 > len(memory) {
		return nil, fmt.Errorf("map patch words are outside original data")
	}
	return &MapPatch{Column: (target - mapAddress) % rowBytes / 2, Row: (target - mapAddress) / rowBytes, Width: width, Height: height, Words: words(memory[address+6 : address+6+width*height*2])}, nil
}

func extractBossThree(memory []byte) (*Boss, error) {
	u32 := func(a int) int { return int(binary.BigEndian.Uint32(memory[a:])) }
	u16 := func(a int) int { return int(binary.BigEndian.Uint16(memory[a:])) }
	i16 := func(a int) int { return int(int16(binary.BigEndian.Uint16(memory[a:]))) }
	d := &BossThreeData{ExitReload: 0x15c7, MountedDescriptor: 0x4b5b8, MountedHealth: u16(0x4a7a4)}
	iris, irisErr := extractBattleshipIris(memory)
	if irisErr != nil {
		return nil, irisErr
	}
	d.Iris = iris
	mapAddress, rowBytes := u32(0x4a30c), u16(0x41b4+2*10+8)
	d.Core, irisErr = extractBattleshipCore(memory, mapAddress, rowBytes)
	if irisErr != nil {
		return nil, irisErr
	}
	patch := func(a int) (*MapPatch, error) { return originalMapPatch(memory, a, mapAddress, rowBytes) }
	for i := 0; i < 10; i++ {
		p, err := patch(u32(0x4b590 + i*4))
		if err != nil {
			return nil, err
		}
		d.InitialPatches = append(d.InitialPatches, p)
	}
	for a := 0x4b3dc; a < 0x4b46a; a += 20 {
		trigger := u16(a)
		if trigger == 0xffff {
			break
		}
		item := BattleshipMountedData{Trigger: trigger, X: i16(a + 2), Y: i16(a + 4), DamageLimit: u16(a+6) & 255, DeathTicks: u16(a+8) & 255, ShotVariant: u16(a+10) & 255}
		var err error
		item.InitialPatch, err = patch(u32(a + 12))
		if err != nil {
			return nil, err
		}
		item.DeathPatch, err = patch(u32(a + 16))
		if err != nil {
			return nil, err
		}
		d.Mounted = append(d.Mounted, item)
	}
	var err error
	d.MountedCollision, err = collisionFrames(memory, u32(int(d.MountedDescriptor)+24), 1)
	if err != nil {
		return nil, err
	}
	for i := 0; i < 7; i++ {
		d.ShotOffsets = append(d.ShotOffsets, [2]int{i16(0x4b5fa + i*4), i16(0x4b5fc + i*4)})
	}
	for i := 0; i < 16; i++ {
		d.ShotVX = append(d.ShotVX, i16(0x4b1a+i*4))
		d.ShotVY = append(d.ShotVY, i16(0x4b1c+i*4))
	}
	for _, v := range memory[0x4a1c:0x4a30] {
		d.AimMap = append(d.AimMap, int(v))
	}
	for i := 0; i < 5; i++ {
		d.AimThresholds = append(d.AimThresholds, u16(0x4a30+i*2))
	}
	return &Boss{Kind: "stage3_battleship", VictoryCounter: 36, VictoryReload: 2, VictoryTicks: 106, Third: d}, nil
}
