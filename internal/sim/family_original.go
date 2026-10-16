package sim

import "fmt"

// updateTerrainStamp translates the stage-four 0x4B418 update. The original
// terrain probe runs before movement and makes tile nineteen destructible.
func (w *World) updateTerrainStamp(enemy *Enemy) bool {
	if enemy.Behavior != BehaviorTerrainStamp {
		return false
	}
	rawX := int(enemy.X - enemy.W/2 + GameOriginX)
	rawY := int(enemy.Y - enemy.H/2 + GameOriginY)
	probeX, probeY := rawX+enemy.StampProbeX, rawY+enemy.StampProbeY
	if probeY > 32 && probeY < 223 {
		column, row := probeX+int(w.Scroll)-48, probeY+int(w.ScrollY)-32
		if column >= 0 && row >= 0 {
			column /= 16
			row /= 16
			if w.RawTileAt(column, row)&0x8000 == 0 {
				w.SetTileAt(column, row, enemy.StampWord)
			}
		}
	}
	if len(enemy.MotionX) > 0 && len(enemy.MotionY) > 0 {
		phase := enemy.MotionTick
		if phase >= len(enemy.MotionX) {
			phase = len(enemy.MotionX) - 1
		}
		if phase >= len(enemy.MotionY) {
			phase = len(enemy.MotionY) - 1
		}
		enemy.X += float64(enemy.MotionX[phase])
		enemy.Y += float64(enemy.MotionY[phase])
		if phase < len(enemy.MotionAnimation) {
			enemy.AnimationOverride = true
			enemy.AnimationFrame = enemy.MotionAnimation[phase]
		}
		if enemy.MotionTick+1 < len(enemy.MotionX) && enemy.MotionTick+1 < len(enemy.MotionY) {
			enemy.MotionTick++
		}
		if enemy.VX != 0 {
			enemy.X--
		}
	} else {
		enemy.X += enemy.VX
		enemy.Y += enemy.VY
	}
	// The original deletion test is after stamping and movement (0x4B450).
	rawX = int(enemy.X - enemy.W/2 + GameOriginX)
	rawY = int(enemy.Y - enemy.H/2 + GameOriginY)
	if rawX < 0 || rawX > 368 || rawY < 0 || rawY >= 224 {
		enemy.Health = 0
	}
	return true
}

// updateGroundHopper translates the walking, facing, jumping and falling
// branches shared by four original stages. Its jump curve is extracted data.
func (w *World) updateGroundHopper(enemy *Enemy) bool {
	d := enemy.Hopper
	if enemy.Behavior != BehaviorGroundHopper || d == nil {
		return false
	}
	rawX := int(enemy.X - enemy.W/2 + GameOriginX)
	rawY := int(enemy.Y - enemy.H/2 + GameOriginY)
	if rawX < d.MinX || rawX > d.MaxX || rawY < 0 || rawY >= d.MaxY {
		enemy.Health = 0
		return true
	}
	standing := func() {
		enemy.Animation = d.StandingRight
		if d.WalkVX < 0 {
			enemy.Animation = d.StandingLeft
		}
		enemy.AnimationOverride = false
		enemy.AnimationStart = 0
		enemy.Age = 1
	}
	facing := func() {
		enemy.BehaviorTicks = max(0, d.FacingPeriod-1)
		oldFrame := enemy.Frame()
		enemy.AnimationOverride = true
		enemy.TargetX = d.FacingRightFrame
		if int(w.Player.X)+18 < int(enemy.X-enemy.W/2+GameOriginX) {
			enemy.TargetX = d.FacingLeftFrame
		}
		// Facing changes the stream index, but preserves its running timer.
		enemy.AnimationFrame = oldFrame
		if max(0, enemy.Age-1)%max(1, enemy.AnimationTicks) == 0 {
			enemy.AnimationFrame = enemy.TargetX
		}
	}
	fall := func() {
		enemy.X += float64(d.FallVX)
		x := int(enemy.X - enemy.W/2 + GameOriginX)
		y := int(enemy.Y - enemy.H/2 + GameOriginY)
		if w.OriginalSolidAt(x+d.LandingProbeX, y+d.LandingProbeY) {
			enemy.BehaviorPhase = 0
			standing()
		} else {
			enemy.Y += float64(d.FallVY)
		}
	}
	switch enemy.BehaviorPhase {
	case -1:
		enemy.X--
		enemy.BehaviorTicks--
		if enemy.BehaviorTicks < 0 {
			facing()
		}
		if max(0, enemy.Age-1)%max(1, enemy.AnimationTicks) == 0 {
			enemy.AnimationFrame = enemy.TargetX
		}
	case 0:
		probeX := d.ForwardProbeRightX
		if d.WalkVX < 0 {
			probeX = d.ForwardProbeLeftX
		}
		if w.OriginalSolidAt(rawX+probeX, rawY+d.FootY) {
			enemy.BehaviorPhase = -1
			facing()
			break
		}
		if w.OriginalSolidAt(rawX+probeX, rawY+d.FootY+d.SupportRows*16) {
			enemy.X += float64(d.WalkVX)
			break
		}
		enemy.BehaviorPhase = 1
		enemy.MotionTick = 0
		enemy.Age = 1
		enemy.Animation = d.JumpRight
		enemy.AnimationFrame = d.JumpInitialRightFrame
		if d.WalkVX < 0 {
			enemy.Animation = d.JumpLeft
			enemy.AnimationFrame = d.JumpInitialLeftFrame
		}
		enemy.AnimationStart = 0
		enemy.AnimationOverride = true
	case 1:
		if enemy.AnimationOverride && enemy.Age > max(1, enemy.AnimationTicks) {
			enemy.AnimationOverride = false
			enemy.Age = 1
		}
		if enemy.MotionTick >= len(d.JumpY) {
			enemy.BehaviorPhase = 2
			fall()
			break
		}
		enemy.X += float64(d.JumpVX)
		enemy.Y += float64(d.JumpY[enemy.MotionTick])
		enemy.MotionTick++
	case 2:
		fall()
	}
	return true
}

