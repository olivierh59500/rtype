package pilot

import (
	"math"

	"github.com/olivierh59500/r-type/internal/sim"
)

// memorizedObservation contains authored knowledge explicitly permitted for
// the experienced-player mode. It contains no live enemy HP, random-generator
// state, hidden actor positions, or simulated alternative futures.
type memorizedObservation struct {
	stage                sim.StageSpec
	scroll, scrollY      float64
	mainTick             int
	organArtFrame        int
	tailUnsafe           []bool
	spirals              []memorizedSpiral
	snake                *memorizedBossTwo
	camera               *rehearsedCamera
	cameraAge, mainPhase int
	irisMovie            *memorizedIrisMovie
	irisMovieActive      bool
	hullMovie            *memorizedIrisMovie
	hullMovieActive      bool
	crawlers             *rehearsedCrawlers
	crawlerLanes         []int
	quarters             []quarterPrediction
	baseBodies           *rehearsedBaseBodies
	baseBodyLanes        []int
}

type memorizedSpiral struct {
	wave      sim.Wave
	age       int
	mainPhase int
}

func (p *Pilot) rememberAuthoredLevel(w *sim.World, o *Observation) {
	if w.Stage < 1 || w.Stage > len(w.Stages) {
		return
	}
	stage := w.Stages[w.Stage-1]
	m := &memorizedObservation{stage: stage, scroll: w.Scroll, scrollY: w.ScrollY, mainTick: w.EventTick}
	o.memory = m
	if stage.Boss != nil && stage.Boss.Kind == "stage1_dobkeratops" {
		if p.firstCamera == nil {
			p.firstCamera = rehearseCamera(stage, 16000)
		}
		m.camera, m.cameraAge, m.mainPhase = p.firstCamera, w.StageTick, int(w.Tick&1)
	}
	if stage.Boss != nil && stage.Boss.Kind == "stage8_bydo" {
		if p.finalCamera == nil {
			p.finalCamera = rehearseCamera(stage, 16000)
		}
		m.camera, m.cameraAge, m.mainPhase = p.finalCamera, w.StageTick, int(w.Tick&1)
	}
	if stage.Boss != nil && stage.Boss.Kind == "stage7_resource_arena" {
		if p.resourceCamera == nil {
			p.resourceCamera = rehearseCamera(stage, 16000)
		}
		m.camera, m.cameraAge, m.mainPhase = p.resourceCamera, w.StageTick, int(w.Tick&1)
	}
	if stage.Boss != nil && stage.Boss.Kind == "stage6_crawler_queue" {
		if p.crawlerCamera == nil {
			p.crawlerCamera = rehearseCamera(stage, 16000)
		}
		m.camera, m.cameraAge, m.mainPhase = p.crawlerCamera, w.StageTick, int(w.Tick&1)
		if p.crawlerRoutes == nil {
			p.crawlerRoutes = rehearseCrawlers(stage, p.crawlerCamera)
		}
		m.crawlers = p.crawlerRoutes
	}
	if stage.Boss != nil && stage.Boss.Kind == "stage4_base" {
		if p.baseCamera == nil {
			p.baseCamera = rehearseCamera(stage, 16000)
		}
		m.camera, m.cameraAge, m.mainPhase = p.baseCamera, w.StageTick, int(w.Tick&1)
		if p.baseBodies == nil {
			p.baseBodies = rehearseBaseBodies(stage, p.baseCamera, (p.Config.ReactionPAL+p.Config.DecisionPAL+1)/2+1)
		}
		m.baseBodies = p.baseBodies
		if m.cameraAge < m.baseBodies.first {
			// A practiced player leaves the authored entry lane before the
			// three bodies appear. No live boss-spawn flag is consulted.
			for index := range m.baseBodies.lanes {
				m.baseBodyLanes = append(m.baseBodyLanes, index)
			}
		}
	}
	if stage.Boss != nil && stage.Boss.Kind == "stage3_battleship" {
		if p.battleshipCamera == nil {
			p.battleshipCamera = rehearseCamera(stage, 16000)
		}
		m.camera, m.cameraAge, m.mainPhase = p.battleshipCamera, w.StageTick, int(w.Tick&1)
		if p.battleshipTileArt == nil {
			p.battleshipTileArt = rememberBattleshipTileArt(stage)
		}
		if p.battleshipIrisMovie == nil {
			p.battleshipIrisMovie = rehearseIrisMovie(stage)
		}
		m.irisMovie = p.battleshipIrisMovie
		m.irisMovieActive = m.irisMovie.matchesVisible(w, m.cameraAge)
		if p.battleshipHullMovie == nil {
			p.battleshipHullMovie = rehearseHullMovie(stage)
		}
		m.hullMovie = p.battleshipHullMovie
		m.hullMovieActive = m.hullMovie.matchesVisible(w, m.cameraAge)
		// A practiced player recognizes decorative cells in the familiar
		// iris animation. Only rendered art is consulted here, never a live
		// collision flag or hidden controller/damage state.
		hazards := o.MapHazards[:0]
		for _, hazard := range o.MapHazards {
			column := int(hazard.X+w.Scroll-16) / 16
			row := int(hazard.Y+w.ScrollY) / 16
			art := perceivedTileArt(w, column, row)
			if p.battleshipTileArt[art] != tileArtDecorative {
				hazards = append(hazards, hazard)
			}
		}
		o.MapHazards = hazards
	}
	p.rememberBossTwo(w, m)
	for _, wave := range stage.Waves {
		if wave.SpiralSentinel == nil {
			continue
		}
		age := int(m.scroll) - wave.Trigger - 1
		if age >= -40 && age <= wave.SpiralSentinel.BaseX-wave.SpiralSentinel.StopX+1 {
			m.spirals = append(m.spirals, memorizedSpiral{wave: wave, age: age, mainPhase: int(w.Tick & 1)})
		}
	}
	if boss := stage.Boss; boss != nil && boss.Kind == "stage1_dobkeratops" && m.scroll >= float64(boss.FreezeScroll) {
		if p.tailMasks == nil {
			p.tailMasks = make(map[int][]bool)
		}
		if p.tailMasks[w.Stage] == nil {
			p.tailMasks[w.Stage] = memorizedTailMask(boss)
		}
		m.tailUnsafe = p.tailMasks[w.Stage]
	}
	actors := o.Actors[:0]
	for _, actor := range o.Actors {
		if boss := m.stage.Boss; boss != nil && boss.Fourth != nil && actor.Appearance == boss.FourthProjectileArt.Sprite {
			// Recognize the familiar fast horizontal lance on its first
			// displayed frame, before optical flow has converged to its speed.
			actor.Projectile = true
			actor.VX, actor.VY = float64(boss.Fourth.ProjectileVX)/2, 0
		}
		if index, ok := m.baseBodies.recognize(actor, m.cameraAge); ok {
			m.rememberBaseBodyLane(index)
			continue
		}
		if p.recognizeQuarterTurner(o, actor) {
			continue
		}
		if matches := m.crawlers.recognize(actor, m.cameraAge); len(matches) > 0 {
			m.crawlerLanes = append(m.crawlerLanes, matches...)
			continue
		}
		if m.isMemorizedTail(actor) || m.isMemorizedSpiral(actor) || m.isMemorizedSnake(actor) {
			continue
		}
		if m.camera != nil && m.recognizeHostileShot(&actor) {
			continue
		}
		actor.Bounds, actor.KnownEnvelope = m.actorEnvelope(actor)
		actors = append(actors, actor)
	}
	o.Actors = actors
	for _, wave := range stage.Waves {
		if wave.HuntingEmitter == nil {
			continue
		}
		for index := range o.Actors {
			actor := &o.Actors[index]
			if actor.Appearance == wave.HuntingEmitter.ProjectileSprite {
				// An experienced player recognizes this fast, indestructible
				// horizontal attack and its authored speed as soon as it appears.
				actor.Projectile = true
				actor.VX, actor.VY = float64(wave.HuntingEmitter.ProjectileVX)/2, 0
			}
		}
	}
	if boss := stage.Boss; boss != nil && boss.Second != nil {
		d := boss.Second
		art := w.RawTileAt(d.PatchColumn, d.PatchRow) & 0xfff
		for frame, words := range d.Patches {
			if len(words) > 0 && words[0]&0xfff == art {
				m.organArtFrame = frame
				break
			}
		}
	}
	// Future static terrain is familiar from previous runs. Animated map changes
	// remain observed dynamically; they are not projected from hidden state.
	first := max(0, int(w.Scroll)/16+21)
	last := min(stage.MapWidth, first+4)
	if m.camera != nil {
		first = max(0, int(w.Scroll)/16)
	}
	for row := 0; row < stage.MapHeight; row++ {
		for column := first; column < last; column++ {
			index := row*stage.MapWidth + column
			if index < len(stage.TileMap) && stage.TileMap[index]&0x8000 != 0 {
				wall := sim.Rect{X: float64(column*16) + 16 - w.Scroll, Y: float64(row*16) - w.ScrollY, W: 16, H: 16}
				if m.camera == nil || !visible(wall) {
					o.Walls = append(o.Walls, wall)
				}
			}
		}
	}
}

