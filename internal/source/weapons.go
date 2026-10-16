package source

import (
	"encoding/binary"
	"fmt"
	"image"
)

// Weapons preserves the original rule tables separately from executable code.
// Each level carries blitter art decoded with its original playfield palette.
type Weapons struct {
	StageResults     *StageResultsData `json:"stage_results,omitempty"`
	Scenes           *SceneRules       `json:"scenes,omitempty"`
	Death            []DeathEffectData `json:"death,omitempty"`
	Pickup           *PickupData       `json:"pickup,omitempty"`
	Player           *PlayerData       `json:"player,omitempty"`
	HostileShot      *HostileShotData  `json:"hostile_shot,omitempty"`
	Force            []ForceWeapon     `json:"force"`
	Shots            []WeaponShot      `json:"shots"`
	Beams            []WeaponBeam      `json:"beams"`
	Ricochet         [][]WeaponRoute   `json:"ricochet"`
	Ground           [][]WeaponRoute   `json:"ground"`
	WaveSmall        []string          `json:"wave_small"`
	WaveLarge        []string          `json:"wave_large"`
	RicochetFrames   []string          `json:"ricochet_frames"`
	GroundFrames     []string          `json:"ground_frames"`
	MissileFrames    []string          `json:"missile_frames"`
	MissileVelocity  [][2]int          `json:"missile_velocity"`
	MissileLaunchVY  [2]int            `json:"missile_launch_vy"`
	MissileCollision []WeaponBounds    `json:"missile_collision"`
	AimMap           []int             `json:"aim_map"`
	AimThresholds    []int             `json:"aim_thresholds"`
}

// ForceWeapon describes one original rank, including the rear medium variant.
type ForceWeapon struct {
	Rank                                                 int      `json:"rank"`
	Frames                                               []string `json:"frames"`
	FrameAddresses                                       []uint32 `json:"frame_addresses"`
	FrameTicks, Width, Height                            int
	ProbeX, ProbeY                                       int
	AttachBackX, AttachBackY, AttachFrontX, AttachFrontY int
	Collision                                            WeaponBounds `json:"collision"`
}

// WeaponBounds is the original collision offset and axis-distance envelope.
type WeaponBounds struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

