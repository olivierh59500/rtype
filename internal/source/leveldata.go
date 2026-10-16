package source

import (
	"encoding/binary"
	"fmt"
	"image"
)

// CollisionFrame contains the original inclusive collision envelope relative
// to the top-left of the corresponding art frame. Width and height are extents.
type CollisionFrame struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// ScrollStep is one original six-byte camera command, with decoded axis data.
// Duration is the original timer reload; a command lasts Duration+1 main ticks.
// Period is the axis timer reload plus one. Axis phase survives command changes.
// Repeat marks the final command, whose negative-duration successor is a marker.
type ScrollStep struct {
	Address  uint32 `json:"address"`
	XCommand int    `json:"x_command"`
	YCommand int    `json:"y_command"`
	Duration int    `json:"duration"`
	XDelta   int    `json:"x_delta"`
	YDelta   int    `json:"y_delta"`
	XPeriod  int    `json:"x_period"`
	YPeriod  int    `json:"y_period"`
	Repeat   bool   `json:"repeat"`
}

// Boss preserves verified stage-one component, trajectory and tile-patch data.
// Original addresses identify provenance only; native Go owns boss behavior.
type Boss struct {
	MaskFlashIndex                                                                                                      int   `json:"mask_flash_index,omitempty"`
	ControllerDeathKinds                                                                                                []int `json:"controller_death_kinds,omitempty"`
	ControllerDeathReload, ControllerDeathXRange, ControllerDeathYRange, ControllerDeathXOffset, ControllerDeathYOffset int
	Seven                                                                                                               *BossSevenData `json:"seven,omitempty"`
	Fifth                                                                                                               *BossFiveData  `json:"fifth,omitempty"`
	Eighth                                                                                                              *BossEightData `json:"eighth,omitempty"`
	Fourth                                                                                                              *BossFourData  `json:"fourth,omitempty"`
	Six                                                                                                                 *BossSixData   `json:"six,omitempty"`
	Third                                                                                                               *BossThreeData `json:"third,omitempty"`
	Second                                                                                                              *BossTwoData   `json:"second,omitempty"`
	Kind                                                                                                                string         `json:"kind"`
	TailTrigger                                                                                                         int            `json:"tail_trigger"`
	TailDescriptor                                                                                                      uint32         `json:"tail_descriptor"`
	TailBaseX                                                                                                           int            `json:"tail_base_x"`
	TailPeriod                                                                                                          int            `json:"tail_period"`
	TailYOffset                                                                                                         int            `json:"tail_y_offset"`
	TailFrames                                                                                                          [][][]int      `json:"tail_frames"`
	ControllerTrigger                                                                                                   int            `json:"controller_trigger"`
	ControllerDescriptor                                                                                                uint32         `json:"controller_descriptor"`
	ControllerHealth                                                                                                    int            `json:"controller_health"`
	ControllerCollision                                                                                                 CollisionFrame `json:"controller_collision"`
	DamageThreshold                                                                                                     int            `json:"damage_threshold"`
	DamageArenaTimer                                                                                                    int            `json:"damage_arena_timer"`
	VictoryCounter                                                                                                      int            `json:"victory_counter"`
	VictoryReload                                                                                                       int            `json:"victory_reload"`
	VictoryTicks                                                                                                        int            `json:"victory_ticks"`
	Score                                                                                                               int            `json:"score"`
	HookTrigger                                                                                                         int            `json:"hook_trigger"`
	FreezeScroll                                                                                                        int            `json:"freeze_scroll"`
	FreezeTicks                                                                                                         int            `json:"freeze_ticks"`
	EscapeScroll                                                                                                        int            `json:"escape_scroll"`
	EyeFrames                                                                                                           [][]uint16     `json:"eye_frames"`
	MouthFrames                                                                                                         [][]uint16     `json:"mouth_frames"`
	EyeColumn                                                                                                           int            `json:"eye_column"`
	EyeRow                                                                                                              int            `json:"eye_row"`
	MouthColumn                                                                                                         int            `json:"mouth_column"`
	MouthRow                                                                                                            int            `json:"mouth_row"`
	EyePeriod                                                                                                           int            `json:"eye_period"`
	MouthPeriod                                                                                                         int            `json:"mouth_period"`
	ProjectileDescriptor                                                                                                uint32         `json:"projectile_descriptor"`
	ProjectileDelays                                                                                                    []int          `json:"projectile_delays"`
	ProjectileStartX                                                                                                    int            `json:"projectile_start_x"`
	ProjectileStartY                                                                                                    int            `json:"projectile_start_y"`
	ProjectileTeleportX                                                                                                 int            `json:"projectile_teleport_offset_x"`
	ProjectileMotionX                                                                                                   []int          `json:"projectile_motion_x"`
	ProjectileMotionY                                                                                                   []int          `json:"projectile_motion_y"`
	ProjectileStraightVX                                                                                                int            `json:"projectile_straight_vx"`
	ProjectileAimLow                                                                                                    int            `json:"projectile_aim_low"`
	ProjectileAimHigh                                                                                                   int            `json:"projectile_aim_high"`
	AimMap                                                                                                              []int          `json:"aim_map"`
	AimThresholds                                                                                                       []int          `json:"aim_thresholds"`
}

