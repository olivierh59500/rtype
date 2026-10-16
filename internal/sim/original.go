package sim

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"

	"github.com/olivierh59500/r-type/internal/source"
)

// OriginalEvent contains extracted data, never executable instructions.
// Original addresses identify the source of sprite and movement data only.
type OriginalEvent struct {
	Priority                     int                          `json:"priority"`
	AutomaticFire                int                          `json:"automatic_fire"`
	Address                      uint32                       `json:"address"`
	HuntingEmitter               *source.HuntingEmitterData   `json:"hunting_emitter"`
	ShieldVehicle                *source.ShieldVehicleData    `json:"shield_vehicle"`
	SpiralSentinel               *source.SpiralSentinelData   `json:"spiral_sentinel"`
	MiniTail                     *source.MiniTailData         `json:"mini_tail"`
	GroundWalker                 *source.GroundWalkerData     `json:"ground_walker"`
	BattleshipTurret             *source.BattleshipTurretData `json:"battleship_turret"`
	FireWorm                     *source.FireWormData         `json:"fire_worm"`
	GroundGunner                 *source.GroundGunnerData     `json:"ground_gunner"`
	Spitter                      *source.SpitterData          `json:"spitter"`
	WallCrawler                  *source.WallCrawlerData      `json:"wall_crawler"`
	Reactor                      *source.ReactorData          `json:"reactor"`
	QuarterTurner                *source.QuarterTurnerData    `json:"quarter_turner"`
	Stalker                      *source.StalkerData          `json:"stalker"`
	TilePulse                    *source.TilePulseData        `json:"tile_pulse"`
	Hopper                       *source.HopperData           `json:"hopper"`
	RouteCrawler                 *source.RouteCrawlerData     `json:"route_crawler"`
	FixedVX                      int                          `json:"fixed_vx"`
	FixedVY                      int                          `json:"fixed_vy"`
	StampWord                    uint16                       `json:"stamp_word"`
	StampProbeX                  int                          `json:"stamp_probe_x"`
	StampProbeY                  int                          `json:"stamp_probe_y"`
	CollisionFrames              []CollisionFrame             `json:"collision_frames"`
	Trigger                      int                          `json:"trigger"`
	X, Y                         int
	Parameter                    uint32 `json:"parameter"`
	Count, Aux                   int
	Descriptor                   uint32 `json:"descriptor"`
	SpawnAddress                 uint32 `json:"spawn_address"`
	UpdateAddress                uint32 `json:"update_address"`
	Health, Score, Width, Height int
	AnimationTicks               int                  `json:"animation_ticks"`
	Animation                    []int                `json:"animation"`
	Formation                    []OriginalFormation  `json:"formation"`
	MotionX                      []int8               `json:"motion_x"`
	MotionY                      []int8               `json:"motion_y"`
	MotionAnimation              []int                `json:"motion_animation"`
	MotionLoop                   bool                 `json:"motion_loop"`
	Behavior                     string               `json:"behavior"`
	Variant                      int                  `json:"variant"`
	LungeVX                      int                  `json:"lunge_vx"`
	LungeVY                      int                  `json:"lunge_vy"`
	AimMap                       []int8               `json:"aim_map"`
	AimThresholds                []int                `json:"aim_thresholds"`
	RelativeScroll               bool                 `json:"relative_scroll"`
	SpawnDelay                   int                  `json:"spawn_delay"`
	TurnTicks                    int                  `json:"turn_ticks"`
	HomingTurns                  int                  `json:"homing_turns"`
	AimAnimation                 []int                `json:"aim_animation"`
	AimOffsetsX                  []int                `json:"aim_offsets_x"`
	AimOffsetsY                  []int                `json:"aim_offsets_y"`
	ShotVX                       []int8               `json:"shot_vx"`
	ShotVY                       []int8               `json:"shot_vy"`
	ShootMin                     int                  `json:"shoot_min"`
	ShootMax                     int                  `json:"shoot_max"`
	Carrier                      *OriginalCarrierData `json:"carrier"`
}