// updateRouteCrawler uses the original four integer routes and queued turns.
// Its shield accepts damage only from the original exposed horizontal side.
func (w *World) updateRouteCrawler(enemy *Enemy) bool {
	d := enemy.RouteCrawler
	if enemy.Behavior != BehaviorRouteCrawler || d == nil || len(d.Routes) != 4 {
		return false
	}
	// The callback accepts last pass's pending hits using the old raw X,
	// before camera compensation or movement, and then clears that hit word.
	oldX := int(enemy.X - enemy.W/2 + GameOriginX)
	playerX, _ := w.originalPlayerAim()
	exposed := oldX >= playerX
	if d.FacingSide == 0 {
		exposed = oldX+d.ShieldOffset < playerX
	}
	enemy.Invincible = !exposed
	enemy.MaxHealth = max(1, d.ContactHealth)
	enemy.Health = enemy.MaxHealth
	if exposed {
		enemy.crawlerDamage += enemy.DamageTaken
	}
	enemy.DamageTaken = 0
	if enemy.crawlerDamage >= d.DamageBudget {
		enemy.Health = 0
		// Source 0x4AA3A flags this node for the common large-gray animator.
		// Its first image/sound are immediate, and no score is awarded here.
		w.bossImmediateDeath(enemy.Entity, d.DeathKind)
		return true
	}
	baseX := int(enemy.X - enemy.W/2 + GameOriginX - w.CameraDeltaX)
	baseY := int(enemy.Y - enemy.H/2 + GameOriginY - w.CameraDeltaY)
	for retry := 0; retry < 8; retry++ {
		route := d.Routes[enemy.Variant&3]
		x, y := baseX+route.DX, baseY+route.DY
		if x < d.MinX || x > d.MaxX || y < d.MinY || y > d.MaxY {
			enemy.Health = 0
			return true
		}
		px, py := x+route.ProbeX, y+route.ProbeY
		column, row := px+int(w.Scroll)-48, py+int(w.ScrollY)-32
		blocked := w.OriginalSolidAt(px, py)
		if column >= 0 && row >= 0 {
			blocked = blocked || w.RawTileAt(column/16+route.NeighborX, row/16+route.NeighborY)&0x8000 != 0
		}
		if !blocked {
			enemy.X = float64(x) - GameOriginX + enemy.W/2
			enemy.Y = float64(y) - GameOriginY + enemy.H/2
			return true
		}
		if enemy.MotionTick >= len(d.Directions) || d.Directions[enemy.MotionTick] < 0 {
			enemy.Variant = (enemy.Variant + 2) & 3
		} else {
			enemy.Variant = d.Directions[enemy.MotionTick] & 3
			enemy.MotionTick++
		}
	}
	// An enclosed route cannot move. The original level scripts avoid such a
	// trap; retain position instead of running an unbounded retry loop.
	return true
}

