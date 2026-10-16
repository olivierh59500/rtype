package source

import "encoding/binary"

// HopperData contains the original ground-walking and jumping tables. This
// family uses tile contact and four native phases; it has no firing routine.
type HopperData struct {
	WalkVX                int    `json:"walk_vx"`
	JumpVX                int    `json:"jump_vx"`
	FallVX                int    `json:"fall_vx"`
	FallVY                int    `json:"fall_vy"`
	ForwardProbeLeftX     int    `json:"forward_probe_left_x"`
	ForwardProbeRightX    int    `json:"forward_probe_right_x"`
	FootY                 int    `json:"foot_y"`
	SupportRows           int    `json:"support_rows"`
	LandingProbeX         int    `json:"landing_probe_x"`
	LandingProbeY         int    `json:"landing_probe_y"`
	LandingSnap           bool   `json:"landing_snap"`
	FacingPeriod          int    `json:"facing_period"`
	FacingLeftFrame       int    `json:"facing_left_frame"`
	FacingRightFrame      int    `json:"facing_right_frame"`
	FacingRightIndex      int    `json:"facing_right_index"`
	MinX                  int    `json:"min_x"`
	MaxX                  int    `json:"max_x"`
	MaxY                  int    `json:"max_y"`
	InitFallStep          bool   `json:"init_fall_step"`
	JumpAnimationStart    int    `json:"jump_animation_start"`
	JumpInitialLeftFrame  int    `json:"jump_initial_left_frame"`
	JumpInitialRightFrame int    `json:"jump_initial_right_frame"`
	JumpY                 []int8 `json:"jump_y"`
	StandingLeft          []int  `json:"standing_left"`
	StandingRight         []int  `json:"standing_right"`
	JumpLeft              []int  `json:"jump_left"`
	JumpRight             []int  `json:"jump_right"`
}

// RouteCrawlerData describes the stage-six crawler's shield and obstacle route.
// The negative direction marker reverses the current route and remains active.
type RouteCrawlerData struct {
	FacingSide       int                `json:"facing_side"`
	InitialDirection int                `json:"initial_direction"`
	Directions       []int              `json:"directions"`
	Routes           []RouteCrawlerStep `json:"routes"`
	LeftAnimation    []int              `json:"left_animation"`
	DamageBudget     int                `json:"damage_budget"`
	ContactHealth    int                `json:"contact_health"`
	DeathKind        int                `json:"death_kind"`
	ShieldOffset     int                `json:"shield_offset"`
	MinX             int                `json:"min_x"`
	MaxX             int                `json:"max_x"`
	MinY             int                `json:"min_y"`
	MaxY             int                `json:"max_y"`
}

type RouteCrawlerStep struct {
	DX        int `json:"dx"`
	DY        int `json:"dy"`
	ProbeX    int `json:"probe_x"`
	ProbeY    int `json:"probe_y"`
	NeighborX int `json:"neighbor_x"`
	NeighborY int `json:"neighbor_y"`
}

// StalkerData contains original grouped spawn coordinates and terrain-aware
// pursuit settings. Escape starts at the original unsigned scroll threshold.
type StalkerData struct {
	Members        []StalkerMember `json:"members"`
	EscapeScroll   int             `json:"escape_scroll"`
	LeftAnimation  []int           `json:"left_animation"`
	RightAnimation []int           `json:"right_animation"`
	RetargetJitter int             `json:"retarget_jitter"`
	BaselineVX     int             `json:"baseline_vx"`
	LeftVX         int             `json:"left_vx"`
	RightVX        int             `json:"right_vx"`
	UpVY           int             `json:"up_vy"`
	DownVY         int             `json:"down_vy"`
	EscapeX        int             `json:"escape_x"`
	EscapeTopY     int             `json:"escape_top_y"`
	EscapeBottomY  int             `json:"escape_bottom_y"`
	EscapeSplitY   int             `json:"escape_split_y"`
}

type StalkerMember struct {
	X     int `json:"x"`
	Y     int `json:"y"`
	Delay int `json:"delay"`
}

