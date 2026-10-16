package sim

import "fmt"

// originalDeathEffect preserves the actor's top-left when the original sprite
// descriptor changes. It begins on the next main update after a collision.
func (w *World) originalDeathEffect(actor Entity, kind int) (Effect, bool) {
	tables := w.weaponTables()
	if tables == nil || kind < 0 || kind >= len(tables.Death) {
		return Effect{}, false
	}
	d := tables.Death[kind]
	return Effect{Entity: w.newEntity(actor.X-actor.W/2+float64(d.Width)/2, actor.Y-actor.H/2+float64(d.Height)/2, 0, 0, float64(d.Width), float64(d.Height)), Delay: 1, Sound: d.Sound, Sprite: fmt.Sprintf("object-0-%05x", d.Descriptor), FrameTicks: d.FrameTicks, Animation: append([]int(nil), d.Animation...), Lifetime: d.Lifetime}, true
}