// updateStalker follows the original delayed group pursuit. Retargeting uses
// integer axis directions and both terrain probes use the same old position.
func (w *World) updateStalker(enemy *Enemy) bool {
	d := enemy.Stalker
	if enemy.Behavior != BehaviorStalker || d == nil {
		return false
	}
	enemy.X += float64(d.BaselineVX)
	x := int(enemy.X - enemy.W/2 + GameOriginX)
	y := int(enemy.Y - enemy.H/2 + GameOriginY)
	if x < 16 || x > 368 || y < 16 || y >= 224 {
		enemy.Health = 0
		return true
	}
	escaping := uint16(d.EscapeScroll) <= uint16(int(w.Scroll))
	enemy.BehaviorTicks--
	if escaping || enemy.BehaviorTicks < 0 {
		tx, ty := int(w.Player.X)+18, int(w.Player.Y)+25
		if escaping {
			tx = d.EscapeX
			ty = d.EscapeTopY
			if y >= d.EscapeSplitY {
				ty = d.EscapeBottomY
			}
		}
		enemy.TargetX, enemy.TargetY = tx, ty
		enemy.VX = float64(d.RightVX)
		enemy.Animation = d.RightAnimation
		if x >= tx {
			enemy.VX = float64(d.LeftVX)
			enemy.Animation = d.LeftAnimation
		}
		enemy.VY = float64(d.DownVY)
		if y >= ty {
			enemy.VY = float64(d.UpVY)
		}
		distance := x - tx
		if distance < 0 {
			distance = -distance
		}
		enemy.BehaviorTicks = distance / 2
		if d.RetargetJitter > 0 {
			enemy.BehaviorTicks += int(w.nextRandom() % uint64(d.RetargetJitter))
		}
	}
	verticalProbe := y + 32
	if enemy.VY < 0 {
		verticalProbe = y - 2
	}
	if !w.OriginalSolidAt(x+8, verticalProbe) {
		enemy.Y += enemy.VY
	}
	horizontalProbe := x + 34
	if enemy.VX < 0 {
		horizontalProbe = x - 2
	}
	if !w.OriginalSolidAt(horizontalProbe, y+8) {
		enemy.X += enemy.VX
	}
	return true
}

// initializeTilePulse preserves the source's temporary velocity coordinates
// and separate saved destination before its first actor callback.
func initializeTilePulse(enemy *Enemy) {
	if enemy.TilePulse == nil || enemy.tilePulseInitialized {
		return
	}
	enemy.TargetX = int(enemy.X-enemy.W/2) + GameOriginX
	enemy.TargetY = int(enemy.Y-enemy.H/2) + GameOriginY
	enemy.X = float64(enemy.TilePulse.StoredVX-GameOriginX) + enemy.W/2
	enemy.Y = -GameOriginY + enemy.H/2
	enemy.AnimationOverride, enemy.AnimationFrame = true, 0
	enemy.tilePulseInitialized = true
}

