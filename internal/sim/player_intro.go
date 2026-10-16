package sim

import (
	"fmt"
	"github.com/olivierh59500/r-type/internal/source"
)

// OriginalPlayerIntro follows core0x22AC independently of manual joystick
// movement. Its tilt shares the original byte reload while position uses PAL.
type OriginalPlayerIntro struct {
	Data         *source.PlayerIntroData
	Phase, Timer int
}

// BeginInitialPlayerIntro starts only the session's first cold-start ship.
// Checkpoints and subsequent human restarts retain the original 0x4204 latch.
func (w *World) BeginInitialPlayerIntro() {
	if w.InitialShipStarted {
		return
	}
	tables := w.weaponTables()
	if tables == nil || tables.Player == nil || tables.Player.Intro == nil {
		return
	}
	w.InitialShipStarted = true
	d := tables.Player.Intro
	w.Player.Intro = &OriginalPlayerIntro{Data: d}
	w.Player.OriginalMotion = &OriginalPlayerMotion{Tilt: d.StartTilt, Data: tables.Player}
	w.Player.X, w.Player.Y = float64(d.StartX-113), float64(d.StartY-37)
	w.Player.VX, w.Player.VY = 0, 0
	if arrow := d.Arrow; arrow != nil {
		e := Enemy{Entity: w.newEntity(float64(arrow.X)-GameOriginX+float64(arrow.Width)/2, float64(arrow.Y)-GameOriginY+float64(arrow.Height)/2, 0, 0, float64(arrow.Width), float64(arrow.Height)), Behavior: BehaviorInitialArrow, IntroArrow: arrow, Descriptor: arrow.Descriptor, Sprite: fmt.Sprintf("object-%d-%05x", w.Stage, arrow.Descriptor), Health: 1, Invincible: true, MoveDelay: arrow.WaitReload, Animation: arrow.Animation, AnimationTicks: arrow.FrameTicks}
		for _, box := range arrow.Collision {
			e.CollisionFrames = append(e.CollisionFrames, CollisionFrame{X: box.X, Y: box.Y, Width: box.Width, Height: box.Height})
		}
		w.Enemies = append(w.Enemies, e)
	}
}

func (w *World) updateInitialArrow(e *Enemy) bool {
	d := e.IntroArrow
	if d == nil {
		return false
	}
	if e.MoveDelay >= 0 {
		e.MoveDelay--
		return true
	}
	e.X += float64(d.VX)
	if int(e.X-e.W/2+GameOriginX) > d.MaximumX {
		e.Health = 0
		return true
	}
	e.MoveDelay--
	return true
}

func (w *World) InitialPlayerIntroActive() bool { return w.Player.Intro != nil }

func (w *World) updateOriginalPlayerIntro() bool {
	p := &w.Player
	s := p.Intro
	if s == nil {
		return false
	}
	d := s.Data
	x := int(p.X) + 113
	oldX := x
	if s.Phase == 0 {
		if w.Scene.Phase != ScenePlaying {
			p.VX, p.VY = 0, 0
			return true
		}
		s.Phase = 1
		s.Timer = 0
	}
	switch s.Phase {
	case 1:
		s.Timer--
		if s.Timer < 0 {
			s.Timer = d.TiltReload
			p.OriginalMotion.Tilt = max(0, p.OriginalMotion.Tilt-1)
		}
		x += d.Step
		if x >= d.OutwardTarget {
			s.Phase = 2
			s.Timer = d.HoldReload
		}
	case 2:
		s.Timer--
		if s.Timer < 0 {
			s.Phase = 3
			s.Timer = d.ReturnTiltDelay
		}
	case 3:
		s.Timer--
		if s.Timer < 0 {
			s.Timer = d.TiltReload
			if p.OriginalMotion.Tilt < 8 {
				p.OriginalMotion.Tilt++
			} else if p.OriginalMotion.Tilt > 8 {
				p.OriginalMotion.Tilt--
			}
		}
		x -= d.Step
		if x <= d.ReturnTarget {
			p.Intro = nil
		}
	}
	// Core 0x2334 and the later death fade share byte 0x420B. Retain its
	// final automatic-route value instead of the earlier palette reload.
	w.Scene.Timer = s.Timer
	p.X = float64(x - 113)
	p.Y = float64(d.StartY - 37)
	p.VX, p.VY = float64(x-oldX), 0
	return true
}
