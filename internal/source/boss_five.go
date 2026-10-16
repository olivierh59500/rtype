package source

import (
	"encoding/binary"
	"fmt"
)

// BossFiveData preserves the controller's integer movement and eight arms.
// All direction, bounce and attack-selection tables come from the original.
type BossFiveData struct {
	Trigger, X, Y                                      int
	Descriptor, PartDescriptor                         uint32
	DamageLimit, PartDamageLimit, Score, PartScore     int
	CoreHealth, PartHealth, CoreDeathKind              int
	PartAnimation                                      []int
	PartAnimationTicks                                 int
	IntroReload, IntroVX                               int
	VelocityX, VelocityY, PartVelocityX, PartVelocityY []int
	BounceDirections                                   []int
	Offsets                                            [][2]int
	AttackOrder                                        [][]int
	MoveMinX, MoveMaxX, MoveMinY, MoveMaxY             int
	PartMinX, PartMaxX, PartMinY, PartMaxY             int
	LoopStart, LoopEnd                                 int
	AimMap                                             []int
	AimThresholds                                      []int
}

func extractBossFive(memory []byte, events []Event) (*Boss, error) {
	u16 := func(a int) int { return int(binary.BigEndian.Uint16(memory[a:])) }
	i16 := func(a int) int { return int(int16(binary.BigEndian.Uint16(memory[a:]))) }
	d := &BossFiveData{PartDescriptor: 0x4b628, DamageLimit: 31, PartDamageLimit: 20, Score: 2000, PartScore: 300, IntroReload: 40, IntroVX: -2, MoveMinX: 48, MoveMaxX: 302, MoveMinY: 48, MoveMaxY: 172, PartMinX: 48, PartMaxX: 328, PartMinY: 48, PartMaxY: 208, LoopStart: 0xb40, LoopEnd: 0xbc0}
	d.CoreHealth, d.PartHealth, d.CoreDeathKind = u16(0x4ad10), u16(0x4ad5c), int(memory[0x4ad31])
	d.PartAnimationTicks = int(memory[d.PartDescriptor+7]) + 1
	animation := int(binary.BigEndian.Uint32(memory[int(d.PartDescriptor)+20:]))
	for index := 0; index < 64 && memory[animation+index] < 128; index++ {
		d.PartAnimation = append(d.PartAnimation, int(memory[animation+index]))
	}
	for _, event := range events {
		if event.SpawnAddress == 0x4ad00 {
			d.Trigger, d.X, d.Y, d.Descriptor = event.Trigger, event.X, event.Y, event.Descriptor
			break
		}
	}
	if d.Descriptor != 0x4b5f8 || d.Trigger != 2752 {
		return nil, fmt.Errorf("unsupported original stage-five boss profile")
	}
	for direction := 0; direction < 16; direction++ {
		d.VelocityX = append(d.VelocityX, i16(0x4b598+direction*4))
		d.VelocityY = append(d.VelocityY, i16(0x4b59a+direction*4))
		d.PartVelocityX = append(d.PartVelocityX, i16(0x4b558+direction*4))
		d.PartVelocityY = append(d.PartVelocityY, i16(0x4b55a+direction*4))
		d.BounceDirections = append(d.BounceDirections, int(memory[0x4b4e4+direction]))
	}
	for arm := 0; arm < 8; arm++ {
		d.Offsets = append(d.Offsets, [2]int{i16(0x4b4f4 + arm*8), i16(0x4b4f6 + arm*8)})
	}
	for quadrant := 0; quadrant < 4; quadrant++ {
		var order []int
		for i := 0; i < 7; i++ {
			order = append(order, int(memory[0x4b5d8+quadrant*8+i]))
		}
		d.AttackOrder = append(d.AttackOrder, order)
	}
	for _, v := range memory[0x4a1c:0x4a30] {
		d.AimMap = append(d.AimMap, int(v))
	}
	for i := 0; i < 5; i++ {
		d.AimThresholds = append(d.AimThresholds, u16(0x4a30+i*2))
	}
	return &Boss{Kind: "stage5_bellmite", Fifth: d, VictoryCounter: 36, VictoryReload: 2, VictoryTicks: 106}, nil
}
