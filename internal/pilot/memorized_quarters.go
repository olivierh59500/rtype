package pilot

import (
	"github.com/olivierh59500/r-type/internal/sim"
	"github.com/olivierh59500/r-type/internal/source"
)

type quarterPose struct {
	x, y, direction, wait, pause, turnFrame int
}

type quarterTrack struct {
	main  int
	poses []quarterPose
}

// quarterMemory infers possible turning clocks from consecutive visible poses
// and sprite frames. It never reads the live actor's direction or timers.
type quarterMemory struct{ tracks map[uint64]quarterTrack }

type quarterPrediction struct {
	boxes  []sim.Rect
	active []bool
}

// advanceQuarterPose projects familiar geometric rules toward a supplied ship
// target. It neither steps a World nor changes terrain, damage or random state.
func advanceQuarterPose(d *source.QuarterTurnerData, p quarterPose, targetX, targetY, scroll int) (quarterPose, bool) {
	tx, ty := uint16(targetX), uint16(targetY)
	p.x += d.BaselineVX
	if p.pause > 0 {
		p.pause--
		return p, true
	}
	if uint16(scroll) >= uint16(d.FreezeTurningScroll) {
		p.wait = 32767
	}
	if p.wait > 0 {
		p.wait--
	}
	gate := false
	switch p.direction {
	case 0:
		gate = ty >= uint16(p.y)
	case 1:
		gate = tx <= uint16(p.x)
	case 2:
		gate = ty <= uint16(p.y)
	case 3:
		gate = tx >= uint16(p.x)
	}
	if p.wait == 0 && gate {
		old := p.direction
		if old == 0 || old == 2 {
			p.direction = 1
			if tx < uint16(p.x) {
				p.direction = 3
			}
		} else {
			p.direction = 2
			if ty < uint16(p.y) {
				p.direction = 0
			}
		}
		p.wait, p.pause = d.TurnPeriod, d.PauseTicks
		frames := d.TurnFrame7
		if old == 0 || old == 3 {
			frames = d.TurnFrame5
		}
		p.turnFrame = frames[p.direction]
	} else {
		p.x += d.DirectionVX[p.direction]
		p.y += d.DirectionVY[p.direction]
	}
	return p, p.x >= 0 && p.x <= d.MaxX && p.y >= 0 && p.y < d.MaxY
}

func quarterFrameMatches(d *source.QuarterTurnerData, p quarterPose, frame int) bool {
	if p.pause > 0 {
		return frame == p.turnFrame
	}
	for _, candidate := range d.Animations[p.direction] {
		if candidate == frame {
			return true
		}
	}
	return false
}

func seedQuarterPoses(d *source.QuarterTurnerData, x, y, frame int) []quarterPose {
	var poses []quarterPose
	for direction := 0; direction < 4; direction++ {
		p := quarterPose{x: x, y: y, direction: direction}
		if quarterFrameMatches(d, p, frame) {
			for wait := 0; wait <= d.TurnPeriod; wait++ {
				p.wait = wait
				poses = append(poses, p)
			}
		}
		if frame != d.TurnFrame5[direction] && frame != d.TurnFrame7[direction] {
			continue
		}
		p.wait, p.turnFrame = d.TurnPeriod, frame
		for pause := 1; pause <= d.PauseTicks; pause++ {
			p.pause = pause
			poses = append(poses, p)
		}
	}
	return poses
}

func (r *quarterMemory) observe(a Actor, wave sim.Wave, main, scroll, targetX, targetY, horizon int) (quarterPrediction, bool) {
	d := wave.QuarterTurner
	if d == nil || a.Appearance != wave.Sprite || len(wave.CollisionFrames) == 0 {
		return quarterPrediction{}, false
	}
	if r.tracks == nil {
		r.tracks = make(map[uint64]quarterTrack)
	}
	x, y := int(a.Bounds.X)+sim.GameOriginX, int(a.Bounds.Y)+sim.GameOriginY
	old, known := r.tracks[a.Track]
	var poses []quarterPose
	seen := make(map[quarterPose]bool)
	if known && main >= old.main && main <= old.main+1 {
		for _, p := range old.poses {
			alive := true
			if main > old.main {
				p, alive = advanceQuarterPose(d, p, targetX, targetY, scroll)
			}
			if p.pause == 0 {
				p.turnFrame = 0
			}
			if alive && p.x == x && p.y == y && quarterFrameMatches(d, p, a.Frame) && !seen[p] {
				seen[p] = true
				poses = append(poses, p)
			}
		}
	}
	if len(poses) == 0 {
		poses = seedQuarterPoses(d, x, y, a.Frame)
	}
	if len(poses) == 0 {
		return quarterPrediction{}, false
	}
	r.tracks[a.Track] = quarterTrack{main, poses}
	prediction := quarterPrediction{boxes: make([]sim.Rect, horizon+1), active: make([]bool, horizon+1)}
	for _, initial := range poses {
		p, alive := initial, true
		for future := 0; future <= horizon && alive; future++ {
			frames := d.Animations[p.direction]
			if future == 0 {
				frames = []int{a.Frame}
			} else if p.pause > 0 {
				frames = []int{p.turnFrame}
			}
			for _, frame := range frames {
				if frame < 0 || frame >= len(wave.CollisionFrames) {
					continue
				}
				cf := wave.CollisionFrames[frame]
				box := sim.Rect{X: float64(p.x-sim.GameOriginX+cf.X) - 3, Y: float64(p.y-sim.GameOriginY+cf.Y) - 3, W: float64(cf.Width + 7), H: float64(cf.Height + 7)}
				if prediction.active[future] {
					previous := prediction.boxes[future]
					right, bottom := max(previous.X+previous.W, box.X+box.W), max(previous.Y+previous.H, box.Y+box.H)
					box.X, box.Y = min(previous.X, box.X), min(previous.Y, box.Y)
					box.W, box.H = right-box.X, bottom-box.Y
				}
				prediction.boxes[future], prediction.active[future] = box, true
			}
			p, alive = advanceQuarterPose(d, p, targetX, targetY, scroll)
		}
	}
	return prediction, true
}

func (p *Pilot) recognizeQuarterTurner(o *Observation, a Actor) bool {
	m := o.memory
	if m.stage.Boss == nil || m.stage.Boss.Kind != "stage4_base" {
		return false
	}
	if p.quarterMemory == nil {
		p.quarterMemory = &quarterMemory{}
	}
	for _, wave := range m.stage.Waves {
		if wave.QuarterTurner == nil || wave.Sprite != a.Appearance {
			continue
		}
		d := wave.QuarterTurner
		horizon := (p.Config.HorizonPAL+p.Config.ReactionPAL+p.Config.DecisionPAL+1)/2 + 2
		// Forecasts hold this observed ship target. Future player movement can
		// change a turn choice; these are tactical hypotheses, not exact bounds
		// for every future input path. Each new delayed observation revises them.
		// Source player words use (+18,+25), independently of BOB origins.
		prediction, ok := p.quarterMemory.observe(a, wave, m.cameraAge, int(m.scroll), int(o.X)+18+d.PlayerOffsetX, int(o.Y)+25+d.PlayerOffsetY, horizon)
		if ok {
			m.quarters = append(m.quarters, prediction)
		}
		return ok
	}
	return false
}
