package pilot

import (
	"github.com/olivierh59500/r-type/internal/sim"
	"math"
	"strings"
)

type Actor struct {
	Track               uint64
	Bounds              sim.Rect
	VX, VY              float64
	Carrier, Projectile bool
	Appearance          string
	Image               string
	Frame               int
	KnownEnvelope       bool
}
type Pickup struct {
	Bounds sim.Rect
	Kind   sim.PowerUpKind
}

// Ordinary observations exclude authored spawns, hidden controllers, HP and
// exact colliders. Memorized mode explicitly adds permitted authored knowledge
// through memory; dynamic image observations retain their reaction delay.
type Observation struct {
	memory                    *memorizedObservation
	Automatic                 bool
	MapHazards                []sim.Rect
	ForceX, ForceY            float64
	Tick                      int
	Stage                     int
	State                     sim.State
	Alive, Combat, Exiting    bool
	X, Y, Speed               float64
	Weapon                    sim.Weapon
	WeaponLevel, Bits, Charge int
	Missiles                  bool
	ForceMode                 sim.ForceMode
	Walls                     []sim.Rect
	Actors                    []Actor
	Pickups                   []Pickup
	AnimatedMap               []sim.Rect
	CameraVX, CameraVY        float64
}
type tracked struct {
	x, y   float64
	tick   int
	vx, vy float64
}
type tileKey struct{ stage, column, row int }
type Sensor struct {
	changedAt               map[tileKey]int
	animatedDanger          map[tileKey]bool
	cameraVX, cameraVY      float64
	actors                  map[uint64]tracked
	tiles                   map[tileKey]uint16
	lastScroll, lastScrollY float64
	lastTick                int
}

func NewSensor() *Sensor {
	return &Sensor{changedAt: make(map[tileKey]int), animatedDanger: make(map[tileKey]bool), actors: make(map[uint64]tracked), tiles: make(map[tileKey]uint16)}
}
func visible(r sim.Rect) bool {
	return r.X+r.W > 16 && r.X < sim.Width && r.Y+r.H > 0 && r.Y < sim.PlayHeight
}

// Observe samples current rendered geometry. Camera displacement models visual
// optical flow; it never exposes future camera commands or level event scripts.
func (s *Sensor) Observe(w *sim.World, tick int) Observation {
	p := w.Player
	o := Observation{Automatic: w.InitialPlayerIntroActive(), ForceX: w.Force.DrawX, ForceY: w.Force.DrawY, Tick: tick, Stage: w.Stage, State: w.State, Alive: p.Alive, Combat: w.ControlsEnabled(), Exiting: w.Boss.Exiting, X: p.X, Y: p.Y, Speed: float64(2 + p.SpeedLevel), Weapon: p.Weapon, WeaponLevel: p.WeaponLevel, Bits: p.Bits, Missiles: p.Missiles, Charge: p.Charge, ForceMode: w.Force.Mode}
	dt := tick - s.lastTick
	if dt > 0 && math.Abs(w.Scroll-s.lastScroll) < 8 && math.Abs(w.ScrollY-s.lastScrollY) < 8 {
		s.cameraVX = 0.75*s.cameraVX + 0.25*(w.Scroll-s.lastScroll)/float64(dt)
		s.cameraVY = 0.75*s.cameraVY + 0.25*(w.ScrollY-s.lastScrollY)/float64(dt)
		o.CameraVX, o.CameraVY = s.cameraVX, s.cameraVY
	}
	s.lastScroll, s.lastScrollY, s.lastTick = w.Scroll, w.ScrollY, tick
	for _, wall := range w.TerrainInView() {
		if visible(wall) {
			o.Walls = append(o.Walls, wall)
		}
	}
	add := func(id uint64, bounds sim.Rect, carrier, projectile bool, appearance string, frame int) {
		if !visible(bounds) {
			return
		}
		x, y := bounds.X+bounds.W/2, bounds.Y+bounds.H/2
		vx, vy := 0.0, 0.0
		if old, found := s.actors[id]; found && tick > old.tick {
			elapsed := float64(tick - old.tick)
			vx, vy = 0.75*old.vx+0.25*(x-old.x)/elapsed, 0.75*old.vy+0.25*(y-old.y)/elapsed
			if math.Abs(vx) > 10 || math.Abs(vy) > 10 {
				vx, vy = 0, 0
			}
		}
		s.actors[id] = tracked{x: x, y: y, tick: tick, vx: vx, vy: vy}
		o.Actors = append(o.Actors, Actor{Track: id, Bounds: bounds, VX: vx, VY: vy, Carrier: carrier, Projectile: projectile, Appearance: appearance, Frame: frame})
	}
	for _, enemy := range w.Enemies {
		if enemy.Health <= 0 || enemy.Hidden || enemy.PendingOriginalPass() {
			continue
		}
		// This recognized blue presentation-arrow template is a visual cue,
		// rather than a hostile object. No collision metadata is consulted.
		if strings.Contains(enemy.Sprite, "-04252") {
			continue
		}
		// Shape recognition does not consult immunity or collision-target flags.
		bounds := enemy.Bounds()
		add(enemy.ID, bounds, enemy.Kind == sim.EnemyCarrier, enemy.W <= 16 && enemy.H <= 16, enemy.Sprite, enemy.Frame())
	}
	for _, bullet := range w.Bullets {
		if bullet.Friendly || bullet.Lifetime <= 0 {
			continue
		}
		bounds := bullet.Bounds()
		count := len(o.Actors)
		add(bullet.ID, bounds, false, true, "enemy-shot", 0)
		if len(o.Actors) > count {
			o.Actors[len(o.Actors)-1].Image = bullet.Sprite
		}
	}
	for _, power := range w.PowerUps {
		bounds := power.Bounds()
		if power.Kind != sim.PowerNone && power.Age > 0 && visible(bounds) {
			o.Pickups = append(o.Pickups, Pickup{bounds, power.Kind})
		}
	}
	// Art-index changes represent animations a player can see. The collision
	// flag bits are discarded here; no hidden weak-point entity is inspected.
	stage := w.Stages[w.Stage-1]
	firstColumn := max(0, int(w.Scroll)/16)
	lastColumn := min(stage.MapWidth, firstColumn+22)
	for row := 0; row < stage.MapHeight; row++ {
		for column := firstColumn; column < lastColumn; column++ {
			r := sim.Rect{X: float64(column*16) + 16 - w.Scroll, Y: float64(row*16) - w.ScrollY, W: 16, H: 16}
			if !visible(r) {
				continue
			}
			key := tileKey{w.Stage, column, row}
			art := w.RawTileAt(column, row) & 0xfff
			if old, exists := s.tiles[key]; exists && old != art {
				s.changedAt[key] = tick
				if math.Abs(o.CameraVX) > 0.05 || math.Abs(o.CameraVY) > 0.05 {
					s.animatedDanger[key] = art != 0
				}
			}
			s.tiles[key] = art
			// A changed visual feature remains perceptible across both PAL
			// display frames and the bounded input sampling interval.
			if when, exists := s.changedAt[key]; exists && tick-when < 4 {
				o.AnimatedMap = append(o.AnimatedMap, r)
			}
			if s.animatedDanger[key] {
				o.MapHazards = append(o.MapHazards, r)
			}
		}
	}
	return o
}
