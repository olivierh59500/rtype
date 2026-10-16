package sim

import "github.com/olivierh59500/r-type/internal/source"

// CollisionFrame preserves an original collision envelope relative to the
// visual top-left. Width and Height are inclusive extents, as in core 0x1634.
type CollisionFrame struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

// BossSpec contains original declarative data, never executable instructions.
type BossSpec struct {
	MaskFlashIndex                                                                                                      int   `json:"mask_flash_index"`
	ControllerDeathKinds                                                                                                []int `json:"controller_death_kinds"`
	ControllerDeathReload, ControllerDeathXRange, ControllerDeathYRange, ControllerDeathXOffset, ControllerDeathYOffset int
	Seven                                                                                                               *source.BossSevenData `json:"seven"`
	SevenArt, SevenGuardArt                                                                                             BossArt               `json:"-"`
	SevenVariantArt                                                                                                     []BossArt             `json:"-"`
	Fifth                                                                                                               *source.BossFiveData  `json:"fifth"`
	FifthArt, FifthPartArt                                                                                              BossArt               `json:"-"`
	FourthProjectileArt                                                                                                 BossArt               `json:"-"`
	Eighth                                                                                                              *source.BossEightData `json:"eighth"`
	EighthCellArt, EighthExplosionArt, EighthSporeArt, EighthMessageArt                                                 BossArt               `json:"-"`
	Fourth                                                                                                              *source.BossFourData  `json:"fourth"`
	FourthArt                                                                                                           []BossArt             `json:"-"`
	FourthWeakArt, FourthTurretArt                                                                                      BossArt               `json:"-"`
	Six                                                                                                                 *source.BossSixData   `json:"six"`
	SixArt                                                                                                              BossArt               `json:"-"`
	Third                                                                                                               *source.BossThreeData `json:"third"`
	Second                                                                                                              *source.BossTwoData   `json:"second"`
	SecondArt                                                                                                           []BossArt             `json:"-"`
	Kind                                                                                                                string                `json:"kind"`
	TailTrigger                                                                                                         int                   `json:"tail_trigger"`
	TailDescriptor                                                                                                      uint32                `json:"tail_descriptor"`
	TailBaseX                                                                                                           int                   `json:"tail_base_x"`
	TailPeriod                                                                                                          int                   `json:"tail_period"`
	TailYOffset                                                                                                         int                   `json:"tail_y_offset"`
	TailFrames                                                                                                          [][][]int             `json:"tail_frames"`
	ControllerTrigger                                                                                                   int                   `json:"controller_trigger"`
	ControllerDescriptor                                                                                                uint32                `json:"controller_descriptor"`
	ControllerHealth                                                                                                    int                   `json:"controller_health"`
	ControllerCollision                                                                                                 CollisionFrame        `json:"controller_collision"`
	DamageThreshold                                                                                                     int                   `json:"damage_threshold"`
	DamageArenaTimer                                                                                                    int                   `json:"damage_arena_timer"`
	VictoryCounter                                                                                                      int                   `json:"victory_counter"`
	VictoryReload                                                                                                       int                   `json:"victory_reload"`
	VictoryTicks                                                                                                        int                   `json:"victory_ticks"`
	Score                                                                                                               int                   `json:"score"`
	HookTrigger                                                                                                         int                   `json:"hook_trigger"`
	FreezeScroll                                                                                                        int                   `json:"freeze_scroll"`
	FreezeTicks                                                                                                         int                   `json:"freeze_ticks"`
	EscapeScroll                                                                                                        int                   `json:"escape_scroll"`
	EyeFrames                                                                                                           [][]uint16            `json:"eye_frames"`
	MouthFrames                                                                                                         [][]uint16            `json:"mouth_frames"`
	EyeColumn                                                                                                           int                   `json:"eye_column"`
	EyeRow                                                                                                              int                   `json:"eye_row"`
	MouthColumn                                                                                                         int                   `json:"mouth_column"`
	MouthRow                                                                                                            int                   `json:"mouth_row"`
	EyePeriod                                                                                                           int                   `json:"eye_period"`
	MouthPeriod                                                                                                         int                   `json:"mouth_period"`
	ProjectileDescriptor                                                                                                uint32                `json:"projectile_descriptor"`
	ProjectileDelays                                                                                                    []int                 `json:"projectile_delays"`
	ProjectileStartX                                                                                                    int                   `json:"projectile_start_x"`
	ProjectileStartY                                                                                                    int                   `json:"projectile_start_y"`
	ProjectileTeleportX                                                                                                 int                   `json:"projectile_teleport_offset_x"`
	ProjectileMotionX                                                                                                   []int                 `json:"projectile_motion_x"`
	ProjectileMotionY                                                                                                   []int                 `json:"projectile_motion_y"`
	ProjectileStraightVX                                                                                                int                   `json:"projectile_straight_vx"`
	ProjectileAimLow                                                                                                    int                   `json:"projectile_aim_low"`
	ProjectileAimHigh                                                                                                   int                   `json:"projectile_aim_high"`
	AimMap                                                                                                              []int8                `json:"aim_map"`
	AimThresholds                                                                                                       []int                 `json:"aim_thresholds"`
	TailArt, ControllerArt, ProjectileArt                                                                               BossArt               `json:"-"`
}

