package source

import "encoding/binary"

// BossFourData preserves the three independently damaged base-stage bodies.
type BossFourData struct {
	ProjectileDescriptor     uint32           `json:"projectile_descriptor"`
	ProjectileVX             int              `json:"projectile_vx"`
	ProjectileMaxX           int              `json:"projectile_max_x"`
	ProjectileAnimation      []int            `json:"projectile_animation"`
	ProjectileAnimationTicks int              `json:"projectile_animation_ticks"`
	Bodies                   []BossFourBody   `json:"bodies"`
	WeakpointDescriptor      uint32           `json:"weakpoint_descriptor"`
	WeakpointCollision       []CollisionFrame `json:"weakpoint_collision"`
	WeakpointHealth          int              `json:"weakpoint_health"`
	DeathTicks               int              `json:"death_ticks"`
	BodyScore                int              `json:"body_score"`
	IntroVX                  int              `json:"intro_vx"`
	MotionOffsetX            int              `json:"motion_offset_x"`
	TurretDescriptor         uint32           `json:"turret_descriptor"`
	TurretOffsets            []int            `json:"turret_offsets"`
	TurretFrames             []int            `json:"turret_frames"`
	TurretShotX, TurretShotY []int
	ShotVX, ShotVY           []int
}
type BossFourBody struct {
	Descriptor       uint32 `json:"descriptor"`
	X, Y, Priority   int
	MotionX, MotionY []int
}

func extractBossFour(memory []byte) (*Boss, error) {
	u32 := func(a int) uint32 { return binary.BigEndian.Uint32(memory[a:]) }
	i16 := func(a int) int { return int(int16(binary.BigEndian.Uint16(memory[a:]))) }
	boss := &Boss{Kind: "stage4_base", FreezeScroll: 0x970, VictoryCounter: 36, VictoryReload: 2, VictoryTicks: 106}
	data := &BossFourData{WeakpointDescriptor: 0x4bb64, WeakpointHealth: 30, DeathTicks: 17, BodyScore: 5000, IntroVX: 3, MotionOffsetX: 32, TurretDescriptor: 0x4bbac, ProjectileDescriptor: 0x4bc5e, ProjectileVX: 6, ProjectileMaxX: 368}
	data.ProjectileAnimationTicks = int(memory[data.ProjectileDescriptor+7]) + 1
	animation := int(u32(int(data.ProjectileDescriptor) + 20))
	for index := 0; index < 64 && memory[animation+index] < 128; index++ {
		data.ProjectileAnimation = append(data.ProjectileAnimation, int(memory[animation+index]))
	}
	for body := 0; body < 3; body++ {
		a := 0x4bab0 + body*10
		item := BossFourBody{Descriptor: u32(a), X: i16(a + 4), Y: i16(a + 6), Priority: i16(a + 8)}
		motion := 0x6cfb2 + body*0x8e8
		for frame := 0; frame < 0x474; frame++ {
			item.MotionX = append(item.MotionX, int(memory[motion+frame]))
			item.MotionY = append(item.MotionY, int(memory[motion+0x474+frame]))
		}
		data.Bodies = append(data.Bodies, item)
	}
	var err error
	data.WeakpointCollision, err = collisionFrames(memory, int(u32(0x4bb64+24)), 4)
	if err != nil {
		return nil, err
	}
	for i := 0; i < 3; i++ {
		data.TurretOffsets = append(data.TurretOffsets, i16(0x4bc22+i*4), i16(0x4bc24+i*4))
	}
	for _, v := range memory[0x4bc2e:0x4bc3e] {
		data.TurretFrames = append(data.TurretFrames, int(v))
	}
	for i := 0; i < 8; i++ {
		data.TurretShotX = append(data.TurretShotX, i16(0x4bc3e+i*4))
		data.TurretShotY = append(data.TurretShotY, i16(0x4bc40+i*4))
	}
	for i := 0; i < 16; i++ {
		data.ShotVX = append(data.ShotVX, i16(0x4b1a+i*4))
		data.ShotVY = append(data.ShotVY, i16(0x4b1c+i*4))
	}
	boss.Fourth = data
	for _, v := range memory[0x4a1c:0x4a30] {
		boss.AimMap = append(boss.AimMap, int(v))
	}
	for i := 0; i < 5; i++ {
		boss.AimThresholds = append(boss.AimThresholds, int(binary.BigEndian.Uint16(memory[0x4a30+i*2:])))
	}
	return boss, nil
}
