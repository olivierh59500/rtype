package pilot

import (
	"github.com/olivierh59500/r-type/internal/sim"
	"github.com/olivierh59500/r-type/internal/source"
)

type bossTwoPoint struct{ x, y, frame int }

type memorizedBossTwo struct {
	data           *source.BossTwoData
	art            []sim.BossArt
	paths          [][]bossTwoPoint
	cycle          int
	age, mainPhase int
}

type bossTwoMemory struct {
	stage   int
	epoch   uint64
	pattern *memorizedBossTwo
}

// rehearseBossTwo integrates authored path offsets once. Route waits and the
// known five-main-frame spacing determine each follower's pose; live actors,
// damage, HP and random state never enter this familiar attack timeline.
func rehearseBossTwo(spec *sim.BossSpec) *memorizedBossTwo {
	m := &memorizedBossTwo{data: spec.Second, art: spec.SecondArt}
	for index, route := range spec.Second.Routes {
		x, y := route.StartX, route.StartY
		var points []bossTwoPoint
		for frame, dx := range route.MotionX {
			x += int(dx)
			y += int(route.MotionY[frame])
			points = append(points, bossTwoPoint{x, y, route.Frames[frame]})
		}
		m.paths = append(m.paths, points)
		if index > 0 {
			m.cycle += len(points) + spec.Second.RouteWait + 1
		}
	}
	return m
}

func (p *Pilot) rememberBossTwo(w *sim.World, m *memorizedObservation) {
	spec := m.stage.Boss
	if spec == nil || spec.Second == nil || m.scroll < float64(spec.Second.Trigger) {
		p.bossTwo = nil
		return
	}
	if p.bossTwo == nil || p.bossTwo.stage != w.Stage {
		p.bossTwo = &bossTwoMemory{stage: w.Stage, epoch: w.Tick + 2, pattern: rehearseBossTwo(spec)}
	}
	pattern := *p.bossTwo.pattern
	pattern.age = -1
	if w.Tick >= p.bossTwo.epoch {
		pattern.age = int((w.Tick - p.bossTwo.epoch) / 2)
	}
	pattern.mainPhase = int(w.Tick & 1)
	m.snake = &pattern
}

func (m *memorizedObservation) isMemorizedSnake(actor Actor) bool {
	if m.snake != nil {
		for _, art := range m.snake.art {
			if actor.Appearance == art.Sprite {
				return true
			}
		}
	}
	return false
}

func (m memorizedBossTwo) snakeEnvelope(component int, future float64) (sim.Rect, bool) {
	age := m.age + int((future+float64(m.mainPhase))/2) - component*m.data.DelayStep - 1
	if age < 0 || len(m.paths) == 0 || m.cycle <= 0 {
		return sim.Rect{}, false
	}
	route := 0
	first := len(m.paths[0]) + m.data.RouteWait + 1
	if age >= first {
		age = (age - first) % m.cycle
		route = 1
		for age >= len(m.paths[route])+m.data.RouteWait+1 {
			age -= len(m.paths[route]) + m.data.RouteWait + 1
			route++
		}
	}
	// The terminal motion sample moves the original actor to raw X=-1;
	// contact remains disabled throughout its eighty-six-frame route wait.
	if age >= len(m.paths[route])-1 {
		return sim.Rect{}, false
	}
	point := m.paths[route][age]
	if point.x < 0 || point.x >= 350 {
		return sim.Rect{}, false
	}
	frame := 0
	definition := m.data.Components[component]
	if !definition.Animated && len(definition.Animation) > point.frame {
		frame = definition.Animation[point.frame]
	}
	if component >= len(m.art) || frame >= len(m.art[component].CollisionFrames) {
		return sim.Rect{}, false
	}
	box := m.art[component].CollisionFrames[frame]
	return sim.Rect{X: float64(point.x-sim.GameOriginX+box.X) - 1, Y: float64(point.y-sim.GameOriginY+box.Y) - 1, W: float64(box.Width + 3), H: float64(box.Height + 3)}, true
}