func (p *Pilot) selectMemorizedTarget(o Observation, tick int) {
	m := o.memory
	if m == nil {
		return
	}
	if boss := m.stage.Boss; boss != nil {
		switch boss.Kind {
		case "stage8_bydo":
			if boss.Eighth != nil && m.scroll >= float64(boss.Eighth.BrainScroll-140) {
				// The familiar upper firing lane reaches the brain while
				// clearing a spore's initial horizontal row. Dynamic attacks
				// still pass through the delayed visible-threat planner.
				p.targetX, p.targetY, p.charge = 85, float64(boss.Eighth.BrainY-sim.GameOriginY+12), true
				p.targetUntil = tick + p.Config.DecisionPAL
				return
			}
		case "stage4_base":
			if m.scroll >= 2000 {
				// Prepare the known final approach instead of chasing late
				// upper-edge formations into the authored ceiling.
				p.targetX, p.targetY, p.charge = 60, 95, true
				if p.baseStation != 0 {
					p.targetY = 75
				}
				epoch := p.fireEpoch
				if p.baseStation != 0 {
					epoch = p.baseStationAt
				}
				if y, known := m.baseLastBodyFiringRow(o, tick, epoch); known {
					p.targetY = y
				}
				p.targetUntil = tick + p.Config.DecisionPAL
				return
			}
		case "stage1_dobkeratops":
			if m.scroll >= float64(boss.ControllerTrigger-80) {
				// Full cannon at Y86..88 reaches the original weak point
				// while the ship remains above straight boss shots. X85 stays
				// outside every authored tail pose at the frozen arena position.
				p.targetX, p.targetY, p.charge = 85, 87, true
				p.targetUntil = tick + p.Config.DecisionPAL
				return
			}
		case "stage2_gomander":
			if boss.Second != nil && m.scroll >= float64(boss.FreezeScroll-140) {
				// The lip at X102..134 blocks a horizontal firing approach.
				// Enter from above, then lower the ship inside the known opening.
				p.targetX, p.targetY, p.charge = 134, 107, true
				openingLeft := float64(boss.Second.PatchColumn*16+16) - m.scroll
				if o.X < 130 && o.Y > 90 {
					p.targetX, p.targetY = o.X, 80
				} else if o.X+4 < openingLeft+2 {
					p.targetY = 80
				}
				p.targetUntil = tick + p.Config.DecisionPAL
				return
			}
		}
	}
	for _, spiral := range m.spirals {
		if spiral.age < 140 {
			continue
		}
		// Enter the known central opening while it is still forward, follow
		// its moving center, then leave through the later right-side gap.
		// Waiting at the left edge would let the full ring trap the ship.
		p.targetX, p.targetY, p.charge = 235, 97, false
		if spiral.age < 440 {
			p.targetX = math.Min(285, float64(spiral.wave.SpiralSentinel.BaseX-spiral.age-16))
		} else if spiral.age >= 560 {
			p.targetX = 185
		}
		p.targetUntil = tick + p.Config.DecisionPAL
		return
	}
	for _, wave := range m.stage.Waves {
		if wave.HuntingEmitter == nil {
			continue
		}
		clock := m.scroll
		if wave.TriggerClock == "tick" {
			clock = float64(m.mainTick)
		}
		until := float64(wave.Trigger) - clock
		if until > 40 || until < -260 {
			continue
		}
		// This thirty-hit hunter is memorized as a cannon target. Charge before
		// it arrives instead of feeding it normal shots and chasing its attack.
		p.targetX, p.targetY, p.charge = 85, wave.Y, true
		for _, actor := range o.Actors {
			if actor.Appearance == wave.Sprite {
				p.targetY = actor.Bounds.Y + actor.Bounds.H/2
				break
			}
		}
		p.targetY = math.Max(40, math.Min(130, p.targetY))
		p.targetUntil = tick + p.Config.DecisionPAL
		return
	}
}

