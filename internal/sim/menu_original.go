package sim

import (
	"fmt"
	"github.com/olivierh59500/r-type/internal/source"
	"strings"
)

type MenuPhase uint8

const (
	MenuTutorial MenuPhase = iota
	MenuTutorialFinal
	MenuFooter
	MenuFadeToCredits
	MenuCredits
	MenuRecords
)

// MenuActor is an original presentation object in logical top-left coordinates.
// Sprite is its full single-frame asset name, rather than an invented graphic.
type MenuActor struct {
	Sprite                   string
	X, Y, W, H, Frame        int
	Variant                  int
	Phase, Ticks, MotionTick int
	Trajectory               []int
	Active                   bool
}
type OriginalMenuState struct {
	RecordRows                                              int
	RecordText                                              []source.MenuText
	PrintRecords                                            bool
	Printing                                                bool
	PrintLine, PrintCharacter                               int
	PrintBudget                                             int64
	PrintLines                                              []source.MenuText
	PrintResume                                             MenuPhase
	Phase                                                   MenuPhase
	Height                                                  int
	Palette                                                 []uint16
	Actors                                                  []MenuActor
	Text                                                    []source.MenuText
	FontImage, FooterImage                                  string
	FooterX, FooterY, FooterWidth, FooterHeight, FooterRows int
	Records                                                 bool
	Delay, FadeCounter                                      int
	PAL                                                     uint64
	Gate, Finished, wasTitle                                bool
}

func (w *World) SetMenu(data *source.MenuData) error {
	if data == nil {
		w.menuData = nil
		w.Menu = nil
		return nil
	}
	if len(data.Palette) != 16 || len(data.RecordsPalette) != 16 || len(data.Actors) != 7 {
		return fmt.Errorf("invalid original presentation menu")
	}
	w.menuData = cloneOriginalData(data)
	w.resetOriginalMenu()
	return nil
}

func (w *World) resetOriginalMenu() {
	d := w.menuData
	if d == nil {
		return
	}
	w.Menu = &OriginalMenuState{Height: d.Height, FontImage: d.FontImage, FooterImage: d.FooterImage, FooterX: d.FooterX, FooterY: d.FooterY, FooterWidth: d.FooterWidth, FooterHeight: d.FooterHeight, wasTitle: true}
	w.initializeMenuTutorial()
}

func (w *World) initializeMenuTutorial() {
	m, d := w.Menu, w.menuData
	m.Phase, m.Delay, m.Gate, m.Finished, m.Records = MenuTutorial, 0, true, false, false
	m.Palette = append(m.Palette[:0], d.Palette...)
	m.Text = nil
	m.Printing = false
	m.PrintLines = nil
	m.RecordRows = 0
	m.RecordText = nil
	m.PrintRecords = false
	m.FooterRows = 0
	m.Actors = nil
	// Core0x12A6 prepends priority-zero actors. The priority-one ship follows
	// the six title objects and therefore observes their shared gate last.
	for _, variant := range []int{1, 2, 3, 4, 5, 6, 0} {
		for _, original := range d.Actors {
			if original.Variant != variant {
				continue
			}
			m.Actors = append(m.Actors, MenuActor{Sprite: original.Sprite, X: original.X - GameOriginX, Y: original.Y - GameOriginY, W: original.Width, H: original.Height, Variant: original.Variant, Trajectory: original.Trajectory, Active: true})
		}
	}
}

func (w *World) updateOriginalMenu() {
	m := w.Menu
	if m == nil {
		return
	}
	if !m.wasTitle {
		w.resetOriginalMenu()
		m = w.Menu
	}
	m.PAL++
	// Core 0x33CC enables the 0x513C menu flag. IRQ 0x26A2 then releases
	// its presentation loop every PAL frame; the game-main loop stays 25 Hz.
	w.advanceOriginalMenu()
}

