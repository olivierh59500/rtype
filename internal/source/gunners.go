package source

import (
	"encoding/binary"
	"fmt"
)

// GroundGunnerData preserves the shared base-creature's range selection,
// animation streams, burst timing, and original arc-projectile tables.
type GroundGunnerData struct {
	WalkSpeed             int              `json:"walk_speed"`
	DistanceLeftBonus     int              `json:"distance_left_bonus"`
	HoldDistance          int              `json:"hold_distance"`
	RestMin               int              `json:"rest_min"`
	RestMax               int              `json:"rest_max"`
	FireMin               int              `json:"fire_min"`
	FireMax               int              `json:"fire_max"`
	RestTicks             int              `json:"rest_ticks"`
	AfterBurstRestTicks   int              `json:"after_burst_rest_ticks"`
	BurstShotCount        int              `json:"burst_shot_count"`
	InitialShotWait       int              `json:"initial_shot_wait"`
	ShotPeriod            int              `json:"shot_period"`
	LeftProbeX            int              `json:"left_probe_x"`
	RightProbeX           int              `json:"right_probe_x"`
	ProbeY                int              `json:"probe_y"`
	WalkLeft              []int            `json:"walk_left"`
	WalkRight             []int            `json:"walk_right"`
	RawLeft               []int            `json:"raw_left"`
	RawRight              []int            `json:"raw_right"`
	RawFire               []int            `json:"raw_fire"`
	WallFrameLeft         int              `json:"wall_frame_left"`
	WallFrameRight        int              `json:"wall_frame_right"`
	FireInitialFrame      int              `json:"fire_initial_frame"`
	AfterBurstFrame       int              `json:"after_burst_frame"`
	ProjectileDescriptor  uint32           `json:"projectile_descriptor"`
	ProjectileSprite      string           `json:"projectile_sprite"`
	ProjectileOffsetX     int              `json:"projectile_offset_x"`
	ProjectileOffsetY     int              `json:"projectile_offset_y"`
	ProjectileWidth       int              `json:"projectile_width"`
	ProjectileHeight      int              `json:"projectile_height"`
	ProjectileFrameTicks  int              `json:"projectile_frame_ticks"`
	ProjectileCollision   []CollisionFrame `json:"projectile_collision"`
	ProjectileX           []int8           `json:"projectile_x"`
	ProjectileY           []int8           `json:"projectile_y"`
	ProjectileFramesRight []int            `json:"projectile_frames_right"`
	ProjectileFramesLeft  []int            `json:"projectile_frames_left"`
	PhaseXBase            int              `json:"phase_x_base"`
	PhaseYBase            int              `json:"phase_y_base"`
	PhaseXJitter          int              `json:"phase_x_jitter"`
	PhaseYJitter          int              `json:"phase_y_jitter"`
	PlayerYDistanceOffset int              `json:"player_y_distance_offset"`
	SparkSprite           string           `json:"spark_sprite"`
	SparkWidth            int              `json:"spark_width"`
	SparkHeight           int              `json:"spark_height"`
	SparkAnimation        []int            `json:"spark_animation"`
	SparkFrameTicks       int              `json:"spark_frame_ticks"`
	SparkLifetime         int              `json:"spark_lifetime"`
	SparkSound            int              `json:"spark_sound"`
}

