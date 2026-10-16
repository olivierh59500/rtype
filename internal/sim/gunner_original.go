package sim

// GroundGunnerState follows the original three phases and animation byte timer.
// Animation streams contain reset markers, not instructions.
type GroundGunnerState struct {
	Phase, VX, Cooldown, ShotTimer, Shots, Sign int
	Stream                                      []int
	Index, Timer, Frame                         int
	Auto                                        bool
}

// ArcProjectileState preserves the two independent source-table phases.
type ArcProjectileState struct {
	XPhase, YPhase, Sign int
}

func (w *World) gunnerDistance(enemy Enemy) (int, int) {
	value := int(w.Player.X) + 18 - int(enemy.X-enemy.W/2+GameOriginX)
	sign := 1
	if value < 0 {
		value = -value
		value += enemy.GroundGunner.DistanceLeftBonus
		sign = -1
	}
	return value, sign
}

func (w *World) selectGunnerMode(enemy *Enemy, state *GroundGunnerState) {
	d := enemy.GroundGunner
	distance, sign := w.gunnerDistance(*enemy)
	if distance >= d.RestMin && distance < d.RestMax {
		state.Phase, state.Cooldown, state.Auto, state.Index = 2, d.RestTicks-1, false, 3
		return
	}
	state.Phase, state.Auto = 0, true
	state.VX = sign * d.WalkSpeed
	if distance <= d.HoldDistance {
		state.VX = -state.VX
	}
	state.Stream = d.RawLeft
	if state.VX >= 0 {
		state.Stream = d.RawRight
	}
}

func advanceGunnerArt(enemy *Enemy, state *GroundGunnerState) {
	state.Timer--
	if state.Timer < 0 {
		state.Timer = max(1, enemy.AnimationTicks) - 1
		if state.Index < 0 || state.Index >= len(state.Stream) || state.Stream[state.Index] < 0 {
			state.Index = 0
		}
		if len(state.Stream) > 0 {
			state.Frame = state.Stream[state.Index]
		}
		if state.Auto {
			state.Index++
		}
	}
	enemy.AnimationOverride, enemy.AnimationFrame = true, state.Frame
}

// updateGroundGunner translates the original walk/rest/burst routines. Changes
// to a frozen frame take effect only when the original animation timer expires.
func (w *World) updateGroundGunner(enemy *Enemy) bool {
	if enemy.Behavior != BehaviorGroundGunner || enemy.GroundGunner == nil {
		return false
	}
	d := enemy.GroundGunner
	if enemy.Gunner == nil {
		enemy.Gunner = &GroundGunnerState{Stream: d.RawLeft, Auto: true}
		w.selectGunnerMode(enemy, enemy.Gunner)
	}
	s := enemy.Gunner
	if int(enemy.X-enemy.W/2+GameOriginX) <= 16 {
		enemy.Health = 0
		return true
	}
	var projectile *Enemy
	switch s.Phase {
	case 0:
		w.selectGunnerMode(enemy, s)
		x := int(enemy.X - enemy.W/2 + GameOriginX)
		y := int(enemy.Y - enemy.H/2 + GameOriginY)
		probe := d.RightProbeX
		if s.VX < 0 {
			probe = d.LeftProbeX
		}
		if w.OriginalSolidAt(x+probe, y+d.ProbeY) {
			s.Phase, s.Cooldown, s.Auto, s.Index = 2, d.RestTicks-1, false, 3
		} else {
			enemy.X += float64(s.VX)
		}
	case 2:
		enemy.X--
		s.Cooldown--
		if s.Cooldown < 0 {
			distance, sign := w.gunnerDistance(*enemy)
			w.selectGunnerMode(enemy, s)
			if distance >= d.FireMin && distance <= d.FireMax {
				s.Phase, s.ShotTimer, s.Shots, s.Sign = 1, d.InitialShotWait-1, d.BurstShotCount, sign
				s.Stream, s.Index, s.Timer, s.Auto = d.RawFire, 3, 0, true
			}
		}
	case 1:
		enemy.X--
		s.ShotTimer--
		if s.ShotTimer < 0 {
			s.ShotTimer = d.ShotPeriod - 1
			if s.Shots > 0 {
				s.Shots--
				created := w.makeGunnerArc(*enemy, s.Sign)
				projectile = &created
			} else {
				w.selectGunnerMode(enemy, s)
				s.Index = max(0, len(s.Stream)-5)
				s.Phase, s.Cooldown = 2, d.AfterBurstRestTicks-1
			}
		}
	}
	advanceGunnerArt(enemy, s)
	// Append only after all caller-pointer writes: slice growth can relocate
	// the active enemy array. New projectiles update on the next main loop.
	if projectile != nil {
		w.Enemies = append(w.Enemies, *projectile)
	}
	return true
}

