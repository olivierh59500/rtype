package pilot

import (
	"math"

	"github.com/olivierh59500/r-type/internal/sim"
	"github.com/olivierh59500/r-type/internal/source"
)

// memorizedIrisMovie rehearses only the authored opening patches and waits.
// Visible art must still match that sequence before it supplies foreknowledge;
// damage, hidden controller state, RNG and alternate worlds are never read.
type memorizedIrisMovie struct {
	column, row, width, height, frames int
	words                              []uint16
}

func rehearseIrisMovie(stage sim.StageSpec) *memorizedIrisMovie {
	r := &memorizedIrisMovie{}
	if stage.Boss == nil || stage.Boss.Third == nil || stage.Boss.Third.Iris == nil {
		return r
	}
	d := stage.Boss.Third.Iris
	left, top, right, bottom := stage.MapWidth, stage.MapHeight, 0, 0
	patches := []*source.MapPatch{d.InitialPatch}
	for _, f := range d.Opening {
		patches = append(patches, f.Patch)
	}
	for _, p := range patches {
		if p != nil {
			left = min(left, p.Column)
			top = min(top, p.Row)
			right = max(right, p.Column+p.Width)
			bottom = max(bottom, p.Row+p.Height)
		}
	}
	if left >= right || top >= bottom {
		return r
	}
	r.column, r.row, r.width, r.height = left, top, right-left, bottom-top
	r.frames = stage.Boss.Third.ExitReload + stage.Boss.VictoryTicks + 100
	area := r.width * r.height
	r.words = make([]uint16, (r.frames+1)*area)
	tiles := make([]uint16, area)
	for y := 0; y < r.height; y++ {
		copy(tiles[y*r.width:(y+1)*r.width], stage.TileMap[(top+y)*stage.MapWidth+left:(top+y)*stage.MapWidth+right])
	}
	apply := func(p *source.MapPatch) {
		if p == nil {
			return
		}
		for i, word := range p.Words {
			x, y := p.Column+i%p.Width-left, p.Row+i/p.Width-top
			if x >= 0 && x < r.width && y >= 0 && y < r.height {
				tiles[y*r.width+x] = word
			}
		}
	}
	for _, p := range stage.Boss.Third.InitialPatches {
		apply(p)
	}
	apply(d.InitialPatch)
	timer, cursor := 0, 0
	active := true
	for main := 0; main <= r.frames; main++ {
		if main > d.Trigger && active {
			timer--
			if timer < 0 {
				if cursor >= len(d.Opening) || d.Opening[cursor].Stop {
					active = false
				} else {
					f := d.Opening[cursor]
					timer = f.Reload
					apply(f.Patch)
					cursor++
				}
			}
		}
		copy(r.words[main*area:(main+1)*area], tiles)
	}
	return r
}

func (r *memorizedIrisMovie) word(column, row, main int) (uint16, bool) {
	if r == nil || r.width == 0 || column < r.column || column >= r.column+r.width || row < r.row || row >= r.row+r.height {
		return 0, false
	}
	main = max(0, min(main, r.frames))
	return r.words[main*r.width*r.height+(row-r.row)*r.width+column-r.column], true
}

func (r *memorizedIrisMovie) containsScreenCell(cell sim.Rect, scrollX, scrollY float64) bool {
	_, ok := r.word(int(math.Round(cell.X+scrollX-16))/16, int(math.Round(cell.Y+scrollY))/16, 0)
	return ok
}

func (r *memorizedIrisMovie) matchesVisible(w *sim.World, main int) bool {
	if r == nil || r.width == 0 {
		return false
	}
	seen := 0
	for y := r.row; y < r.row+r.height; y++ {
		for x := r.column; x < r.column+r.width; x++ {
			if !visible(sim.Rect{X: float64(x*16) + 16 - w.Scroll, Y: float64(y*16) - w.ScrollY, W: 16, H: 16}) {
				continue
			}
			// Terrain is drawn before the current main pass's map hook.
			word, _ := r.word(x, y, max(0, main-1))
			if perceivedTileArt(w, x, y) != word&0xfff {
				return false
			}
			seen++
		}
	}
	return seen > 0
}

func (r *memorizedIrisMovie) solidAt(m *memorizedObservation, x, y, future float64) bool {
	main := m.cameraAge + int((future+float64(m.mainPhase))/2)
	point := m.camera.points[min(main, len(m.camera.points)-1)]
	column, row := int(x)+point.x-16, int(y)+point.y
	if column < 0 || row < 0 {
		return false
	}
	// The player probe also precedes that pass's map hook. Newly copied
	// words first participate in terrain contacts on the following pass.
	word, _ := r.word(column/16, row/16, max(0, main-1))
	return word&0x8000 != 0
}
