package pilot

import (
	"math"

	"github.com/olivierh59500/r-type/internal/sim"
)

type baseBodyPose struct{ x, y int }

type rehearsedBaseBody struct {
	sprite string
	box    sim.Rect
	poses  []baseBodyPose
}

// rehearsedBaseBodies records the three familiar body routes, including their
// entry from the left. It contains no live damage, immunity or random state.
type rehearsedBaseBodies struct {
	first      int
	lanes      []rehearsedBaseBody
	turrets    []baseTurret
	lanceZones []sim.Rect
}

type baseTurret struct {
	parent, x, y int
	sprite       string
}

func rehearseBaseBodies(stage sim.StageSpec, camera *rehearsedCamera, reactionMain int) *rehearsedBaseBodies {
	r := &rehearsedBaseBodies{}
	if stage.Boss == nil || stage.Boss.Fourth == nil || camera == nil {
		return r
	}
	for main, point := range camera.points {
		if point.x >= stage.Boss.FreezeScroll {
			// The map reaches the threshold after this main's spawn check.
			// The actual constructor runs at the start of the next main pass.
			r.first = main + 1
			break
		}
	}
	d := stage.Boss.Fourth
	if r.first == 0 || len(d.Bodies) != 3 || len(stage.Boss.FourthArt) != 3 {
		return r
	}
	if len(d.TurretOffsets) >= 6 {
		for turret, parent := range []int{0, 0, 1} {
			r.turrets = append(r.turrets, baseTurret{parent, d.TurretOffsets[turret*2], d.TurretOffsets[turret*2+1], stage.Boss.FourthTurretArt.Sprite})
		}
	}
	poses := make([]baseBodyPose, 3)
	cursors := make([]int, 3)
	shotDX, shotDY := 0, 0
	for _, vx := range d.ShotVX {
		shotDX = max(shotDX, vx, -vx)
	}
	for _, vy := range d.ShotVY {
		shotDY = max(shotDY, vy, -vy)
	}
	for body, record := range d.Bodies {
		art := stage.Boss.FourthArt[body]
		if len(record.MotionX) == 0 || len(record.MotionX) != len(record.MotionY) || len(art.CollisionFrames) == 0 {
			return &rehearsedBaseBodies{}
		}
		cf := art.CollisionFrames[0]
		left, top, right, bottom := cf.X, cf.Y, cf.X+cf.Width, cf.Y+cf.Height
		if body < len(d.WeakpointCollision) {
			// The separate contact actor shares the visible parent's anchor.
			// Its authored envelope can extend outside the parent's body box.
			weak := d.WeakpointCollision[body]
			left, top = min(left, weak.X), min(top, weak.Y)
			right, bottom = max(right, weak.X+weak.Width), max(bottom, weak.Y+weak.Height)
		}
		for _, turret := range r.turrets {
			if turret.parent != body {
				continue
			}
			for _, cf := range stage.Boss.FourthTurretArt.CollisionFrames {
				left, top = min(left, turret.x+cf.X), min(top, turret.y+cf.Y)
				right, bottom = max(right, turret.x+cf.X+cf.Width), max(bottom, turret.y+cf.Y+cf.Height)
			}
			// Keep room for a newly emitted common shot before its image can
			// traverse the delayed tactical view. All authored muzzle offsets
			// remain possible; no live firing countdown or RNG is consulted.
			bodyDX, bodyDY := d.IntroVX, 0
			for i := range record.MotionX {
				next := (i + 1) % len(record.MotionX)
				dx, dy := record.MotionX[next]-record.MotionX[i], record.MotionY[next]-record.MotionY[i]
				bodyDX, bodyDY = max(bodyDX, dx, -dx), max(bodyDY, dy, -dy)
			}
			padX, padY := (shotDX+bodyDX)*reactionMain, (shotDY+bodyDY)*reactionMain
			for muzzle, mx := range d.TurretShotX {
				my := d.TurretShotY[muzzle]
				left, top = min(left, turret.x+mx+5-padX), min(top, turret.y+my+4-padY)
				right, bottom = max(right, turret.x+mx+11+padX), max(bottom, turret.y+my+10+padY)
			}
		}
		poses[body] = baseBodyPose{record.X, record.Y}
		r.lanes = append(r.lanes, rehearsedBaseBody{sprite: art.Sprite, box: sim.Rect{X: float64(left) - 2, Y: float64(top) - 2, W: float64(right - left + 5), H: float64(bottom - top + 5)}})
	}
	intro := true
	for main := r.first; main < max(len(camera.points), 40000); main++ {
		// Authored priorities visit body one before zero and two. A body
		// crossing its entry threshold changes the shared mode immediately.
		for _, body := range []int{1, 0, 2} {
			record, p := d.Bodies[body], &poses[body]
			if intro {
				p.x += d.IntroVX
				if p.x >= record.MotionX[0]+d.MotionOffsetX {
					intro = false
					p.x -= d.IntroVX
				}
			} else {
				cursor := cursors[body]
				p.x, p.y = record.MotionX[cursor]+d.MotionOffsetX, record.MotionY[cursor]
				cursors[body] = (cursor + 1) % len(record.MotionX)
			}
		}
		for body, p := range poses {
			r.lanes[body].poses = append(r.lanes[body].poses, p)
		}
	}
	if len(stage.Boss.FourthProjectileArt.CollisionFrames) > 0 {
		cf := stage.Boss.FourthProjectileArt.CollisionFrames[0]
		for age := range r.lanes[2].poses {
			var zone sim.Rect
			for born := max(0, age-reactionMain); born <= age; born++ {
				origin := baseBodyPose{d.Bodies[2].X, d.Bodies[2].Y}
				if born > 0 {
					origin = r.lanes[2].poses[born-1]
				}
				// The original third body emits before moving: X+60,
				// Y+20+(RNG mod 40). Cover either first-visit phase and every
				// possible recent emission through the human reaction window.
				box := sim.Rect{X: float64(origin.x+60+(age-born)*d.ProjectileVX-sim.GameOriginX+cf.X) - 2, Y: float64(origin.y+20-sim.GameOriginY+cf.Y) - 2, W: float64(cf.Width + 1 + d.ProjectileVX + 4), H: float64(cf.Height + 1 + 39 + 4)}
				if born > max(0, age-reactionMain) {
					right, bottom := max(zone.X+zone.W, box.X+box.W), max(zone.Y+zone.H, box.Y+box.H)
					box.X, box.Y = min(zone.X, box.X), min(zone.Y, box.Y)
					box.W, box.H = right-box.X, bottom-box.Y
				}
				zone = box
			}
			r.lanceZones = append(r.lanceZones, zone)
		}
	}
	return r
}

