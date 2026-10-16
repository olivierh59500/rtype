package sim

import (
	"github.com/olivierh59500/r-type/internal/source"
)

// OriginalForcePhase replaces the original routine pointers with native states.
type OriginalForcePhase uint8

const (
	ForceEntering OriginalForcePhase = iota
	ForceStationary
	ForceRecall
	ForceDocked
	ForceLaunched
)

// OriginalForceMotion is the original controller's meaningful state. Positions
// use hardware-sprite coordinates until the World presentation conversion.
// No instruction, register file or emulated memory is needed for its updates.
type OriginalForceMotion struct {
	X, Y, DrawX, DrawY           int
	Rank, ShownRank              int
	Side, Region                 int
	Phase                        OriginalForcePhase
	LaunchVelocity               int
	VerticalDelay, VerticalSpeed int
	Frame, FrameDelay, Spin      int
	Blocked                      bool
	ButtonDown                   bool
	HintX, HintY                 int
	Age                          int
}

// NewOriginalForceMotion reproduces 29F0/2A1C when the first Force is acquired.
func NewOriginalForceMotion(rank int) OriginalForceMotion {
	return OriginalForceMotion{X: 118, Y: 138, DrawX: 118, DrawY: 138, Rank: rank, ShownRank: rank, Phase: ForceEntering, Spin: 1}
}

// Step executes the Force controller once per PAL interrupt. The supplied
// data are original animation, attachment and collision tables, not code.
// Tile processing is applied by World after this pure movement transition.
func (f *OriginalForceMotion) Step(playerX, playerY int, buttonDown bool, ranks []source.ForceWeapon) (attached, launched bool) {
	if len(ranks) != 4 {
		return false, false
	}
	f.Age++
	if f.Rank < 0 {
		f.Rank = 0
	}
	if f.Rank > 3 {
		f.Rank = 3
	}
	f.advanceAnimation(ranks)
	shape := ranks[f.Rank]
	// SPRxPOS is prepared before the original routine moves the Force.
	f.DrawX, f.DrawY = f.X, f.Y
	f.HintX, f.HintY = 0, 0
	forcePressed := false
	if f.Phase == ForceEntering || f.Phase == ForceStationary || f.Phase == ForceDocked {
		forcePressed = buttonDown && !f.ButtonDown
		f.ButtonDown = buttonDown
	}

	switch f.Phase {
	case ForceEntering:
		f.HintX = -4
		if forcePressed {
			f.Phase = ForceRecall
		}
		attached = f.contact(playerX, playerY, shape)
		f.X += 2
		if f.X >= 192 {
			f.Region = 1
			f.Phase = ForceStationary
		}
	case ForceStationary:
		if forcePressed {
			f.Phase = ForceRecall
		}
		attached = f.contact(playerX, playerY, shape)
		target, region := 192, 1
		if playerX < 270 {
			target, region = 338, -1
		}
		if f.Region != region {
			f.Region = 0
			if difference := target - f.X; absInt(difference) >= 4 {
				f.HintX = -3 * signInt(difference)
				f.X += signInt(difference) * 2
				return attached, false
			}
			f.Region = region
			f.VerticalDelay, f.VerticalSpeed = 0, 0
			return attached, false
		}
		difference := playerY - f.Y
		if absInt(difference) > 4 {
			f.VerticalDelay--
			if f.VerticalDelay < 0 {
				f.VerticalDelay = 16
				f.VerticalSpeed, f.Spin = signInt(difference), signInt(difference)
			}
			f.Y += f.VerticalSpeed
			f.HintY = -(f.VerticalSpeed + 1)
		} else {
			f.VerticalSpeed, f.VerticalDelay = 0, 16
		}
	case ForceRecall:
		if absInt(playerY-f.Y) >= 2 {
			f.HintY = -2 * signInt(playerY-f.Y)
			f.Y += signInt(playerY - f.Y)
		}
		if absInt(playerX+4-f.X) >= 3 {
			f.HintX = -2 * signInt(playerX+4-f.X)
			f.X += signInt(playerX + 4 - f.X)
		}
		attached = f.contact(playerX, playerY, shape)
	case ForceDocked:
		if f.Side < 0 {
			f.X, f.Y = playerX+shape.AttachBackX, playerY+shape.AttachBackY
		} else {
			f.X, f.Y = playerX+shape.AttachFrontX, playerY+shape.AttachFrontY
		}
		if forcePressed {
			f.LaunchVelocity = 8
			if f.Side < 0 {
				f.LaunchVelocity = -8
			}
			f.Side, f.Phase = 0, ForceLaunched
			if f.Rank == 2 {
				f.Rank = 1
			}
			launched = true
		}
	case ForceLaunched:
		f.HintY = -f.LaunchVelocity
		if f.Blocked {
			f.HintY = 0
		}
		if !f.Blocked {
			f.X += f.LaunchVelocity
		}
		if f.Blocked || f.X <= 134 || f.X >= 422 {
			f.Region, f.Phase = 0, ForceStationary
		}
	}
	if f.X <= 102 || f.X > 454 {
		if f.Rank == 2 {
			f.Rank = 1
		}
		f.Region, f.Side, f.VerticalDelay, f.VerticalSpeed, f.Blocked = 0, 0, 0, 0, false
		f.X, f.Y, f.Phase = 118, 138, ForceEntering
	}
	return attached, launched
}

