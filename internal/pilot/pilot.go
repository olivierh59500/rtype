package pilot

import (
	"math"
	"sort"

	"github.com/olivierh59500/r-type/internal/sim"
)

type Config struct {
	ReactionPAL, DecisionPAL, HorizonPAL, BeamWidth int
	ArenaScan                                       bool
	Memorized                                       bool
}

const baseStationMain = 5118

func DefaultConfig() Config {
	return Config{ReactionPAL: 6, DecisionPAL: 4, HorizonPAL: 80, BeamWidth: 36}
}

type Pilot struct {
	forceSwitch, forceSwitchAt int
	forceDodgeY                float64
	Decisions                  []Decision
	Progress                   func(Progress)
	Config                     Config
	sensor                     *Sensor
	queue                      []Observation
	last                       sim.Input
	lastDecision               int
	moveUntil                  int
	tailMasks                  map[int][]bool
	bossTwo                    *bossTwoMemory
	battleshipCamera           *rehearsedCamera
	battleshipTileArt          map[uint16]uint8
	battleshipIrisMovie        *memorizedIrisMovie
	battleshipHullMovie        *memorizedIrisMovie
	battleshipRoute            *rehearsedBattleshipRoute
	battleshipMotor            sim.Input
	battleshipShotSensor       *Sensor
	battleshipShotQueue        []Observation
	battleshipShotView         Observation
	battleshipShotDecision     int
	baseCamera                 *rehearsedCamera
	baseBodies                 *rehearsedBaseBodies
	quarterMemory              *quarterMemory
	firstCamera                *rehearsedCamera
	crawlerCamera              *rehearsedCamera
	crawlerRoutes              *rehearsedCrawlers
	resourceCamera             *rehearsedCamera
	finalCamera                *rehearsedCamera
	baseStation, baseStationAt int
	// Prefix contains already verified normal controls for completed stages.
	// It never restores or injects a simulation state.
	Prefix           *Replay
	lastAlive        bool
	targetX, targetY float64
	targetUntil      int
	charge           bool
	fireEpoch        int
	lastStage        int
	animated         []sim.Rect
	animationTick    int
	stoppedTicks     int
}

// Decision is pilot-only telemetry; it contains no hidden engine diagnostics.
type Decision struct {
	Tick                   int
	X, Y, TargetX, TargetY float64
	Charge                 bool
	Animated               int
	Stopped                int
	Force                  sim.ForceMode
}

func New(config Config) *Pilot {
	if config.DecisionPAL < 1 {
		config.DecisionPAL = 4
	}
	if config.ReactionPAL < 0 {
		config.ReactionPAL = 0
	}
	if config.HorizonPAL < config.DecisionPAL {
		config.HorizonPAL = 32
	}
	if config.BeamWidth < 1 {
		config.BeamWidth = 36
	}
	return &Pilot{Config: config, sensor: NewSensor(), lastDecision: -1000, targetX: 95, targetY: 90}
}

