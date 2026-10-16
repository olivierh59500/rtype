package game

import (
	"fmt"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/olivierh59500/r-type/internal/sim"
	"image"
	"image/color"
	"strings"
)

func (g *Game) scoreDisplay() bool {
	return g.World.State == sim.ScoreEntry || g.World.State == sim.ScoreTable
}

func (g *Game) displayHeight() int {
	if g.World.State == sim.Title && g.World.Menu != nil {
		return g.World.Menu.Height
	}
	return sim.Height
}

func (g *Game) drawRecords(screen *ebiten.Image) {
	offset, clip := g.presentationGeometry()
	defer vector.DrawFilledRect(screen, 0, 0, float32(clip), float32(screen.Bounds().Dy()), color.Black, false)
	menuRecords := g.AtTitle() && g.World.Menu != nil && g.World.Menu.Records
	for index, entry := range g.World.Records {
		if menuRecords && index >= g.World.Menu.RecordRows {
			break
		}
		if row := g.images["records-row"]; row != nil {
			if g.AtTitle() {
				row = g.paletteImage("records-row.png", row)
			}
			row = row.SubImage(image.Rect(0, index*12, 64, index*12+12)).(*ebiten.Image)
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(float64(32+offset), float64(entry.Y-2))
			screen.DrawImage(row, op)
		}
		if !menuRecords {
			g.recordName(screen, fmt.Sprintf("%6d", entry.Score%1000000), 112+offset, entry.Y)
			g.recordName(screen, entry.Name, 192+offset, entry.Y)
		}
	}
	if menuRecords {
		for _, line := range g.World.Menu.RecordText {
			g.recordName(screen, line.Text, line.X+offset, line.Y)
		}
		return
	}
	if s := g.World.RecordEntry; s != nil {
		g.recordName(screen, g.World.RecordPrompt, 56+offset, 180)
		// Icons H/C, Delete, Space and End keep their original uppercase codes.
		choice := g.World.RecordChoices[s.Choice]
		index := strings.IndexByte(g.fontChars, choice)
		if font := g.recordFont(); font != nil && index >= 0 {
			glyph := font.SubImage(image.Rect(index*8, 0, index*8+8, 8)).(*ebiten.Image)
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(float64(192+offset+len(s.Name)*8), float64(g.World.Records[s.Row].Y))
			screen.DrawImage(glyph, op)
		}
	}
}

func (g *Game) recordName(screen *ebiten.Image, name string, x, y int) {
	font := g.recordFont()
	if font == nil {
		return
	}
	for position, character := range []byte(name) {
		index := strings.IndexByte(g.fontChars, character)
		if index < 0 {
			continue
		}
		glyph := font.SubImage(image.Rect(index*8, 0, index*8+8, 8)).(*ebiten.Image)
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(x+position*8), float64(y))
		screen.DrawImage(glyph, op)
	}
}

func (g *Game) recordFont() *ebiten.Image {
	if font := g.images["records-font"]; font != nil {
		if g.AtTitle() || g.World.StageResult != nil {
			return g.paletteImage("records-font.png", font)
		}
		return font
	}
	return g.images["font"]
}