// OriginalCarrierData contains original motion and animation tables only.
type OriginalCarrierData struct {
	Kind                   int    `json:"kind"`
	Velocity               int    `json:"velocity"`
	MotionY                []int8 `json:"motion_y"`
	AnimationFloat         []int  `json:"animation_float"`
	AnimationGround        []int  `json:"animation_ground"`
	AnimationFloatReverse  []int  `json:"animation_float_reverse"`
	AnimationGroundReverse []int  `json:"animation_ground_reverse"`
}

type OriginalFormation struct{ Y, Delay, Animation int }
type OriginalCheckpoint struct {
	Scroll       int    `json:"scroll"`
	EventAddress uint32 `json:"event_address"`
}
type OriginalStage struct {
	Palette               []uint16        `json:"palette"`
	Weapons               *source.Weapons `json:"weapons"`
	Boss                  *BossSpec       `json:"boss"`
	ScrollScript          []ScrollStep    `json:"scroll_script"`
	InitialScrollY        float64         `json:"initial_scroll_y"`
	Number, Width, Height int
	MapWidth              int                  `json:"map_width"`
	MapHeight             int                  `json:"map_height"`
	TileMap               []uint16             `json:"tile_map"`
	ScrollSpeed           float64              `json:"scroll_speed"`
	Events                []OriginalEvent      `json:"events"`
	Checkpoints           []OriginalCheckpoint `json:"checkpoints"`
	EventClock            string               `json:"event_clock"`
}

// RecoveryReport separates verified data from remaining implementation gaps.
type RecoveryReport struct {
	Stage                                             int
	RecoveredEvents, ApproximateEvents, MissingEvents int
	MissingSpriteWaves                                int
	MapObjects                                        int
	Notes                                             []string
}