func extractScrollScript(memory []byte, address, start, end int) ([]ScrollStep, error) {
	if start < 0 || end > len(memory) || address < start || address+6 > end {
		return nil, fmt.Errorf("camera script address is outside its level header")
	}
	var steps []ScrollStep
	for count := 0; count < 512 && address+6 <= end; count++ {
		x := int(int16(binary.BigEndian.Uint16(memory[address:])))
		y := int(int16(binary.BigEndian.Uint16(memory[address+2:])))
		duration := int(int16(binary.BigEndian.Uint16(memory[address+4:])))
		if duration < 0 {
			if len(steps) == 0 {
				return nil, fmt.Errorf("camera script starts with an end marker")
			}
			steps[len(steps)-1].Repeat = true
			return steps, nil
		}
		if x < -14 || x > 14 || y < -14 || y > 14 || len(memory) < 0x4908 {
			return nil, fmt.Errorf("camera script contains an invalid axis command")
		}
		steps = append(steps, ScrollStep{Address: uint32(address), XCommand: x, YCommand: y, Duration: duration,
			XDelta: int(int8(memory[0x48e9+x])), YDelta: int(int8(memory[0x48e9+y])),
			XPeriod: int(memory[0x48f8+abs(x)]) + 1, YPeriod: int(memory[0x48f8+abs(y)]) + 1})
		address += 6
	}
	return nil, fmt.Errorf("camera script has no bounded end marker")
}

