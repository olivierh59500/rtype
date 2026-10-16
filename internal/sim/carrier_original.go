package sim

import "math"

// updatePowerCarrier translates the common shootable power container's three
// states at original RAM 0x2E0E, 0x2E74 and 0x2EA0. Its animation and sine steps
// come from the extracted disk, rather than generated movement approximations.
func (w *World) updatePowerCarrier(enemy *Enemy) bool {
	if enemy.Behavior != BehaviorPowerCarrier || len(enemy.MotionY) != 58 {
		return false
	}
	x := enemy.X - enemy.W/2 + GameOriginX
	y := enemy.Y - enemy.H/2 + GameOriginY
	if x < 16 || x > 370 || y < 16 || y >= 224 {
		enemy.Health = 0
		return true
	}
	lead := func() float64 {
		if enemy.CarrierVX < 0 {
			return enemy.X - enemy.W/2 - 4
		}
		return enemy.X - enemy.W/2 + 29
	}
	setAnimation := func(ground bool) {
		frames := enemy.CarrierFloatAnimation
		if ground {
			frames = enemy.CarrierGroundAnimation
		}
		if enemy.CarrierVX != enemy.CarrierInitialVX {
			frames = enemy.CarrierAnimationReverseFloat
			if ground {
				frames = enemy.CarrierAnimationReverseGround
			}
		}
		enemy.Animation = frames
		enemy.AnimationStart = 0
		enemy.Age = 1
	}
	land := func() {
		originalY := int(enemy.Y - enemy.H/2 + GameOriginY)
		// The original rounds at bit three, then aligns the sprite eleven
		// pixels above the rounded boundary (RAM 0x2F20).
		rounded := originalY &^ 15
		if originalY&8 != 0 {
			rounded += 16
		}
		enemy.Y = float64(rounded-11) - GameOriginY + enemy.H/2
		enemy.CarrierPhase = 2
		setAnimation(true)
	}
	floating := func() {
		enemy.X += float64(enemy.CarrierVX)
		enemy.Y += float64(enemy.MotionY[enemy.CarrierMotionTick])
		enemy.CarrierMotionTick = (enemy.CarrierMotionTick + 1) % 58
		baseY := enemy.Y - enemy.H/2
		if w.carrierSolidAt(lead(), baseY+6) {
			enemy.CarrierPhase = 1
			return
		}
		if w.carrierSolidAt(lead(), baseY+22) {
			land()
		}
	}
	switch enemy.CarrierPhase {
	case 0:
		floating()
	case 1:
		if !w.carrierSolidAt(lead(), enemy.Y-enemy.H/2+23) {
			enemy.Y += 2
			enemy.X--
			break
		}
		land()
		fallthrough
	case 2:
		baseY := enemy.Y - enemy.H/2
		blocked := w.carrierSolidAt(lead(), baseY+15)
		if blocked || !w.carrierSolidAt(lead(), baseY+31) {
			if blocked {
				enemy.CarrierVX = -enemy.CarrierVX
			}
			enemy.CarrierPhase = 0
			enemy.CarrierMotionTick = 16
			setAnimation(false)
			floating()
			break
		}
		if enemy.CarrierVX < 0 {
			enemy.X -= 2
		} else {
			enemy.X += 2
		}
	}
	return true
}

// carrierSolidAt performs the original point lookup against collision tile
// flags. It also supports synthetic rectangle levels used by independent tests.
func (w *World) carrierSolidAt(x, y float64) bool {
	if math.IsNaN(x) || math.IsNaN(y) {
		return false
	}
	if w.Stage >= 1 && w.Stage <= len(w.Stages) && w.Stages[w.Stage-1].MapWidth > 0 {
		return w.OriginalSolidAt(int(x+GameOriginX), int(y+GameOriginY))
	}
	for _, tile := range w.TerrainInView() {
		if x >= tile.X && x < tile.X+tile.W && y >= tile.Y && y < tile.Y+tile.H {
			return true
		}
	}
	return false
}
