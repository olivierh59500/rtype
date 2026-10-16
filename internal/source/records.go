package source

import (
	"encoding/binary"
	"image"
	"strings"
)

// HighScoreEntry preserves the authored name, packed BCD score and row origin.
type HighScoreEntry struct {
	Name  string `json:"name"`
	Score int    `json:"score"`
	Y     int    `json:"y"`
}

type RecordsData struct {
	RowImage, FontImage string
	Entries             []HighScoreEntry `json:"entries"`
	Choices             string           `json:"choices"`
	Prompt              string           `json:"prompt"`
}

// extractRecordImages decodes the actual rank backing copied by 0x3732.
// The five-track presentation bank is data for extraction only; its routines
// never enter the native application's runtime bundle.
func extractRecordImages(memory, presentation []byte) (image.Image, image.Image) {
	palette := append([]uint16{0}, words(memory[0x56c8:0x56c8+30])...)
	row := image.NewPaletted(image.Rect(0, 0, 64, 120), indexedPalette(palette, false))
	for entry := 0; entry < 10; entry++ {
		for y := 0; y < 12; y++ {
			for x := 0; x < 64; x++ {
				value := 0
				for plane := 0; plane < 3; plane++ {
					address := 0x50832 - 0x4a300 + x/32*48 + y*4 + (x%32)/8 + plane*0x210
					if x >= 32 {
						address += entry * 48
					}
					value |= int(presentation[address]>>(7-x%8)&1) << plane
				}
				row.SetColorIndex(x, entry*12+y, uint8(value))
			}
		}
	}
	font := image.NewPaletted(image.Rect(0, 0, 48*8, 8), indexedPalette(palette, false))
	for index := 0; index < 48; index++ {
		for y := 0; y < 8; y++ {
			for x := 0; x < 8; x++ {
				value := int(memory[0x2ceb8+index*8+y]>>(7-x)&1) | int(memory[0x2ceb8+0x180+index*8+y]>>(7-x)&1)<<1
				font.SetColorIndex(index*8+x, y, uint8(value))
			}
		}
	}
	return row, font
}

func extractRecords(memory []byte) *RecordsData {
	d := &RecordsData{Choices: string(memory[0x570b:0x573a])}
	text := func(address, maximum int) string {
		end := address
		for end < address+maximum && memory[end] != 0 {
			end++
		}
		return strings.TrimRight(string(memory[address:end]), " ")
	}
	d.Prompt = text(0x573c, 32)
	for index := 0; index < 10; index++ {
		entry := 0x575c + index*12
		position := int(binary.BigEndian.Uint32(memory[entry:]))
		name := int(binary.BigEndian.Uint32(memory[entry+4:]))
		d.Entries = append(d.Entries, HighScoreEntry{Name: text(name, 16), Score: bcdScore(binary.BigEndian.Uint32(memory[entry+8:])), Y: position/208 + 2 - 32})
	}
	return d
}
