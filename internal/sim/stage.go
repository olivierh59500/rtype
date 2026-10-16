package sim

import (
	"errors"
	"fmt"
	"math"

	"github.com/olivierh59500/r-type/internal/source"
)

// Wave is a single timed spawn, measured in original main-loop updates. The
// default PAL game advances one main-loop update per two 50 Hz World steps.
type Wave struct {
	Priority                                                     int
	AutomaticFire                                                int
	OriginalAddress                                              uint32
	HuntingEmitter                                               *source.HuntingEmitterData
	ShieldVehicle                                                *source.ShieldVehicleData
	SpiralSentinel                                               *source.SpiralSentinelData
	MiniTail                                                     *source.MiniTailData
	GroundWalker                                                 *source.GroundWalkerData
	BattleshipTurret                                             *source.BattleshipTurretData
	FireWorm                                                     *source.FireWormData
	WormHeadArt, WormBodyArt, WormTailArt                        BossArt
	GroundGunner                                                 *source.GroundGunnerData
	Spitter                                                      *source.SpitterData
	WallCrawler                                                  *source.WallCrawlerData
	Reactor                                                      *source.ReactorData
	QuarterTurner                                                *source.QuarterTurnerData
	Stalker                                                      *source.StalkerData
	TilePulse                                                    *source.TilePulseData
	TargetX, TargetY                                             int
	NoCollidable                                                 bool
	Hopper                                                       *source.HopperData
	RouteCrawler                                                 *source.RouteCrawlerData
	StampWord                                                    uint16
	StampProbeX, StampProbeY                                     int
	Trigger, Group                                               int
	TriggerClock                                                 string
	Hidden                                                       bool
	CollisionFrames                                              []CollisionFrame
	Tick                                                         int
	Kind                                                         EnemyKind
	X, Y, VX, VY                                                 float64
	Health, Score, Pattern                                       int
	Drop                                                         PowerUpKind
	W, H                                                         float64
	Sprite                                                       string
	Descriptor                                                   uint32
	Animation                                                    []int
	AnimationTicks                                               int
	AnimationStart                                               int
	MotionX, MotionY                                             []int8
	MoveDelay                                                    int
	MotionLoop                                                   bool
	MotionAnimation                                              []int
	DriftDuringDelay                                             bool
	Behavior                                                     EnemyBehavior
	Variant                                                      int
	LungeVX, LungeVY                                             float64
	AimMap                                                       []int8
	AimThresholds                                                []int
	TurnTicks, HomingTurns                                       int
	HomingClampY, Invincible                                     bool
	AimAnimation                                                 []int
	AimOffsetsX, AimOffsetsY                                     []int
	ShotVX, ShotVY                                               []int8
	ShootMin, ShootMax                                           int
	CarrierKind                                                  PowerUpKind
	CarrierVX, CarrierInitialVX, CarrierPhase, CarrierMotionTick int
	CarrierFloatAnimation, CarrierGroundAnimation                []int
	CarrierAnimationReverseFloat, CarrierAnimationReverseGround  []int
}

// Checkpoint restores the stage timeline and scroll after the player dies.
type Checkpoint struct {
	EventAddress uint32
	Tick         int
	Scroll       float64
}

// StageSpec is a declarative level description. Terrain coordinates are stage
// coordinates: screen X = terrain X - World.Scroll. Source must identify whether
// the description is extracted, observed, or provisional.
type StageSpec struct {
	Cave                *source.CaveData
	CaveShotArt         BossArt
	Palette             []uint16
	Weapons             *source.Weapons
	CheckpointReset     *source.StageCheckpointReset
	ScrollScript        []ScrollStep
	InitialScrollY      float64
	Boss                *BossSpec
	Name, Source        string
	LengthTicks         int
	ScrollSpeed         float64
	Checkpoints         []Checkpoint
	Waves               []Wave
	Terrain             []Rect
	TileMap             []uint16
	MapWidth, MapHeight int
}