// TilePulseData contains the original city-stage map replacement and timing.
// Ghost placement suppresses collision until activation; the visible pulse
// remains invulnerable to shots but uses its original contact envelope.
type TilePulseData struct {
	Delay                 int      `json:"delay"`
	PreEffects            int      `json:"pre_effects"`
	EffectDescriptor      uint32   `json:"effect_descriptor"`
	SpawnEffectDescriptor uint32   `json:"spawn_effect_descriptor"`
	EffectSprite          string   `json:"effect_sprite"`
	EffectWidth           int      `json:"effect_width"`
	EffectHeight          int      `json:"effect_height"`
	EffectAnimation       []int    `json:"effect_animation"`
	EffectAnimationTicks  int      `json:"effect_animation_ticks"`
	EffectLifetime        int      `json:"effect_lifetime"`
	EffectDelay           int      `json:"effect_delay"`
	EffectSound           int      `json:"effect_sound"`
	EffectXJitter         int      `json:"effect_x_jitter"`
	EffectYJitter         int      `json:"effect_y_jitter"`
	EffectXOffset         int      `json:"effect_x_offset"`
	EffectYOffset         int      `json:"effect_y_offset"`
	EffectJitterOffset    int      `json:"effect_jitter_offset"`
	TargetByteOffset      int      `json:"target_byte_offset"`
	TargetColumn          int      `json:"target_column"`
	TargetRow             int      `json:"target_row"`
	Width                 int      `json:"width"`
	Height                int      `json:"height"`
	Words                 []uint16 `json:"words"`
	StoredVX              int      `json:"stored_vx"`
	PostVX                int      `json:"post_vx"`
	StopAnimationIndex    int      `json:"stop_animation_index"`
	Sound                 int      `json:"sound"`
}

// QuarterTurnerData holds the base-stage creature's clockwise direction cycles
// and fixed frames displayed during its six-update turning pause.
type QuarterTurnerData struct {
	InitialDirection    int     `json:"initial_direction"`
	FreezeTurningScroll int     `json:"freeze_turning_scroll"`
	BaselineVX          int     `json:"baseline_vx"`
	DirectionVX         []int   `json:"direction_vx"`
	DirectionVY         []int   `json:"direction_vy"`
	TurnPeriod          int     `json:"turn_period"`
	PauseTicks          int     `json:"pause_ticks"`
	PlayerOffsetX       int     `json:"player_offset_x"`
	PlayerOffsetY       int     `json:"player_offset_y"`
	ProbeX              int     `json:"probe_x"`
	ProbeY              int     `json:"probe_y"`
	ClearTileFlag       uint16  `json:"clear_tile_flag"`
	MaxX                int     `json:"max_x"`
	MaxY                int     `json:"max_y"`
	Animations          [][]int `json:"animations"`
	RawStreams          [][]int `json:"raw_streams"`
	FrameTicks          int     `json:"frame_ticks"`
	TurnFrame5          []int   `json:"turn_frame_5"`
	TurnFrame7          []int   `json:"turn_frame_7"`
}

// SpitterData describes the two den-stage moving projectile emitters. Its
// projectiles are invulnerable actors rather than Force-blockable enemy shots.
type SpitterData struct {
	VX                       int              `json:"vx"`
	Health                   int              `json:"health"`
	InitialCooldownRange     int              `json:"initial_cooldown_range"`
	FirePeriod               int              `json:"fire_period"`
	ProjectileYBase          int              `json:"projectile_y_base"`
	ProjectileYRange         int              `json:"projectile_y_range"`
	ProjectileDescriptor     uint32           `json:"projectile_descriptor"`
	ProjectileVX             int              `json:"projectile_vx"`
	ProjectileMinX           int              `json:"projectile_min_x"`
	ProjectileSprite         string           `json:"projectile_sprite"`
	ProjectileWidth          int              `json:"projectile_width"`
	ProjectileHeight         int              `json:"projectile_height"`
	ProjectileAnimation      []int            `json:"projectile_animation"`
	ProjectileAnimationTicks int              `json:"projectile_animation_ticks"`
	ProjectileCollision      []CollisionFrame `json:"projectile_collision"`
	SpawnCount               int              `json:"spawn_count"`
	SpawnX                   int              `json:"spawn_x"`
	SpawnYBase               int              `json:"spawn_y_base"`
	SpawnYRange              int              `json:"spawn_y_range"`
	SpawnDelayStep           int              `json:"spawn_delay_step"`
	SpawnAnimationCycle      int              `json:"spawn_animation_cycle"`
}

// WallCrawlerData contains the city-stage vertical wall-contact rule.
type WallCrawlerData struct {
	VX            int `json:"vx"`
	VY            int `json:"vy"`
	UnsupportedVX int `json:"unsupported_vx"`
	ProbeX        int `json:"probe_x"`
	ProbeY        int `json:"probe_y"`
	MinX          int `json:"min_x"`
}