func (m *memorizedObservation) isMemorizedTail(actor Actor) bool {
	return m.tailUnsafe != nil && m.stage.Boss != nil && actor.Appearance == m.stage.Boss.TailArt.Sprite
}

func (m *memorizedObservation) isMemorizedSpiral(actor Actor) bool {
	for _, spiral := range m.spirals {
		if actor.Appearance == spiral.wave.Sprite {
			return true
		}
	}
	return false
}

// spiralEnvelope rehearses the authored circular route from its event clock,
// including each member's entry delay and the head's shared-center decrement.
// It reads no live actor phase, health, damage or random-generator state.
func (spiral memorizedSpiral) spiralEnvelope(segment int, future float64) (sim.Rect, bool) {
	d := spiral.wave.SpiralSentinel
	age := spiral.age + int((future+float64(spiral.mainPhase))/2)
	phase := age - segment*d.DelayStep - 1
	if phase < 0 || age > d.BaseX-d.StopX || len(d.MotionX) == 0 || len(d.MotionY) != len(d.MotionX) {
		return sim.Rect{}, false
	}
	base := d.BaseX - age
	if segment < d.Head {
		base--
	}
	phase %= len(d.MotionX)
	x, y := base+int(d.MotionX[phase]), d.BaseY+int(d.MotionY[phase])
	if x < 0 || x >= 350 {
		return sim.Rect{}, false
	}
	box := sim.CollisionFrame{Width: int(spiral.wave.W), Height: int(spiral.wave.H)}
	if len(spiral.wave.CollisionFrames) > 0 {
		box = spiral.wave.CollisionFrames[0]
	}
	return sim.Rect{X: float64(x-sim.GameOriginX+box.X) - 1, Y: float64(y-sim.GameOriginY+box.Y) - 1, W: float64(box.Width + 3), H: float64(box.Height + 3)}, true
}