// Next never steps or modifies the engine. Reactive decisions use delayed
// observations; familiar routes can execute prepared forty-millisecond motors
// using own-ship feedback without additional tactical threat observations.
func (p *Pilot) Next(w *sim.World, tick int) sim.Input {
	if p.Prefix != nil && tick < len(p.Prefix.Frames) {
		return p.Prefix.InputAt(tick)
	}
	// Arm the practiced late firing rhythm on its prepared odd-PAL entry.
	// Movement still passes through delayed visible threats and body routes.
	if p.Config.Memorized && w.Stage == 4 && w.Player.Alive && (w.StageTick > baseStationMain || w.StageTick == baseStationMain && w.Tick&1 == 1) {
		if p.baseStation == 0 {
			p.baseStation, p.baseStationAt = 1, tick
		}
	}
	if p.baseStation != 0 && w.Stage != 4 {
		p.sensor, p.queue, p.animated = NewSensor(), nil, nil
		p.baseStation, p.targetUntil, p.stoppedTicks, p.animationTick = 0, 0, 0, 0
	}
	if p.Config.Memorized && w.Stage == 3 && w.StageTick >= battleshipRehearsalEntry && w.Player.Alive && w.Player.SpeedLevel == 1 {
		if input, ok := p.rehearsedBattleshipInput(w); ok {
			return input
		}
	}
	if p.battleshipRoute != nil && w.Stage != 3 {
		// Rehearsed motors do not sample new threats. Start fresh delayed
		// observations when the next level returns to reactive play.
		p.sensor, p.queue, p.animated = NewSensor(), nil, nil
		p.battleshipRoute = nil
		p.battleshipShotSensor, p.battleshipShotQueue = nil, nil
		p.battleshipShotView = Observation{}
		p.targetUntil, p.stoppedTicks, p.animationTick = 0, 0, 0
	}
	if p.Config.Memorized && w.Stage == 3 && w.Player.Alive && !p.lastAlive {
		// A fresh ship sees a restored map; previous damage animations must
		// not become permanent phantom obstacles after the blackout.
		p.sensor, p.queue, p.animated = NewSensor(), nil, nil
		p.targetUntil, p.stoppedTicks, p.animationTick = 0, 0, 0
	}
	p.lastAlive = w.Player.Alive
	current := p.sensor.Observe(w, tick)
	if p.Config.Memorized {
		p.rememberAuthoredLevel(w, &current)
	}
	p.queue = append(p.queue, current)
	// Hold the human start gesture for 80 ms so an SDL host polling at PAL
	// cadence observes the same recorded level without scheduler stretching.
	if tick < 4 {
		return sim.Input{Start: true}
	}
	if current.State == sim.Title {
		return sim.Input{Start: tick == 0}
	}
	if current.State != sim.Playing || !current.Alive || !current.Combat || current.Automatic || current.Exiting {
		p.last = sim.Input{}
		return p.last
	}
	if tick-p.lastDecision < p.Config.DecisionPAL {
		if p.moveUntil > 0 && tick >= p.moveUntil {
			// A rehearsed tap releases at its preplanned time. This does not
			// inspect threats or make another tactical decision between updates.
			p.last.Up, p.last.Down, p.last.Left, p.last.Right = false, false, false, false
		}
		return p.last
	}
	index := -1
	for i := range p.queue {
		if p.queue[i].Tick <= tick-p.Config.ReactionPAL {
			index = i
		}
	}
	if index < 0 {
		return sim.Input{}
	}
	o := p.queue[index]
	p.queue = p.queue[max(0, index-1):]
	if !o.Alive || o.State != sim.Playing {
		p.last = sim.Input{}
		return p.last
	}
	if p.lastStage != o.Stage {
		p.lastStage = o.Stage
		p.targetUntil = 0
		p.animated = nil
		p.stoppedTicks = 0
		p.fireEpoch = tick
	}
	p.lastDecision = tick
	// Own-ship position is continuous motor feedback; threat perception retains
	// the configured delay. Inputs still change only at the decision cadence.
	o.X, o.Y = current.X, current.Y
	if o.memory != nil && o.memory.camera != nil {
		o.Speed = current.Speed
	}
	if math.Abs(o.CameraVX) < 0.05 && math.Abs(o.CameraVY) < 0.05 && o.Combat {
		p.stoppedTicks += p.Config.DecisionPAL
	} else {
		p.stoppedTicks = 0
	}
	if len(o.AnimatedMap) > 0 {
		if p.stoppedTicks < 12 {
			p.animated = nil
		}
		for _, candidate := range o.AnimatedMap {
			found := false
			for _, known := range p.animated {
				if math.Abs(candidate.X-known.X) < 2 && math.Abs(candidate.Y-known.Y) < 2 {
					found = true
					break
				}
			}
			if !found {
				p.animated = append(p.animated, candidate)
			}
		}
		if len(p.animated) > 24 {
			p.animated = p.animated[len(p.animated)-24:]
		}
		p.animationTick = tick
	}
	wasCharging := p.charge
	p.selectTarget(o, tick)
	if p.Config.Memorized {
		p.selectMemorizedTarget(o, tick)
		if p.charge && !wasCharging {
			p.fireEpoch = tick
		}
	}
	forcePulse := false
	if o.ForceMode == sim.ForceNone || o.ForceMode == sim.ForceFront {
		p.forceSwitch = 0
	}
	if o.ForceMode == sim.ForceBack && p.forceSwitch == 0 {
		p.forceSwitch, p.forceSwitchAt = 1, tick
		forcePulse = true
		p.forceDodgeY = o.Y + 40
		if o.Y >= 95 {
			p.forceDodgeY = o.Y - 40
		}
	}
	if p.forceSwitch == 1 {
		p.targetX = math.Min(120, o.X)
		p.targetY = p.forceDodgeY
		if o.ForceMode == sim.ForceFree && o.ForceX > o.X+35 && tick-p.forceSwitchAt > 20 {
			p.forceSwitch = 2
			p.forceSwitchAt = tick
			forcePulse = true
		}
	} else if p.forceSwitch == 2 {
		p.targetY = o.ForceY - 6
	}
	p.Decisions = append(p.Decisions, Decision{tick, current.X, current.Y, p.targetX, p.targetY, p.charge, len(p.animated), p.stoppedTicks, o.ForceMode})
	input := p.plan(o, tick)
	if o.Combat {
		if p.charge {
			input.Fire = (tick-p.fireEpoch)%54 < 52
		} else {
			input.Fire = (tick-p.fireEpoch)%10 < 4
		}
	}
	if p.Config.Memorized {
		if fire, override := p.memorizedFire(o, tick); override {
			input.Fire = fire
		}
	}
	input.Force = forcePulse
	p.last = input
	return input
}

