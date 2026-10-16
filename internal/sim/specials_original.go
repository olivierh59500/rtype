package sim

import "github.com/olivierh59500/r-type/internal/source"

type HuntingEmitterState struct {
	Phase, DX, DY, Target, Rest, Repeat, Escape, Index, Timer int
	Retreat                                                   bool
}
type ShieldVehicleState struct {
	Travel, Damage, Debris int
	Dying                  bool
}
type SpiralSentinelState struct {
	Data              *source.SpiralSentinelData
	BaseX, LastTick   int
	Broken, Stopped   bool
	Members           []uint64
	Automatic, Frozen []bool
}

func (w *World) initializeHunter(enemy *Enemy) {
	if enemy.Hunter != nil {
		return
	}
	enemy.Hunter = &HuntingEmitterState{Escape: enemy.HuntingEmitter.EscapeReload}
	w.selectHunterTarget(enemy)
}

func (w *World) selectHunterTarget(enemy *Enemy) {
	d, s := enemy.HuntingEmitter, enemy.Hunter
	x, y := int(enemy.X-enemy.W/2+GameOriginX), int(enemy.Y-enemy.H/2+GameOriginY)
	playerY := int(w.Player.Y) + 25
	dx := d.PositiveVX
	if y >= playerY-d.NearLow && y <= playerY+d.NearHigh {
		s.Phase, s.DY = 1, 0
		rangeX := int(w.nextRandom() % uint64(d.NearRange))
		if s.Retreat || x+rangeX > d.MaxTargetX {
			dx = d.NegativeVX
		} else {
			w.nextRandom()
		}
		if dx < 0 {
			rangeX = -rangeX
		}
		s.Target = x + rangeX
	} else {
		s.Phase = 0
		s.Target = playerY + d.TargetYOffset + int(w.nextRandom()%uint64(d.TargetYRange))
		if s.Target > d.MaxTargetY {
			s.Target = d.MaxTargetY
		}
		distance := s.Target - y
		s.DY = d.VY
		if distance < 0 {
			s.DY = -s.DY
			distance = -distance
		}
		if s.Retreat || x+distance > d.MaxTargetX {
			dx = d.NegativeVX
		} else {
			w.nextRandom()
		}
	}
	s.DX, s.Index = dx, 1
	if dx < 0 {
		s.Index = 2
	}
}

func hunterArt(enemy *Enemy) {
	s := enemy.Hunter
	s.Timer--
	if s.Timer < 0 {
		s.Timer = max(1, enemy.AnimationTicks) - 1
		enemy.AnimationFrame = s.Index
	}
	enemy.AnimationOverride = true
}

func (w *World) updateHuntingEmitter(enemy *Enemy) bool {
	if enemy.HuntingEmitter == nil {
		return false
	}
	d := enemy.HuntingEmitter
	if enemy.Behavior == BehaviorHunterShot {
		enemy.X += float64(d.ProjectileVX)
		if int(enemy.X-enemy.W/2+GameOriginX) < 0 {
			enemy.Health = 0
		}
		return true
	}
	w.initializeHunter(enemy)
	s := enemy.Hunter
	move := false
	var shot *Enemy
	x, y := int(enemy.X-enemy.W/2+GameOriginX), int(enemy.Y-enemy.H/2+GameOriginY)
	if s.Phase == 2 {
		s.Rest--
		if s.Rest < 0 {
			s.Repeat--
			if s.Repeat >= 0 {
				w.selectHunterTarget(enemy)
			} else {
				s.Repeat = d.RepeatBase + int(w.nextRandom()%uint64(d.RepeatRange))
				s.Phase, s.DY, s.DX, s.Target, s.Index = 1, 0, d.PositiveVX, x+10, 2
				created := Enemy{Entity: w.newEntity(float64(x+d.ProjectileOffsetX)-GameOriginX+float64(d.ProjectileWidth)/2, float64(y+d.ProjectileOffsetY)-GameOriginY+float64(d.ProjectileHeight)/2, 0, 0, float64(d.ProjectileWidth), float64(d.ProjectileHeight)), Behavior: BehaviorHunterShot, HuntingEmitter: d, Health: 1, Invincible: true, NoFriendlyTarget: true, Descriptor: d.ProjectileDescriptor, Sprite: d.ProjectileSprite, Animation: d.ProjectileAnimation, AnimationTicks: d.ProjectileTicks}
				for _, box := range d.ProjectileCollision {
					created.CollisionFrames = append(created.CollisionFrames, CollisionFrame{X: box.X, Y: box.Y, Width: box.Width, Height: box.Height})
				}
				shot = &created
			}
		}
	} else {
		value, velocity := y, s.DY
		if s.Phase == 1 {
			value, velocity = x, s.DX
		}
		if (velocity >= 0 && value > s.Target) || (velocity < 0 && value < s.Target) {
			s.Phase, s.Rest, s.Index = 2, int(w.nextRandom()%uint64(d.RestRange)), 0
		} else {
			move = true
		}
	}
	if move {
		enemy.X += float64(s.DX)
		if s.Phase == 0 {
			enemy.Y += float64(s.DY)
		}
		s.Escape--
		if s.Escape < 0 {
			s.Retreat = true
		}
		if int(enemy.X-enemy.W/2+GameOriginX) < 0 {
			enemy.Health = 0
		}
	}
	hunterArt(enemy)
	if shot != nil {
		w.Enemies = append(w.Enemies, *shot)
	}
	return true
}

