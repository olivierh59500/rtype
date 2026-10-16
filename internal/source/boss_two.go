package source

import (
	"encoding/binary"
	"fmt"
)

// BossTwoData preserves the original organ's tile phases and eleven delayed
// route-following components. All array contents are reconstructed from ADF.
type BossTwoData struct {
	DeathDelays                                                      []int `json:"death_delays"`
	FlashIndices                                                     []int `json:"flash_indices"`
	FlashValues, NormalValues                                        []uint16
	ExplosionXRange, ExplosionYRange, ExplosionXBase, ExplosionYBase int
	ExplosionKinds                                                   []int              `json:"explosion_kinds"`
	Trigger                                                          int                `json:"trigger"`
	Components                                                       []BossTwoComponent `json:"components"`
	Routes                                                           []BossTwoRoute     `json:"routes"`
	DelayStep                                                        int                `json:"delay_step"`
	RouteWait                                                        int                `json:"route_wait"`
	PartHitScore                                                     int                `json:"part_hit_score"`
	DamagedAnimation                                                 []int              `json:"damaged_animation"`
	ControllerX                                                      int                `json:"controller_x"`
	ControllerY                                                      int                `json:"controller_y"`
	ControllerCollisions                                             []CollisionFrame   `json:"controller_collisions"`
	PatchColumn                                                      int                `json:"patch_column"`
	PatchRow                                                         int                `json:"patch_row"`
	Patches                                                          [][]uint16         `json:"patches"`
	ClosedWait                                                       int                `json:"closed_wait"`
	WaitReduction                                                    int                `json:"wait_reduction"`
	MinimumWait                                                      int                `json:"minimum_wait"`
	PatchReload                                                      int                `json:"patch_reload"`
	OpenHold                                                         int                `json:"open_hold"`
}

type BossTwoComponent struct {
	Descriptor     uint32 `json:"descriptor"`
	Animation      []int  `json:"animation"`
	AnimationTicks int    `json:"animation_ticks"`
	Animated       bool   `json:"animated"`
}

type BossTwoRoute struct {
	StartX  int    `json:"start_x"`
	StartY  int    `json:"start_y"`
	MotionX []int8 `json:"motion_x"`
	MotionY []int8 `json:"motion_y"`
	Frames  []int  `json:"frames"`
}

func extractBossTwo(memory []byte, mapAddress, rowBytes int) (*Boss, error) {
	u16 := func(address int) int { return int(binary.BigEndian.Uint16(memory[address:])) }
	i16 := func(address int) int { return int(int16(binary.BigEndian.Uint16(memory[address:]))) }
	u32 := func(address int) int { return int(binary.BigEndian.Uint32(memory[address:])) }
	animation := func(address int) []int {
		var frames []int
		for index := 0; index < 64 && address+index < len(memory); index++ {
			value := int(int8(memory[address+index]))
			if value < 0 {
				break
			}
			frames = append(frames, value)
		}
		return frames
	}
	boss := &Boss{Kind: "stage2_gomander", ControllerTrigger: 2570, ControllerDescriptor: 0x4b074, ControllerHealth: 30, Score: 9000, HookTrigger: 2570, FreezeScroll: 2570, VictoryCounter: 36, VictoryReload: 2, VictoryTicks: 106}
	data := &BossTwoData{Trigger: 1981, DelayStep: 5, RouteWait: 85, PartHitScore: 200, ControllerX: 179, ControllerY: 128, ClosedWait: 100, WaitReduction: 10, MinimumWait: 10, PatchReload: 2, OpenHold: 20}
	data.ExplosionXRange, data.ExplosionYRange, data.ExplosionXBase, data.ExplosionYBase = int(memory[0x4a8dd]), int(memory[0x4a8d1]), u16(0x4a8e4), u16(0x4a8da)
	data.ExplosionKinds = []int{0, 1, 2}
	for index, offset := range []int{0, 4, 8, 0x34} {
		data.FlashIndices = append(data.FlashIndices, (u16(0x5360+offset)-0x180)/2)
		data.FlashValues = append(data.FlashValues, uint16(u16(0x4a946)))
		data.NormalValues = append(data.NormalValues, uint16(u16(0x4a984+index*6)))
	}
	for component := 0; component < 11; component++ {
		data.DeathDelays = append(data.DeathDelays, 11-component)
		address := 0x4b174 + component*8
		pointer := u32(address + 4)
		descriptor := u32(address)
		data.Components = append(data.Components, BossTwoComponent{Descriptor: uint32(descriptor), Animation: animation(pointer), AnimationTicks: int(memory[descriptor+7]) + 1, Animated: pointer == 0x4b386})
	}
	data.DamagedAnimation = animation(0x4b38a)
	for address := 0x4b0ce; address < 0x4b170; address += 18 {
		length := u16(address+4) + 1
		x, y, frames := u32(address+6), u32(address+10), u32(address+14)
		if length < 1 || length > 4096 || x < 0 || x+length > len(memory) || y < 0 || y+length > len(memory) || frames < 0 || frames+length > len(memory) {
			return nil, fmt.Errorf("stage-two boss route is outside original data")
		}
		route := BossTwoRoute{StartX: i16(address), StartY: i16(address + 2)}
		for index := 0; index < length; index++ {
			route.MotionX = append(route.MotionX, int8(memory[x+index]))
			route.MotionY = append(route.MotionY, int8(memory[y+index]))
			route.Frames = append(route.Frames, int(memory[frames+index]))
		}
		data.Routes = append(data.Routes, route)
	}
	for frame := 0; frame < 4; frame++ {
		data.Patches = append(data.Patches, words(memory[0x4b034+frame*16:0x4b034+(frame+1)*16]))
	}
	data.PatchRow, data.PatchColumn = (0x5e0c0-mapAddress)/rowBytes, (0x5e0c0-mapAddress)%rowBytes/2
	var err error
	data.ControllerCollisions, err = collisionFrames(memory, u32(0x4b074+24), 4)
	if err != nil {
		return nil, err
	}
	boss.Second = data
	return boss, nil
}