func (p *Pilot) selectTarget(o Observation, tick int) {
	if tick < p.targetUntil {
		return
	}
	p.targetX, p.targetY = 95, 92
	p.charge = false
	best := 0.0
	for _, pickup := range o.Pickups {
		value := 1.0
		switch pickup.Kind {
		case sim.PowerSpeed:
			if o.Speed >= 3 && p.Config.Memorized {
				value = 0
			} else if o.Speed >= 3 {
				value = 0.4
			} else {
				value = 2
			}
		case sim.PowerBit:
			if o.Bits >= 2 {
				value = 0
			} else {
				value = 1.6
			}
		case sim.PowerMissiles:
			if o.Missiles {
				value = 0.2
			} else {
				value = 1.8
			}
		case sim.PowerBlue, sim.PowerRed, sim.PowerYellow:
			if o.ForceMode == sim.ForceNone {
				value = 4
			} else if o.WeaponLevel < 3 {
				value = 2
			} else {
				value = 0.2
			}
		}
		x, y := pickup.Bounds.X+pickup.Bounds.W/2, pickup.Bounds.Y+pickup.Bounds.H/2
		priority := value / (30 + math.Abs(x-o.X) + 0.6*math.Abs(y-o.Y))
		if priority > best {
			best = priority
			p.targetX, p.targetY = x, y
		}
	}
	if best > 0 {
		p.targetUntil = tick + 12
		return
	}
	best = 0
	for _, actor := range o.Actors {
		if actor.Projectile || actor.Bounds.X < o.X+50 {
			continue
		}
		x, y := actor.Bounds.X+actor.Bounds.W/2, actor.Bounds.Y+actor.Bounds.H/2
		if !actor.Carrier && (y < 35 || y > 135) {
			continue
		}
		value := 1.0
		if actor.Carrier {
			value = 10
		}
		priority := value / (40 + math.Abs(x-o.X) + math.Abs(y-o.Y))
		if priority > best {
			best = priority
			p.targetY = y
		}
	}
	// A stopped camera and animated map feature describe a visible arena. Aim
	// at its current animation, without consulting boss HP or hidden geometry.
	if p.stoppedTicks >= 20 && tick-p.animationTick < 250 {
		var target *sim.Rect
		for i := range p.animated {
			r := &p.animated[i]
			if r.X > 110 && r.Y > 24 && r.Y < 150 && (target == nil || r.X > target.X) {
				target = r
			}
		}
		if target != nil {
			// Remember the complete vertical feature, rather than chasing the
			// first changing 16-pixel cell of a larger visible eye or aperture.
			top, bottom := target.Y, target.Y+target.H
			for _, r := range p.animated {
				if math.Abs(r.X-target.X) < 2 && r.Y <= bottom && r.Y+r.H >= top {
					top = math.Min(top, r.Y)
					bottom = math.Max(bottom, r.Y+r.H)
				}
			}
			p.targetY = (top+bottom)/2 + 10
			if p.Config.ArenaScan {
				// Sweep three nearby visible rows over several beam cycles. This
				// models searching an unfamiliar opening without reading its HP
				// or invisible collision rectangle.
				p.targetY = (top+bottom)/2 + float64((p.stoppedTicks/96)%3-1)*8
			}
			// A short firing lane reduces interception by the visible moving
			// body, and brings an attached Force toward the animated feature.
			p.targetX = math.Max(95, target.X-40)
			p.charge = true
		}
	}
	p.targetY = math.Max(20, math.Min(173, p.targetY))
	p.targetX = math.Max(45, math.Min(235, p.targetX))
	p.targetUntil = tick + 12
}

