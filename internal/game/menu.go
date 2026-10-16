package game

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/olivierh59500/r-type/internal/sim"
	"image"
	"image/color"
	"strings"
)

// presentationGeometry maps simulation's raw origin to the independently
// decoded bitplane fetch. Older local manifests use the verified original
// fallback: column 48 and sixteen hidden pixels at DIWSTRT=0x2C91.
func (g *Game) presentationGeometry() (offset, clip int) {
	display := g.display
	if g.AtTitle() && g.menuDisplay != nil {
		display = g.menuDisplay
	}
	if display == nil {
		return -16, 16
	}
	return sim.GameOriginX - display.BufferOriginX, display.VisibleX
}

// drawMenu presents the game's authored tutorial, credits and record pages.
// Original object positions are already normalized to the 192-row display.
func (g *Game) drawMenu(screen *ebiten.Image) {
	m := g.World.Menu
	offset, clip := g.presentationGeometry()
	g.updatePalette()
	defer vector.DrawFilledRect(screen, 0, 0, float32(clip), float32(screen.Bounds().Dy()), color.Black, false)
	if m.Records {
		g.drawRecords(screen)
		return
	}
	for _, actor := range m.Actors {
		g.sprite(screen, actor.Sprite, float64(actor.X+offset)+float64(actor.W)/2, float64(actor.Y)+float64(actor.H)/2)
	}
	font := g.paletteImage(m.FontImage, g.images[m.FontImage])
	if font != nil {
		for _, line := range m.Text {
			for position, character := range []byte(line.Text) {
				index := strings.IndexByte(g.fontChars, character)
				if index < 0 {
					index = len(g.fontChars) - 1
				}
				glyph := font.SubImage(image.Rect(index*8, 0, index*8+8, 8)).(*ebiten.Image)
				op := &ebiten.DrawImageOptions{}
				op.GeoM.Translate(float64(line.X+offset+position*8), float64(line.Y))
				screen.DrawImage(glyph, op)
			}
		}
	}
	if m.FooterRows > 0 {
		footer := g.paletteImage(m.FooterImage, g.images[m.FooterImage])
		if footer != nil {
			portion := footer.SubImage(image.Rect(0, 0, m.FooterWidth, min(m.FooterRows, m.FooterHeight))).(*ebiten.Image)
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(float64(m.FooterX+offset), float64(m.FooterY))
			screen.DrawImage(portion, op)
		}
	}
}