// BossArt is resolved from the original descriptor and sprite manifest.
type BossArt struct {
	Sprite                        string
	Width, Height, AnimationTicks int
	Animation                     []int
	CollisionFrames               []CollisionFrame
}

// BossState describes the active encounter independently of presentation.
type BossState struct {
	Cave                                                      *OriginalCaveState
	Battleship                                                *OriginalBattleshipCoreState
	Iris                                                      *OriginalBattleshipIrisState
	MountedCursor                                             int
	Seven                                                     *OriginalBossSevenState
	Final                                                     *OriginalFinalBossState
	FinalPalette                                              []uint16
	BodiesRemaining                                           int
	BossIntro                                                 bool
	QueueCursor, QueueTimer                                   int
	SnakeRoute                                                int
	OrganPhase, OrganFrame, OrganTimer, OrganSteps, OrganWait int
	Clearing, Exiting, ReadyToAdvance, HitFlash               bool
	victoryCounter, victoryTimer, victoryStartTick            int
	// VictoryInitialized distinguishes disabled controls from an exit request
	// that has not yet reached the next common main-pass sequencer.
	VictoryInitialized                                         bool
	Active, ControllerActive, Defeated, Escaping, CameraPaused bool
	Health, DamageTaken, ArenaTicks, VictoryTicks              int
	TailX, TailFrame, TailTicks                                int
	EyeActive                                                  bool
	EyeFrame, EyeTicks                                         int
	MouthFrame                                                 int
	MouthTicks                                                 uint8
	ControllerID                                               uint64
	MusicSection                                               int
}

type bossRole uint8

const (
	bossNone bossRole = iota
	bossTail
	bossController
	bossProjectile
	bossTwoPart
	bossFourBody
	bossFourWeak
	bossFourTurret
	bossFourShot
	bossFiveController
	bossFivePart
	bossThreeMounted
	bossEightCell
	bossEightSpore
	bossEightMessage
	bossEightBrain
	bossSevenController
	bossSevenGuard
	bossSevenFalling
	bossThreeIris
	bossThreeCore
	bossThreeCoreShot
	bossCaveCreature
	bossCaveShot
)

