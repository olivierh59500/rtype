package pilot

import (
	"math"

	"github.com/olivierh59500/r-type/internal/sim"
	"github.com/olivierh59500/r-type/internal/source"
)

type crawlerPose struct{ x, y, direction, cursor int }

type rehearsedCrawlerLane struct {
	first  int
	sprite string
	box    sim.Rect
	poses  []crawlerPose
}

// rehearsedCrawlers contains authored routes through the original static map.
// A visible image must match a recorded pose before it supplies foreknowledge.
// It does not consult live route cursors, shielding, damage, HP or random state.
type rehearsedCrawlers struct{ lanes []rehearsedCrawlerLane }

// advanceCrawlerPose rehearses the two familiar terrain probes and the queued
// turns. This geometric model never constructs or advances a game world.
func advanceCrawlerPose(stage sim.StageSpec, d *source.RouteCrawlerData, p crawlerPose, dx, dy, scrollX, scrollY int) (crawlerPose, bool) {
	baseX, baseY := p.x-dx, p.y-dy
	solid := func(column, row int) bool {
		index := row*stage.MapWidth + column
		return column >= 0 && row >= 0 && column < stage.MapWidth && row < stage.MapHeight && index < len(stage.TileMap) && stage.TileMap[index]&0x8000 != 0
	}
	for tries := 0; tries < 8; tries++ {
		route := d.Routes[p.direction&3]
		x, y := baseX+route.DX, baseY+route.DY
		if x < d.MinX || x > d.MaxX || y < d.MinY || y > d.MaxY {
			return p, false
		}
		probeX, probeY := x+route.ProbeX+scrollX-48, y+route.ProbeY+scrollY-32
		blocked := false
		if probeX >= 0 && probeY >= 0 {
			column, row := probeX/16, probeY/16
			blocked = solid(column, row) || solid(column+route.NeighborX, row+route.NeighborY)
		}
		if !blocked {
			p.x, p.y = x, y
			return p, true
		}
		if p.cursor >= len(d.Directions) || d.Directions[p.cursor] < 0 {
			p.direction = (p.direction + 2) & 3
		} else {
			p.direction = d.Directions[p.cursor] & 3
			p.cursor++
		}
	}
	return p, true
}

func rehearseCrawlers(stage sim.StageSpec, camera *rehearsedCamera) *rehearsedCrawlers {
	r := &rehearsedCrawlers{}
	if stage.Boss == nil || stage.Boss.Six == nil || camera == nil {
		return r
	}
	d := stage.Boss.Six
	point := func(main int) cameraPoint {
		p := camera.points[min(main, len(camera.points)-1)]
		p.x = min(p.x, d.FreezeScroll)
		return p
	}
	add := func(sprite string, boxes []sim.CollisionFrame, route *source.RouteCrawlerData, rawX, rawY, main int, newborn bool) {
		if route == nil || len(route.Routes) != 4 || len(boxes) == 0 {
			return
		}
		left, top, right, bottom := boxes[0].X, boxes[0].Y, boxes[0].X+boxes[0].Width, boxes[0].Y+boxes[0].Height
		for _, box := range boxes[1:] {
			left, top = min(left, box.X), min(top, box.Y)
			right, bottom = max(right, box.X+box.Width), max(bottom, box.Y+box.Height)
		}
		lane := rehearsedCrawlerLane{first: main, sprite: sprite, box: sim.Rect{X: float64(left), Y: float64(top), W: float64(right - left + 1), H: float64(bottom - top + 1)}}
		pose := crawlerPose{x: rawX, y: rawY, direction: route.InitialDirection}
		for age := main; age < len(camera.points); age++ {
			if age != main || !newborn {
				a, b := point(age-1), point(age)
				var alive bool
				pose, alive = advanceCrawlerPose(stage, route, pose, b.x-a.x, b.y-a.y, b.x, b.y)
				if !alive {
					break
				}
			}
			lane.poses = append(lane.poses, pose)
		}
		r.lanes = append(r.lanes, lane)
	}
	// The familiar event list advances one authored group per main update.
	// This records spawn times from data, without reading the live event cursor.
	cursor, freezeMain := 0, 0
	for main := 1; main < len(camera.points); main++ {
		before, after := point(main-1), point(main)
		if freezeMain == 0 && after.x == d.FreezeScroll {
			freezeMain = main
		}
		group := -1
		for cursor < len(stage.Waves) {
			wave := stage.Waves[cursor]
			clock := before.x
			if wave.TriggerClock == "tick" {
				clock = main
			}
			if clock < wave.Trigger || group >= 0 && group != wave.Group {
				break
			}
			group = wave.Group
			cursor++
			add(wave.Sprite, wave.CollisionFrames, wave.RouteCrawler, int(wave.X-wave.W/2)+sim.GameOriginX, int(wave.Y-wave.H/2)+sim.GameOriginY, main, false)
		}
	}
	// Queue newborns appear after the actor pass. Their first movement occurs
	// one main update later, with the camera already stopped.
	if freezeMain > 0 {
		for index := range d.Queues {
			add(stage.Boss.SixArt.Sprite, stage.Boss.SixArt.CollisionFrames, &d.Queues[index], d.SpawnX, d.SpawnY, freezeMain+index*(d.SpawnReload+1), true)
		}
	}
	return r
}

func (r *rehearsedCrawlers) recognize(actor Actor, main int) []int {
	if r == nil {
		return nil
	}
	var matches []int
	for index := range r.lanes {
		lane := &r.lanes[index]
		age := main - lane.first
		if lane.sprite != actor.Appearance || age < 0 || age >= len(lane.poses) {
			continue
		}
		p := lane.poses[age]
		if math.Abs(float64(p.x-sim.GameOriginX)-actor.Bounds.X) <= 1 && math.Abs(float64(p.y-sim.GameOriginY)-actor.Bounds.Y) <= 1 {
			matches = append(matches, index)
		}
	}
	return matches
}

func (m *memorizedObservation) crawlerEnvelope(index int, future float64) (sim.Rect, bool) {
	lane := &m.crawlers.lanes[index]
	age := m.cameraAge + int((future+float64(m.mainPhase))/2) - lane.first
	if age < 0 || age >= len(lane.poses) {
		return sim.Rect{}, false
	}
	p := lane.poses[age]
	box := lane.box
	// Recognition admits a one-pixel image offset. Keep that uncertainty on
	// every side of the future contact envelope, including its near edges.
	box.X += float64(p.x - sim.GameOriginX - 1)
	box.Y += float64(p.y - sim.GameOriginY - 1)
	box.W += 2
	box.H += 2
	return box, true
}