func (r *rehearsedBaseBodies) recognize(a Actor, main int) (int, bool) {
	if r == nil {
		return 0, false
	}
	age := main - r.first
	for index, lane := range r.lanes {
		if lane.sprite != a.Appearance || age < 0 || age >= len(lane.poses) {
			continue
		}
		p := lane.poses[age]
		if math.Abs(a.Bounds.X-float64(p.x-sim.GameOriginX)) <= 1 && math.Abs(a.Bounds.Y-float64(p.y-sim.GameOriginY)) <= 1 {
			return index, true
		}
	}
	for _, turret := range r.turrets {
		lane := &r.lanes[turret.parent]
		if a.Appearance != turret.sprite || age < 0 || age >= len(lane.poses) {
			continue
		}
		p := lane.poses[age]
		if math.Abs(a.Bounds.X-float64(p.x+turret.x-sim.GameOriginX)) <= 1 && math.Abs(a.Bounds.Y-float64(p.y+turret.y-sim.GameOriginY)) <= 1 {
			return turret.parent, true
		}
	}
	return 0, false
}

func (m *memorizedObservation) rememberBaseBodyLane(index int) {
	for _, existing := range m.baseBodyLanes {
		if index == existing {
			return
		}
	}
	m.baseBodyLanes = append(m.baseBodyLanes, index)
}

func (m *memorizedObservation) baseBodyEnvelope(index int, future float64) (sim.Rect, bool) {
	lane := &m.baseBodies.lanes[index]
	age := m.cameraAge + int((future+float64(m.mainPhase))/2) - m.baseBodies.first
	if age < 0 || age >= len(lane.poses) {
		return sim.Rect{}, false
	}
	p := lane.poses[age]
	box := lane.box
	box.X += float64(p.x - sim.GameOriginX)
	box.Y += float64(p.y - sim.GameOriginY)
	return box, true
}

func (m *memorizedObservation) baseLanceZone(future float64) (sim.Rect, bool) {
	if m.baseBodies == nil {
		return sim.Rect{}, false
	}
	age := m.cameraAge + int((future+float64(m.mainPhase))/2) - m.baseBodies.first
	if age < 0 || age >= len(m.baseBodies.lanceZones) {
		return sim.Rect{}, false
	}
	return m.baseBodies.lanceZones[age], true
}

// baseLastBodyFiringRow follows the familiar weak-point offset of the sole
// recognized remaining body. Only its visible parent selects the target;
// hidden weak actors, live damage and HP are never inspected.
func (m *memorizedObservation) baseLastBodyFiringRow(o Observation, tick, epoch int) (float64, bool) {
	if m.baseBodies == nil || len(m.baseBodyLanes) != 1 || m.stage.Boss == nil || m.stage.Boss.Fourth == nil || m.stage.Weapons == nil || len(m.stage.Weapons.Beams) == 0 {
		return 0, false
	}
	body := m.baseBodyLanes[0]
	d := m.stage.Boss.Fourth
	if body >= len(d.WeakpointCollision) {
		return 0, false
	}
	lane := &m.baseBodies.lanes[body]
	weak, beam := d.WeakpointCollision[body], m.stage.Weapons.Beams[0]
	releasePAL := (52 - (tick-epoch)%56 + 56) % 56
	age := m.cameraAge + (tick-o.Tick+releasePAL+m.mainPhase)/2 - m.baseBodies.first
	if age < 0 || age >= len(lane.poses) {
		return 0, false
	}
	// Lead the next scheduled full cannon, including its travel to the known
	// weak-point column. The projection uses coordinates and the authored
	// fourteen-pixel cannon speed rather than an alternate game simulation.
	muzzleRight := int(o.X) + 18 + beam.X + beam.Collision.X + beam.Collision.Width
	impact := age
	for attempt := 0; attempt < 2; attempt++ {
		target := lane.poses[impact].x + weak.X
		flight := max(0, (target-muzzleRight+13)/14)
		impact = min(age+flight, len(lane.poses)-1)
	}
	p := lane.poses[impact]
	// Aim through the upper weak-point edge: the invulnerable parent below
	// consumes a full cannon if its lower collision edge overlaps the body.
	y := float64(p.y+weak.Y-25-beam.Y-beam.Collision.Y) - float64(beam.Collision.Height)/2
	return y, true
}