// DefaultStages provides bounded development scenarios for all eight stages.
// These are NOT the original wave tables, enemy AI, map geometry, or timing.
// Their replacement requires verified extraction or frame-by-frame comparison.
func DefaultStages() []StageSpec {
	names := []string{"The Encounter", "The Cave", "The Battleship", "The Base", "The Den", "The Transport", "The City", "The Bydo Empire"}
	stages := make([]StageSpec, len(names))
	for index, name := range names {
		stage := StageSpec{Name: name, Source: "Provisional development script; not recovered from the Amiga disk", LengthTicks: 3300, ScrollSpeed: 0.75,
			Checkpoints: []Checkpoint{{Tick: 0, Scroll: 0}, {Tick: 1500, Scroll: 1125}}}
		// A short first carrier gives every checkpoint a way to regain the Force.
		for _, tick := range []int{180, 1650} {
			stage.Waves = append(stage.Waves, Wave{Tick: tick, Kind: EnemyCarrier, X: Width + 20, Y: 92, VX: -1.3, Health: 2, Score: 200, Drop: PowerRed})
		}
		for group := 0; group < 15; group++ {
			tick := 300 + group*165
			y := float64(32 + (group*37+index*17)%110)
			for member := 0; member < 5; member++ {
				stage.Waves = append(stage.Waves, Wave{Tick: tick + member*15, Kind: EnemyFighter, X: Width + 12, Y: y, VX: -1.8 - float64(index)*0.08, Health: 1, Score: 100, Pattern: (group + index) % 3})
			}
			if group%3 == 1 {
				stage.Waves = append(stage.Waves, Wave{Tick: tick + 70, Kind: EnemyTurret, X: Width + 12, Y: 174, VX: -0.75, Health: 3 + index/2, Score: 200})
			}
		}
		items := []PowerUpKind{PowerSpeed, PowerBlue, PowerMissiles, PowerYellow, PowerBit}
		for item, kind := range items {
			stage.Waves = append(stage.Waves, Wave{Tick: 650 + item*420, Kind: EnemyCarrier, X: Width + 20, Y: float64(50 + item%3*35), VX: -1.15, Health: 2, Score: 200, Drop: kind})
		}
		// Deliberately simple stage-specific collision geometry. No fabricated
		// terrain imagery is implied by these rectangles.
		for obstacle := 0; obstacle < 5; obstacle++ {
			x := float64(500 + obstacle*340)
			height := float64(16 + (index+obstacle)%3*8)
			if (index+obstacle)%2 == 0 {
				stage.Terrain = append(stage.Terrain, Rect{x, 0, 88, height})
			} else {
				stage.Terrain = append(stage.Terrain, Rect{x, PlayHeight - height, 88, height})
			}
		}
		stage.Waves = append(stage.Waves, Wave{Tick: 2900, Kind: EnemyBoss, X: Width + 50, Y: 92, VX: -0.75, Health: 80 + index*15, Score: 5000, Pattern: index})
		stages[index] = stage
	}
	return stages
}