func (f *OriginalForceMotion) contact(playerX, playerY int, shape source.ForceWeapon) bool {
	// Core 1634 selects one envelope on each side of the origin. Its signed
	// comparisons are asymmetric; it never adds the two object dimensions.
	dx, dy := f.X+shape.Collision.X-playerX, f.Y+shape.Collision.Y-playerY
	width, height := shape.Collision.Width, shape.Collision.Height
	if dx >= 0 {
		width = 18
	}
	if dy >= 0 {
		height = 8
	}
	if absInt(dx) > width || absInt(dy) > height {
		return false
	}
	f.Side = 1
	if playerX+6 > f.X {
		f.Side = -1
		if f.Rank == 1 {
			f.Rank = 2
		}
	}
	f.Phase = ForceDocked
	return true
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}
func signInt(value int) int {
	if value < 0 {
		return -1
	}
	if value > 0 {
		return 1
	}
	return 0
}

// updateOriginalForce integrates the verified hardware-coordinate controller.
func (w *World) updateOriginalForce(input Input, forcePressed bool) {
	if w.Force.Mode == ForceNone {
		return
	}
	data := w.weaponTables()
	if data == nil || len(data.Force) != 4 {
		w.updateForceProvisional(input, forcePressed)
		return
	}
	f := &w.Force
	if f.Original == nil {
		state := NewOriginalForceMotion(forceRank(f.Level, f.Mode))
		f.Original = &state
	}
	state := f.Original
	state.Rank = forceRank(f.Level, f.Mode)
	attached, launched := state.Step(int(w.Player.X)+113, int(w.Player.Y)+37, input.Force, data.Force)
	w.applyOriginalForceTerrain(state, data.Force[state.Rank])
	shape := data.Force[state.Rank]
	f.X, f.Y = float64(state.X-111), float64(state.Y-37)+float64(shape.Height-14)/2
	f.DrawX, f.DrawY = float64(state.DrawX-111), float64(state.DrawY-37)+float64(shape.Height-14)/2
	f.HitX, f.HitY = float64(state.X-127+shape.Collision.X), float64(state.Y-44+shape.Collision.Y)
	f.HitWidth, f.HitHeight = float64(shape.Collision.Width), float64(shape.Collision.Height)
	f.W, f.H, f.Age = 32, float64(shape.Height), state.Age
	f.Frame = state.Frame
	if len(shape.Frames) > state.Frame && state.Frame >= 0 {
		f.Sprite = shape.Frames[state.Frame]
	}
	switch state.Phase {
	case ForceDocked:
		if state.Side < 0 {
			f.Mode = ForceBack
		} else {
			f.Mode = ForceFront
		}
	case ForceRecall:
		f.Mode = ForceReturning
	default:
		f.Mode = ForceFree
	}
	if attached || launched {
		w.Events = append(w.Events, Event{Kind: EventForce, X: f.X, Y: f.Y})
	}
}