func (m *memorizedObservation) precisionLane() bool {
	if m.camera != nil {
		return true
	}
	for _, spiral := range m.spirals {
		if spiral.age >= 140 {
			return true
		}
	}
	if m.stage.Boss == nil {
		return false
	}
	boss := m.stage.Boss
	return boss.Kind == "stage1_dobkeratops" && m.scroll >= float64(boss.ControllerTrigger-80) || boss.Kind == "stage2_gomander" && m.scroll >= float64(boss.FreezeScroll-140)
}

func (m *memorizedObservation) actorEnvelope(actor Actor) (sim.Rect, bool) {
	if boss := m.stage.Boss; boss != nil && boss.Fourth != nil && actor.Appearance == boss.FourthProjectileArt.Sprite && actor.Frame >= 0 && actor.Frame < len(boss.FourthProjectileArt.CollisionFrames) {
		box := boss.FourthProjectileArt.CollisionFrames[actor.Frame]
		return sim.Rect{X: actor.Bounds.X + float64(box.X), Y: actor.Bounds.Y + float64(box.Y), W: float64(box.Width + 1), H: float64(box.Height + 1)}, true
	}
	if m.camera != nil && m.stage.Weapons != nil && m.stage.Weapons.HostileShot != nil && len(m.stage.Weapons.HostileShot.Frames) > 0 && actor.Image == m.stage.Weapons.HostileShot.Frames[0] {
		// Source extents include both endpoints; planner rectangles are half-open.
		return sim.Rect{X: actor.Bounds.X + 5, Y: actor.Bounds.Y + 4, W: 7, H: 7}, true
	}
	boss := m.stage.Boss
	if boss != nil && actor.Appearance == boss.ProjectileArt.Sprite && actor.Frame >= 0 && actor.Frame < len(boss.ProjectileArt.CollisionFrames) {
		box := boss.ProjectileArt.CollisionFrames[actor.Frame]
		return sim.Rect{X: actor.Bounds.X + float64(box.X), Y: actor.Bounds.Y + float64(box.Y), W: float64(box.Width), H: float64(box.Height)}, true
	}
	return actor.Bounds, false
}