// SetStages validates and copies level data. It may only be changed outside an
// active game, because changing a running timeline makes replays ambiguous.
func (w *World) SetStages(stages []StageSpec) error {
	if w.State == Playing || w.State == Paused {
		return errors.New("cannot replace stages during a game")
	}
	if len(stages) == 0 {
		return errors.New("at least one stage is required")
	}
	copyOf := make([]StageSpec, len(stages))
	for index, stage := range stages {
		if stage.Weapons != nil && stage.Weapons.StageResults != nil {
			d := stage.Weapons.StageResults
			if d.CPUClockHz <= 0 || d.PALClockHz <= 0 || d.ClearCycles <= 0 || d.PrintCharacterCycles <= 0 || d.GlyphRowCycles <= 0 || d.GlyphPlaneCycles <= 0 || len(d.FontCharacters) == 0 {
				return fmt.Errorf("stage %d completion timing is missing; recreate runtime assets with cmd/extract", index+1)
			}
		}
		if stage.LengthTicks <= 0 || stage.ScrollSpeed < 0 || math.IsNaN(stage.ScrollSpeed) || math.IsInf(stage.ScrollSpeed, 0) {
			return fmt.Errorf("stage %d has invalid duration or scroll speed", index+1)
		}
		if len(stage.Checkpoints) == 0 || stage.Checkpoints[0].Tick != 0 || stage.Checkpoints[0].Scroll != 0 {
			return fmt.Errorf("stage %d must start with checkpoint zero", index+1)
		}
		for i, checkpoint := range stage.Checkpoints {
			if checkpoint.Tick < 0 || checkpoint.Tick >= stage.LengthTicks || checkpoint.Scroll < 0 || math.IsNaN(checkpoint.Scroll) || math.IsInf(checkpoint.Scroll, 0) || (i > 0 && checkpoint.Tick <= stage.Checkpoints[i-1].Tick) {
				return fmt.Errorf("stage %d has invalid checkpoint", index+1)
			}
		}
		for _, wave := range stage.Waves {
			if wave.Tick < 0 || wave.Tick >= stage.LengthTicks || wave.Kind > EnemyBoss || wave.Health <= 0 || wave.Score < 0 || wave.Drop > PowerBit || wave.W < 0 || wave.H < 0 || wave.MoveDelay < 0 || wave.AnimationTicks < 0 || !finite(wave.X, wave.Y, wave.VX, wave.VY, wave.W, wave.H) {
				return fmt.Errorf("stage %d has invalid wave", index+1)
			}
		}
		for _, rect := range stage.Terrain {
			if rect.W <= 0 || rect.H <= 0 || !finite(rect.X, rect.Y, rect.W, rect.H) {
				return fmt.Errorf("stage %d has invalid terrain", index+1)
			}
		}
		copyOf[index] = stage
		if err := validateStageCave(stage, index+1); err != nil {
			return err
		}
		copyOf[index].Boss = cloneBossSpec(stage.Boss)
		copyOf[index].Cave = cloneOriginalData(stage.Cave)
		copyOf[index].CaveShotArt.CollisionFrames = append([]CollisionFrame(nil), stage.CaveShotArt.CollisionFrames...)
		copyOf[index].Weapons = cloneOriginalData(stage.Weapons)
		if err := validateStageCheckpointReset(stage, index+1); err != nil {
			return err
		}
		copyOf[index].CheckpointReset = cloneOriginalData(stage.CheckpointReset)
		copyOf[index].ScrollScript = append([]ScrollStep(nil), stage.ScrollScript...)
		if len(stage.TileMap) > 0 && (stage.MapWidth <= 0 || stage.MapHeight <= 0 || stage.MapWidth > 4096 || stage.MapHeight > 256 || len(stage.TileMap) != stage.MapWidth*stage.MapHeight) {
			return fmt.Errorf("stage %d has invalid tile map", index+1)
		}
		copyOf[index].Checkpoints = append([]Checkpoint(nil), stage.Checkpoints...)
		copyOf[index].Waves = append([]Wave(nil), stage.Waves...)
		for i := range copyOf[index].Waves {
			copyOf[index].Waves[i].Animation = append([]int(nil), stage.Waves[i].Animation...)
			copyOf[index].Waves[i].MotionX = append([]int8(nil), stage.Waves[i].MotionX...)
			copyOf[index].Waves[i].MotionY = append([]int8(nil), stage.Waves[i].MotionY...)
			copyOf[index].Waves[i].MotionAnimation = append([]int(nil), stage.Waves[i].MotionAnimation...)
			copyOf[index].Waves[i].CollisionFrames = append([]CollisionFrame(nil), stage.Waves[i].CollisionFrames...)
			copyOf[index].Waves[i].Hopper = cloneOriginalData(stage.Waves[i].Hopper)
			copyOf[index].Waves[i].RouteCrawler = cloneOriginalData(stage.Waves[i].RouteCrawler)
			copyOf[index].Waves[i].Stalker = cloneOriginalData(stage.Waves[i].Stalker)
			copyOf[index].Waves[i].TilePulse = cloneOriginalData(stage.Waves[i].TilePulse)
			copyOf[index].Waves[i].QuarterTurner = cloneOriginalData(stage.Waves[i].QuarterTurner)
			copyOf[index].Waves[i].Reactor = cloneOriginalData(stage.Waves[i].Reactor)
			copyOf[index].Waves[i].GroundWalker = cloneOriginalData(stage.Waves[i].GroundWalker)
			copyOf[index].Waves[i].BattleshipTurret = cloneOriginalData(stage.Waves[i].BattleshipTurret)
			copyOf[index].Waves[i].Spitter = cloneOriginalData(stage.Waves[i].Spitter)
			copyOf[index].Waves[i].WallCrawler = cloneOriginalData(stage.Waves[i].WallCrawler)
			copyOf[index].Waves[i].GroundGunner = cloneOriginalData(stage.Waves[i].GroundGunner)
			copyOf[index].Waves[i].FireWorm = cloneOriginalData(stage.Waves[i].FireWorm)
			copyOf[index].Waves[i].HuntingEmitter = cloneOriginalData(stage.Waves[i].HuntingEmitter)
			copyOf[index].Waves[i].ShieldVehicle = cloneOriginalData(stage.Waves[i].ShieldVehicle)
			copyOf[index].Waves[i].SpiralSentinel = cloneOriginalData(stage.Waves[i].SpiralSentinel)
			copyOf[index].Waves[i].MiniTail = cloneOriginalData(stage.Waves[i].MiniTail)
			copyOf[index].Waves[i].CarrierFloatAnimation = append([]int(nil), stage.Waves[i].CarrierFloatAnimation...)
			copyOf[index].Waves[i].CarrierGroundAnimation = append([]int(nil), stage.Waves[i].CarrierGroundAnimation...)
			copyOf[index].Waves[i].CarrierAnimationReverseFloat = append([]int(nil), stage.Waves[i].CarrierAnimationReverseFloat...)
			copyOf[index].Waves[i].CarrierAnimationReverseGround = append([]int(nil), stage.Waves[i].CarrierAnimationReverseGround...)
			copyOf[index].Waves[i].AimMap = append([]int8(nil), stage.Waves[i].AimMap...)
			copyOf[index].Waves[i].AimThresholds = append([]int(nil), stage.Waves[i].AimThresholds...)
			copyOf[index].Waves[i].AimAnimation = append([]int(nil), stage.Waves[i].AimAnimation...)
			copyOf[index].Waves[i].AimOffsetsX = append([]int(nil), stage.Waves[i].AimOffsetsX...)
			copyOf[index].Waves[i].AimOffsetsY = append([]int(nil), stage.Waves[i].AimOffsetsY...)
			copyOf[index].Waves[i].ShotVX = append([]int8(nil), stage.Waves[i].ShotVX...)
			copyOf[index].Waves[i].ShotVY = append([]int8(nil), stage.Waves[i].ShotVY...)
		}
		copyOf[index].Terrain = append([]Rect(nil), stage.Terrain...)
		copyOf[index].TileMap = append([]uint16(nil), stage.TileMap...)
		copyOf[index].Palette = append([]uint16(nil), stage.Palette...)
	}
	w.Stages = copyOf
	return nil
}