func (w *World) updateShieldVehicle(enemy *Enemy) bool {
	d := enemy.ShieldVehicle
	if d == nil {
		return false
	}
	if enemy.Vehicle == nil {
		enemy.Vehicle = &ShieldVehicleState{Travel: d.TravelReload}
	}
	s := enemy.Vehicle
	if s.Dying {
		s.Debris--
		if s.Debris < 0 {
			w.specialDeath(*enemy, d.Death)
			w.addScore(d.Score)
			enemy.Health = 0
			return true
		}
		y := int(enemy.Y-enemy.H/2+GameOriginY) + d.DebrisOffsetY + int(w.nextRandom()%uint64(d.DebrisYRange))
		x := int(enemy.X-enemy.W/2+GameOriginX) + d.DebrisOffsetX + int(w.nextRandom()%uint64(d.DebrisXRange))
		temp := *enemy
		temp.X, temp.Y = float64(x)-GameOriginX+enemy.W/2, float64(y)-GameOriginY+enemy.H/2
		// Source 0x4B5EA puts the independently allocated debris after the
		// vehicle's own priority-ten node, preserving its death replacement.
		w.specialDebris(temp, d.Death, 40)
		return true
	}
	if int(enemy.X-enemy.W/2+GameOriginX) >= int(w.Player.X)+18 {
		s.Damage += enemy.DamageTaken
	}
	enemy.DamageTaken = 0
	if s.Damage >= d.DamageLimit {
		s.Dying, s.Debris = true, d.DebrisReload
	}
	if s.Travel >= 0 {
		s.Travel--
		if s.Travel&1 != 0 {
			enemy.X++
		}
	} else {
		enemy.X--
		if int(enemy.X-enemy.W/2+GameOriginX) < 0 {
			enemy.Health = 0
		}
	}
	return true
}

func (w *World) specialDeath(enemy Enemy, d source.SpecialEffectData) {
	effect := w.specialDeathEffect(enemy, d)
	effect.ActorOwner, effect.Priority = enemy.ID, enemy.Priority
	w.Effects = append(w.Effects, effect)
	w.Events = append(w.Events, Event{Kind: EventSound, Value: d.Sound, X: enemy.X - enemy.W/2, Y: enemy.Y - enemy.H/2})
}

func (w *World) specialDebris(enemy Enemy, d source.SpecialEffectData, priority int) {
	effect := w.specialDeathEffect(enemy, d)
	effect.Priority, effect.Sound = priority, d.Sound
	w.Effects = append(w.Effects, effect)
}

func (w *World) specialDeathEffect(enemy Enemy, d source.SpecialEffectData) Effect {
	x, y := enemy.X-enemy.W/2, enemy.Y-enemy.H/2
	return Effect{Entity: w.newEntity(x+float64(d.Width)/2, y+float64(d.Height)/2, 0, 0, float64(d.Width), float64(d.Height)), Sprite: d.Sprite, Lifetime: d.Ticks, FrameTicks: 1, Animation: d.Animation}
}