// LoadStages adapts an extracted manifest to the independent Go engine.
func LoadStages(reader io.Reader) ([]StageSpec, []RecoveryReport, error) {
	var data struct {
		Levels  []OriginalStage `json:"levels"`
		Sprites []struct {
			Name           string `json:"name"`
			Address        uint32 `json:"address"`
			Level          int    `json:"level"`
			Frame          int    `json:"frame"`
			Width, Height  int
			CollisionFrame *CollisionFrame `json:"collision_frame"`
		} `json:"sprites"`
	}
	if err := json.NewDecoder(reader).Decode(&data); err != nil {
		return nil, nil, fmt.Errorf("read original stage manifest: %w", err)
	}
	if len(data.Levels) == 0 {
		return nil, nil, fmt.Errorf("manifest contains no original stages")
	}
	stages := make([]StageSpec, len(data.Levels))
	reports := make([]RecoveryReport, len(data.Levels))
	for index, original := range data.Levels {
		stage, report, err := StageFromOriginal(original)
		if err != nil {
			return nil, nil, err
		}
		stages[index], reports[index] = stage, report
		if stage.Boss != nil {
			if third := stage.Boss.Third; third != nil && (third.MountedHealth <= 0 || third.Iris == nil || third.Iris.Health <= 0) {
				return nil, nil, fmt.Errorf("stage %d battleship contact thresholds are missing; recreate runtime assets with cmd/extract", original.Number)
			}
			art := func(address uint32) BossArt {
				var result BossArt
				for _, sprite := range data.Sprites {
					if sprite.Address != address || sprite.Level != original.Number || !strings.HasPrefix(sprite.Name, "object-") {
						continue
					}
					end := strings.LastIndexByte(sprite.Name, '-')
					if end >= 0 {
						result.Sprite = sprite.Name[:end]
					}
					result.Width, result.Height = sprite.Width, sprite.Height
					if sprite.CollisionFrame != nil {
						for len(result.CollisionFrames) <= sprite.Frame {
							result.CollisionFrames = append(result.CollisionFrames, CollisionFrame{})
						}
						result.CollisionFrames[sprite.Frame] = *sprite.CollisionFrame
					}
				}
				for _, event := range original.Events {
					if event.Descriptor == address {
						result.Animation = event.Animation
						result.AnimationTicks = event.AnimationTicks
					}
				}
				return result
			}
			if stage.Boss.Six != nil {
				for _, queue := range stage.Boss.Six.Queues {
					if queue.ContactHealth <= 0 {
						return nil, nil, fmt.Errorf("stage %d queued crawler contact health is missing; recreate runtime assets with cmd/extract", original.Number)
					}
				}
				stages[index].Boss.SixArt = art(stage.Boss.Six.Descriptor)
			}
			if stage.Boss.Seven != nil {
				d := stage.Boss.Seven
				if d.ControllerHealth <= 0 || d.GuardHealth <= 0 {
					return nil, nil, fmt.Errorf("stage %d arena contact thresholds are missing; recreate runtime assets with cmd/extract", original.Number)
				}
				stages[index].Boss.SevenArt = art(d.Descriptor)
				stages[index].Boss.SevenGuardArt = art(d.GuardDescriptor)
				for _, variant := range d.Variants {
					stages[index].Boss.SevenVariantArt = append(stages[index].Boss.SevenVariantArt, art(variant.Descriptor))
				}
			}
			if stage.Boss.Fifth != nil {
				if stage.Boss.Fifth.CoreHealth <= 0 || stage.Boss.Fifth.PartHealth <= 0 || len(stage.Boss.Fifth.PartAnimation) == 0 || stage.Boss.Fifth.PartAnimationTicks <= 0 {
					return nil, nil, fmt.Errorf("stage %d Bellmite contact/animation data is missing; recreate runtime assets with cmd/extract", original.Number)
				}
				stages[index].Boss.FifthArt = art(stage.Boss.Fifth.Descriptor)
				stages[index].Boss.FifthPartArt = art(stage.Boss.Fifth.PartDescriptor)
				stages[index].Boss.FifthPartArt.Animation = stage.Boss.Fifth.PartAnimation
				stages[index].Boss.FifthPartArt.AnimationTicks = stage.Boss.Fifth.PartAnimationTicks
			}
			if stage.Boss.Eighth != nil {
				d := stage.Boss.Eighth
				if d.CellAnimationTicks <= 0 || d.CellExplosionStopIndex <= 0 || d.SporeAnimationTicks <= 0 || len(d.SporeAnimation) == 0 {
					return nil, nil, fmt.Errorf("stage %d final actor animation data is missing; recreate runtime assets with cmd/extract", original.Number)
				}
				stages[index].Boss.EighthCellArt = art(d.CellDescriptor)
				stages[index].Boss.EighthExplosionArt = art(d.CellExplosionDescriptor)
				stages[index].Boss.EighthSporeArt = art(d.SporeDescriptor)
				stages[index].Boss.EighthMessageArt = art(d.MessageDescriptor)
			}
			stages[index].Boss.TailArt = art(stage.Boss.TailDescriptor)
			stages[index].Boss.ControllerArt = art(stage.Boss.ControllerDescriptor)
			stages[index].Boss.ProjectileArt = art(stage.Boss.ProjectileDescriptor)
			if stage.Boss.Fourth != nil {
				stages[index].Boss.FourthProjectileArt = art(stage.Boss.Fourth.ProjectileDescriptor)
				stages[index].Boss.FourthProjectileArt.Animation = stage.Boss.Fourth.ProjectileAnimation
				stages[index].Boss.FourthProjectileArt.AnimationTicks = stage.Boss.Fourth.ProjectileAnimationTicks
				for _, body := range stage.Boss.Fourth.Bodies {
					stages[index].Boss.FourthArt = append(stages[index].Boss.FourthArt, art(body.Descriptor))
				}
				stages[index].Boss.FourthWeakArt = art(stage.Boss.Fourth.WeakpointDescriptor)
				stages[index].Boss.FourthWeakArt.Width, stages[index].Boss.FourthWeakArt.Height = 0, 0
				stages[index].Boss.FourthTurretArt = art(stage.Boss.Fourth.TurretDescriptor)
			}
			if stage.Boss.Second != nil {
				for _, component := range stage.Boss.Second.Components {
					stages[index].Boss.SecondArt = append(stages[index].Boss.SecondArt, art(component.Descriptor))
				}
				stages[index].Boss.ControllerArt.Width, stages[index].Boss.ControllerArt.Height = 0, 1
			}
			// The projectile spawner references a descriptor rather than an
			// event record; its original animation table enumerates all frames.
			projectile := &stages[index].Boss.ProjectileArt
			for frame := 0; frame < len(projectile.CollisionFrames); frame++ {
				projectile.Animation = append(projectile.Animation, frame)
			}
			projectile.AnimationTicks = 1
		}
		if len(data.Sprites) > 0 {
			visible := stage.Waves[:0]
			for _, wave := range stage.Waves {
				if wave.FireWorm != nil {
					art := func(address uint32) BossArt {
						var result BossArt
						for _, sprite := range data.Sprites {
							if sprite.Address != address || sprite.Level != original.Number {
								continue
							}
							end := strings.LastIndexByte(sprite.Name, '-')
							if end >= 0 {
								result.Sprite = sprite.Name[:end]
							}
							result.Width, result.Height = sprite.Width, sprite.Height
							if sprite.CollisionFrame != nil {
								for len(result.CollisionFrames) <= sprite.Frame {
									result.CollisionFrames = append(result.CollisionFrames, CollisionFrame{})
								}
								result.CollisionFrames[sprite.Frame] = *sprite.CollisionFrame
							}
						}
						return result
					}
					wave.WormHeadArt, wave.WormBodyArt, wave.WormTailArt = art(wave.FireWorm.HeadDescriptor), art(wave.FireWorm.BodyDescriptor), art(wave.FireWorm.TailDescriptor)
				}
				found := false
				for _, sprite := range data.Sprites {
					if sprite.Address != wave.Descriptor || (sprite.Level != original.Number && sprite.Level != 0) || !strings.HasPrefix(sprite.Name, "object-") {
						continue
					}
					end := strings.LastIndexByte(sprite.Name, '-')
					if end >= 0 {
						wave.Sprite = sprite.Name[:end]
						found = true
					}
					if sprite.Level == original.Number {
						break
					}
				}
				if found {
					visible = append(visible, wave)
				} else {
					reports[index].MissingSpriteWaves++
				}
			}
			stages[index].Waves = visible
		}
	}
	return stages, reports, nil
}

