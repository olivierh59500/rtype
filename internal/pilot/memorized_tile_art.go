package pilot

import (
	"math"

	"github.com/olivierh59500/r-type/internal/sim"
	"github.com/olivierh59500/r-type/internal/source"
)

// irisEnvelope remembers the map-backed eye's possible contact poses. Its
// anchor follows the authored camera from its spawn, without reading the live
// hidden controller or its damage. The union avoids predicting its pose from
// information that a player cannot yet see.
func (m *memorizedObservation) irisEnvelope(future float64) (sim.Rect, bool) {
	if m.camera == nil || m.stage.Boss == nil || m.stage.Boss.Third == nil || m.stage.Boss.Third.Iris == nil {
		return sim.Rect{}, false
	}
	d := m.stage.Boss.Third.Iris
	main := m.cameraAge + int((future+float64(m.mainPhase))/2)
	if main < d.Trigger || len(d.Collision) == 0 {
		return sim.Rect{}, false
	}
	birth := m.camera.points[min(d.Trigger, len(m.camera.points)-1)]
	now := m.camera.points[min(main, len(m.camera.points)-1)]
	left, top, right, bottom := math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, box := range d.Collision {
		left = min(left, float64(box.X))
		top = min(top, float64(box.Y))
		right = max(right, float64(box.X+box.Width+1))
		bottom = max(bottom, float64(box.Y+box.Height+1))
	}
	return sim.Rect{X: float64(d.X-sim.GameOriginX+birth.x-now.x) + left, Y: float64(d.Y-sim.GameOriginY+birth.y-now.y) + top, W: right - left, H: bottom - top}, true
}

const (
	tileArtDecorative uint8 = 1
	tileArtSolid      uint8 = 2
)

// rememberBattleshipTileArt learns collision meaning from authored images and
// map patches. Ambiguous art keeps both bits and remains conservatively unsafe.
func rememberBattleshipTileArt(stage sim.StageSpec) map[uint16]uint8 {
	known := make(map[uint16]uint8)
	add := func(words []uint16) {
		for _, word := range words {
			kind := tileArtDecorative
			if word&0x8000 != 0 {
				kind = tileArtSolid
			}
			known[word&0xfff] |= kind
		}
	}
	patch := func(p *source.MapPatch) {
		if p != nil {
			add(p.Words)
		}
	}
	add(stage.TileMap)
	if stage.Boss == nil || stage.Boss.Third == nil {
		return known
	}
	d := stage.Boss.Third
	for _, p := range d.InitialPatches {
		patch(p)
	}
	for _, gun := range d.Mounted {
		patch(gun.InitialPatch)
		patch(gun.DeathPatch)
	}
	if d.Core != nil {
		for _, p := range d.Core.IdlePatches {
			patch(p)
		}
		for _, p := range d.Core.CyclePatches {
			patch(p)
		}
	}
	if d.Iris != nil {
		patch(d.Iris.InitialPatch)
		for _, p := range d.Iris.Opening {
			patch(p.Patch)
		}
		for _, p := range d.Iris.DamageSteps {
			patch(p.Patch)
		}
		for _, bank := range d.Iris.Flash {
			for _, p := range bank {
				patch(p.Patch)
			}
		}
	}
	for _, wave := range stage.Waves {
		if wave.Reactor == nil {
			continue
		}
		for _, pair := range wave.Reactor.PatchPairs {
			patch(pair.First)
			patch(pair.Second)
		}
		for _, p := range wave.Reactor.ClearPatches {
			patch(p)
		}
		for _, p := range wave.Reactor.BlinkPatches {
			patch(p)
		}
	}
	return known
}

// perceivedTileArt reads the terrain image retained by the renderer. A map
// hook's next collision state must not become early visual information.
func perceivedTileArt(w *sim.World, column, row int) uint16 {
	stage := w.Stages[w.Stage-1]
	if column < 0 || column >= stage.MapWidth || row < 0 || row >= stage.MapHeight {
		return 0
	}
	index := row*stage.MapWidth + column
	if !w.TerrainImageValid {
		return w.RawTileAt(column, row) & 0xfff
	}
	word := stage.TileMap[index]
	if overlay, ok := w.TerrainImageChanges[index]; ok {
		word = overlay
	}
	return word & 0xfff
}
