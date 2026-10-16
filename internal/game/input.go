package game

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/olivierh59500/r-type/internal/sim"
)

// A touch keeps its initial role until released, even after crossing a boundary.
type touchRole uint8

const (
	stickRole touchRole = iota + 1
	fireRole
	forceRole
	pauseRole
	startRole
)

type controls struct {
	keys           []ebiten.Key
	buttons        []ebiten.GamepadButton
	humanActive    bool
	touches        map[ebiten.TouchID]touchRole
	ids            []ebiten.TouchID
	gamepads       []ebiten.GamepadID
	blocked        bool
	mouseRole      touchRole
	stickX, stickY float64
	fire, force    bool
}

func (c *controls) reset() {
	c.touches = make(map[ebiten.TouchID]touchRole)
	c.blocked = true
	c.mouseRole = 0
	c.stickX = 0
	c.stickY = 0
	c.fire = false
	c.force = false
	c.humanActive = false
}

func (g *Game) roleAt(x, y int) touchRole {
	if g.AtTitle() || g.AttractDemo() || g.World.State == sim.GameOver || g.World.State == sim.Complete {
		return startRole
	}
	if x < 80 {
		return stickRole
	}
	if x >= 400 {
		if y < 42 {
			return pauseRole
		}
		if y < 108 {
			return forceRole
		}
		return fireRole
	}
	return 0
}

func (g *Game) readInput() sim.Input {
	c := &g.input
	in := sim.Input{Left: ebiten.IsKeyPressed(ebiten.KeyArrowLeft) || ebiten.IsKeyPressed(ebiten.KeyA), Right: ebiten.IsKeyPressed(ebiten.KeyArrowRight) || ebiten.IsKeyPressed(ebiten.KeyD), Up: ebiten.IsKeyPressed(ebiten.KeyArrowUp) || ebiten.IsKeyPressed(ebiten.KeyW), Down: ebiten.IsKeyPressed(ebiten.KeyArrowDown) || ebiten.IsKeyPressed(ebiten.KeyS), Fire: ebiten.IsKeyPressed(ebiten.KeyControlRight) || ebiten.IsKeyPressed(ebiten.KeyZ), Force: ebiten.IsKeyPressed(ebiten.KeySpace) || ebiten.IsKeyPressed(ebiten.KeyX), Start: ebiten.IsKeyPressed(ebiten.KeyEnter), Pause: ebiten.IsKeyPressed(ebiten.KeyP)}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		g.Back()
	}
	c.gamepads = ebiten.AppendGamepadIDs(c.gamepads[:0])
	humanGamepad := false
	for _, id := range c.gamepads {
		c.buttons = inpututil.AppendPressedGamepadButtons(id, c.buttons[:0])
		humanGamepad = humanGamepad || len(c.buttons) > 0
		if !ebiten.IsStandardGamepadLayoutAvailable(id) {
			continue
		}
		x := ebiten.StandardGamepadAxisValue(id, ebiten.StandardGamepadAxisLeftStickHorizontal)
		y := ebiten.StandardGamepadAxisValue(id, ebiten.StandardGamepadAxisLeftStickVertical)
		in.Left = in.Left || x < -0.2 || ebiten.IsStandardGamepadButtonPressed(id, ebiten.StandardGamepadButtonLeftLeft)
		in.Right = in.Right || x > .2 || ebiten.IsStandardGamepadButtonPressed(id, ebiten.StandardGamepadButtonLeftRight)
		in.Up = in.Up || y < -0.2 || ebiten.IsStandardGamepadButtonPressed(id, ebiten.StandardGamepadButtonLeftTop)
		in.Down = in.Down || y > .2 || ebiten.IsStandardGamepadButtonPressed(id, ebiten.StandardGamepadButtonLeftBottom)
		in.Fire = in.Fire || ebiten.IsStandardGamepadButtonPressed(id, ebiten.StandardGamepadButtonRightBottom)
		in.Force = in.Force || ebiten.IsStandardGamepadButtonPressed(id, ebiten.StandardGamepadButtonRightLeft)
		in.Start = in.Start || ebiten.IsStandardGamepadButtonPressed(id, ebiten.StandardGamepadButtonCenterRight)
		in.Pause = in.Pause || ebiten.IsStandardGamepadButtonPressed(id, ebiten.StandardGamepadButtonCenterLeft)
	}
	c.ids = ebiten.AppendTouchIDs(c.ids[:0])
	c.keys = inpututil.AppendPressedKeys(c.keys[:0])
	c.humanActive = len(c.keys) > 0 || len(c.ids) > 0 || humanGamepad || in.Left || in.Right || in.Up || in.Down || in.Fire || in.Force || in.Start || in.Pause || ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) || ebiten.IsMouseButtonPressed(ebiten.MouseButtonRight) || ebiten.IsMouseButtonPressed(ebiten.MouseButtonMiddle)
	if c.blocked {
		if !c.humanActive {
			c.blocked = false
		}
		return sim.Input{}
	}
	c.stickX, c.stickY, c.fire, c.force = 0, 0, false, false
	alive := make(map[ebiten.TouchID]bool, len(c.ids))
	for _, id := range c.ids {
		alive[id] = true
		x, y := ebiten.TouchPosition(id)
		role, exists := c.touches[id]
		if !exists {
			role = g.roleAt(x, y)
			c.touches[id] = role
		}
		g.applyTouch(&in, role, x, y)
	}
	for id := range c.touches {
		if !alive[id] {
			delete(c.touches, id)
		}
	}
	if g.touch && ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		x, y := ebiten.CursorPosition()
		if c.mouseRole == 0 {
			c.mouseRole = g.roleAt(x, y)
		}
		g.applyTouch(&in, c.mouseRole, x, y)
	} else {
		c.mouseRole = 0
	}
	return in
}

func (g *Game) applyTouch(in *sim.Input, role touchRole, x, y int) {
	switch role {
	case stickRole:
		dx, dy := float64(x-40), float64(y-140)
		length := math.Hypot(dx, dy)
		if length > 27 {
			dx *= 27 / length
			dy *= 27 / length
		}
		g.input.stickX, g.input.stickY = dx, dy
		in.Left = in.Left || dx < -7
		in.Right = in.Right || dx > 7
		in.Up = in.Up || dy < -7
		in.Down = in.Down || dy > 7
	case fireRole:
		in.Fire = true
		g.input.fire = true
	case forceRole:
		in.Force = true
		g.input.force = true
	case pauseRole:
		in.Pause = true
	case startRole:
		in.Start = true
	}
}
