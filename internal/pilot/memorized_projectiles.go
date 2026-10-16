package pilot

import "math"

// recognizeHostileShot uses the displayed projectile image and an observed
// direction. Familiar impact sparks are harmless; live shots retain delayed
// perception but use the authored sixteen-direction speed table.
func (m *memorizedObservation) recognizeHostileShot(actor *Actor) bool {
	if m.stage.Weapons == nil || m.stage.Weapons.HostileShot == nil {
		return false
	}
	d := m.stage.Weapons.HostileShot
	for frame, image := range d.Frames {
		if actor.Image != image {
			continue
		}
		if frame > 0 {
			return true
		}
		length := math.Hypot(actor.VX, actor.VY)
		if length < 0.2 {
			return false
		}
		best, score := 0, -2.0
		for direction, vx := range d.VelocityX {
			vy := d.VelocityY[direction]
			dot := (actor.VX*float64(vx) + actor.VY*float64(vy)) / (length * math.Hypot(float64(vx), float64(vy)))
			if dot > score {
				best, score = direction, dot
			}
		}
		actor.VX, actor.VY = float64(d.VelocityX[best])/2, float64(d.VelocityY[best])/2
		return false
	}
	return false
}