func finite(values ...float64) bool {
	for _, value := range values {
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return false
		}
	}
	return true
}

// TerrainInView returns screen-space collision rectangles for the active stage.
func (w *World) TerrainInView() []Rect {
	if w.Stage < 1 || w.Stage > len(w.Stages) {
		return nil
	}
	var visible []Rect
	for _, rect := range w.Stages[w.Stage-1].Terrain {
		rect.X -= w.Scroll
		rect.Y -= w.ScrollY
		if rect.X+rect.W > 0 && rect.X < Width {
			visible = append(visible, rect)
		}
	}
	stage := w.Stages[w.Stage-1]
	if len(stage.TileMap) > 0 {
		first := int(w.Scroll) / 16
		last := first + Width/16 + 1
		if first < 0 {
			first = 0
		}
		if last > stage.MapWidth {
			last = stage.MapWidth
		}
		for row := 0; row < stage.MapHeight; row++ {
			for column := first; column < last; column++ {
				// Core 0x832 tests bit 15 of the original 16-pixel tile word.
				if w.RawTileAt(column, row)&0x8000 != 0 {
					visible = append(visible, Rect{X: float64(column*16) + 16 - w.Scroll, Y: float64(row*16) - w.ScrollY, W: 16, H: 16})
				}
			}
		}
	}
	return visible
}