// memorizedTailMask records every authored pose, including one pixel of camera
// uncertainty. This conservative rehearsed envelope avoids falsely projecting
// a curved tail segment along its instantaneous linear velocity into a lane
// that is clear for the complete original forty-pose cycle.
func memorizedTailMask(boss *sim.BossSpec) []bool {
	mask := make([]bool, sim.Width*sim.PlayHeight)
	base := max(0, boss.TailBaseX-(boss.FreezeScroll-boss.TailTrigger+1))
	for jitter := -1; jitter <= 1; jitter++ {
		for _, pose := range boss.TailFrames {
			for segment, point := range pose {
				if len(point) < 2 {
					continue
				}
				frame := 5
				if segment == len(pose)-1 {
					frame = 0
				}
				if frame >= len(boss.TailArt.CollisionFrames) {
					continue
				}
				box := boss.TailArt.CollisionFrames[frame]
				left := point[0] + base + jitter - sim.GameOriginX + box.X
				top := point[1] + boss.TailYOffset - sim.GameOriginY + box.Y
				for y := max(0, top-3); y <= min(sim.PlayHeight-1, top+box.Height+3); y++ {
					for x := max(0, left-7); x <= min(sim.Width-1, left+box.Width+9); x++ {
						mask[y*sim.Width+x] = true
					}
				}
			}
		}
	}
	return mask
}

func (p *Pilot) memorizedFire(o Observation, tick int) (bool, bool) {
	if o.memory == nil || !p.charge {
		return false, false
	}
	boss := o.memory.stage.Boss
	if boss != nil && boss.Kind == "stage4_base" && o.memory.scroll >= 2000 {
		epoch := p.fireEpoch
		if p.baseStation != 0 {
			epoch = p.baseStationAt
		}
		return (tick-epoch)%56 < 52, true
	}
	if boss != nil && boss.Eighth != nil && o.memory.scroll >= float64(boss.Eighth.BrainScroll-140) {
		return (tick-p.fireEpoch)%56 < 52, true
	}
	if boss != nil && boss.Kind == "stage2_gomander" && o.memory.scroll >= float64(boss.FreezeScroll) {
		// A medium beam fits this inside-mouth station; a full beam's far
		// terrain probe reaches the right lip. Its visible opening cue retains
		// the same dynamic-threat reaction delay.
		if o.Charge >= 12 {
			return false, true
		}
		return o.Charge > 0 || o.memory.organArtFrame > 0, true
	}
	if boss == nil || boss.Kind != "stage1_dobkeratops" || !o.memory.precisionLane() {
		return false, false
	}
	// The deliberate cycle holds for 52 PAL ticks and releases for four.
	// A 54-tick period aliases against four-tick decisions and releases every108.
	// Holding52PAL also guarantees a complete24-main-update charge after the
	// initial normal-shot edge, independent of the PAL/main-loop phase.
	return (tick-p.fireEpoch)%56 < 52, true
}