// updateTilePulse reproduces the original city-stage timed map replacement.
// Its ghost phase has no contact collision; activation exposes original art.
func (w *World) updateTilePulse(enemy *Enemy) bool {
	d := enemy.TilePulse
	if enemy.Behavior != BehaviorTilePulse || d == nil {
		return false
	}
	if enemy.BehaviorPhase == 0 {
		initializeTilePulse(enemy)
		enemy.TargetX += d.StoredVX
		enemy.BehaviorTicks++
		if enemy.BehaviorTicks <= d.PreEffects {
			x, y := enemy.TargetX+d.EffectXOffset, enemy.TargetY+d.EffectYOffset
			if d.EffectXJitter > 0 {
				x += int(w.nextRandom() % uint64(d.EffectXJitter))
			}
			if d.EffectYJitter > 0 {
				y += int(w.nextRandom() % uint64(d.EffectYJitter))
			}
			x += d.EffectJitterOffset
			y += d.EffectJitterOffset
			sprite := d.EffectSprite
			if sprite == "" {
				sprite = fmt.Sprintf("object-%d-%05x", w.Stage, d.EffectDescriptor)
			}
			w.Effects = append(w.Effects, Effect{Entity: w.newEntity(float64(x)-GameOriginX+float64(d.EffectWidth)/2, float64(y)-GameOriginY+float64(d.EffectHeight)/2, 0, 0, float64(d.EffectWidth), float64(d.EffectHeight)), Lifetime: d.EffectLifetime, Delay: d.EffectDelay, Sound: d.EffectSound, Sprite: sprite, FrameTicks: d.EffectAnimationTicks, Animation: d.EffectAnimation})
		}
		if enemy.BehaviorTicks < d.Delay {
			return true
		}
		for row := 0; row < d.Height; row++ {
			for column := 0; column < d.Width; column++ {
				index := row*d.Width + column
				if index < len(d.Words) {
					w.SetTileAt(d.TargetColumn+column, d.TargetRow+row, d.Words[index])
				}
			}
		}
		enemy.X = float64(enemy.TargetX) - GameOriginX + enemy.W/2
		enemy.Y = float64(enemy.TargetY) - GameOriginY + enemy.H/2
		enemy.Hidden = false
		enemy.NoCollidable = false
		enemy.Invincible = true
		enemy.BehaviorPhase = 1
		enemy.AnimationOverride = false
		enemy.Age = 1
		enemy.AnimationStart = 0
		w.Events = append(w.Events, Event{Kind: EventSound, Value: d.Sound, X: enemy.X, Y: enemy.Y})
		return true
	}
	enemy.X += float64(d.PostVX)
	if (enemy.Age-1)/max(1, enemy.AnimationTicks) >= d.StopAnimationIndex {
		enemy.Health = 0
	}
	return true
}

// updateQuarterTurner follows the original four direction routines. The six
// turning updates hold a source art frame while camera-relative drift continues.
func (w *World) updateQuarterTurner(enemy *Enemy) bool {
	d := enemy.QuarterTurner
	if enemy.Behavior != BehaviorQuarterTurner || d == nil || len(d.DirectionVX) != 4 || len(d.DirectionVY) != 4 || len(d.Animations) != 4 {
		return false
	}
	enemy.X += float64(d.BaselineVX)
	x := int(enemy.X - enemy.W/2 + GameOriginX)
	y := int(enemy.Y - enemy.H/2 + GameOriginY)
	px, py := x+d.ProbeX, y+d.ProbeY
	if w.OriginalTileAt(px, py)&(0x8000|d.ClearTileFlag) == (0x8000 | d.ClearTileFlag) {
		column, row := px+int(w.Scroll)-48, py+int(w.ScrollY)-32
		if column >= 0 && row >= 0 {
			w.SetTileAt(column/16, row/16, 0)
		}
	}
	if enemy.BehaviorPhase > 0 {
		enemy.BehaviorPhase--
		if enemy.BehaviorPhase == 0 {
			enemy.AnimationOverride = false
			enemy.Age = 1
			enemy.AnimationStart = 0
		}
		return true
	}
	if uint16(d.FreezeTurningScroll) <= uint16(int(w.Scroll)) {
		enemy.BehaviorTicks = 32767
	}
	if enemy.BehaviorTicks > 0 {
		enemy.BehaviorTicks--
	}
	playerX, playerY := w.originalPlayerAim()
	// Direction gates use unsigned word comparisons, including EE=-75 after
	// the death fade hides the ship. Preserve that wrapped target coordinate.
	targetX, targetY := uint16(playerX+d.PlayerOffsetX), uint16(playerY+d.PlayerOffsetY)
	direction := enemy.Variant & 3
	gate := false
	switch direction {
	case 0:
		gate = targetY >= uint16(y)
	case 1:
		gate = targetX <= uint16(x)
	case 2:
		gate = targetY <= uint16(y)
	case 3:
		gate = targetX >= uint16(x)
	}
	if enemy.BehaviorTicks == 0 && gate {
		next := 0
		if direction == 0 || direction == 2 {
			next = 1
			if targetX < uint16(x) {
				next = 3
			}
		} else {
			next = 2
			if targetY < uint16(y) {
				next = 0
			}
		}
		enemy.Variant = next
		enemy.BehaviorTicks = d.TurnPeriod
		enemy.BehaviorPhase = d.PauseTicks
		enemy.Animation = d.Animations[next]
		enemy.AnimationOverride = true
		frames := d.TurnFrame7
		if direction == 0 || direction == 3 {
			frames = d.TurnFrame5
		}
		if next < len(frames) {
			enemy.AnimationFrame = frames[next]
		}
	} else {
		enemy.X += float64(d.DirectionVX[direction])
		enemy.Y += float64(d.DirectionVY[direction])
	}
	// Source 0x4B56A checks retirement after every direction call, including
	// the update that starts a turn. Existing pause updates return above.
	x = int(enemy.X - enemy.W/2 + GameOriginX)
	y = int(enemy.Y - enemy.H/2 + GameOriginY)
	if x < 0 || x > d.MaxX || y < 0 || y >= d.MaxY {
		enemy.Health = 0
	}
	return true
}