func (w *World) advanceOriginalMenu() {
	m, d := w.Menu, w.menuData
	if m.Printing {
		w.advanceMenuPrinting()
		return
	}
	if m.Delay > 0 {
		m.Delay--
		return
	}
	switch m.Phase {
	case MenuTutorial:
		w.updateMenuActors()
		if m.Finished {
			m.Phase = MenuTutorialFinal
		}
	case MenuTutorialFinal:
		w.updateMenuActors()
		w.beginMenuPrinting(d.FinalText, MenuFooter)
	case MenuFooter:
		m.FooterRows++
		m.Delay = 2
		if m.FooterRows >= d.FooterHeight {
			m.Phase = MenuFadeToCredits
			m.Delay = 200
			m.FadeCounter = 15
		}
	case MenuFadeToCredits, MenuCredits, MenuRecords:
		for color := 1; color < 16; color++ {
			for shift := 0; shift <= 8; shift += 4 {
				if m.Palette[color]&(15<<shift) != 0 {
					m.Palette[color] -= uint16(1 << shift)
				}
			}
		}
		m.Delay = 4
		m.FadeCounter--
		if m.FadeCounter >= 0 {
			return
		}
		switch m.Phase {
		case MenuFadeToCredits:
			m.Phase = MenuCredits
			m.Palette = append(m.Palette[:0], d.Palette...)
			m.Actors = nil
			m.FooterRows = 0
			w.beginMenuPrinting(d.Credits, MenuCredits)
		case MenuCredits:
			m.Phase = MenuRecords
			m.Palette = append(m.Palette[:0], d.RecordsPalette...)
			m.Text = nil
			m.Actors = nil
			m.Records = true
			w.beginMenuRecordsPrinting()
		case MenuRecords:
			w.initializeMenuTutorial()
		}
	}
}

// beginMenuPrinting starts core0x3924's blocking authored-text pass. Main-loop
// counters pause, while PAL input and music continue during the glyph waits.
func (w *World) beginMenuPrinting(lines []source.MenuText, resume MenuPhase) {
	m := w.Menu
	m.Printing = true
	m.PrintRecords = false
	m.PrintLines = append([]source.MenuText(nil), lines...)
	m.PrintLine, m.PrintCharacter, m.PrintBudget = 0, 0, 0
	m.PrintBudget = -int64(w.menuData.PrintInitialCycles) * int64(max(1, w.menuData.PALClockHz))
	m.PrintResume = resume
	m.Text = nil
	w.refreshMenuPrinting()
}

func (w *World) beginMenuRecordsPrinting() {
	var lines []source.MenuText
	for _, record := range w.Records {
		name := record.Name
		if len(name) > 16 {
			name = name[:16]
		}
		name += strings.Repeat(" ", 16-len(name))
		lines = append(lines, source.MenuText{X: 112, Y: record.Y, Text: fmt.Sprintf("%6d", record.Score%1000000)}, source.MenuText{X: 192, Y: record.Y, Text: name})
	}
	w.beginMenuPrinting(lines, MenuRecords)
	m := w.Menu
	m.PrintRecords = true
	m.RecordRows = 1
	m.PrintBudget = -int64(w.menuData.RecordInitialCycles) * int64(max(1, w.menuData.PALClockHz))
	w.refreshMenuPrinting()
}

func (w *World) refreshMenuPrinting() {
	m := w.Menu
	m.Text = nil
	for index, line := range m.PrintLines {
		if index < m.PrintLine {
			m.Text = append(m.Text, line)
			continue
		}
		if index == m.PrintLine {
			text := line.Text[:m.PrintCharacter]
			if m.PrintCharacter < len(line.Text) {
				text += "_"
			}
			line.Text = text
			m.Text = append(m.Text, line)
		}
		break
	}
	if m.PrintRecords {
		m.RecordText = append(m.RecordText[:0], m.Text...)
		m.Text = nil
	}
}

