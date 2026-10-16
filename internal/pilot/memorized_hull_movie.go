package pilot

import (
	"github.com/olivierh59500/r-type/internal/sim"
	"github.com/olivierh59500/r-type/internal/source"
)

// rehearseHullMovie remembers the intact, authored central-hull sequence.
// It shares the iris movie's compact storage and visible-art validation, but
// reads no live core phase, HP, pending damage, random state or alternate world.
func rehearseHullMovie(stage sim.StageSpec) *memorizedIrisMovie {
	r := &memorizedIrisMovie{}
	if stage.Boss == nil || stage.Boss.Third == nil || stage.Boss.Third.Core == nil {
		return r
	}
	d := stage.Boss.Third.Core
	left, top, right, bottom := stage.MapWidth, stage.MapHeight, 0, 0
	patches := append(append([]*source.MapPatch(nil), d.IdlePatches...), d.CyclePatches...)
	for _, p := range patches {
		if p != nil {
			left, top = min(left, p.Column), min(top, p.Row)
			right, bottom = max(right, p.Column+p.Width), max(bottom, p.Row+p.Height)
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
	phase, timer, cursor, wait := 0, 0, 0, 0
	for main := 0; main <= r.frames; main++ {
		if main > 0 && main <= stage.Boss.Third.ExitReload {
			switch phase {
			case 0:
				if main >= d.AnimationTrigger {
					timer--
					if timer < 0 {
						apply(d.IdlePatches[cursor%len(d.IdlePatches)])
						cursor, timer = cursor+1, d.IdleReload
					}
				}
				if main >= d.Trigger {
					wait--
					if wait < 0 {
						phase, cursor, timer, wait = 1, 0, 0, d.CycleCount
					}
				}
			case 1, 3:
				timer--
				if timer < 0 {
					apply(d.CyclePatches[cursor])
					cursor, timer, wait = cursor+1, d.PatchReload, wait-1
					if wait == 0 {
						if phase == 1 {
							phase, timer = 2, d.CycleHold
						} else {
							phase, cursor, timer, wait = 0, 0, 0, d.CycleWait
						}
					}
				}
			case 2:
				timer--
				if timer < 0 {
					phase, wait = 3, d.CycleCount
				}
			}
		}
		copy(r.words[main*area:(main+1)*area], tiles)
	}
	return r
}

// forecastMapCell replaces only a currently recognized movie footprint.
// Unexpected displayed damage leaves ordinary delayed observations in charge.
func (m *memorizedObservation) forecastMapCell(cell sim.Rect) bool {
	return m.irisMovieActive && m.irisMovie.containsScreenCell(cell, m.scroll, m.scrollY) ||
		m.hullMovieActive && m.hullMovie.containsScreenCell(cell, m.scroll, m.scrollY)
}

// rehearsedCorePose follows the authored post-camera projection from spawn.
// It is a prediction model, not a read of the live hidden damage target.
func (m *memorizedObservation) rehearsedCorePose() sim.Rect {
	d := m.stage.Boss.Third.Core
	birth := m.camera.projection[d.Trigger]
	now := m.camera.projection[max(d.Trigger, m.cameraAge)]
	return sim.Rect{X: float64(d.X - sim.GameOriginX + birth.x - now.x), Y: float64(d.Y - sim.GameOriginY + birth.y - now.y), W: 31, H: 19}
}
