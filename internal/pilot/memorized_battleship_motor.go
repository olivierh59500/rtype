package pilot

import "github.com/olivierh59500/r-type/internal/sim"

const battleshipRehearsalEntry = 2550
const battleshipPrecharge = 2680

// rehearsedBattleshipInput executes a prepared forty-millisecond motor route.
// Own-ship feedback follows the prepared route. Visible common shots retain
// delayed perception and eighty-millisecond tactical sampling; private shot
// records, enemy HP, phase, RNG and alternate worlds are never inspected.
func (p *Pilot) rehearsedBattleshipInput(w *sim.World) (sim.Input, bool) {
	stage := w.Stages[2]
	if p.battleshipRoute == nil {
		if p.battleshipCamera == nil {
			p.battleshipCamera = rehearseCamera(stage, 16000)
		}
		if p.battleshipIrisMovie == nil {
			p.battleshipIrisMovie = rehearseIrisMovie(stage)
		}
		if p.battleshipHullMovie == nil {
			p.battleshipHullMovie = rehearseHullMovie(stage)
		}
		p.battleshipRoute = rehearseBattleshipRoute(stage, p.battleshipCamera, p.battleshipIrisMovie, p.battleshipHullMovie, 3)
	}
	r := p.battleshipRoute
	p.observeBattleshipShots(w)
	// The rehearsed three-pixel motor lattice must match own-ship feedback.
	// Other pickup/checkpoint configurations return to the visible controller.
	if (int(w.Player.X)-r.minX)%r.speed != 0 || (int(w.Player.Y)-r.minY)%r.speed != 0 {
		return sim.Input{}, false
	}
	if w.Tick&1 == 0 {
		x, y, ok := r.waypoint(w.Player.X, w.Player.Y, w.StageTick, 1)
		if !ok && w.StageTick < r.end {
			return sim.Input{}, false
		}
		if w.StageTick >= r.end {
			m := memorizedObservation{stage: stage, camera: p.battleshipCamera, cameraAge: w.StageTick}
			core := m.rehearsedCorePose()
			x, y = 37, core.Y+9
		}
		p.battleshipMotor = sim.Input{Left: x < w.Player.X, Right: x > w.Player.X, Up: y < w.Player.Y, Down: y > w.Player.Y}
		p.battleshipMotor = p.avoidBattleshipShots(w, p.battleshipMotor)
	}
	input := p.battleshipMotor
	input.Fire = w.StageTick%4 < 2
	if w.StageTick >= battleshipPrecharge {
		input.Fire = true
		m := memorizedObservation{stage: stage, camera: p.battleshipCamera, cameraAge: w.StageTick}
		core := m.rehearsedCorePose()
		if w.Player.Charge >= sim.MaxCharge && w.Player.X+20 < core.X && w.Player.Y > core.Y && w.Player.Y < core.Y+18 {
			input.Fire = false
		}
	}
	return input, true
}