type action struct{ dx, dy, hold int }

var actions = []action{{0, 0, 0}, {0, -1, 0}, {0, 1, 0}, {-1, 0, 0}, {1, 0, 0}, {-1, -1, 0}, {-1, 1, 0}, {1, -1, 0}, {1, 1, 0}}

type path struct {
	x, y, cost float64
	fatalAt    int
	first      action
	previous   action
}

func (p *Pilot) plan(o Observation, tick int) sim.Input {
	lag := float64(tick - o.Tick)
	x, y := o.X, o.Y
	choices := actions
	if p.Config.Memorized && p.Config.DecisionPAL > 2 && o.memory != nil && o.memory.precisionLane() {
		choices = append([]action(nil), actions...)
		for _, move := range actions[1:] {
			move.hold = 2
			choices = append(choices, move)
		}
	}
	beam := []path{{x: x, y: y}}
	survivalFirst := o.memory != nil && o.memory.camera != nil
	depth := max(1, p.Config.HorizonPAL/p.Config.DecisionPAL)
	for step := 0; step < depth; step++ {
		candidates := make([]path, 0, len(beam)*len(choices))
		for _, parent := range beam {
			for _, move := range choices {
				next := parent
				next.previous = move
				if step == 0 {
					next.first = move
				}
				if survivalFirst && next.fatalAt != 0 {
					// Death is an absorbing prediction. Continuing through a
					// wall cannot earn a better future firing-lane score.
					candidates = append(candidates, next)
					continue
				}
				for frame := 1; frame <= p.Config.DecisionPAL; frame++ {
					if move.hold == 0 || frame <= move.hold {
						if px, py, rehearsed := rehearsedMovement(o, next.x, next.y, move); rehearsed {
							next.x, next.y = px, py
						} else {
							next.x = math.Max(37, math.Min(305, next.x+float64(move.dx)*o.Speed))
							next.y = math.Max(12, math.Min(180, next.y+float64(move.dy)*o.Speed))
						}
					}
					future := lag + float64(step*p.Config.DecisionPAL+frame)
					risk := danger(o, next.x, next.y, future)
					next.cost += risk
					if survivalFirst && risk >= 5e5 {
						next.fatalAt = step*p.Config.DecisionPAL + frame
						break
					}
				}
				if survivalFirst && next.fatalAt != 0 {
					candidates = append(candidates, next)
					continue
				}
				dx, dy := next.x-p.targetX, next.y-p.targetY
				next.cost += 0.012*dx*dx + 0.021*dy*dy
				if step > 0 && move != parent.previous {
					next.cost += 3
				}
				if move.dx != 0 || move.dy != 0 {
					next.cost += 0.5
				}
				candidates = append(candidates, next)
			}
		}
		sort.Slice(candidates, func(i, j int) bool {
			if survivalFirst && candidates[i].fatalAt != candidates[j].fatalAt {
				if candidates[i].fatalAt == 0 {
					return true
				}
				if candidates[j].fatalAt == 0 {
					return false
				}
				return candidates[i].fatalAt > candidates[j].fatalAt
			}
			return candidates[i].cost < candidates[j].cost
		})
		beam = beam[:0]
		seen := make(map[[2]int]bool)
		for _, candidate := range candidates {
			key := [2]int{int(candidate.x / 4), int(candidate.y / 4)}
			if seen[key] {
				continue
			}
			seen[key] = true
			beam = append(beam, candidate)
			if len(beam) >= p.Config.BeamWidth {
				break
			}
		}
	}
	if len(beam) == 0 {
		return sim.Input{}
	}
	move := beam[0].first
	p.moveUntil = 0
	if move.hold > 0 {
		p.moveUntil = tick + move.hold
	}
	return sim.Input{Left: move.dx < 0, Right: move.dx > 0, Up: move.dy < 0, Down: move.dy > 0}
}