func (w *World) spawnSpiralSentinel(template Enemy) {
	d := template.SpiralSentinel
	s := &SpiralSentinelState{Data: d, BaseX: d.BaseX, LastTick: -1, Automatic: make([]bool, d.Count), Frozen: make([]bool, d.Count)}
	for segment := 0; segment < d.Count; segment++ {
		enemy := template
		enemy.Entity = w.newEntity(float64(d.SpawnX)-GameOriginX+template.W/2, float64(d.SpawnY)-GameOriginY+template.H/2, 0, 0, template.W, template.H)
		enemy.Spiral = s
		enemy.bossSegment = segment
		enemy.bossDelay = segment * d.DelayStep
		enemy.BehaviorTicks = d.BreakInitial - segment*d.BreakStep
		enemy.AnimationOverride = true
		for _, v := range d.AimMap {
			enemy.AimMap = append(enemy.AimMap, int8(v))
		}
		enemy.AimThresholds = d.AimThresholds
		s.Members = append(s.Members, enemy.ID)
		w.Enemies = append(w.Enemies, enemy)
	}
}

func (w *World) updateSpiralSentinel(enemy *Enemy) bool {
	s := enemy.Spiral
	if s == nil {
		return false
	}
	if w.actorUpdateID == 0 {
		if s.LastTick == w.StageTick {
			return true
		}
		s.LastTick = w.StageTick
	}
	d := s.Data
	first, last := len(s.Members)-1, 0
	if w.actorUpdateID != 0 {
		first, last = enemy.bossSegment, enemy.bossSegment
	}
	for segment := first; segment >= last; segment-- {
		var e *Enemy
		if w.actorUpdateID != 0 {
			e = enemy
		} else {
			for index := range w.Enemies {
				if w.Enemies[index].ID == s.Members[segment] {
					e = &w.Enemies[index]
					break
				}
			}
		}
		if e == nil || e.Health <= 0 {
			continue
		}
		if s.Stopped {
			e.Health = 0
			continue
		}
		if !s.Broken {
			distance := int(w.Player.X) + 18 - s.BaseX
			if distance < 0 {
				distance = -distance
			}
			if distance >= d.ShotNear {
				e.ShootMin = 0
				e.AutomaticFire = 0
			} else if e.ShootMin == 0 {
				e.ShootMin = d.ShotMin + int(w.nextRandom()%uint64(d.ShotRange))
				e.ShootCooldown = e.ShootMin
				e.AutomaticFire, e.AutomaticFireTimer = e.ShootMin, e.ShootMin
			}
		}
		if e.bossDelay >= 0 {
			e.bossDelay--
			if e.bossDelay < 0 {
				s.Automatic[segment] = true
				e.bossMotion = 0
			}
		} else if len(d.MotionX) > 0 {
			phase := e.MotionTick % len(d.MotionX)
			e.X = float64(s.BaseX+int(d.MotionX[phase])) - GameOriginX + e.W/2
			e.Y = float64(d.BaseY+int(d.MotionY[phase])) - GameOriginY + e.H/2
			e.MotionTick = (phase + 1) % len(d.MotionX)
			if s.Broken && !s.Frozen[segment] {
				e.BehaviorTicks--
				if e.BehaviorTicks < 0 {
					// Source 0x4B186 allocates separate priority-ten debris.
					// The live cursor reaches its appended node this same pass.
					w.specialDebris(*e, d.Death, 10)
					w.addScore(e.Score)
					s.Frozen[segment], s.Automatic[segment] = true, false
					e.ShootMin = 0
					e.AutomaticFire = 0
					e.AnimationStart = d.BrokenIndex
					e.bossMotion = 0
				}
			}
		}
		if segment == d.Head {
			s.BaseX--
			if s.BaseX == d.StopX {
				s.Stopped = true
			}
			if e.AnimationStart < 5 {
				e.DamageTaken = 0
			} else if e.DamageTaken >= d.DamageLimit {
				s.Broken = true
			}
		}
		stream := d.Body
		if segment == d.Head {
			stream = d.HeadStream
		}
		e.bossMotion--
		if e.bossMotion < 0 {
			e.bossMotion = max(1, e.AnimationTicks) - 1
			index := e.AnimationStart
			if index < 0 || index >= len(stream) || stream[index] < 0 {
				index = 0
			}
			e.AnimationFrame = stream[index]
			e.AnimationStart = index
			if s.Automatic[segment] {
				e.AnimationStart++
			}
		}
		if e.ShootMin > 0 && w.weaponTables() == nil {
			e.ShootCooldown--
			if e.ShootCooldown < 0 {
				e.ShootCooldown = e.ShootMin
				direction := w.originalAimDirection(*e)
				w.addBullet(EnemyShot, false, e.X-e.W/2+5, e.Y-e.H/2+4, float64(d.ShotVX[direction]), float64(d.ShotVY[direction]), 12, 12, 1, 250, false)
			}
		}
	}
	return true
}
