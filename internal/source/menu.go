package source

import (
	"encoding/binary"
	"fmt"
	"image"
)

// MenuText keeps an authored string and its logical viewport coordinates.
type MenuText struct {
	Address uint32 `json:"address"`
	X, Y    int
	Text    string `json:"text"`
}

// MenuActor preserves one original tutorial object and its optional X path.
type MenuActor struct {
	Variant             int    `json:"variant"`
	Descriptor          uint32 `json:"descriptor"`
	Sprite              string `json:"sprite"`
	Width, Height, X, Y int
	Trajectory          []int `json:"trajectory,omitempty"`
}

// MenuData supplies original menu colors, text and art to the native sequence.
type MenuData struct {
	Display                                                                                           *DisplayData `json:"display,omitempty"`
	PALClockHz                                                                                        int          `json:"pal_clock_hz"`
	PrintDelayWord                                                                                    int          `json:"print_delay_word"`
	PrintCharacterCycles                                                                              int          `json:"print_character_cycles"`
	PrintGlyphCycles                                                                                  int          `json:"print_glyph_cycles"`
	PrintInitialCycles, PrintLineCycles, PrintFinishCycles                                            int
	RecordCharacterCycles, RecordInitialCycles, RecordPairCycles, RecordRowCycles, RecordFinishCycles int
	CPUClockHz                                                                                        int         `json:"cpu_clock_hz"`
	FontCharacters                                                                                    string      `json:"font_characters"`
	Music                                                                                             *LevelMusic `json:"music,omitempty"`
	Height                                                                                            int         `json:"height"`
	FooterImage                                                                                       string      `json:"footer_image"`
	FooterX, FooterY, FooterWidth, FooterHeight                                                       int
	Palette, RecordsPalette                                                                           []uint16
	Credits, FinalText                                                                                []MenuText
	FontImage                                                                                         string      `json:"font_image"`
	Actors                                                                                            []MenuActor `json:"actors"`
}

func extractMenu(memory, presentation []byte, writeImage func(string, image.Image) error) (*MenuData, []Sprite, error) {
	if len(memory) < 0x5b00 || len(presentation) != 5*5632 {
		return nil, nil, fmt.Errorf("original menu ranges are unavailable")
	}
	u16 := func(a int) int { return int(binary.BigEndian.Uint16(memory[a:])) }
	u32 := func(a int) int { return int(binary.BigEndian.Uint32(memory[a:])) }
	d := &MenuData{Height: 192, FooterImage: "menu-footer.png", FooterX: 40, FooterY: 178, FooterWidth: 112, FooterHeight: 13, Palette: append([]uint16{0}, words(memory[0x56aa:0x56c8])...), RecordsPalette: append([]uint16{0}, words(memory[0x56c8:0x56e6])...), FontImage: "menu-font.png"}
	d.Display = extractDisplay(memory, true)
	d.PALClockHz = 50
	d.PrintDelayWord = u16(0x392e)
	// Isolated 0x3936 timing includes its delay and two glyph draws.
	d.PrintCharacterCycles = d.PrintDelayWord*14 + 4612
	d.PrintGlyphCycles = 34
	d.PrintInitialCycles, d.PrintLineCycles, d.PrintFinishCycles = 44, 136, 130
	d.RecordCharacterCycles = u16(0x37bc)*14 + 4612
	d.RecordInitialCycles, d.RecordPairCycles, d.RecordRowCycles, d.RecordFinishCycles = 4604, 114, 4684, 128
	d.CPUClockHz = 7093790
	d.FontCharacters = string(memory[0x570b:0x573b])
	textList := func(start int) ([]MenuText, error) {
		var out []MenuText
		for count := 0; count < 32; count++ {
			address := u32(start + count*8)
			if int32(address) < 0 {
				return out, nil
			}
			text := u32(start + count*8 + 4)
			if address < 0x2d1ba || address >= 0x48bba || text < 0x570b || text >= len(memory) {
				return nil, fmt.Errorf("invalid original menu text")
			}
			end := text
			for end < len(memory) && end < text+256 && memory[end] != 0 {
				end++
			}
			if end == len(memory) || end == text+256 {
				return nil, fmt.Errorf("unbounded menu text")
			}
			offset := address - 0x2d1ba
			out = append(out, MenuText{Address: uint32(address), X: offset%208*8 - 32, Y: offset/208 - 32, Text: string(memory[text:end])})
		}
		return nil, fmt.Errorf("menu text list has no end marker")
	}
	var err error
	d.Credits, err = textList(0x5962)
	if err != nil {
		return nil, nil, err
	}
	d.FinalText, err = textList(0x58c4)
	if err != nil {
		return nil, nil, err
	}
	font := image.NewPaletted(image.Rect(0, 0, 48*8, 8), indexedPalette(d.Palette, false))
	for glyph := 0; glyph < 48; glyph++ {
		for y := 0; y < 8; y++ {
			for x := 0; x < 8; x++ {
				index := int(memory[0x2ceb8+glyph*8+y]>>(7-x)&1) | int(memory[0x2ceb8+0x180+glyph*8+y]>>(7-x)&1)<<1
				font.SetColorIndex(glyph*8+x, y, uint8(index))
			}
		}
	}
	if err = writeImage(d.FontImage, font); err != nil {
		return nil, nil, err
	}
	bank := append([]byte(nil), memory...)
	copy(bank[0x4a300:], presentation)
	footer := image.NewPaletted(image.Rect(0, 0, d.FooterWidth, d.FooterHeight), indexedPalette(d.Palette, false))
	for y := 0; y < d.FooterHeight; y++ {
		for x := 0; x < d.FooterWidth; x++ {
			footer.SetColorIndex(x, y, uint8(planarIndex(bank, 0x50282, x, y, 14)))
		}
	}
	if err = writeImage(d.FooterImage, footer); err != nil {
		return nil, nil, err
	}
	var sprites []Sprite
	for actor := 0; actor < 7; actor++ {
		descriptor := u32(0x55ac + actor*8)
		path := u32(0x55b0 + actor*8)
		size := u16(descriptor)
		width, height := (size&63-1)*16, (size>>6)/4
		colors, mask, stride := u32(descriptor+12), u32(descriptor+16), u16(descriptor+4)
		if width < 1 || height < 1 || mask != width/8*height*4 || stride != mask*2 || colors < 0x4a300 || colors+stride > 0x4a300+len(presentation) {
			return nil, nil, fmt.Errorf("invalid original menu sprite %#x", descriptor)
		}
		name := fmt.Sprintf("menu-%05x-0", descriptor)
		file := "sprites/" + name + ".png"
		if err = writeImage(file, blitterSprite(bank, colors, colors+mask, width, height, d.Palette)); err != nil {
			return nil, nil, err
		}
		entry := MenuActor{Variant: 6 - actor, Descriptor: uint32(descriptor), Sprite: name, Width: width, Height: height, X: 378, Y: 100}
		if entry.Variant == 5 {
			entry.Y = 110
		}
		if entry.Variant == 0 {
			entry.X, entry.Y = 43, 0
		}
		if path != 0 {
			if path < 0x4a300 || path+45 > 0x4a300+len(presentation) {
				return nil, nil, fmt.Errorf("invalid original tutorial trajectory")
			}
			for _, value := range bank[path : path+45] {
				entry.Trajectory = append(entry.Trajectory, int(value))
			}
		}
		d.Actors = append(d.Actors, entry)
		sprites = append(sprites, Sprite{Name: name, File: file, Address: uint32(descriptor), Width: width, Height: height})
	}
	return d, sprites, nil
}
