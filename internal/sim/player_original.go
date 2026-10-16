package sim

import "github.com/olivierh59500/r-type/internal/source"

// OriginalPlayerMotion translates the five hardware sprite poses and their
// sixteen-step bank transition. Tilt is updated on every PAL interrupt.
type OriginalPlayerMotion struct {
	Tilt int
	Data *source.PlayerData
}

// originalPlayerAim preserves the source's EE coordinates independently of
// the anchored death effect. Hardware X20 becomes EE X-75 after the next PAL
// interrupt; existing enemies continue aiming there during the death fade.
func (w *World) originalPlayerAim() (int, int) {
	x, y := int(w.Player.X)+18, int(w.Player.Y)+25
	if w.Scene.DeathAimHidden {
		x = -75
	}
	return x, y
}

func (w *World) updateOriginalPlayerMovement(input Input) bool {
	tables := w.weaponTables()
	if tables == nil || tables.Player == nil {
		return false
	}
	// Core 0x219A stops movement as soon as a collision requests death,
	// before the main callback clears the active ship and control flags.
	if w.Scene.DeathRequested {
		return true
	}
	p, d := &w.Player, tables.Player
	if p.OriginalMotion == nil {
		p.OriginalMotion = &OriginalPlayerMotion{Tilt: 8, Data: d}
	}
	x, y := int(p.X)+113, int(p.Y)+37
	previousX, previousY := x, y
	speed := min(d.MaximumSpeed, d.InitialSpeed+p.SpeedLevel)
	// The original compares before moving, preserving speed-dependent edge
	// overshoot. Opposite joystick directions give Left and Up priority.
	if input.Left {
		if x > d.LeftLimit {
			x -= speed
		}
	} else if input.Right && x <= d.RightLimit {
		x += speed
	}
	direction := 0
	if input.Up && y > d.TopLimit {
		y -= speed
		direction = -1
	} else if !input.Up && input.Down && y < d.BottomLimit {
		y += speed
		direction = 1
	}
	if direction == 0 {
		if p.OriginalMotion.Tilt < 8 {
			direction = 1
		}
		if p.OriginalMotion.Tilt > 8 {
			direction = -1
		}
	}
	p.OriginalMotion.Tilt = max(0, min(16, p.OriginalMotion.Tilt+direction))
	p.X, p.Y = float64(x-113), float64(y-37)
	p.VX, p.VY = float64(x-previousX), float64(y-previousY)
	return true
}

// SpriteFrame returns the exact original pose after gradual bank transitions.
func (p Player) SpriteFrame() int {
	if p.OriginalMotion != nil {
		return p.OriginalMotion.Tilt / 4
	}
	if p.VY < 0 {
		return 0
	}
	if p.VY > 0 {
		return 4
	}
	return 2
}

func (w *World) originalPlayerTerrainHit() bool {
	// Core 0x1836 probes one map point at EEX+18, EEY+6. EEX is the
	// hardware X minus95; EEY is the hardware Y minus12 (0x260E).
	return w.OriginalSolidAt(int(w.Player.X)+36, int(w.Player.Y)+31)
}
