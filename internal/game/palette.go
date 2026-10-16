package game

import (
	"fmt"
	"image"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/olivierh59500/r-type/internal/sim"
)

// updatePalette invalidates only the current native palette variants. Original
// decoded indices stay intact, including different indices with equal RGB.
func (g *Game) updatePalette() {
	palette := g.World.PlayfieldPalette()
	if g.World.State == sim.Title && g.World.Menu != nil {
		palette = g.World.Menu.Palette
	}
	same := len(palette) == len(g.activePalette)
	if same {
		for index, word := range palette {
			if word != g.activePalette[index] {
				same = false
				break
			}
		}
	}
	if same {
		return
	}
	g.activePalette = append(g.activePalette[:0], palette...)
	g.paletteImages = make(map[string]*ebiten.Image)
}

// flashImage follows the original blitter's mask-as-color operation. Every
// opaque source pixel becomes palette entry15; its original mask is preserved.
func (g *Game) flashImage(file string, original *ebiten.Image) *ebiten.Image {
	indices := g.bundle.IndexedImages[file]
	if indices == nil || len(indices.Palette) < 16 {
		return original
	}
	shade := indices.Palette[15]
	if len(g.activePalette) == 16 {
		word := g.activePalette[15]
		shade = color.RGBA{R: byte(word>>8&15) * 17, G: byte(word>>4&15) * 17, B: byte(word&15) * 17, A: 255}
	}
	r, gc, b, _ := shade.RGBA()
	key := fmt.Sprintf("%s/%04x%04x%04x", file, r, gc, b)
	if cached := g.flashImages[key]; cached != nil {
		return cached
	}
	img := image.NewNRGBA(indices.Bounds())
	for y := indices.Bounds().Min.Y; y < indices.Bounds().Max.Y; y++ {
		for x := indices.Bounds().Min.X; x < indices.Bounds().Max.X; x++ {
			if indices.ColorIndexAt(x, y) < 16 {
				img.Set(x, y, shade)
			}
		}
	}
	result := ebiten.NewImageFromImage(img)
	if g.flashImages == nil {
		g.flashImages = make(map[string]*ebiten.Image)
	}
	g.flashImages[key] = result
	return result
}

// paletteImage performs the original twelve-bit palette lookup in Go. The
// seventeen-entry sprite PNGs reserve index sixteen for their original masks;
// hardware sprites, title art and the independent HUD are not indexed here.
func (g *Game) paletteImage(file string, original *ebiten.Image) *ebiten.Image {
	if original == nil || len(g.activePalette) < 16 {
		return original
	}
	indices := g.bundle.IndexedImages[file]
	if indices == nil || len(indices.Palette) > 17 {
		return original
	}
	if cached := g.paletteImages[file]; cached != nil {
		return cached
	}
	img := image.NewNRGBA(indices.Bounds())
	for y := indices.Bounds().Min.Y; y < indices.Bounds().Max.Y; y++ {
		for x := indices.Bounds().Min.X; x < indices.Bounds().Max.X; x++ {
			index := int(indices.ColorIndexAt(x, y))
			if index >= 16 {
				continue
			}
			word := g.activePalette[index]
			offset := img.PixOffset(x, y)
			img.Pix[offset], img.Pix[offset+1], img.Pix[offset+2], img.Pix[offset+3] = byte(word>>8&15)*17, byte(word>>4&15)*17, byte(word&15)*17, 255
		}
	}
	result := ebiten.NewImageFromImage(img)
	if g.paletteImages == nil {
		g.paletteImages = make(map[string]*ebiten.Image)
	}
	g.paletteImages[file] = result
	return result
}
