package source

import "encoding/binary"

// StageResultsData describes the original cleared-stage text and score reveal.
// The score thresholds and digit indices are the table at core RAM 0x4720.
type StageResultsData struct {
	Heading, Label, FontCharacters                       string
	Palette                                              []uint16
	Thresholds                                           []StageResultThreshold
	RevealPeriod                                         int
	CPUClockHz, PALClockHz                               int
	ClearCycles, PaletteCycles, PrintInitialCycles       int
	PrintCharacterCycles, PrintGlyphCycles, CursorCycles int
	PrintLineCycles, PrintFinishCycles                   int
	GlyphRowCycles, GlyphPlaneCycles                     int
	CursorPaintCycles, CharacterPaintLeadCycles          int
}
type StageResultThreshold struct{ Score, LastDigit int }

func extractStageResults(memory []byte) *StageResultsData {
	text := func(address int) string {
		end := address
		for end < address+64 && memory[end] != 0 {
			end++
		}
		return string(memory[address:end])
	}
	d := &StageResultsData{Heading: text(0x4748), Label: text(0x4766), Palette: append([]uint16{0}, words(memory[0x56c8:0x56e6])...), RevealPeriod: int(memory[0xa41]) + 1}
	d.FontCharacters = string(memory[0x570b:0x573b])
	// Original 0x366A clears one long per 44-cycle loop. The caller,
	// setup/return and final branch together contribute another 44 cycles.
	clearStart := int(binary.BigEndian.Uint32(memory[0x366c:]))
	clearEnd := int(binary.BigEndian.Uint32(memory[0x3674:]))
	d.ClearCycles = (clearEnd-clearStart)/4*44 + 44
	d.CPUClockHz, d.PALClockHz = 7093790, 50
	d.PaletteCycles, d.PrintInitialCycles = 516, 68
	d.PrintCharacterCycles = int(binary.BigEndian.Uint16(memory[0x446:]))*14 + 4612
	d.PrintGlyphCycles, d.CursorCycles = 34, 2994
	d.PrintLineCycles, d.PrintFinishCycles = 114, 100
	// First-buffer plane stores occur before the glyph routine returns.
	// Preserve their separate row/plane boundaries for a partially painted cell.
	d.GlyphRowCycles, d.GlyphPlaneCycles = 170, 32
	d.CursorPaintCycles, d.CharacterPaintLeadCycles = 1582, 1412
	for a := 0x4720; a < 0x473e; a += 6 {
		d.Thresholds = append(d.Thresholds, StageResultThreshold{Score: bcdScore(binary.BigEndian.Uint32(memory[a:])), LastDigit: int(binary.BigEndian.Uint16(memory[a+4:]))})
	}
	return d
}
