package pilot

import "github.com/olivierh59500/r-type/internal/sim"

type cameraPoint struct{ x, y int }

// rehearsedCamera is an authored movement timeline, not an alternative world.
// Axis deadlines keep their phase when a command changes; the new period takes
// effect only when that axis next moves. No live camera cursor is inspected.
type rehearsedCamera struct {
	points     []cameraPoint
	projection []cameraPoint
}

func rehearseCamera(stage sim.StageSpec, updates int) *rehearsedCamera {
	r := &rehearsedCamera{points: []cameraPoint{{0, int(stage.InitialScrollY)}}, projection: []cameraPoint{{}}}
	if len(stage.ScrollScript) == 0 {
		return r
	}
	x, y := 0, int(stage.InitialScrollY)
	projectX, projectY := 0, 0
	xNext, yNext, commandNext, cursor := 1, 1, 1, 0
	xPeriod, yPeriod, dx, dy := 1, 1, 0, 0
	for main := 1; main <= updates; main++ {
		if main == xNext {
			x += dx
			xNext = main + xPeriod
		}
		if main == yNext {
			y += dy
			yNext = main + yPeriod
		}
		if main == commandNext {
			step := stage.ScrollScript[cursor]
			dx, dy = step.XDelta, step.YDelta
			xPeriod, yPeriod = step.XPeriod, step.YPeriod
			commandNext = main + step.Duration + 1
			if !step.Repeat && cursor+1 < len(stage.ScrollScript) {
				cursor++
			}
		}
		r.points = append(r.points, cameraPoint{x, y})
		if main == xNext-1 {
			projectX += dx
		}
		if main == yNext-1 {
			projectY += dy
		}
		r.projection = append(r.projection, cameraPoint{projectX, projectY})
	}
	return r
}

func (m *memorizedObservation) cameraDisplacement(future float64) (float64, float64, bool) {
	if m.camera == nil || len(m.camera.points) == 0 {
		return 0, 0, false
	}
	from := min(m.cameraAge, len(m.camera.points)-1)
	to := min(from+int((future+float64(m.mainPhase))/2), len(m.camera.points)-1)
	a, b := m.camera.points[from], m.camera.points[to]
	if m.stage.Boss != nil && (m.stage.Boss.Kind == "stage1_dobkeratops" || m.stage.Boss.Kind == "stage4_base" || m.stage.Boss.Kind == "stage6_crawler_queue" || m.stage.Boss.Kind == "stage7_resource_arena" || m.stage.Boss.Kind == "stage8_bydo") {
		freeze := m.stage.Boss.FreezeScroll
		if m.stage.Boss.Six != nil {
			freeze = m.stage.Boss.Six.FreezeScroll
		}
		if m.stage.Boss.Seven != nil {
			freeze = m.stage.Boss.Seven.FreezeScroll
		}
		if m.stage.Boss.Eighth != nil {
			freeze = m.stage.Boss.Eighth.FreezeScroll
		}
		return max(0, min(float64(b.x-a.x), float64(freeze)-m.scroll)), 0, true
	}
	return float64(b.x - a.x), float64(b.y - a.y), true
}
