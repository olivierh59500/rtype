package game

import (
	"github.com/hajimehoshi/ebiten/v2"
	"image"
	"image/color"
)

// loadChargeImages transcribes core 0x2056/0x20BA's three32-bit HUD masks.
// Each charge tick sets four pixels in plane zero across five rows; the other
// two original planes remain untouched. Copper COLOR01 writes provide the
// recovered blue gradient for each of the five filled scanlines.
func (g *Game) loadChargeImages(colors []uint16) {
	hud := g.bundle.IndexedImages["hud.png"]
	if hud == nil || len(hud.Palette) != 8 {
		return
	}
	for charge := 0; charge <= 24; charge++ {
		img := image.NewRGBA(image.Rect(0, 0, 96, 5))
		for y := 0; y < 5; y++ {
			for x := 0; x < 96; x++ {
				index := hud.ColorIndexAt(128+x, 1+y) &^ 1
				if x < charge*4 {
					index |= 1
				}
				shade := hud.Palette[index]
				if index == 1 {
					word := colors[y]
					shade = color.RGBA{R: byte(word>>8&15) * 17, G: byte(word>>4&15) * 17, B: byte(word&15) * 17, A: 255}
				}
				img.Set(x, y, shade)
			}
		}
		g.chargeImages = append(g.chargeImages, ebiten.NewImageFromImage(img))
	}
}