type WeaponShot struct {
	Number int    `json:"number"`
	VX     int    `json:"vx"`
	VY     int    `json:"vy"`
	Frame  string `json:"frame"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// WeaponBeam stores the original thirty-byte wave-cannon strength record.
type WeaponBeam struct {
	ProbeColumn int          `json:"probe_column"`
	Threshold   int          `json:"threshold"`
	X           int          `json:"x"`
	Y           int          `json:"y"`
	Damage      int          `json:"damage"`
	Collision   WeaponBounds `json:"collision"`
	Frame       string       `json:"frame"`
	Frames      []string     `json:"frames"`
	Width       int          `json:"width"`
	Height      int          `json:"height"`
}

// WeaponRoute is one ten-byte terrain decision: move, next direction, tile.
// Original tile offsets remain available as evidence; TileX/TileY describe
// the corresponding neighboring tile without depending on map row stride.
type WeaponRoute struct {
	DX         int `json:"dx"`
	DY         int `json:"dy"`
	Next       int `json:"next"`
	TileOffset int `json:"tile_offset"`
	TileX      int `json:"tile_x"`
	TileY      int `json:"tile_y"`
}

// extractWeapons decodes the static player tables and emits original art. It
// does not interpret CPU instructions or require the original player binary.
func extractWeapons(memory []byte, palette []uint16, level int, writeImage func(string, image.Image) error) (Weapons, []Sprite, error) {
	var data Weapons
	var sprites []Sprite
	if len(memory) < 0x2de00 || len(palette) < 16 {
		return data, nil, fmt.Errorf("weapon extraction requires original memory and palette")
	}
	data.Player = extractPlayerData(memory)
	data.Death = extractDeathEffects(memory)
	data.Scenes = extractSceneRules(memory)
	data.StageResults = extractStageResults(memory)
	var pickupErr error
	data.Pickup, pickupErr = extractPickupData(memory)
	if pickupErr != nil {
		return data, nil, pickupErr
	}
	u16 := func(address int) int { return int(binary.BigEndian.Uint16(memory[address:])) }
	i16 := func(address int) int { return int(int16(binary.BigEndian.Uint16(memory[address:]))) }
	u32 := func(address int) int { return int(binary.BigEndian.Uint32(memory[address:])) }
	if level < 1 || level > 8 {
		return data, nil, fmt.Errorf("weapon extraction requires an original level number")
	}
	rowBytes := u16(0x41b4 + (level-1)*10 + 8)
	data.MissileLaunchVY = [2]int{int(int8(memory[0x556c+0x19])), int(int8(memory[0x5588+0x19]))}
	if rowBytes < 4 {
		return data, nil, fmt.Errorf("invalid original weapon map stride")
	}
	write := func(name string, address, width, height int, img image.Image) (string, error) {
		file := "sprites/" + name + ".png"
		if err := writeImage(file, img); err != nil {
			return "", err
		}
		sprites = append(sprites, Sprite{Name: name, File: file, Width: width, Height: height, Address: uint32(address), Frame: 0, Level: level})
		return name, nil
	}
	blitter := func(label string, address, colors, mask, width, height int) (string, error) {
		length := width / 8 * height * 4
		if width <= 0 || width%16 != 0 || height <= 0 || colors < 0 || mask < 0 || colors+length > len(memory) || mask+length > len(memory) {
			return "", fmt.Errorf("invalid original weapon image %s", label)
		}
		return write(fmt.Sprintf("weapon-%d-%s", level, label), address, width, height, blitterSprite(memory, colors, mask, width, height, palette))
	}
	// Core 0x130A selects this shared image; 0x1422 advances it by 0xE0
	// for four impact frames. Each 16 by 14 frame has 112 color bytes
	// followed by the original four-plane transparency mask.
	data.HostileShot = &HostileShotData{Width: 16, Height: 14, PoolSize: 16, RandomRange: 75}
	for direction := 0; direction < 16; direction++ {
		data.HostileShot.VelocityX = append(data.HostileShot.VelocityX, i16(0x4b1a+direction*4))
		data.HostileShot.VelocityY = append(data.HostileShot.VelocityY, i16(0x4b1c+direction*4))
	}
	for frame := 0; frame < 5; frame++ {
		address := 0x19d90 + frame*0xe0
		name, err := blitter(fmt.Sprintf("hostile-%d", frame), address, address, address+0x70, 16, 14)
		if err != nil {
			return data, nil, err
		}
		data.HostileShot.Frames = append(data.HostileShot.Frames, name)
	}
	// Hardware-sprite colors use the dedicated original sprite palette.
	spritePalette := words(memory[0x4232:0x4252])
	for rank := 0; rank < 4; rank++ {
		a := 0x516c + rank*16
		first, last, step, height := u32(a), u32(a+4), u32(a+8), u16(a+14)
		if step <= 0 || step%4 != 0 || first < 0x6000 || last < first || last+step > len(memory) {
			return data, nil, fmt.Errorf("invalid original Force rank %d animation", rank)
		}
		force := ForceWeapon{Rank: rank, FrameTicks: int(memory[a+13]) + 1, Width: 32, Height: height, ProbeX: i16(0x51cc + rank*4), ProbeY: i16(0x51ce + rank*4), AttachBackX: i16(0x51ac + rank*8), AttachBackY: i16(0x51ae + rank*8), AttachFrontX: i16(0x51b0 + rank*8), AttachFrontY: i16(0x51b2 + rank*8), Collision: WeaponBounds{X: i16(0x51dc + rank*8), Y: i16(0x51de + rank*8), Width: i16(0x51e0 + rank*8), Height: i16(0x51e2 + rank*8)}}
		for frame, address := 0, first; address <= last; frame, address = frame+1, address+step {
			name := fmt.Sprintf("force-rank-%d-%d", rank, frame)
			// Repeated stage records reference identical original hardware art.
			if level == 1 {
				file, err := write(name, address, 32, height, attachedSprite(memory, address, 32, height, step/4, spritePalette))
				if err != nil {
					return data, nil, err
				}
				name = file
			}
			force.Frames = append(force.Frames, name)
			force.FrameAddresses = append(force.FrameAddresses, uint32(address))
		}
		data.Force = append(data.Force, force)
	}
	for number := 0; number < 5; number++ {
		a := 0x4c3a + number*16
		size := u16(a + 6)
		width, height := (size&63-1)*16, (size>>6)/4
		frame, err := blitter(fmt.Sprintf("spread-%d", number), a, u32(a+8), u32(a+12), width, height)
		if err != nil {
			return data, nil, err
		}
		data.Shots = append(data.Shots, WeaponShot{Number: number, VX: i16(a), VY: i16(a + 2), Frame: frame, Width: width, Height: height})
	}
	for number := 0; number < 4; number++ {
		a := 0x4ca2 + number*30
		size := u16(a + 18)
		width, height := (size&63-1)*16, (size>>6)/4
		colors, stride := u32(a+22), u32(a+26)
		frame, err := blitter(fmt.Sprintf("beam-%d", number), a, colors, colors+stride, width, height)
		if err != nil {
			return data, nil, err
		}
		alternate, err := blitter(fmt.Sprintf("beam-%d-alt", number), a, colors+2*stride, colors+3*stride, width, height)
		if err != nil {
			return data, nil, err
		}
		data.Beams = append(data.Beams, WeaponBeam{ProbeColumn: i16(a+14) / 2, Threshold: u16(a), X: i16(a + 2), Y: i16(a + 4), Damage: u16(a + 20), Collision: WeaponBounds{X: i16(a + 6), Y: i16(a + 8), Width: i16(a + 10), Height: i16(a + 12)}, Frame: frame, Frames: []string{frame, alternate}, Width: width, Height: height})
	}
	// The CPU indexes the basic ricochet route as direction*40 and the ground
	// route as direction*40 plus a ten-byte decision position.
	for direction := 0; direction < 4; direction++ {
		routes := make([]WeaponRoute, 4)
		for choice := range routes {
			a := 0x4d1a + direction*40 + choice*10
			offset := int(int32(binary.BigEndian.Uint32(memory[a+6:])))
			// Core 0xD18 rewrites these eight offsets after selecting a level.
			// Translate the relocation directly instead of executing that code.
			switch direction*40 + choice*10 + 6 {
			case 0x06:
				offset = -rowBytes + 2
			case 0x1a, 0x42:
				offset = -rowBytes
			case 0x2e:
				offset = -rowBytes - 2
			case 0x56:
				offset = rowBytes - 2
			case 0x6a, 0x92:
				offset = rowBytes
			case 0x7e:
				offset = rowBytes + 2
			}
			dy := 0
			if offset > 100 {
				dy = 1
			} else if offset < -100 {
				dy = -1
			}
			dx := (offset - dy*rowBytes) / 2
			routes[choice] = WeaponRoute{DX: i16(a), DY: i16(a + 2), Next: i16(a + 4), TileOffset: offset, TileX: dx, TileY: dy}
		}
		data.Ricochet = append(data.Ricochet, routes)
	}
	for direction := 0; direction < 16; direction++ {
		routes := make([]WeaponRoute, 4)
		for choice := range routes {
			a := 0x4e6e + direction*40 + choice*10
			offset := int(int32(binary.BigEndian.Uint32(memory[a+6:])))
			for _, relocation := range []struct{ address, sign int }{{0x466e, 1}, {0x4688, -1}} {
				for index := 0; index < 32; index++ {
					position := i16(relocation.address + index*2)
					if position < 0 {
						break
					}
					if position == direction*40+choice*10+6 {
						offset = relocation.sign * rowBytes
					}
				}
			}
			dy := 0
			if offset > 100 {
				dy = 1
			} else if offset < -100 {
				dy = -1
			}
			dx := (offset - dy*rowBytes) / 2
			routes[choice] = WeaponRoute{DX: i16(a), DY: i16(a + 2), Next: i16(a + 4), TileOffset: offset, TileX: dx, TileY: dy}
		}
		data.Ground = append(data.Ground, routes)
	}
	for frame := 0; frame < 2; frame++ {
		name, err := blitter(fmt.Sprintf("ricochet-%d", frame), 0x19b90+frame*256, 0x19b90+frame*256, 0x19c10+frame*256, 16, 16)
		if err != nil {
			return data, nil, err
		}
		data.RicochetFrames = append(data.RicochetFrames, name)
	}
	for frame := 0; frame < 4; frame++ {
		a := u32(0x4e5e + frame*4)
		name, err := blitter(fmt.Sprintf("ground-%d", frame), a, a, a+0x70, 16, 14)
		if err != nil {
			return data, nil, err
		}
		data.GroundFrames = append(data.GroundFrames, name)
	}
	for frame := 0; frame < 6; frame++ {
		a := u32(0x4c8a + frame*4)
		name, err := blitter(fmt.Sprintf("wave-large-%d", frame), a, a, a+0x3c0, 64, 30)
		if err != nil {
			return data, nil, err
		}
		data.WaveLarge = append(data.WaveLarge, name)
	}
	for stream := 0; stream < 2; stream++ {
		for direction := 0; direction < 2; direction++ {
			for frame := 0; frame < 2; frame++ {
				a := 0x15650 + stream*0x200 + direction*0x100 + frame*0x80
				name, err := blitter(fmt.Sprintf("wave-small-%d-%d-%d", stream, direction, frame), a, a, a+0x40, 32, 4)
				if err != nil {
					return data, nil, err
				}
				data.WaveSmall = append(data.WaveSmall, name)
			}
		}
	}
	for direction := 0; direction < 16; direction++ {
		data.MissileVelocity = append(data.MissileVelocity, [2]int{i16(0x542c + direction*4), i16(0x542e + direction*4)})
		data.MissileCollision = append(data.MissileCollision, WeaponBounds{X: i16(0x54ec + direction*8), Y: i16(0x54ee + direction*8), Width: i16(0x54f0 + direction*8), Height: i16(0x54f2 + direction*8)})
		for frame := 0; frame < 2; frame++ {
			a := u32(0x546c + (direction+frame*16)*4)
			name, err := blitter(fmt.Sprintf("missile-%d-%d", direction, frame), a, a, a+0x80, 16, 16)
			if err != nil {
				return data, nil, err
			}
			data.MissileFrames = append(data.MissileFrames, name)
		}
	}
	for index := 0; index < 20; index++ {
		data.AimMap = append(data.AimMap, int(memory[0x4a1c+index]))
	}
	for index := 0; index < 5; index++ {
		data.AimThresholds = append(data.AimThresholds, u16(0x4a30+index*2))
	}
	return data, sprites, nil
}