// initializeSpitter consumes original spawn randomness before member delays.
func (w *World) initializeSpitter(enemy *Enemy) {
	d := enemy.Spitter
	if d == nil {
		return
	}
	if d.SpawnCount > 0 {
		y := d.SpawnYBase
		if d.SpawnYRange > 0 {
			y += int(w.nextRandom() % uint64(d.SpawnYRange))
		}
		enemy.Y = float64(y) - GameOriginY + enemy.H/2
		enemy.AnimationStart = enemy.Variant
	}
	if d.InitialCooldownRange > 0 {
		enemy.BehaviorTicks = int(w.nextRandom() % uint64(d.InitialCooldownRange))
	}
	enemy.BehaviorPhase = 1
}

// updateSpitter produces original invulnerable contact actors. Their flag-two
// immunity prevents the Force or friendly projectiles from intercepting them.
func (w *World) updateSpitter(enemy *Enemy) bool {
	d := enemy.Spitter
	if d == nil {
		return false
	}
	if enemy.Behavior == BehaviorSpitterProjectile {
		enemy.X += float64(d.ProjectileVX)
		if int(enemy.X-enemy.W/2+GameOriginX) <= d.ProjectileMinX {
			enemy.Health = 0
		}
		return true
	}
	if enemy.Behavior != BehaviorSpitter {
		return false
	}
	if enemy.MoveDelay > 0 {
		enemy.MoveDelay--
		return true
	}
	if enemy.BehaviorPhase == 0 {
		w.initializeSpitter(enemy)
	}
	enemy.X += float64(d.VX)
	if int(enemy.X-enemy.W/2+GameOriginX) < 0 {
		enemy.Health = 0
		return true
	}
	enemy.BehaviorTicks--
	if enemy.BehaviorTicks >= 0 {
		return true
	}
	enemy.BehaviorTicks = d.FirePeriod
	x := int(enemy.X - enemy.W/2 + GameOriginX)
	y := int(enemy.Y-enemy.H/2+GameOriginY) + d.ProjectileYBase
	if d.ProjectileYRange > 0 {
		y += int(w.nextRandom() % uint64(d.ProjectileYRange))
	}
	projectile := Enemy{Entity: w.newEntity(float64(x)-GameOriginX+float64(d.ProjectileWidth)/2, float64(y)-GameOriginY+float64(d.ProjectileHeight)/2, 0, 0, float64(d.ProjectileWidth), float64(d.ProjectileHeight)), Behavior: BehaviorSpitterProjectile, Spitter: d, Health: 1, Invincible: true, NoFriendlyTarget: true, Descriptor: d.ProjectileDescriptor, Sprite: d.ProjectileSprite, Animation: d.ProjectileAnimation, AnimationTicks: d.ProjectileAnimationTicks}
	for _, frame := range d.ProjectileCollision {
		projectile.CollisionFrames = append(projectile.CollisionFrames, CollisionFrame{X: frame.X, Y: frame.Y, Width: frame.Width, Height: frame.Height})
	}
	w.Enemies = append(w.Enemies, projectile)
	return true
}

// updateWallCrawler retains the original five city-stage wall-contact walkers.
func (w *World) updateWallCrawler(enemy *Enemy) bool {
	d := enemy.WallCrawler
	if enemy.Behavior != BehaviorWallCrawler || d == nil {
		return false
	}
	enemy.X += float64(d.VX)
	x := int(enemy.X - enemy.W/2 + GameOriginX)
	y := int(enemy.Y - enemy.H/2 + GameOriginY)
	if w.OriginalSolidAt(x+d.ProbeX, y+d.ProbeY) {
		enemy.Y += float64(d.VY)
	} else {
		enemy.X += float64(d.UnsupportedVX)
		if int(enemy.X-enemy.W/2+GameOriginX) <= d.MinX {
			enemy.Health = 0
		}
	}
	return true
}
