package pilot

import (
	"math"

	"github.com/olivierh59500/r-type/internal/sim"
)

// observeBattleshipShots samples displayed images, not the native hostile-pool
// records. The tactical view accepts only observations old enough to satisfy
// the configured human reaction delay and changes at the decision cadence.
func (p *Pilot) observeBattleshipShots(w *sim.World) {
	if p.battleshipShotSensor == nil {
		p.battleshipShotSensor = NewSensor()
		p.battleshipShotDecision = -p.Config.DecisionPAL
	}
	tick := int(w.Tick)
	o := p.battleshipShotSensor.Observe(w, tick)
	m := &memorizedObservation{stage: w.Stages[2], camera: p.battleshipCamera, cameraAge: w.StageTick, mainPhase: tick & 1}
	o.memory = m
	actors := o.Actors[:0]
	for _, a := range o.Actors {
		if m.stage.Weapons == nil || m.stage.Weapons.HostileShot == nil || len(m.stage.Weapons.HostileShot.Frames) == 0 || a.Image != m.stage.Weapons.HostileShot.Frames[0] {
			continue
		}
		m.recognizeHostileShot(&a)
		a.Bounds, a.KnownEnvelope = m.actorEnvelope(a)
		actors = append(actors, a)
	}
	o.Actors, o.Walls, o.MapHazards = actors, nil, nil
	p.battleshipShotQueue = append(p.battleshipShotQueue, o)
	if tick-p.battleshipShotDecision < p.Config.DecisionPAL {
		return
	}
	for len(p.battleshipShotQueue) > 0 && p.battleshipShotQueue[0].Tick <= tick-p.Config.ReactionPAL {
		p.battleshipShotView = p.battleshipShotQueue[0]
		p.battleshipShotQueue = p.battleshipShotQueue[1:]
	}
	p.battleshipShotDecision = tick
}

func (p *Pilot) avoidBattleshipShots(w *sim.World, nominal sim.Input) sim.Input {
	o := p.battleshipShotView
	if o.memory == nil || len(o.Actors) == 0 {
		return nominal
	}
	o.X, o.Y, o.Speed = w.Player.X, w.Player.Y, float64(2+w.Player.SpeedLevel)
	lag := float64(int(w.Tick) - o.Tick)
	risk := func(move sim.Input) float64 {
		x, y := o.X, o.Y
		dx, dy := 0, 0
		if move.Left {
			dx = -1
		} else if move.Right {
			dx = 1
		}
		if move.Up {
			dy = -1
		} else if move.Down {
			dy = 1
		}
		cost := 0.0
		for pal := 1; pal <= 10; pal++ {
			if pal <= 2 {
				x, y, _ = rehearsedMovement(o, x, y, action{dx: dx, dy: dy})
			}
			cost += danger(o, x, y, lag+float64(pal))
		}
		if p.battleshipRoute.at(x, y, w.StageTick+1) >= battleshipRouteBlocked {
			return math.Inf(1)
		}
		return cost
	}
	bestRisk := risk(nominal)
	if bestRisk < 5e5 {
		return nominal
	}
	best := nominal
	for _, a := range rehearsalActionOrder {
		candidate := sim.Input{Left: a.dx < 0, Right: a.dx > 0, Up: a.dy < 0, Down: a.dy > 0}
		v := risk(candidate)
		if v < bestRisk {
			best, bestRisk = candidate, v
		}
	}
	return best
}