func annotateGroundGunner(memory []byte, event *Event, start, end, level int) bool {
	profiles := map[int]uint32{1: 0x4a83c, 4: 0x4ab12, 7: 0x4aa28}
	spawn, ok := profiles[level]
	if !ok || event.SpawnAddress != spawn || event.UpdateAddress != spawn+0x2a {
		return false
	}
	u32 := func(address int) int { return int(binary.BigEndian.Uint32(memory[address:])) }
	u16 := func(address int) int { return int(binary.BigEndian.Uint16(memory[address:])) }
	update := int(event.UpdateAddress)
	left, right, fire := u32(update+0x162), u32(update+0x16e), u32(update+0xd8)
	ending := u32(update + 0x76)
	if left < start || right < start || fire < start || ending+5 > end {
		return false
	}
	stream := func(address int, raw bool) []int {
		var frames []int
		for pointer := address; pointer < ending+5; pointer++ {
			value := int(int8(memory[pointer]))
			if value < 0 && !raw {
				break
			}
			frames = append(frames, value)
		}
		return frames
	}
	d := int(event.Descriptor)
	data := &GroundGunnerData{WalkSpeed: 2, DistanceLeftBonus: 30, HoldDistance: 70, RestMin: 60, RestMax: 80, FireMin: 55, FireMax: 85, RestTicks: 21, AfterBurstRestTicks: 11, BurstShotCount: int(memory[d+9]), InitialShotWait: u16(d+6) + 1, ShotPeriod: 11, LeftProbeX: -16, RightProbeX: 33, ProbeY: 11, WalkLeft: stream(left, false), WalkRight: stream(right, false), RawLeft: stream(left, true), RawRight: stream(right, true), RawFire: stream(fire, true), WallFrameLeft: int(memory[left+3]), WallFrameRight: int(memory[right+3]), FireInitialFrame: int(memory[fire+3]), AfterBurstFrame: int(memory[ending]), ProjectileOffsetX: 20, ProjectileOffsetY: -8, PhaseXBase: 19, PhaseYBase: 16, PhaseXJitter: 10, PhaseYJitter: 15, PlayerYDistanceOffset: -31}
	projectile := u32(update + 0x18e)
	data.ProjectileDescriptor = uint32(projectile)
	data.ProjectileSprite = fmt.Sprintf("object-%d-%05x", level, projectile)
	size := u16(projectile)
	data.ProjectileWidth, data.ProjectileHeight = (size&63-1)*16, (size>>6)/4
	data.ProjectileFrameTicks = int(memory[projectile+7]) + 1
	projectileUpdate := u32(projectile + 28)
	vertical, horizontal, frames := u32(projectileUpdate+2), u32(projectileUpdate+8), u32(projectileUpdate+14)
	if vertical <= 0 || horizontal <= 0 || frames <= 0 {
		return false
	}
	master := u32(projectile + 20)
	for index := 0; index < 256 && horizontal+index < len(memory) && vertical+index < len(memory) && frames+index < len(memory); index++ {
		x, y := int8(memory[horizontal+index]), int8(memory[vertical+index])
		data.ProjectileX = append(data.ProjectileX, x)
		data.ProjectileY = append(data.ProjectileY, y)
		if x == 127 || y == 127 {
			break
		}
		frame := int(memory[frames+index])
		if frame > 8 || master+frame+9 >= len(memory) {
			return false
		}
		data.ProjectileFramesRight = append(data.ProjectileFramesRight, int(memory[master+frame]))
		data.ProjectileFramesLeft = append(data.ProjectileFramesLeft, int(memory[master+frame+9]))
	}
	data.ProjectileCollision, _ = collisionFrames(memory, u32(projectile+24), 16)
	spark := u32(0x4922)
	sparkSize := u16(spark)
	data.SparkSprite = fmt.Sprintf("object-%d-%05x", level, spark)
	data.SparkWidth, data.SparkHeight = (sparkSize&63-1)*16, (sparkSize>>6)/4
	data.SparkFrameTicks, data.SparkLifetime, data.SparkSound = int(memory[spark+7])+1, int(memory[0x4922+7])+1, 11
	for p := u32(spark + 20); p > 0 && p < len(memory) && len(data.SparkAnimation) < 64; p++ {
		value := int(int8(memory[p]))
		if value < 0 {
			break
		}
		data.SparkAnimation = append(data.SparkAnimation, value)
	}
	event.Behavior, event.GroundGunner = "ground_gunner", data
	return true
}