func cloneBossSpec(original *BossSpec) *BossSpec {
	if original == nil {
		return nil
	}
	copyOf := *original
	copyOf.ControllerDeathKinds = append([]int(nil), original.ControllerDeathKinds...)
	copyOf.Seven = cloneOriginalData(original.Seven)
	copyOf.SevenVariantArt = append([]BossArt(nil), original.SevenVariantArt...)
	copyOf.Fifth = cloneOriginalData(original.Fifth)
	copyOf.Eighth = cloneOriginalData(original.Eighth)
	copyOf.Fourth = cloneOriginalData(original.Fourth)
	copyOf.FourthArt = append([]BossArt(nil), original.FourthArt...)
	copyOf.Six = cloneOriginalData(original.Six)
	copyOf.Third = cloneOriginalData(original.Third)
	if original.Second != nil {
		second := *original.Second
		second.DeathDelays = append([]int(nil), original.Second.DeathDelays...)
		second.FlashIndices = append([]int(nil), original.Second.FlashIndices...)
		second.FlashValues = append([]uint16(nil), original.Second.FlashValues...)
		second.NormalValues = append([]uint16(nil), original.Second.NormalValues...)
		second.ExplosionKinds = append([]int(nil), original.Second.ExplosionKinds...)
		second.Components = append([]source.BossTwoComponent(nil), original.Second.Components...)
		for index := range second.Components {
			second.Components[index].Animation = append([]int(nil), second.Components[index].Animation...)
		}
		second.Routes = append([]source.BossTwoRoute(nil), original.Second.Routes...)
		for index := range second.Routes {
			route := &second.Routes[index]
			route.MotionX = append([]int8(nil), route.MotionX...)
			route.MotionY = append([]int8(nil), route.MotionY...)
			route.Frames = append([]int(nil), route.Frames...)
		}
		second.DamagedAnimation = append([]int(nil), second.DamagedAnimation...)
		second.ControllerCollisions = append([]source.CollisionFrame(nil), second.ControllerCollisions...)
		second.Patches = append([][]uint16(nil), second.Patches...)
		for index := range second.Patches {
			second.Patches[index] = append([]uint16(nil), second.Patches[index]...)
		}
		copyOf.Second = &second
	}
	copyOf.SecondArt = append([]BossArt(nil), original.SecondArt...)
	copyOf.TailFrames = make([][][]int, len(original.TailFrames))
	for frame := range original.TailFrames {
		copyOf.TailFrames[frame] = make([][]int, len(original.TailFrames[frame]))
		for segment := range original.TailFrames[frame] {
			copyOf.TailFrames[frame][segment] = append([]int(nil), original.TailFrames[frame][segment]...)
		}
	}
	copyTiles := func(rows [][]uint16) [][]uint16 {
		out := make([][]uint16, len(rows))
		for row := range rows {
			out[row] = append([]uint16(nil), rows[row]...)
		}
		return out
	}
	copyOf.EyeFrames, copyOf.MouthFrames = copyTiles(original.EyeFrames), copyTiles(original.MouthFrames)
	copyOf.ProjectileDelays = append([]int(nil), original.ProjectileDelays...)
	copyOf.ProjectileMotionX = append([]int(nil), original.ProjectileMotionX...)
	copyOf.ProjectileMotionY = append([]int(nil), original.ProjectileMotionY...)
	copyOf.AimMap = append([]int8(nil), original.AimMap...)
	copyOf.AimThresholds = append([]int(nil), original.AimThresholds...)
	cloneArt := func(art BossArt) BossArt {
		art.Animation = append([]int(nil), art.Animation...)
		art.CollisionFrames = append([]CollisionFrame(nil), art.CollisionFrames...)
		return art
	}
	copyOf.TailArt, copyOf.ControllerArt, copyOf.ProjectileArt = cloneArt(original.TailArt), cloneArt(original.ControllerArt), cloneArt(original.ProjectileArt)
	copyOf.SixArt = cloneArt(original.SixArt)
	copyOf.EighthCellArt = cloneArt(original.EighthCellArt)
	copyOf.EighthExplosionArt = cloneArt(original.EighthExplosionArt)
	copyOf.EighthSporeArt = cloneArt(original.EighthSporeArt)
	copyOf.EighthMessageArt = cloneArt(original.EighthMessageArt)
	copyOf.SevenArt, copyOf.SevenGuardArt = cloneArt(original.SevenArt), cloneArt(original.SevenGuardArt)
	for index := range copyOf.SevenVariantArt {
		copyOf.SevenVariantArt[index] = cloneArt(copyOf.SevenVariantArt[index])
	}
	copyOf.FifthArt, copyOf.FifthPartArt = cloneArt(original.FifthArt), cloneArt(original.FifthPartArt)
	for index := range copyOf.SecondArt {
		copyOf.SecondArt[index] = cloneArt(copyOf.SecondArt[index])
	}
	for index := range copyOf.FourthArt {
		copyOf.FourthArt[index] = cloneArt(copyOf.FourthArt[index])
	}
	copyOf.FourthWeakArt, copyOf.FourthTurretArt = cloneArt(original.FourthWeakArt), cloneArt(original.FourthTurretArt)
	copyOf.FourthProjectileArt = cloneArt(original.FourthProjectileArt)
	return &copyOf
}
