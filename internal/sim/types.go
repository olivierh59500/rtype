// Package sim implements the game rules independently of graphics, audio, input
// devices, and the original Amiga executable. Every Step advances one PAL tick.
package sim

import (
	"github.com/olivierh59500/r-type/internal/source"
	"math"
)

const (
	Width         = 320
	Height        = 208
	HUDHeight     = 16
	PlayHeight    = Height - HUDHeight
	TickRate      = 50
	MainTickRate  = 25
	MaxCharge     = 24
	MaxSpeedLevel = 3
	GameOriginX   = 32
	GameOriginY   = 32
)

// State identifies the current screen and whether gameplay advances.
type State uint8

const (
	Title State = iota
	Playing
	Paused
	GameOver
	Complete
	ScoreEntry
	ScoreTable
)

// Input contains button levels, not edges. The simulation detects edges itself.
type Input struct {
	Left, Right, Up, Down     bool
	Fire, Force, Start, Pause bool
}

// Entity positions are centers in logical screen pixels. Velocities are pixels
// per tick. This coordinate convention also applies to every embedded entity.
type Entity struct {
	ID                 uint64
	X, Y, VX, VY, W, H float64
	Age                int
}

func (e Entity) Bounds() Rect { return Rect{e.X - e.W/2, e.Y - e.H/2, e.W, e.H} }

// Rect uses a top-left origin. Touching edges do not count as a collision.
type Rect struct{ X, Y, W, H float64 }

func (r Rect) Overlaps(other Rect) bool {
	return r.X < other.X+other.W && r.X+r.W > other.X &&
		r.Y < other.Y+other.H && r.Y+r.H > other.Y
}

type Weapon uint8

const (
	WeaponNormal Weapon = iota
	WeaponRed
	WeaponBlue
	WeaponYellow
)

type Player struct {
	Intro *OriginalPlayerIntro
	Entity
	OriginalMotion                                                  *OriginalPlayerMotion
	Alive                                                           bool
	Charge, SpeedLevel, WeaponLevel, Bits, Invulnerable, DeathTicks int
	Weapon                                                          Weapon
	Missiles                                                        bool
}

type ForceMode uint8

const (
	ForceNone ForceMode = iota
	ForceFront
	ForceBack
	ForceFree
	ForceReturning
)

type Force struct {
	Entity
	Mode                            ForceMode
	Level                           int
	Original                        *OriginalForceMotion
	Sprite                          string
	Frame                           int
	DrawX, DrawY                    float64
	HitX, HitY, HitWidth, HitHeight float64
}

type EnemyKind uint8

const (
	EnemyFighter EnemyKind = iota
	EnemyDrifter
	EnemyTurret
	EnemyCarrier
	EnemyBoss
)

// EnemyBehavior selects a native Go implementation of a recovered rule family.
type EnemyBehavior uint8

const (
	BehaviorProvisional EnemyBehavior = iota
	BehaviorApproachLunge
	BehaviorHoming
	BehaviorMountedTurret
	BehaviorPowerCarrier
	BehaviorTerrainStamp
	BehaviorGroundHopper
	BehaviorRouteCrawler
	BehaviorStalker
	BehaviorTilePulse
	BehaviorQuarterTurner
	BehaviorBattleshipReactor
	BehaviorSpitter
	BehaviorSpitterProjectile
	BehaviorWallCrawler
	BehaviorFixedProjectile
	BehaviorGroundGunner
	BehaviorArcProjectile
	BehaviorFireWorm
	BehaviorMiniTail
	BehaviorRespawningTurret
	BehaviorBattleshipTurretShot
	BehaviorHuntingEmitter
	BehaviorHunterShot
	BehaviorShieldVehicle
	BehaviorSpiralSentinel
	BehaviorInitialArrow
	BehaviorGroundWalker
	BehaviorWalkerHoming
	BehaviorWalkerCannon
	BehaviorSineFormation
)