// StageFromOriginal reconstructs authored events, camera commands and native
// controller data. Unknown development fixtures are counted explicitly; every
// event from the supported original release has a translated native behavior.
func StageFromOriginal(original OriginalStage) (StageSpec, RecoveryReport, error) {
	report := RecoveryReport{Stage: original.Number, Notes: []string{"Player movement uses 50 Hz PAL interrupts; original enemy, weapon and map updates use a 25 Hz main loop.", "Separate boss and map controllers retain their authored data and timing."}}
	if original.MapWidth <= 0 || original.MapHeight <= 0 || original.MapWidth > 4096 || original.MapHeight > 256 || len(original.TileMap) != original.MapWidth*original.MapHeight {
		return StageSpec{}, report, fmt.Errorf("stage %d has invalid extracted tile map", original.Number)
	}
	speed := original.ScrollSpeed
	if speed <= 0 || !finite(speed) {
		speed = 1
		report.Notes = append(report.Notes, "Scroll script is approximated as one pixel per main-loop tick.")
	}
	stage := StageSpec{Name: fmt.Sprintf("Stage %d", original.Number), Source: "Original tile flags, event triggers, descriptors and native movement tables", Palette: append([]uint16(nil), original.Palette...), ScrollSpeed: speed, LengthTicks: int(math.Ceil(float64(original.MapWidth*16)/speed)) + TickRate, TileMap: append([]uint16(nil), original.TileMap...), MapWidth: original.MapWidth, MapHeight: original.MapHeight, Checkpoints: []Checkpoint{{}}, Boss: cloneBossSpec(original.Boss), ScrollScript: append([]ScrollStep(nil), original.ScrollScript...), InitialScrollY: original.InitialScrollY, Weapons: original.Weapons}
	for _, checkpoint := range original.Checkpoints {
		if checkpoint.Scroll > 0 {
			stage.Checkpoints = append(stage.Checkpoints, Checkpoint{Tick: int(math.Ceil(float64(checkpoint.Scroll) / speed)), Scroll: float64(checkpoint.Scroll), EventAddress: checkpoint.EventAddress})
		} else if checkpoint.Scroll == 0 {
			stage.Checkpoints[0].EventAddress = checkpoint.EventAddress
		}
	}
	sort.Slice(stage.Checkpoints, func(a, b int) bool { return stage.Checkpoints[a].Tick < stage.Checkpoints[b].Tick })
	unique := stage.Checkpoints[:0]
	for _, checkpoint := range stage.Checkpoints {
		if len(unique) == 0 || checkpoint.Tick > unique[len(unique)-1].Tick {
			unique = append(unique, checkpoint)
		}
	}
	stage.Checkpoints = unique
	for eventIndex, event := range original.Events {
		if stage.Boss != nil && stage.Boss.Fifth != nil && event.Descriptor == stage.Boss.Fifth.Descriptor {
			report.RecoveredEvents++
			continue
		}
		if stage.Boss != nil && stage.Boss.Kind == "stage2_gomander" && event.SpawnAddress == 0x4a99c {
			report.RecoveredEvents++
			continue
		}
		if stage.Boss != nil && (event.Descriptor == stage.Boss.TailDescriptor || event.Descriptor == stage.Boss.ControllerDescriptor) {
			report.RecoveredEvents++
			continue
		}
		tick := int(math.Ceil(float64(event.Trigger) / speed))
		if original.EventClock == "tick" {
			tick = event.Trigger
		}
		if tick < 0 {
			report.MissingEvents++
			continue
		}
		if tick >= stage.LengthTicks {
			stage.LengthTicks = tick + TickRate*2
		}
		health := event.Health
		if health < 1 {
			health = 1
		}
		width, height := float64(event.Width), float64(event.Height)
		if width <= 0 {
			width = 32
		}
		if height <= 0 {
			height = 16
		}
		wave := Wave{Priority: event.Priority, AutomaticFire: event.AutomaticFire, OriginalAddress: event.Address, Tick: tick, Kind: EnemyFighter, Health: health, Score: event.Score, W: width, H: height, Descriptor: event.Descriptor, Sprite: fmt.Sprintf("object-%d-%05x", original.Number, event.Descriptor), Animation: event.Animation, AnimationTicks: event.AnimationTicks}
		wave.Trigger, wave.Group = event.Trigger, eventIndex+1
		wave.TriggerClock = original.EventClock
		if wave.TriggerClock == "" {
			wave.TriggerClock = "scroll"
		}
		wave.CollisionFrames = append([]CollisionFrame(nil), event.CollisionFrames...)
		wave.X, wave.Y = float64(event.X)-GameOriginX+width/2, float64(event.Y)-GameOriginY+height/2
		if event.HuntingEmitter != nil {
			wave.Behavior, wave.HuntingEmitter = BehaviorHuntingEmitter, event.HuntingEmitter
			stage.Waves = append(stage.Waves, wave)
			report.RecoveredEvents++
			continue
		}
		if event.ShieldVehicle != nil {
			wave.Behavior, wave.ShieldVehicle, wave.Invincible = BehaviorShieldVehicle, event.ShieldVehicle, true
			stage.Waves = append(stage.Waves, wave)
			report.RecoveredEvents++
			continue
		}
		if event.SpiralSentinel != nil {
			wave.Behavior, wave.SpiralSentinel, wave.Invincible = BehaviorSpiralSentinel, event.SpiralSentinel, true
			stage.Waves = append(stage.Waves, wave)
			report.RecoveredEvents++
			continue
		}
		wave.Invincible = event.Health < 0
		if event.Descriptor == 0 {
			report.MissingEvents++
			continue
		}
		wave.X = float64(event.X) - GameOriginX + width/2
		wave.Y = float64(event.Y) - GameOriginY + height/2
		if event.FireWorm != nil {
			wave.FireWorm = event.FireWorm
			wave.Behavior = BehaviorFireWorm
			stage.Waves = append(stage.Waves, wave)
			report.RecoveredEvents++
			continue
		}
		if event.MiniTail != nil {
			wave.MiniTail = event.MiniTail
			wave.Behavior = BehaviorMiniTail
			stage.Waves = append(stage.Waves, wave)
			report.RecoveredEvents++
			continue
		}
		if event.Behavior == "ground_gunner" && event.GroundGunner != nil {
			wave.Behavior = BehaviorGroundGunner
			wave.GroundGunner = event.GroundGunner
			stage.Waves = append(stage.Waves, wave)
			report.RecoveredEvents++
			continue
		}
		if event.Behavior == "ground_walker" && event.GroundWalker != nil {
			wave.Behavior = BehaviorGroundWalker
			wave.GroundWalker = event.GroundWalker
			stage.Waves = append(stage.Waves, wave)
			report.RecoveredEvents++
			continue
		}
		if event.Behavior == "battleship_respawning_turret" && event.BattleshipTurret != nil {
			wave.Behavior = BehaviorRespawningTurret
			wave.BattleshipTurret = event.BattleshipTurret
			wave.Health = 8000
			stage.Waves = append(stage.Waves, wave)
			report.RecoveredEvents++
			continue
		}
		if event.Behavior == "spitter" && event.Spitter != nil {
			wave.Behavior = BehaviorSpitter
			wave.Spitter = event.Spitter
			wave.Health = event.Spitter.Health
			if event.Spitter.SpawnCount > 0 {
				for member := 0; member < event.Spitter.SpawnCount; member++ {
					spawn := wave
					spawn.X = float64(event.Spitter.SpawnX) - GameOriginX + width/2
					spawn.MoveDelay = member * event.Spitter.SpawnDelayStep
					cycle := event.Spitter.SpawnAnimationCycle
					if cycle > 0 {
						spawn.Variant = member % cycle
					}
					stage.Waves = append(stage.Waves, spawn)
				}
			} else {
				stage.Waves = append(stage.Waves, wave)
			}
			report.RecoveredEvents++
			continue
		}
		if event.Behavior == "wall_crawler" && event.WallCrawler != nil {
			wave.Behavior = BehaviorWallCrawler
			wave.WallCrawler = event.WallCrawler
			stage.Waves = append(stage.Waves, wave)
			report.RecoveredEvents++
			continue
		}
		if event.Behavior == "battleship_reactor" && event.Reactor != nil {
			wave.Behavior = BehaviorBattleshipReactor
			wave.Reactor = event.Reactor
			stage.Waves = append(stage.Waves, wave)
			report.RecoveredEvents++
			continue
		}
		if event.Behavior == "quarter_turner" && event.QuarterTurner != nil {
			wave.Behavior = BehaviorQuarterTurner
			wave.QuarterTurner = event.QuarterTurner
			wave.Variant = event.QuarterTurner.InitialDirection
			wave.AnimationTicks = event.QuarterTurner.FrameTicks
			stage.Waves = append(stage.Waves, wave)
			report.RecoveredEvents++
			continue
		}
		if event.Behavior == "stalker" && event.Stalker != nil {
			wave.Behavior = BehaviorStalker
			wave.Stalker = event.Stalker
			for _, member := range event.Stalker.Members {
				spawn := wave
				spawn.X = float64(member.X) - GameOriginX + width/2
				spawn.Y = float64(member.Y) - GameOriginY + height/2
				spawn.MoveDelay = member.Delay
				stage.Waves = append(stage.Waves, spawn)
			}
			report.RecoveredEvents++
			continue
		}
		if event.Behavior == "tile_pulse" && event.TilePulse != nil {
			wave.Behavior = BehaviorTilePulse
			wave.TilePulse = event.TilePulse
			wave.Hidden = true
			wave.NoCollidable = true
			stage.Waves = append(stage.Waves, wave)
			report.RecoveredEvents++
			continue
		}
		if event.Behavior == "ground_hopper" && event.Hopper != nil {
			wave.Behavior = BehaviorGroundHopper
			wave.Hopper = event.Hopper
			stage.Waves = append(stage.Waves, wave)
			report.RecoveredEvents++
			continue
		}
		if event.Behavior == "route_crawler" && event.RouteCrawler != nil {
			if event.RouteCrawler.ContactHealth <= 0 {
				return stage, report, fmt.Errorf("stage %d crawler contact health is missing; recreate runtime assets with cmd/extract", original.Number)
			}
			wave.Behavior = BehaviorRouteCrawler
			wave.RouteCrawler = event.RouteCrawler
			wave.Variant = event.RouteCrawler.InitialDirection
			stage.Waves = append(stage.Waves, wave)
			report.RecoveredEvents++
			continue
		}
		if event.Behavior == "terrain_stamp" {
			wave.Behavior = BehaviorTerrainStamp
			wave.VX = float64(event.FixedVX)
			wave.VY = float64(event.FixedVY)
			wave.StampWord = event.StampWord
			wave.StampProbeX = event.StampProbeX
			wave.StampProbeY = event.StampProbeY
			wave.MotionX = event.MotionX
			wave.MotionY = event.MotionY
			wave.MotionAnimation = event.MotionAnimation
			wave.MotionLoop = false
			stage.Waves = append(stage.Waves, wave)
			report.RecoveredEvents++
			continue
		}
		if event.Behavior == "power_carrier" && event.Carrier != nil && len(event.Carrier.MotionY) == 58 {
			carrier := event.Carrier
			wave.Kind = EnemyCarrier
			wave.Behavior = BehaviorPowerCarrier
			wave.Health = 1
			wave.CarrierKind = PowerUpKind(carrier.Kind + 1)
			// Original color IDs are blue, red, yellow. Public controls and
			// presentation use the stable red, blue, yellow Go enumeration.
			if carrier.Kind == 0 {
				wave.CarrierKind = PowerBlue
			}
			if carrier.Kind == 1 {
				wave.CarrierKind = PowerRed
			}
			wave.Drop = wave.CarrierKind
			wave.CarrierVX = carrier.Velocity
			wave.CarrierInitialVX = carrier.Velocity
			wave.CarrierMotionTick = 43
			wave.CarrierFloatAnimation = carrier.AnimationFloat
			wave.CarrierGroundAnimation = carrier.AnimationGround
			wave.CarrierAnimationReverseFloat = carrier.AnimationFloatReverse
			wave.CarrierAnimationReverseGround = carrier.AnimationGroundReverse
			wave.MotionY = carrier.MotionY
			stage.Waves = append(stage.Waves, wave)
			report.RecoveredEvents++
			continue
		}
		if event.Behavior == "mounted_turret" && len(event.AimMap) >= 20 && len(event.AimThresholds) >= 5 {
			wave.Kind = EnemyTurret
			wave.Behavior = BehaviorMountedTurret
			wave.AimMap = event.AimMap
			wave.AimThresholds = event.AimThresholds
			wave.AimAnimation = event.AimAnimation
			wave.AimOffsetsX = event.AimOffsetsX
			wave.AimOffsetsY = event.AimOffsetsY
			wave.ShotVX = event.ShotVX
			wave.ShotVY = event.ShotVY
			wave.ShootMin = event.ShootMin
			wave.ShootMax = event.ShootMax
			stage.Waves = append(stage.Waves, wave)
			report.RecoveredEvents++
			continue
		}
		if event.Behavior == "approach_lunge" {
			wave.Behavior = BehaviorApproachLunge
			wave.Variant = event.Variant
			wave.LungeVX = float64(event.LungeVX)
			wave.LungeVY = float64(event.LungeVY)
			stage.Waves = append(stage.Waves, wave)
			report.RecoveredEvents++
			continue
		}
		if event.Behavior == "homing" && len(event.MotionX) >= 16 && len(event.MotionY) >= 16 && len(event.AimMap) >= 20 && len(event.AimThresholds) >= 5 {
			wave.Behavior = BehaviorHoming
			wave.Variant = event.Variant
			wave.MotionX = event.MotionX
			wave.MotionY = event.MotionY
			wave.AimMap = event.AimMap
			wave.AimThresholds = event.AimThresholds
			wave.TurnTicks = event.TurnTicks
			wave.HomingTurns = event.HomingTurns
			wave.HomingClampY = event.HomingTurns == 0
			stage.Waves = append(stage.Waves, wave)
			report.RecoveredEvents++
			continue
		}
		if event.Behavior == "motion_single" && len(event.MotionX) > 0 && len(event.MotionY) > 0 {
			wave.MotionX = event.MotionX
			wave.MotionY = event.MotionY
			wave.MotionLoop = event.MotionLoop
			wave.MotionAnimation = event.MotionAnimation
			if event.RelativeScroll {
				wave.VX = -speed
			}
			stage.Waves = append(stage.Waves, wave)
			report.RecoveredEvents++
			continue
		}
		if len(event.Formation) > 0 && len(event.MotionY) > 0 {
			for _, member := range event.Formation {
				spawn := wave
				spawn.X = 368 - GameOriginX + width/2
				spawn.Y = float64(member.Y) - GameOriginY + height/2
				spawn.VX = -4
				spawn.MoveDelay = member.Delay
				spawn.AnimationStart = member.Animation
				spawn.MotionY = append([]int8(nil), event.MotionY...)
				spawn.MotionLoop = true
				spawn.Behavior = BehaviorSineFormation
				stage.Waves = append(stage.Waves, spawn)
			}
			report.RecoveredEvents++
			continue
		}
		if len(event.MotionX) > 0 && len(event.MotionY) > 0 {
			count := event.Count + 1
			if count < 1 || count > 32 {
				report.MissingEvents++
				continue
			}
			for member := 0; member < count; member++ {
				spawn := wave
				spawn.X = float64(event.X) - GameOriginX + width/2
				spawn.Y = float64(event.Y) - GameOriginY + height/2
				spawn.VX = float64(event.Aux)
				delay := event.SpawnDelay
				if delay <= 0 {
					delay = 7
				}
				spawn.MoveDelay = member * delay
				spawn.DriftDuringDelay = true
				spawn.MotionX = append([]int8(nil), event.MotionX...)
				spawn.MotionY = append([]int8(nil), event.MotionY...)
				spawn.MotionAnimation = append([]int(nil), event.MotionAnimation...)
				spawn.MotionLoop = event.MotionLoop
				stage.Waves = append(stage.Waves, spawn)
			}
			report.RecoveredEvents++
			continue
		}
		// Unknown spawn code currently becomes one provisional object at the
		// recovered coordinates, rather than fabricated original formation data.
		wave.X = float64(event.X) - GameOriginX + width/2
		wave.Y = float64(event.Y) - GameOriginY + height/2
		if event.X == 0 && event.Y == 0 {
			report.MissingEvents++
			continue
		}
		wave.VX = -speed
		if health >= 6 {
			wave.Kind = EnemyCarrier
			wave.Drop = PowerRed
		}
		if health >= 40 {
			wave.Kind = EnemyBoss
			wave.Drop = PowerNone
		}
		stage.Waves = append(stage.Waves, wave)
		report.ApproximateEvents++
	}
	if len(original.Events) == 0 {
		report.Notes = append(report.Notes, "No enemy event tables were supplied; this stage has original terrain only.")
	}
	sort.SliceStable(stage.Waves, func(a, b int) bool { return stage.Waves[a].Tick < stage.Waves[b].Tick })
	return stage, report, nil
}

func motionIndex(tick, length int, loop bool) int {
	if loop {
		return tick % length
	}
	if tick >= length {
		return length - 1
	}
	return tick
}
