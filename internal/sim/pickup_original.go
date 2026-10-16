package sim

// Frame preserves the original item numbering independently of the public
// upgrade constants, whose historical Red/Blue order is different.
func (p PowerUp) Frame() int {
	switch p.Kind {
	case PowerBlue:
		return 0
	case PowerRed:
		return 1
	case PowerYellow:
		return 2
	case PowerSpeed:
		return 3
	case PowerMissiles:
		return 4
	case PowerBit:
		return 5
	}
	return 0
}

func (p PowerUp) overlapsPlayer(player Player) bool {
	if p.Original == nil {
		return p.Bounds().Overlaps(player.Bounds())
	}
	frame := p.Frame()
	if frame >= len(p.Original.Collision) {
		return false
	}
	box := p.Original.Collision[frame]
	e := Enemy{Entity: p.Entity, CollisionFrames: []CollisionFrame{{X: box.X, Y: box.Y, Width: box.Width, Height: box.Height}}}
	return e.overlapsHit(player.HitBounds())
}