// Enemy pattern parameters are data, allowing extracted wave tables to replace
// the provisional stage scripts without embedding binary instructions.
type Enemy struct {
	CaveShot                           *OriginalCaveShotState
	Priority                           int
	AutomaticFire, AutomaticFireTimer  int
	automaticFireEligible              bool
	IntroArrow                         *source.IntroArrowData
	maskFlashPending                   bool
	MaskFlash                          bool
	HuntingEmitter                     *source.HuntingEmitterData
	Hunter                             *HuntingEmitterState
	ShieldVehicle                      *source.ShieldVehicleData
	Vehicle                            *ShieldVehicleState
	SpiralSentinel                     *source.SpiralSentinelData
	Spiral                             *SpiralSentinelState
	MiniTail                           *OriginalMiniTailState
	GroundWalker                       *source.GroundWalkerData
	Walker                             *OriginalGroundWalkerState
	BattleshipTurret                   *source.BattleshipTurretData
	FireWorm                           *OriginalFireWormState
	FixedMinXInclusive, FixedNoYCutoff bool
	// OriginalCollisionPaused retains core 0x14D4's complete collision gate
	// for the update that predecrements the original three-tick hurt timer.
	OriginalCollisionPaused                                             bool
	bossParent                                                          uint64
	bossDeathDelay                                                      int
	Gunner                                                              *GroundGunnerState
	Arc                                                                 *ArcProjectileState
	GroundGunner                                                        *source.GroundGunnerData
	reactorPatchPhase, reactorPatchTick, reactorPatchPair, reactorBlink int
	Spitter                                                             *source.SpitterData
	WallCrawler                                                         *source.WallCrawlerData
	NoFriendlyTarget                                                    bool
	Reactor                                                             *source.ReactorData
	QuarterTurner                                                       *source.QuarterTurnerData
	bossAdvance                                                         bool
	bossRoute                                                           int
	bossAnimationIndex, bossAnimationTimer                              int
	bossInitialized, bossBroken                                         bool
	Stalker                                                             *source.StalkerData
	TilePulse                                                           *source.TilePulseData
	tilePulseInitialized                                                bool
	TargetX, TargetY                                                    int
	NoCollidable                                                        bool
	Hopper                                                              *source.HopperData
	RouteCrawler                                                        *source.RouteCrawlerData
	crawlerDamage                                                       int
	StampWord                                                           uint16
	StampProbeX, StampProbeY                                            int
	Hidden, AnimationOverride                                           bool
	AnimationFrame                                                      int
	CollisionFrames                                                     []CollisionFrame
	DamageTaken                                                         int
	bossRole                                                            bossRole
	bossSegment, bossDelay, bossMotion                                  int
	bossTargetX                                                         float64
	Entity
	Kind                                                         EnemyKind
	Health, MaxHealth, Score, Pattern                            int
	Drop                                                         PowerUpKind
	BaseY                                                        float64
	ContactCooldown                                              int
	Sprite                                                       string
	Descriptor                                                   uint32
	Animation                                                    []int
	AnimationTicks                                               int
	AnimationStart                                               int
	MotionX, MotionY                                             []int8
	MoveDelay                                                    int
	MotionTick                                                   int
	MotionLoop                                                   bool
	MotionAnimation                                              []int
	DriftDuringDelay                                             bool
	Behavior                                                     EnemyBehavior
	Variant, BehaviorPhase, BehaviorTicks                        int
	LungeVX, LungeVY                                             float64
	AimMap                                                       []int8
	AimThresholds                                                []int
	TurnTicks, HomingTurns                                       int
	HomingClampY, Invincible                                     bool
	AimAnimation                                                 []int
	AimOffsetsX, AimOffsetsY                                     []int
	ShotVX, ShotVY                                               []int8
	ShootMin, ShootMax, ShootCooldown                            int
	CameraX, CameraY                                             float64
	CarrierKind                                                  PowerUpKind
	CarrierVX, CarrierInitialVX, CarrierPhase, CarrierMotionTick int
	CarrierFloatAnimation, CarrierGroundAnimation                []int
	CarrierAnimationReverseFloat, CarrierAnimationReverseGround  []int
}

// Frame identifies an original atlas frame using an extracted animation table.
func (e Enemy) Frame() int {
	if e.AnimationOverride {
		return e.AnimationFrame
	}
	if e.Behavior == BehaviorMountedTurret && len(e.AimAnimation) > e.Variant {
		return e.AimAnimation[e.Variant]
	}
	if len(e.MotionAnimation) > 0 {
		index := e.MotionTick - 1
		if index < 0 {
			index = 0
		}
		return e.MotionAnimation[motionIndex(index, len(e.MotionAnimation), e.MotionLoop)]
	}
	if len(e.Animation) == 0 {
		return 0
	}
	period := e.AnimationTicks
	if period < 1 {
		period = 4
	}
	age := e.Age - 1
	if age < 0 {
		age = 0
	}
	return e.Animation[(e.AnimationStart+age/period)%len(e.Animation)]
}

type BulletKind uint8

const (
	Shot BulletKind = iota
	Beam
	RedLaser
	BlueLaser
	YellowLaser
	Missile
	ForceShot
	EnemyShot
)

type Bullet struct {
	Hostile      *OriginalHostileShot
	Original     *OriginalWeaponMotion
	Sprite       string
	PierceBudget int
	Entity
	Kind                      BulletKind
	Friendly                  bool
	Damage, Lifetime, Bounces int
	Piercing                  bool
	HitIDs                    []uint64
}

type PowerUpKind uint8

const (
	PowerNone PowerUpKind = iota
	PowerRed
	PowerBlue
	PowerYellow
	PowerSpeed
	PowerMissiles
	PowerBit
)