func (w *World) advanceMenuPrinting() {
	m, d := w.Menu, w.menuData
	clock, frequency := d.CPUClockHz, d.PALClockHz
	if clock <= 0 {
		clock = 7093790
	}
	if frequency <= 0 {
		frequency = 50
	}
	m.PrintBudget += int64(clock)
	for m.PrintLine < len(m.PrintLines) {
		line := m.PrintLines[m.PrintLine]
		if m.PrintCharacter >= len(line.Text) {
			if m.PrintLine == len(m.PrintLines)-1 {
				finish := d.PrintFinishCycles
				if m.PrintRecords {
					finish = d.RecordFinishCycles
				}
				cost := int64(finish) * int64(frequency)
				if m.PrintBudget < cost {
					w.refreshMenuPrinting()
					return
				}
				m.PrintBudget -= cost
			} else {
				overhead := d.PrintLineCycles
				if m.PrintRecords {
					if m.PrintLine&1 == 0 {
						overhead = d.RecordPairCycles
					} else {
						overhead = d.RecordRowCycles
						m.RecordRows++
					}
				}
				m.PrintBudget -= int64(overhead) * int64(frequency)
			}
			m.PrintLine++
			m.PrintCharacter = 0
			continue
		}
		glyph := strings.IndexByte(d.FontCharacters, line.Text[m.PrintCharacter])
		if glyph < 0 {
			glyph = max(0, len(d.FontCharacters)-1)
		}
		cycles := d.PrintCharacterCycles + d.PrintGlyphCycles*glyph
		if m.PrintRecords {
			cycles = d.RecordCharacterCycles + d.PrintGlyphCycles*glyph
		}
		if cycles <= 0 {
			cycles = 119300
		}
		cost := int64(cycles) * int64(frequency)
		if m.PrintBudget < cost {
			w.refreshMenuPrinting()
			return
		}
		m.PrintBudget -= cost
		m.PrintCharacter++
	}
	m.Printing = false
	m.Text = append([]source.MenuText(nil), m.PrintLines...)
	if m.PrintRecords {
		m.RecordText = append(m.RecordText[:0], m.Text...)
		m.Text = nil
	}
	m.Phase = m.PrintResume
	if m.PrintResume == MenuFooter {
		m.FooterRows = 1
		m.Delay = 2
	} else if m.PrintResume == MenuRecords {
		m.Delay = 250
		m.FadeCounter = 15
	} else {
		m.Delay = 200
		m.FadeCounter = 15
	}
}

func (w *World) updateMenuActors() {
	m := w.Menu
	for i := range m.Actors {
		actor := &m.Actors[i]
		if !actor.Active {
			continue
		}
		if actor.Variant == 0 {
			if m.Gate && m.Finished == false && anyMenuActorExiting(m.Actors) {
				actor.X += 6
				actor.Y += 3
				if actor.Y+GameOriginY >= 150 {
					m.Gate = false
					actor.Variant = -1
					actor.Phase = -1
					actor.Ticks = 16
				}
			}
			continue
		}
		switch actor.Phase {
		case 0:
			actor.X -= 5
			if actor.X+GameOriginX <= 70 {
				actor.Phase = 1
				actor.Ticks = 45
			}
		case 1:
			if actor.MotionTick < len(actor.Trajectory) {
				actor.X = actor.Trajectory[actor.MotionTick] + 70 - GameOriginX
				actor.MotionTick++
			}
			actor.Ticks--
			if actor.Ticks == 0 {
				actor.Phase = -1
				actor.Ticks = 16
				m.Gate = true
			}
		case -1:
			if !m.Gate {
				actor.Y += 3
				actor.Ticks--
				if actor.Ticks == 0 {
					actor.Active = false
					m.Finished = true
				}
			}
		}
	}
}

func anyMenuActorExiting(actors []MenuActor) bool {
	for _, actor := range actors {
		if actor.Variant > 0 && actor.Phase < 0 {
			return true
		}
	}
	return false
}