func (w *World) makeGunnerArc(parent Enemy, sign int) Enemy {
	d := parent.GroundGunner
	x := int(parent.X-parent.W/2+GameOriginX) + d.ProjectileOffsetX
	y := int(parent.Y-parent.H/2+GameOriginY) + d.ProjectileOffsetY
	distanceX := int(w.Player.X) + 18 - x
	if distanceX < 0 {
		distanceX = -distanceX
		distanceX += d.DistanceLeftBonus
	}
	distanceY := int(w.Player.Y) + 25 + d.PlayerYDistanceOffset - y
	if distanceY < 0 {
		distanceY = -distanceY
	}
	xPhase, yPhase := d.PhaseXBase, d.PhaseYBase
	if d.PhaseYJitter > 0 {
		yPhase += int(w.nextRandom() % uint64(d.PhaseYJitter))
	}
	if d.PhaseXJitter > 0 {
		xPhase += int(w.nextRandom() % uint64(d.PhaseXJitter))
	}
	xPhase = max(0, xPhase-distanceX/8)
	yPhase = max(0, yPhase-distanceY/8)
	arc := Enemy{Entity: w.newEntity(float64(x)-GameOriginX+float64(d.ProjectileWidth)/2, float64(y)-GameOriginY+float64(d.ProjectileHeight)/2, 0, 0, float64(d.ProjectileWidth), float64(d.ProjectileHeight)), Behavior: BehaviorArcProjectile, GroundGunner: d, Arc: &ArcProjectileState{XPhase: xPhase, YPhase: yPhase, Sign: sign}, Health: 1, Descriptor: d.ProjectileDescriptor, Sprite: d.ProjectileSprite, AnimationTicks: d.ProjectileFrameTicks, AnimationOverride: true}
	for _, frame := range d.ProjectileCollision {
		arc.CollisionFrames = append(arc.CollisionFrames, CollisionFrame{X: frame.X, Y: frame.Y, Width: frame.Width, Height: frame.Height})
	}
	return arc
}

func (w *World) updateArcProjectile(enemy *Enemy) bool {
	if enemy.Behavior != BehaviorArcProjectile || enemy.GroundGunner == nil || enemy.Arc == nil {
		return false
	}
	d, s := enemy.GroundGunner, enemy.Arc
	if s.XPhase < 0 || s.YPhase < 0 || s.XPhase >= len(d.ProjectileX) || s.YPhase >= len(d.ProjectileY) || d.ProjectileX[s.XPhase] == 127 || d.ProjectileY[s.YPhase] == 127 {
		enemy.Health = 0
		w.finishArcProjectile(*enemy)
		return true
	}
	vertical := s.YPhase
	xMove := int(d.ProjectileX[s.XPhase])
	if xMove == 0 {
		xMove = 1
	}
	enemy.X += float64(s.Sign * xMove * 2)
	enemy.Y -= float64(d.ProjectileY[s.YPhase])
	s.XPhase++
	s.YPhase++
	x, y := int(enemy.X-enemy.W/2+GameOriginX), int(enemy.Y-enemy.H/2+GameOriginY)
	if x < 32 || x > 368 || y <= 16 || y > 224 {
		enemy.Health = 0
		return true
	}
	frames := d.ProjectileFramesRight
	if s.Sign < 0 {
		frames = d.ProjectileFramesLeft
	}
	if vertical < len(frames) {
		enemy.AnimationFrame = frames[vertical]
	}
	if w.OriginalSolidAt(x+8, y+6) {
		enemy.Health = 0
		w.finishArcProjectile(*enemy)
	}
	return true
}

// finishArcProjectile resolves common death family one, matching original
// descriptor 0x4972 and sound eleven instead of a generic explosion effect.
func (w *World) finishArcProjectile(enemy Enemy) {
	d := enemy.GroundGunner
	if d == nil {
		return
	}
	x, y := enemy.X-enemy.W/2, enemy.Y-enemy.H/2
	w.Effects = append(w.Effects, Effect{ActorOwner: enemy.ID, Priority: enemy.Priority, Entity: w.newEntity(x+float64(d.SparkWidth)/2, y+float64(d.SparkHeight)/2, 0, 0, float64(d.SparkWidth), float64(d.SparkHeight)), Lifetime: d.SparkLifetime, Sprite: d.SparkSprite, Animation: d.SparkAnimation, FrameTicks: d.SparkFrameTicks})
	w.Events = append(w.Events, Event{Kind: EventSound, Value: d.SparkSound, X: enemy.X, Y: enemy.Y})
}
