package pilot

// rehearsedMovement follows familiar joystick bounds, including the original
// compare-before-move edge overshoot. A neutral input cannot clamp or teleport
// a ship that already occupies that legal overshoot position.
func rehearsedMovement(o Observation, x, y float64, move action) (float64, float64, bool) {
	if o.memory == nil || o.memory.camera == nil || o.memory.stage.Weapons == nil || o.memory.stage.Weapons.Player == nil {
		return x, y, false
	}
	d := o.memory.stage.Weapons.Player
	if move.dx < 0 && x+113 > float64(d.LeftLimit) {
		x -= o.Speed
	} else if move.dx > 0 && x+113 <= float64(d.RightLimit) {
		x += o.Speed
	}
	if move.dy < 0 && y+37 > float64(d.TopLimit) {
		y -= o.Speed
	} else if move.dy > 0 && y+37 < float64(d.BottomLimit) {
		y += o.Speed
	}
	return x, y, true
}
