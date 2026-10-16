package game

import "github.com/hajimehoshi/ebiten/v2"

// drawOriginalWeapons uses the original blitter pose instead of the stored
// post-movement collision position. Non-missile weapons precede the actor pass;
// missiles follow it. PAL-only frames retain the previous main-loop image.
func (g *Game) drawOriginalWeapons(screen *ebiten.Image, layer int) {
	for _, weapon := range g.World.WeaponImages {
		if weapon.Layer == layer {
			g.bob(screen, weapon.Sprite, weapon.X, weapon.Y)
		}
	}
}