func annotateFamilies(memory []byte, event *Event, start, end, level int) bool {
	u32 := func(address int) int { return int(binary.BigEndian.Uint32(memory[address:])) }
	u16 := func(address int) int { return int(binary.BigEndian.Uint16(memory[address:])) }
	i16 := func(address int) int { return int(int16(binary.BigEndian.Uint16(memory[address:]))) }
	animation := func(address int) []int {
		var frames []int
		for count := 0; count < 64 && address >= start && address+count < end; count++ {
			frame := int(int8(memory[address+count]))
			if frame < 0 {
				break
			}
			frames = append(frames, frame)
		}
		return frames
	}
	if level == 4 && event.SpawnAddress == 0x4b3d0 && event.UpdateAddress == 0x4b418 {
		event.Behavior = "terrain_stamp"
		event.FixedVX = int(int16(uint16(event.Parameter >> 16)))
		event.FixedVY = int(int16(uint16(event.Parameter)))
		event.StampWord = uint16(u16(0x4b43a))
		event.StampProbeX = u16(0x4b420)
		event.StampProbeY = u16(0x4b424)
		frame := 0
		if event.FixedVX < 0 {
			frame = 12
		} else if event.FixedVX > 0 {
			frame = 4
		}
		if event.FixedVY < 0 {
			frame = 0
		} else if event.FixedVY > 0 {
			frame = 8
		}
		event.Animation = []int{frame}
		pointer := int(uint32(uint16(event.Count))<<16 | uint32(uint16(event.Aux)))
		if pointer >= start && pointer+14 <= end {
			length := u16(pointer) + 1
			x, y, a := u32(pointer+2), u32(pointer+6), u32(pointer+10)
			if length < 4096 && x > 0 && y > 0 && a > 0 && x+length <= len(memory) && y+length <= len(memory) && a+length <= len(memory) {
				for index := 0; index < length; index++ {
					event.MotionX = append(event.MotionX, int(int8(memory[x+index])))
					event.MotionY = append(event.MotionY, int(int8(memory[y+index])))
					event.MotionAnimation = append(event.MotionAnimation, int(memory[a+index]))
				}
			}
		}
		return true
	}
	if level == 5 && ((event.SpawnAddress == 0x4ac10 && event.UpdateAddress == 0x4ac86) || (event.SpawnAddress == 0x4ac36 && event.UpdateAddress == 0x4ac8e)) {
		data := &SpitterData{VX: -2, Health: 20, InitialCooldownRange: 50, FirePeriod: 25, ProjectileYBase: 8, ProjectileYRange: 32, ProjectileDescriptor: 0x4b3d4, ProjectileVX: -7, ProjectileMinX: 16, ProjectileSprite: "object-5-4b3d4", SpawnCount: 1, SpawnX: event.X, SpawnYBase: event.Y}
		if event.SpawnAddress == 0x4ac36 {
			data.VX, data.Health, data.InitialCooldownRange, data.FirePeriod = -3, 1, 70, 50
			data.ProjectileYBase, data.ProjectileYRange = 10, 0
			data.SpawnCount, data.SpawnX, data.SpawnYBase, data.SpawnYRange = int(event.Parameter>>16)+1, 368, event.Count, int(event.Parameter&0xffff)
			data.SpawnDelayStep, data.SpawnAnimationCycle = 24, 8
		}
		d := int(data.ProjectileDescriptor)
		size := u16(d)
		data.ProjectileWidth, data.ProjectileHeight = (size&63-1)*16, (size>>6)/4
		data.ProjectileAnimation = animation(u32(d + 20))
		data.ProjectileAnimationTicks = int(memory[d+7]) + 1
		frames := 1
		for _, frame := range data.ProjectileAnimation {
			frames = max(frames, frame+1)
		}
		data.ProjectileCollision, _ = collisionFrames(memory, u32(d+24), frames)
		event.Behavior, event.Spitter, event.Health = "spitter", data, data.Health
		return true
	}
	if level == 7 && event.SpawnAddress == 0x4b4e2 && event.UpdateAddress == 0x4b4fc {
		velocity := int(int16(uint16(event.Parameter >> 16)))
		data := &WallCrawlerData{VX: -1, VY: velocity, UnsupportedVX: -2, ProbeX: -8, ProbeY: -2, MinX: 16}
		if velocity < 0 {
			data.ProbeY = 21
		}
		event.Behavior, event.WallCrawler, event.Health = "wall_crawler", data, 1
		if len(event.Animation) > 0 {
			event.Animation = event.Animation[:1]
		}
		return true
	}
	if level == 4 && event.SpawnAddress == 0x4b4a6 && event.UpdateAddress == 0x4b506 {
		data := &QuarterTurnerData{InitialDirection: int(event.Parameter >> 16), FreezeTurningScroll: int(event.Parameter & 0xffff), BaselineVX: -1, DirectionVX: []int{0, 2, 0, -2}, DirectionVY: []int{-2, 0, 2, 0}, TurnPeriod: 18, PauseTicks: 6, PlayerOffsetX: 5, PlayerOffsetY: -8, ProbeX: 16, ProbeY: 16, ClearTileFlag: 0x1000, MaxX: 368, MaxY: 242, FrameTicks: event.AnimationTicks}
		if data.InitialDirection > 3 {
			return false
		}
		for direction := 0; direction < 4; direction++ {
			pointer := u32(0x4b4f6 + direction*4)
			if pointer < start || pointer+9 > end {
				return false
			}
			data.Animations = append(data.Animations, animation(pointer))
			var raw []int
			for _, value := range memory[pointer : pointer+9] {
				raw = append(raw, int(int8(value)))
			}
			data.RawStreams = append(data.RawStreams, raw)
			data.TurnFrame5 = append(data.TurnFrame5, int(memory[pointer+5]))
			data.TurnFrame7 = append(data.TurnFrame7, int(memory[pointer+7]))
		}
		event.Behavior, event.QuarterTurner = "quarter_turner", data
		event.Animation = append([]int(nil), data.Animations[data.InitialDirection]...)
		return true
	}
	hoppers := map[int]uint32{1: 0x4adf0, 4: 0x4adc2, 6: 0x4aac6, 7: 0x4aedc}
	if spawn, ok := hoppers[level]; ok && event.SpawnAddress == spawn && event.UpdateAddress == spawn+0x42 {
		update := int(event.UpdateAddress)
		direction := int(int16(uint16(event.Parameter >> 16)))
		data := &HopperData{WalkVX: direction, JumpVX: 3, FallVX: -1, FallVY: 4, ForwardProbeLeftX: -3, ForwardProbeRightX: 32, FootY: 12, SupportRows: 1, LandingProbeX: 8, LandingProbeY: 26, FacingPeriod: u16(update+0x50) + 1, FacingLeftFrame: 0, FacingRightFrame: u16(update + 0x5a), MinX: u16(update + 2), MaxX: u16(update + 10), MaxY: u16(update + 18), InitFallStep: true, JumpAnimationStart: 3}
		if direction < 0 {
			data.JumpVX = -3
		}
		data.StandingRight = animation(u32(int(spawn) + 18))
		standingLeft := u32(int(spawn) + 34)
		data.StandingLeft = animation(standingLeft)
		data.FacingRightIndex = data.FacingRightFrame
		data.FacingRightFrame = int(memory[standingLeft+data.FacingRightIndex])
		rightJump, leftJump := u32(update+0xa8), u32(update+0xb4)
		data.JumpRight = animation(rightJump)
		data.JumpLeft = animation(leftJump)
		// The first jump selects stream index three, beyond the two looping
		// frames and their marker. It briefly uses the final standing frame.
		data.JumpInitialLeftFrame = int(memory[leftJump+3])
		data.JumpInitialRightFrame = int(memory[rightJump+3])
		jump := u32(update + 0xcc)
		for index := 0; index < 256 && jump >= start && jump+index < end; index++ {
			value := int8(memory[jump+index])
			if value == 127 {
				break
			}
			data.JumpY = append(data.JumpY, value)
		}
		event.Behavior, event.Hopper = "ground_hopper", data
		return true
	}
	if level == 6 && event.SpawnAddress == 0x4a9b0 && event.UpdateAddress == 0x4a9ee {
		data := &RouteCrawlerData{FacingSide: int(byte(event.Parameter >> 24)), InitialDirection: int(byte(event.Parameter >> 16)), DamageBudget: 4, ContactHealth: u16(0x4a9e6), DeathKind: int(memory[0x4a9e1]), ShieldOffset: 48, MaxX: 350, MaxY: 225, LeftAnimation: animation(0x4ae19)}
		for index := 0; index < 6; index++ {
			value := int(int8(memory[int(event.Address)+8+index]))
			data.Directions = append(data.Directions, value)
			if value < 0 {
				break
			}
		}
		// RAM 0xF4 retains a compiled default until the original loader fills it.
		// Read the actual stage width from the original level table instead.
		rowBytes := u16(0x41b4 + (level-1)*10 + 8)
		for direction := 0; direction < 4; direction++ {
			a := 0x4ad94 + direction*10
			offset := i16(a + 8)
			dy := 0
			if offset >= rowBytes {
				dy = 1
			} else if offset <= -rowBytes {
				dy = -1
			}
			data.Routes = append(data.Routes, RouteCrawlerStep{DX: i16(a), DY: i16(a + 2), ProbeX: i16(a + 4), ProbeY: i16(a + 6), NeighborX: (offset - dy*rowBytes) / 2, NeighborY: dy})
		}
		event.Behavior, event.RouteCrawler, event.Health = "route_crawler", data, data.ContactHealth
		if data.FacingSide == 0 {
			event.Animation = append([]int(nil), data.LeftAnimation...)
		}
		return true
	}
	if level == 7 && event.SpawnAddress == 0x4b3ec && event.UpdateAddress == 0x4b42c {
		variant := int(event.Parameter & 0xffff)
		if variant > 31 || 0x4bde4+variant*4+4 > end {
			return false
		}
		patch := u32(0x4bde4 + variant*4)
		if patch < start || patch+2 > end {
			return false
		}
		width, height := int(memory[patch])+1, int(memory[patch+1])+1
		if width > 64 || height > 64 || patch+2+width*height*2 > end {
			return false
		}
		offset := int(int16(uint16(event.Parameter >> 16)))
		rowBytes := u16(0x41b4 + (level-1)*10 + 8)
		data := &TilePulseData{Delay: 16, PreEffects: 3, SpawnEffectDescriptor: 0x4be14, EffectXJitter: 20, EffectYJitter: 10, EffectJitterOffset: -5, EffectYOffset: 16, TargetByteOffset: offset, TargetColumn: offset % rowBytes / 2, TargetRow: offset / rowBytes, Width: width, Height: height, Words: words(memory[patch+2 : patch+2+width*height*2]), StoredVX: -1, PostVX: -1, StopAnimationIndex: 25, Sound: 12}
		// The temporary actor sets state byte five to one. Core 0x111E then
		// resolves common death family zero from 0x491A before drawing it.
		effect := u32(0x491a)
		size := u16(effect)
		data.EffectDescriptor = uint32(effect)
		data.EffectSprite = "object-7-0493a"
		data.EffectWidth, data.EffectHeight = (size&63-1)*16, (size>>6)/4
		data.EffectAnimationTicks = int(memory[effect+7]) + 1
		data.EffectLifetime = int(memory[0x491a+7]) + 1
		data.EffectDelay = 1
		data.EffectSound = 10
		effectAnimation := u32(effect + 20)
		for count := 0; count < 64 && effectAnimation > 0 && effectAnimation+count < len(memory); count++ {
			frame := int(int8(memory[effectAnimation+count]))
			if frame < 0 {
				break
			}
			data.EffectAnimation = append(data.EffectAnimation, frame)
		}
		if event.Count != 0 {
			data.EffectYOffset = 26
		}
		event.Behavior, event.TilePulse, event.Health = "tile_pulse", data, -1
		return true
	}
	stalkers := map[int]uint32{1: 0x4af72, 4: 0x4af44, 5: 0x4a808, 7: 0x4b05e}
	if spawn, ok := stalkers[level]; ok && event.SpawnAddress == spawn && event.UpdateAddress == spawn+0x30 {
		data := &StalkerData{EscapeScroll: int(event.Parameter >> 16), RetargetJitter: 25, BaselineVX: -1, LeftVX: -1, RightVX: 2, UpVY: -1, DownVY: 1, EscapeX: 4, EscapeTopY: 10, EscapeBottomY: 224, EscapeSplitY: 136}
		pointer := int(uint32(uint16(event.Count))<<16 | uint32(uint16(event.Aux)))
		delay, delayStep := 0, int(int16(uint16(event.Parameter)))
		for count := 0; count < 64 && pointer >= start && pointer+4 <= end; count++ {
			x, y := i16(pointer), i16(pointer+2)
			if x < 0 {
				break
			}
			data.Members = append(data.Members, StalkerMember{X: x, Y: y, Delay: delay})
			delay += delayStep
			pointer += 4
		}
		update := int(event.UpdateAddress)
		data.RightAnimation = animation(u32(update + 0x5c))
		data.LeftAnimation = animation(u32(update + 0x68))
		event.Behavior, event.Stalker = "stalker", data
		return true
	}
	return false
}
