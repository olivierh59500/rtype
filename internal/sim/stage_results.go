package sim

import "strings"

type stageResultGlyph struct {
	cycles          int64
	line, completed int
	cursor          bool
}

// StageResultRow keeps the source character currently present in each font
// bitplane of one cell row. The original printer writes those planes separately.
type StageResultRow struct{ Plane0, Plane1 byte }

type stageResultPaint struct {
	cycles            int64
	line, cell, plane int
	code              byte
}

// StageResultState is the original stage-clear interstitial. Digits contains
// eight source-font cells, with leading spaces and the current reveal frame.
// Core 0x42C retains the last playfield's low-four-bit display scroll values.
type StageResultState struct {
	Stage, Score                         int
	FineScrollX, FineScrollY             int
	Digits                               string
	Ready                                bool
	Palette                              []uint16
	Heading, Label                       string
	Printing                             bool
	VisibleHeading, VisibleLabel         string
	PaintedRows                          [2][]StageResultRow
	printPAL, printCycles, paletteCycles int64
	printClockHz, printPALHz             int
	printGlyphs                          []stageResultGlyph
	paintEvents                          []stageResultPaint
	fullPalette                          []uint16
	waitPAL                              int
	lastDigit, column, repeat, period    int
}

func (w *World) beginStageResult() bool {
	tables := w.weaponTables()
	if tables == nil || tables.StageResults == nil {
		return false
	}
	data := tables.StageResults
	digit := 0
	for _, threshold := range data.Thresholds {
		if w.StageScore >= threshold.Score {
			digit = threshold.LastDigit
			break
		}
	}
	heading := []byte(data.Heading)
	if len(heading) > 12 {
		heading[12] = byte('0' + w.Stage)
	}
	w.StageResult = &StageResultState{Stage: w.Stage, Score: w.StageScore, FineScrollX: int(w.Scroll) & 15, FineScrollY: int(w.ScrollY) & 15, Palette: append([]uint16(nil), data.Palette...), Heading: string(heading), Label: data.Label, lastDigit: digit, column: digit, repeat: data.RevealPeriod, period: data.RevealPeriod}
	s := w.StageResult
	s.Printing = true
	s.fullPalette, s.Palette = s.Palette, make([]uint16, len(data.Palette))
	s.printClockHz, s.printPALHz = data.CPUClockHz, data.PALClockHz
	s.paletteCycles = int64(data.ClearCycles + data.PaletteCycles)
	s.PaintedRows[0] = make([]StageResultRow, len(s.Heading)*8)
	s.PaintedRows[1] = make([]StageResultRow, len(s.Label)*8)
	paint := func(start int64, line, column int, code byte) {
		for row := 0; row < 8; row++ {
			for plane := 0; plane < 2; plane++ {
				s.paintEvents = append(s.paintEvents, stageResultPaint{cycles: start + int64(row*data.GlyphRowCycles+plane*data.GlyphPlaneCycles), line: line, cell: column*8 + row, plane: plane, code: code})
			}
		}
	}
	cycles := s.paletteCycles + int64(data.PrintInitialCycles)
	for line, text := range []string{s.Heading, s.Label} {
		if line > 0 {
			cycles += int64(data.PrintLineCycles)
		}
		for column, character := range []byte(text) {
			paint(cycles+int64(data.CursorPaintCycles), line, column, '_')
			s.printGlyphs = append(s.printGlyphs, stageResultGlyph{cycles: cycles + int64(data.CursorCycles), line: line, completed: column, cursor: true})
			glyph := strings.IndexByte(data.FontCharacters, character)
			if glyph < 0 {
				glyph = max(0, len(data.FontCharacters)-1)
			}
			cycles += int64(data.PrintCharacterCycles + data.PrintGlyphCycles*glyph)
			paint(cycles-int64(data.CharacterPaintLeadCycles), line, column, character)
			s.printGlyphs = append(s.printGlyphs, stageResultGlyph{cycles: cycles, line: line, completed: column + 1})
		}
	}
	s.printCycles = cycles + int64(data.PrintFinishCycles)
	return true
}

// advanceStageResultPrinting follows the original blocking CPU printer at PAL
// boundaries. Interrupt-driven hardware sprites and music remain independent.
func (w *World) advanceStageResultPrinting() {
	s := w.StageResult
	s.printPAL++
	cycles := s.printPAL * int64(s.printClockHz) / int64(s.printPALHz)
	if cycles >= s.paletteCycles {
		s.Palette = s.fullPalette
	}
	for len(s.paintEvents) > 0 && cycles >= s.paintEvents[0].cycles {
		event := s.paintEvents[0]
		s.paintEvents = s.paintEvents[1:]
		if event.plane == 0 {
			s.PaintedRows[event.line][event.cell].Plane0 = event.code
		} else {
			s.PaintedRows[event.line][event.cell].Plane1 = event.code
		}
	}
	for len(s.printGlyphs) > 0 && cycles >= s.printGlyphs[0].cycles {
		event := s.printGlyphs[0]
		s.printGlyphs = s.printGlyphs[1:]
		text := s.Heading
		if event.line == 1 {
			text = s.Label
		}
		text = text[:event.completed]
		if event.cursor {
			text += "_"
		}
		if event.line == 0 {
			s.VisibleHeading = text
		} else {
			s.VisibleLabel = text
		}
	}
	if cycles >= s.printCycles {
		s.Printing = false
		// 0x5136 has expired during the blocking clear/print. The first
		// score-frame wait releases on the next PAL, then every two PALs.
		s.waitPAL = 1
		w.revealStageScoreDigits()
	}
}

func (w *World) advanceStageResultPAL() {
	if w.StageResult.Printing {
		w.advanceStageResultPrinting()
		return
	}
	w.StageResult.waitPAL--
	if w.StageResult.waitPAL <= 0 {
		w.advanceStageResult()
		w.StageResult.waitPAL = max(1, w.MainTickDivider)
	}
}

func (w *World) revealStageScoreDigits() {
	s := w.StageResult
	digits := []byte("        ")
	score := s.Score
	for digit := s.lastDigit; digit >= 0; digit-- {
		value := score % 10
		score /= 10
		if s.column >= digit {
			value = int(w.nextRandom() % 10)
		}
		digits[7-(s.lastDigit-digit)] = byte('0' + value)
	}
	s.Digits = string(digits)
}

func (w *World) advanceStageResult() {
	s := w.StageResult
	if s == nil || s.Ready {
		return
	}
	s.repeat--
	if s.repeat <= 0 {
		s.repeat = s.period
		s.column--
		if s.column == -2 {
			s.Ready = true
			w.StageScore = 0
			return
		}
	}
	w.revealStageScoreDigits()
}