func danger(o Observation, x, y, future float64) float64 {
	hull := sim.Rect{X: x - 10, Y: y - 6, W: 20, H: 12}
	if o.memory != nil && o.memory.camera != nil {
		// All five source banking poses fit this inclusive body envelope.
		hull = sim.Rect{X: x - 9, Y: y - 3, W: 17, H: 8}
	}
	if o.memory != nil && o.memory.precisionLane() && o.memory.camera == nil {
		// Include both source endpoints and the one-row banking variation.
		hull = sim.Rect{X: x - 9, Y: y - 3, W: 17, H: 8}
	}
	cost := 0.0
	cameraX, cameraY := o.CameraVX*future, o.CameraVY*future
	if o.memory != nil {
		if dx, dy, known := o.memory.cameraDisplacement(future); known {
			cameraX, cameraY = dx, dy
		}
	}
	for _, wall := range o.Walls {
		if o.memory != nil && o.memory.forecastMapCell(wall) {
			continue
		}
		wall.X -= cameraX
		wall.Y -= cameraY
		// Terrain clearance follows the ship's visible central cockpit, rather
		// than treating its decorative wings as a solid rectangle.
		terrainHull := sim.Rect{X: x, Y: y - 3, W: 7, H: 6}
		hit := terrainHull.Overlaps(wall)
		if o.memory != nil && o.memory.camera != nil {
			// Original 0x1836 checks a single point only in the main pass;
			// the intervening PAL movement can traverse a clear edge safely.
			terrainHull = sim.Rect{X: x + 4, Y: y - 1}
			hit = (int(future)+o.memory.mainPhase)&1 == 0 && terrainHull.X >= wall.X && terrainHull.X < wall.X+wall.W && terrainHull.Y >= wall.Y && terrainHull.Y < wall.Y+wall.H
		}
		if hit {
			cost += 1e6
		} else {
			distance := rectDistance(terrainHull, wall)
			if distance < 10 {
				cost += 90 / (distance + 1)
			}
		}
	}
	for _, actor := range o.Actors {
		r := actor.Bounds
		knownEnvelope := actor.KnownEnvelope
		travel := future
		if o.memory != nil && o.memory.stage.Boss != nil && o.memory.stage.Boss.Fourth != nil && actor.Appearance == o.memory.stage.Boss.FourthProjectileArt.Sprite {
			travel = float64(int((future+float64(o.memory.mainPhase))/2) * 2)
		}
		if o.memory != nil && o.memory.camera != nil && actor.KnownEnvelope && actor.Image != "" {
			// 0x1354 probes the old position, then moves the shot once per
			// main update. Forecasting its post-move image misses crossing hits.
			travel = float64(max(0, int((future+float64(o.memory.mainPhase))/2)-1) * 2)
		}
		r.X += actor.VX * travel
		r.Y += actor.VY * travel
		// Visual acceleration and the PAL-sampled velocity estimate leave a
		// bounded uncertainty margin; this is not an exact engine trajectory.
		margin := 3 + math.Min(6, future*0.08)
		if knownEnvelope {
			margin = 1 + math.Min(2, future*0.02)
		}
		r.X -= margin
		r.Y -= margin
		// A visible sprite is not its collision envelope. Keep additional space
		// ahead of moving BOB art rather than assuming transparent edges safe.
		r.W += 2 * margin
		if !knownEnvelope {
			r.W += 16
		}
		r.H += 2 * margin
		contactHull := hull
		if o.memory != nil && o.memory.camera != nil && actor.Image != "" {
			// The common blue-shot pool probes a wider player extent.
			contactHull = sim.Rect{X: x - 11, Y: y - 3, W: 21, H: 8}
		}
		if contactHull.Overlaps(r) {
			cost += 5e5
		} else {
			distance := rectDistance(hull, r)
			if distance < 28 && !knownEnvelope {
				cost += 140 / (distance + 2)
			}
		}
	}
	if o.memory != nil && o.memory.tailUnsafe != nil {
		ix, iy := int(x), int(y)
		if ix >= 0 && ix < sim.Width && iy >= 0 && iy < sim.PlayHeight && o.memory.tailUnsafe[iy*sim.Width+ix] {
			cost += 5e5
		}
	}
	if o.memory != nil {
		for _, index := range o.memory.baseBodyLanes {
			if box, ok := o.memory.baseBodyEnvelope(index, future); ok && hull.Overlaps(box) {
				cost += 5e5
			}
			if index == 2 {
				if box, ok := o.memory.baseLanceZone(future); ok && hull.Overlaps(box) {
					cost += 5e5
				}
			}
		}
		for _, quarter := range o.memory.quarters {
			age := int((future + float64(o.memory.mainPhase)) / 2)
			if age >= 0 && age < len(quarter.boxes) && quarter.active[age] && hull.Overlaps(quarter.boxes[age]) {
				cost += 5e5
			}
		}
		for _, index := range o.memory.crawlerLanes {
			if box, ok := o.memory.crawlerEnvelope(index, future); ok && hull.Overlaps(box) {
				cost += 5e5
			}
		}
		if o.memory.irisMovieActive && (int(future)+o.memory.mainPhase)&1 == 0 && o.memory.irisMovie.solidAt(o.memory, x+4, y-1, future) {
			cost += 1e6
		}
		if o.memory.hullMovieActive && (int(future)+o.memory.mainPhase)&1 == 0 && o.memory.hullMovie.solidAt(o.memory, x+4, y-1, future) {
			cost += 1e6
		}
		if iris, active := o.memory.irisEnvelope(future); active && hull.Overlaps(iris) {
			cost += 5e5
		}
		if snake := o.memory.snake; snake != nil {
			for component := range snake.data.Components {
				if r, active := snake.snakeEnvelope(component, future); active && hull.Overlaps(r) {
					cost += 5e5
				}
			}
		}
		for _, spiral := range o.memory.spirals {
			for segment := 0; segment < spiral.wave.SpiralSentinel.Count; segment++ {
				if r, active := spiral.spiralEnvelope(segment, future); active && hull.Overlaps(r) {
					cost += 5e5
				}
			}
		}
	}
	for _, r := range o.MapHazards {
		if o.memory != nil && o.memory.forecastMapCell(r) {
			continue
		}
		r.X -= cameraX
		r.Y -= cameraY
		r.X -= 5
		r.Y -= 3
		r.W += 10
		r.H += 6
		if hull.Overlaps(r) {
			cost += 5e5
		}
	}
	return cost
}
func rectDistance(a, b sim.Rect) float64 {
	dx := math.Max(0, math.Max(a.X-b.X-b.W, b.X-a.X-a.W))
	dy := math.Max(0, math.Max(a.Y-b.Y-b.H, b.Y-a.Y-a.H))
	return math.Hypot(dx, dy)
}