// applyOriginalForceTerrain follows the tile probes at 2862-29EE. Each probe
// uses original game coordinates and the mutable map's collision flags.
func (w *World) applyOriginalForceTerrain(f *OriginalForceMotion, shape source.ForceWeapon) {
	if !w.Player.Alive {
		return
	}
	x, y := f.X-95+shape.ProbeX, f.Y-12+shape.ProbeY
	f.Blocked = false
	if f.Side != 0 {
		f.Blocked = w.OriginalSolidHit(x+3, y)
		f.Blocked = w.OriginalSolidHit(x+3, y+16) || f.Blocked
		return
	}
	// A wide corridor probe seeks an open row before the local four-way
	// bumps. This is intentionally discrete rather than swept rectangles.
	look := 64
	if f.HintX > 0 {
		look = -64
	}
	col, row := w.originalProjectileTile(x+look, y)
	if w.RawTileAt(col, row)&0x8000 != 0 {
		upper, lower := row, row
		open := func(r int) bool {
			return w.RawTileAt(col, r)&0x8000 == 0 && w.RawTileAt(col+1, r)&0x8000 == 0 && w.RawTileAt(col+2, r)&0x8000 == 0
		}
		for attempt := 0; attempt < 10; attempt++ {
			if upper > 1 {
				upper--
				if open(upper) {
					f.Y -= 2
					f.HintY += 4
					break
				}
			}
			if lower < 11 {
				lower++
				if open(lower) {
					f.Y += 2
					f.HintY -= 4
					break
				}
			}
		}
	}
	// Side signs select the original neighboring map cell, which is the
	// opposite side of a positive movement hint from the Force routine.
	probe := func(dx, dy int) bool {
		blocked := w.OriginalSolidHit(x+dx*16, y+dy*16)
		if blocked {
			f.Blocked = true
		}
		return blocked
	}
	if f.HintX < 0 {
		if probe(1, 0) {
			f.X += f.HintX
		}
	} else if f.HintX > 0 {
		if probe(-1, 0) {
			f.X += f.HintX
		}
	} else {
		if probe(1, 0) {
			f.X--
			f.Region = 0
		} else if probe(-1, 0) {
			f.X++
			f.Region = 0
		}
	}
	if f.HintY < 0 {
		if probe(0, 1) {
			f.Y += f.HintY
		}
	} else if f.HintY > 0 {
		if probe(0, -1) {
			f.Y += f.HintY
		}
	} else {
		if probe(0, 1) {
			f.Y--
		} else if probe(0, -1) {
			f.Y++
		}
	}
}

func forceRank(level int, mode ForceMode) int {
	if level <= 1 {
		return 0
	}
	if level >= 3 {
		return 3
	}
	if mode == ForceBack {
		return 2
	}
	return 1
}

func (w *World) weaponTables() *source.Weapons {
	if w.Stage < 1 || w.Stage > len(w.Stages) {
		return nil
	}
	return w.Stages[w.Stage-1].Weapons
}

// updateOriginalBits preserves the original three-slot position history. Its
// two PAL-tick lag is separate from the 25 Hz blitter and weapon updates.
func (w *World) updateOriginalBits() {
	if !w.bitInitialized {
		w.bitInitialized = true
		for bit := 0; bit < 2; bit++ {
			w.bitRead[bit], w.bitWrite[bit] = 1, 0
			w.bitFrameDelay[bit] = 0
			for slot := 0; slot < 3; slot++ {
				y := 30.0
				if bit == 1 {
					y = 270
				}
				w.bitHistory[bit][slot] = Entity{X: 0, Y: y}
			}
		}
	}
	for bit := 0; bit < w.Player.Bits && bit < 2; bit++ {
		offset := -25.0
		if bit == 1 {
			offset = 29
		}
		w.bitHistory[bit][w.bitWrite[bit]] = Entity{X: w.Player.X + 118, Y: w.Player.Y + 37 + offset}
		raw := w.bitHistory[bit][w.bitRead[bit]]
		raw.Y = clamp(raw.Y, 34, 270)
		w.bits[bit] = Entity{X: raw.X - 119, Y: raw.Y - 36, W: 8, H: 8}
		w.bitDraw[bit] = Entity{X: raw.X - 119, Y: raw.Y - 38, W: 16, H: 12}
		w.bitFrameDelay[bit]--
		if w.bitFrameDelay[bit] < 0 {
			w.bitFrameDelay[bit] = 3
			w.bitFrames[bit] = (w.bitFrames[bit] + 1) % 12
		}
		w.bitRead[bit] = (w.bitRead[bit] + 1) % 3
		w.bitWrite[bit] = (w.bitWrite[bit] + 1) % 3
	}
}

// Bounds preserves the original Force collision rectangle independently of its
// wider hardware-sprite image and the display position prepared one tick ago.
func (f Force) Bounds() Rect {
	if f.Original == nil {
		return f.Entity.Bounds()
	}
	return Rect{f.HitX, f.HitY, f.HitWidth, f.HitHeight}
}

// BitDrawPosition returns the original satellite's hardware-sprite centre.
func (w *World) BitDrawPosition(index int) Entity {
	if index < 0 || index >= 2 {
		return Entity{}
	}
	return w.bitDraw[index]
}

// BitFrame returns the original hardware-sprite animation frame of a satellite.
func (w *World) BitFrame(index int) int {
	if index < 0 || index >= 2 {
		return 0
	}
	return w.bitFrames[index]
}
