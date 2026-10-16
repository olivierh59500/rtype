package game

import (
	"image/color"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/olivierh59500/r-type/internal/sim"
)

// drawStageResult replaces the playfield after the automatic ship exit. The
// original score reveal runs in native Go while the completion score finishes.
func (g *Game) drawStageResult(screen *ebiten.Image) {
	result := g.World.StageResult
	if result == nil {
		return
	}
	g.updatePalette()
	offset, clip := g.presentationGeometry()
	// These are direct framebuffer writes. Unlike BOBs, the printer never
	// adds 0x48C8/0x48CA before the retained display scroll is applied.
	offset += 15 - result.FineScrollX
	if result.Printing {
		g.drawStageResultPrinting(screen, offset, -result.FineScrollY)
	} else {
		g.recordName(screen, result.Heading, 56+offset, 20-result.FineScrollY)
		g.recordName(screen, result.Label, 96+offset, 58-result.FineScrollY)
	}
	g.recordName(screen, result.Digits, 184+offset, 58-result.FineScrollY)
	g.drawHardwareSprites(screen)
	vector.DrawFilledRect(screen, 0, 0, float32(clip), sim.PlayHeight, color.Black, false)
	// Original clear 0x366A starts after the separate score-strip storage.
	g.drawHUD(screen)
}

// drawStageResultPrinting combines the two font bitplanes row by row. A PAL
// boundary can interrupt a glyph store before its routine returns, leaving a
// mixture of the old cursor and the arriving character in the original buffer.
func (g *Game) drawStageResultPrinting(screen *ebiten.Image, offsetX, offsetY int) {
	result := g.World.StageResult
	font := g.bundle.IndexedImages["records-font.png"]
	if font == nil {
		return
	}
	for line, rows := range result.PaintedRows {
		width := len(rows)
		if width == 0 {
			continue
		}
		if g.resultPrintImages[line] == nil || g.resultPrintImages[line].Bounds().Dx() != width {
			if g.resultPrintImages[line] != nil {
				g.resultPrintImages[line].Deallocate()
			}
			g.resultPrintImages[line] = ebiten.NewImage(width, 8)
			g.resultPrintPixels[line] = make([]byte, width*8*4)
		}
		pixels := g.resultPrintPixels[line]
		for column := 0; column < width/8; column++ {
			for row := 0; row < 8; row++ {
				cell := rows[column*8+row]
				lower, upper := strings.IndexByte(g.fontChars, cell.Plane0), strings.IndexByte(g.fontChars, cell.Plane1)
				for x := 0; x < 8; x++ {
					index := uint8(0)
					if lower >= 0 {
						index |= font.ColorIndexAt(lower*8+x, row) & 1
					}
					if upper >= 0 {
						index |= font.ColorIndexAt(upper*8+x, row) & 2
					}
					word := result.Palette[index]
					at := (row*width + column*8 + x) * 4
					pixels[at], pixels[at+1], pixels[at+2], pixels[at+3] = byte(word>>8&15)*17, byte(word>>4&15)*17, byte(word&15)*17, 255
				}
			}
		}
		g.resultPrintImages[line].WritePixels(pixels)
		x, y := 56, 20
		if line == 1 {
			x, y = 96, 58
		}
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(x+offsetX), float64(y+offsetY))
		screen.DrawImage(g.resultPrintImages[line], op)
	}
}