func abs(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func collisionFrames(memory []byte, address, count int) ([]CollisionFrame, error) {
	if address <= 0 || count <= 0 || count > 128 || address > len(memory)-count*8 {
		return nil, fmt.Errorf("collision table is outside the original data")
	}
	frames := make([]CollisionFrame, count)
	for index := range frames {
		data := memory[address+index*8:]
		frames[index] = CollisionFrame{X: int(int16(binary.BigEndian.Uint16(data))), Y: int(int16(binary.BigEndian.Uint16(data[2:]))), Width: int(binary.BigEndian.Uint16(data[4:])), Height: int(binary.BigEndian.Uint16(data[6:]))}
		if frames[index].Width > 1024 || frames[index].Height > 1024 {
			return nil, fmt.Errorf("collision table contains an invalid envelope")
		}
	}
	return frames, nil
}

func extractTileAtlas(memory []byte, address, count int, palette []uint16) (*image.Paletted, error) {
	if address < 0 || count < 1 || count > 4096 || address > len(memory)-count*128 || len(palette) != 16 {
		return nil, fmt.Errorf("tile bank is outside the original data")
	}
	img := image.NewPaletted(image.Rect(0, 0, 256, (count+15)/16*16), indexedPalette(palette, false))
	for tile := 0; tile < count; tile++ {
		for y := 0; y < 16; y++ {
			for x := 0; x < 16; x++ {
				value := planarIndex(memory, address+tile*128, x, y, 2)
				img.SetColorIndex(tile%16*16+x, tile/16*16+y, uint8(value))
			}
		}
	}
	return img, nil
}

func extractBossOne(memory []byte, mapAddress, rowBytes int) (*Boss, error) {
	boss := &Boss{Kind: "stage1_dobkeratops", TailTrigger: 3382, TailDescriptor: 0x4c1ca, TailBaseX: 300, TailPeriod: 2, TailYOffset: -10,
		ControllerTrigger: 3570, ControllerDescriptor: 0x4c22a, ControllerHealth: 30, DamageThreshold: 20, DamageArenaTimer: 50,
		VictoryCounter: 36, VictoryReload: 2, VictoryTicks: 106, Score: 6200, HookTrigger: 3390, FreezeScroll: 3677, FreezeTicks: 1250, EscapeScroll: 3897,
		EyePeriod: 11, MouthPeriod: 75, ProjectileDescriptor: 0x4c26c, ProjectileStartX: 368, ProjectileStartY: 125,
		ProjectileTeleportX: -20, ProjectileStraightVX: -6, ProjectileAimLow: 100, ProjectileAimHigh: 157}
	boss.MaskFlashIndex = 15
	boss.ControllerDeathKinds = []int{0, 2}
	boss.ControllerDeathReload = int(binary.BigEndian.Uint16(memory[0x4b826:]))
	boss.ControllerDeathXRange = int(memory[0x4b84b])
	boss.ControllerDeathYRange = int(binary.BigEndian.Uint16(memory[0x4b83a:]))
	boss.ControllerDeathXOffset = -int(binary.BigEndian.Uint16(memory[0x4b852:]))
	boss.ControllerDeathYOffset = -int(binary.BigEndian.Uint16(memory[0x4b842:]))
	frames, err := collisionFrames(memory, 0x4c254, 1)
	if err != nil {
		return nil, err
	}
	boss.ControllerCollision = frames[0]
	for frame := 0; frame < 40; frame++ {
		var segments [][]int
		for segment := 0; segment < 16; segment++ {
			address := 0x78ab8 + frame*32 + segment*2
			segments = append(segments, []int{int(memory[address]), int(memory[address+1])})
		}
		boss.TailFrames = append(boss.TailFrames, segments)
	}
	for index := 0; index < 4; index++ {
		eye := int(binary.BigEndian.Uint32(memory[0x4c16c+index*4:]))
		mouth := int(binary.BigEndian.Uint32(memory[0x4c194+index*4:]))
		if eye < 0 || eye+8 > len(memory) || mouth < 0 || mouth+8 > len(memory) {
			return nil, fmt.Errorf("boss tile patch is outside original data")
		}
		boss.EyeFrames = append(boss.EyeFrames, words(memory[eye:eye+8]))
		boss.MouthFrames = append(boss.MouthFrames, words(memory[mouth:mouth+8]))
	}
	boss.EyeRow, boss.EyeColumn = (0x5bb5e-mapAddress)/rowBytes, (0x5bb5e-mapAddress)%rowBytes/2
	boss.MouthRow, boss.MouthColumn = (0x5b72a-mapAddress)/rowBytes, (0x5b72a-mapAddress)%rowBytes/2
	for index := 0; index < 5; index++ {
		boss.ProjectileDelays = append(boss.ProjectileDelays, 1+index*3)
	}
	for index := 0; index < 42; index++ {
		boss.ProjectileMotionX = append(boss.ProjectileMotionX, int(int8(memory[0x79607+index])))
		boss.ProjectileMotionY = append(boss.ProjectileMotionY, int(int8(memory[0x79631+index])))
	}
	for _, value := range memory[0x4a1c:0x4a30] {
		boss.AimMap = append(boss.AimMap, int(value))
	}
	for index := 0; index < 5; index++ {
		boss.AimThresholds = append(boss.AimThresholds, int(binary.BigEndian.Uint16(memory[0x4a30+index*2:])))
	}
	return boss, nil
}