type PowerUp struct {
	Priority int
	Entity
	Kind     PowerUpKind
	Original *source.PickupData
}
type Effect struct {
	actorAdvanced bool
	MaskFlash     bool
	ActorOwner    uint64
	Priority      int
	Delay, Sound  int
	Sprite        string
	FrameTicks    int
	Animation     []int
	Entity
	Lifetime int
}

// Frame resolves an original effect animation while preserving synthetic FX.
func (e Effect) Frame() int {
	if len(e.Animation) == 0 {
		return e.Age / 4 % 5
	}
	period := e.FrameTicks
	if period < 1 {
		period = 1
	}
	age := e.Age - 1
	if age < 0 {
		age = 0
	}
	index := age / period
	if index >= len(e.Animation) {
		index = len(e.Animation) - 1
	}
	return e.Animation[index]
}

type EventKind uint8

const (
	EventShot EventKind = iota
	EventBeam
	EventEnemyDestroyed
	EventPlayerDestroyed
	EventPowerUp
	EventForce
	EventStage
	EventExtraLife
	EventBossStarted
	EventBossDefeated
	EventSound
)

// Events are cleared at the next Step, including while paused. A presentation
// layer can consume them once to trigger sound effects without affecting rules.
type Event struct {
	Kind  EventKind
	X, Y  float64
	Value int
}

// World owns all gameplay state; it has no wall clock or package-global state.
type World struct {
	actorPassRunning    bool
	actorCombatEnabled  bool
	actorUpdateID       uint64
	actorPassComplete   bool
	ActorImages         []OriginalActorImage
	ActorImagesValid    bool
	TerrainImageChanges map[int]uint16
	TerrainImageValid   bool
	actorList           originalActorList
	// PeakOriginalActorLinks is retrospective telemetry at list synchronization
	// points. It does not reserve slots or change original allocation behavior.
	PeakOriginalActorLinks int
	InitialShipStarted     bool
	StageScore             int
	StageResult            *StageResultState
	// HoldStageAdvance lets the audio host retain the original completion
	// screen while its four-channel jingle still owns the stage transition.
	HoldStageAdvance                                bool
	Menu                                            *OriginalMenuState
	menuData                                        *source.MenuData
	WeaponImages                                    []OriginalWeaponImage
	Scene                                           OriginalSceneState
	EventTick                                       int
	Records                                         []source.HighScoreEntry
	RecordChoices, RecordPrompt                     string
	RecordEntry                                     *RecordEntryState
	RecordTimer                                     int
	RecordRevision                                  uint64
	rngOverride                                     func(*World) uint64
	rngBeamSource                                   func() uint16
	InitialLives, DisplayLives                      int
	ExtraLifeThresholds                             []int
	extraLifeCursor, lastLifeScore                  int
	CameraDeltaX, CameraDeltaY                      float64
	camera                                          cameraState
	spawnCursor                                     int
	Boss                                            BossState
	Tiles                                           []uint16
	TileChanges                                     map[int]uint16
	State                                           State
	Tick                                            uint64
	MainTickDivider                                 int
	StageTick, Stage, Loop                          int
	Scroll, ScrollY                                 float64
	Score, HighScore, Lives                         int
	Player                                          Player
	Force                                           Force
	forceHardware                                   OriginalForceMotion
	Enemies                                         []Enemy
	Bullets                                         []Bullet
	PowerUps                                        []PowerUp
	Effects                                         []Effect
	Events                                          []Event
	Stages                                          []StageSpec
	Checkpoint                                      int
	seed, randomState, nextID                       uint64
	previous                                        Input
	gamePrevious                                    Input
	pendingForce                                    bool
	missileCooldown, forceCooldown, stageClearTicks int
	bitInitialized                                  bool
	bitRead, bitWrite                               [2]int
	bitHistory                                      [2][3]Entity
	bits                                            [2]Entity
	bitDraw                                         [2]Entity
	bitFrames, bitFrameDelay                        [2]int
	originalMissileCooldown                         int
}

// DisplayedCharge separates the bar cleared at death/victory initialization
// from the charge byte retained until checkpoint or next-stage setup.
func (w *World) DisplayedCharge() int {
	if w.Scene.DeathInitialized || w.Boss.VictoryInitialized {
		return 0
	}
	return w.Player.Charge
}

func (w *World) ChargeFraction() float64 { return float64(w.DisplayedCharge()) / MaxCharge }

func clamp(v, low, high float64) float64 { return math.Max(low, math.Min(high, v)) }

func (w *World) newEntity(x, y, vx, vy, width, height float64) Entity {
	w.nextID++
	return Entity{ID: w.nextID, X: x, Y: y, VX: vx, VY: vy, W: width, H: height}
}