func (w *World) spawnWaves() {
	group := -1
	for index, wave := range w.Stages[w.Stage-1].Waves {
		if wave.TriggerClock == "" {
			if wave.Tick != w.StageTick {
				continue
			}
		} else {
			if index < w.spawnCursor {
				continue
			}
			clock := int(w.Scroll)
			if wave.TriggerClock == "tick" {
				clock = w.StageTick
				if w.sceneRules() != nil {
					clock = w.EventTick
				}
			}
			if clock < wave.Trigger {
				break
			}
			if group >= 0 && group != wave.Group {
				break
			}
			group = wave.Group
			w.spawnCursor = index + 1
		}
		if wave.FireWorm != nil {
			w.spawnOriginalWorm(wave)
			continue
		}
		if wave.MiniTail != nil {
			w.spawnOriginalMiniTails(wave)
			continue
		}
		if boss := w.Stages[w.Stage-1].Boss; boss != nil && boss.Eighth != nil && wave.Descriptor == boss.Eighth.SporeDescriptor {
			// Source 0x4A8BE uses event coordinates/direction and a zero wait;
			// arena 0x4A8E4 uses fixed coordinates and waits twenty updates.
			// Both then use the shared, mutable original turn reload.
			w.spawnFinalSporeAt(boss, int(wave.X-wave.W/2)+GameOriginX, int(wave.Y-wave.H/2)+GameOriginY, wave.Variant, 0)
			continue
		}
		width, height := 16.0, 14.0
		if wave.Kind == EnemyCarrier {
			width, height = 24, 18
		}
		if wave.Kind == EnemyTurret {
			width, height = 20, 18
		}
		if wave.Kind == EnemyBoss {
			width, height = 72, 96
		}
		if wave.W > 0 {
			width = wave.W
		}
		if wave.H > 0 {
			height = wave.H
		}
		enemy := Enemy{
			Priority:      wave.Priority,
			AutomaticFire: wave.AutomaticFire,
			Entity:        w.newEntity(wave.X, wave.Y, wave.VX, wave.VY, width, height),
			Kind:          wave.Kind, Health: wave.Health, MaxHealth: wave.Health, Score: wave.Score,
			Pattern: wave.Pattern, Drop: wave.Drop, BaseY: wave.Y,
			Sprite: wave.Sprite, Descriptor: wave.Descriptor,
			Hidden: wave.Hidden, CollisionFrames: wave.CollisionFrames,
			StampWord: wave.StampWord, StampProbeX: wave.StampProbeX, StampProbeY: wave.StampProbeY,
			Hopper: wave.Hopper, RouteCrawler: wave.RouteCrawler,
			Stalker: wave.Stalker, TilePulse: wave.TilePulse, TargetX: wave.TargetX, TargetY: wave.TargetY, NoCollidable: wave.NoCollidable,
			QuarterTurner:    wave.QuarterTurner,
			Reactor:          wave.Reactor,
			GroundWalker:     wave.GroundWalker,
			BattleshipTurret: wave.BattleshipTurret,
			Animation:        wave.Animation, AnimationTicks: wave.AnimationTicks, AnimationStart: wave.AnimationStart,
			MotionX: wave.MotionX, MotionY: wave.MotionY, MoveDelay: wave.MoveDelay,
			MotionLoop: wave.MotionLoop, MotionAnimation: wave.MotionAnimation, DriftDuringDelay: wave.DriftDuringDelay,
			Behavior: wave.Behavior, Variant: wave.Variant, LungeVX: wave.LungeVX, LungeVY: wave.LungeVY,
			AimMap: wave.AimMap, AimThresholds: wave.AimThresholds,
			TurnTicks: wave.TurnTicks, HomingTurns: wave.HomingTurns, HomingClampY: wave.HomingClampY, Invincible: wave.Invincible,
			AimAnimation: wave.AimAnimation, AimOffsetsX: wave.AimOffsetsX, AimOffsetsY: wave.AimOffsetsY,
			ShotVX: wave.ShotVX, ShotVY: wave.ShotVY, ShootMin: wave.ShootMin, ShootMax: wave.ShootMax,
			CarrierKind: wave.CarrierKind, CarrierVX: wave.CarrierVX, CarrierInitialVX: wave.CarrierInitialVX, CarrierPhase: wave.CarrierPhase, CarrierMotionTick: wave.CarrierMotionTick,
			CarrierFloatAnimation: wave.CarrierFloatAnimation, CarrierGroundAnimation: wave.CarrierGroundAnimation,
			CarrierAnimationReverseFloat: wave.CarrierAnimationReverseFloat, CarrierAnimationReverseGround: wave.CarrierAnimationReverseGround,
			CameraX: w.Scroll, CameraY: w.ScrollY,
		}
		if len(w.Stages[w.Stage-1].ScrollScript) == 0 {
			enemy.CameraX -= w.Stages[w.Stage-1].ScrollSpeed
		}
		enemy.Spitter, enemy.WallCrawler = wave.Spitter, wave.WallCrawler
		enemy.GroundGunner = wave.GroundGunner
		if enemy.Behavior == BehaviorTilePulse {
			initializeTilePulse(&enemy)
		}
		if enemy.Behavior == BehaviorQuarterTurner && enemy.QuarterTurner != nil {
			// Source 0x4B4DC arms the first turn delay before actor insertion.
			enemy.BehaviorTicks = enemy.QuarterTurner.TurnPeriod
		}
		enemy.HuntingEmitter, enemy.ShieldVehicle, enemy.SpiralSentinel = wave.HuntingEmitter, wave.ShieldVehicle, wave.SpiralSentinel
		w.initializeOriginalAutomaticFire(&enemy)
		if enemy.Behavior == BehaviorSpiralSentinel {
			w.spawnSpiralSentinel(enemy)
			continue
		}
		if enemy.Behavior == BehaviorSpitter {
			w.initializeSpitter(&enemy)
		}
		w.Enemies = append(w.Enemies, enemy)
		if enemy.Behavior == BehaviorGroundHopper && enemy.Hopper != nil && enemy.Hopper.InitFallStep {
			index := len(w.Enemies) - 1
			w.Enemies[index].BehaviorPhase = 2
			w.updateGroundHopper(&w.Enemies[index])
		}
	}
}
