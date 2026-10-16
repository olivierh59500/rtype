package sim

import "fmt"

// NewWorld creates a title screen with the requested original random state.
// Zero is a valid initial state and triggers the original beam-sampling branch.
func NewWorld(seed uint64) *World {
	w := &World{seed: seed, randomState: seed, Stages: DefaultStages(), MainTickDivider: 2, InitialLives: 3}
	w.Restart()
	w.State = Title
	return w
}

// Restart starts a new run, preserving stage data and the session high score.
// The original signed spawn counter permits three ships; its HUD shows two
// spare icons while the first ship is active.
// Extracted initial lives and extra-life thresholds are preserved between runs.
func (w *World) Restart() {
	w.HoldStageAdvance = false
	w.StageScore, w.StageResult = 0, nil
	w.RecordEntry = nil
	w.RecordTimer = 0
	w.State = Playing
	w.Tick = 0
	w.Stage, w.Loop, w.StageTick, w.Checkpoint = 1, 0, 0, 0
	w.Scroll, w.ScrollY = 0, 0
	if w.InitialLives < 1 {
		w.InitialLives = 3
	}
	w.Score, w.Lives, w.DisplayLives = 0, w.InitialLives, max(0, w.InitialLives-1)
	w.extraLifeCursor, w.lastLifeScore = 0, 0
	w.nextID = 0
	w.previous = Input{}
	w.gamePrevious = Input{}
	w.pendingForce = false
	w.resetCheckpoint()
}

// StartStage starts a new development run at an original stage, initializing
// its own tile bank, checkpoint, camera route and boss controller.
func (w *World) StartStage(stage int) error {
	if stage < 1 || stage > len(w.Stages) {
		return fmt.Errorf("stage %d is outside the loaded game", stage)
	}
	w.Restart()
	w.Stage = stage
	w.Checkpoint = 0
	w.resetCheckpoint()
	return nil
}

// CancelInput drops remembered button levels, including an unfinished charge.
// Hosts call this when focus, touch capture, or Android
// lifecycle changes cancel an input gesture. It never advances either clock.
func (w *World) CancelInput() {
	w.previous, w.gamePrevious = Input{}, Input{}
	w.pendingForce = false
	w.Player.Charge = 0
}

// Step advances at most one tick. It never reads a clock, sleeps, or renders.
// Pause, title and terminal screens consume input without advancing gameplay.
func (w *World) Step(input Input) {
	w.actorPassComplete = false
	w.Events = w.Events[:0]
	defer func() { w.previous = input }()
	if w.stepScoreFlow(input) {
		return
	}
	startPressed := input.Start && !w.previous.Start
	pausePressed := input.Pause && !w.previous.Pause
	if w.Menu != nil {
		if w.State == Title {
			w.updateOriginalMenu()
		} else {
			w.Menu.wasTitle = false
		}
	}
	if w.State == Title || w.State == GameOver || w.State == Complete {
		if startPressed || input.Fire && !w.previous.Fire {
			fromTitle := w.State == Title
			w.Restart()
			if fromTitle {
				w.BeginInitialPlayerIntro()
			}
		}
		return
	}
	if pausePressed {
		if w.State == Playing {
			w.State = Paused
		} else if w.State == Paused {
			w.State = Playing
		}
		return
	}
	if w.State == Paused {
		return
	}
	w.Tick++
	w.advanceFinalPowersPAL()
	w.advanceInactiveOriginalForce()
	divider := w.MainTickDivider
	if divider < 1 {
		divider = 2
	}
	mainTick := w.Tick%uint64(divider) == 0
	nativeScenes := w.sceneRules() != nil
	// Core 0x364 gives an existing death request priority over events/Fire.
	// During the collision pass, the control and active flags remain set.
	combatAtEntry := w.ControlsEnabled() && !w.Scene.DeathRequested
	waitingForStageExit := w.Boss.Exiting && !w.Ending()
	palMotionDone := false
	if nativeScenes && w.Scene.Phase != SceneBlackout {
		// The source PAL input handler runs before the following scene/main
		// work. A newly completed fade becomes visible to the next input IRQ.
		if w.Ending() {
			input.Fire = false
			if !w.Scene.DeathRequested {
				input.Force = false
			}
			w.pendingForce = false
		}
		if w.Player.Alive && !w.Scene.DeathRequested {
			w.updatePlayerMovement(input)
		}
		palMotionDone = true
	}
	if nativeScenes && mainTick && w.Scene.DeathRequested && !w.Scene.DeathInitialized && w.Player.Alive && w.Force.Mode != ForceNone {
		// COPER releases the main wait, then runs the active Force before
		// returning to the pending death branch. That branch removes the
		// ship before the usual active path below can run this update.
		w.updateForce(input, input.Force && !w.previous.Force)
	}
	if nativeScenes && !w.advanceOriginalScene(mainTick) {
		return
	}
	if mainTick && !w.Boss.ReadyToAdvance && !waitingForStageExit {
		if w.weaponTables() != nil {
			w.advanceEffectsForPass(true)
		} else {
			w.advanceEffects()
		}
	}
	if !w.Player.Alive && !nativeScenes {
		w.Player.DeathTicks--
		if w.Player.DeathTicks <= 0 {
			if w.Lives <= 0 {
				w.State = GameOver
			} else {
				w.resetCheckpoint()
			}
		}
		return
	}
	if w.Player.Invulnerable > 0 {
		w.Player.Invulnerable--
	}
	// Core 0x219A polls movement on PAL interrupts. Core 0x268A clears the
	// game-main-loop wait flag once every two interrupts in its default mode.
	if w.Ending() {
		input.Fire = false
		if !w.Scene.DeathRequested {
			input.Force = false
		}
		w.pendingForce = false
	}
	if w.Player.Alive && !w.Scene.DeathRequested && !palMotionDone {
		w.updatePlayerMovement(input)
	}
	if w.State != Playing {
		return
	}
	// Core0x42C waits for the completion-jingle flag after the PAL ship
	// reaches its exit target. Its actor/map/event main-loop pass has ended.
	if w.Boss.ReadyToAdvance {
		if w.weaponTables() != nil && w.Player.Alive {
			w.updateOriginalBits()
		}
		if w.StageResult != nil {
			w.advanceStageResultPAL()
		} else if mainTick {
			w.beginStageResult()
		}
		if (w.StageResult != nil && w.StageResult.Ready || w.StageResult == nil && mainTick) && !w.HoldStageAdvance {
			w.advanceStage()
		}
		return
	}
	if w.weaponTables() != nil && w.Player.Alive {
		w.updateForce(input, input.Force && !w.previous.Force)
		w.updateOriginalBits()
	}
	if waitingForStageExit {
		// After 0x7AA installs 0x42C, its unready branch goes directly to
		// 0x3CA. PAL ship/Force/bit motion continues, while the camera,
		// actors, weapons, map hooks and victory counters remain frozen.
		return
	}
	if combatAtEntry {
		w.pendingForce = w.pendingForce || (input.Force && !w.previous.Force)
	} else {
		w.pendingForce = false
	}
	if !mainTick {
		return
	}
	// Core 0x364 gives death priority, then services an existing F9 exit
	// request at 0x372 before the event,
	// camera and actor passes. A request raised during this pass waits for
	// the next main entry; PAL ship movement remains independent.
	if w.Boss.Clearing && w.Player.Alive {
		w.updateBossVictory(w.Stages[w.Stage-1].Boss)
		combatAtEntry = false
	}
	gameInput := input
	gameInput.Force = gameInput.Force || w.pendingForce
	forcePressed := w.pendingForce
	w.pendingForce = false
	defer func() {
		if combatAtEntry {
			w.gamePrevious = gameInput
		} else if !nativeScenes {
			w.gamePrevious = Input{}
		}
	}()
	if w.missileCooldown > 0 {
		w.missileCooldown--
	}
	if w.forceCooldown > 0 {
		w.forceCooldown--
	}
	if combatAtEntry {
		w.updatePlayerWeapons(gameInput)
		if w.weaponTables() == nil {
			w.updateForce(gameInput, forcePressed)
		}
		w.fireDetachedForce(gameInput)
		if nativeScenes {
			w.EventTick++
		}
		w.spawnWaves()
	}
	w.spawnBossIfDue()
	if len(w.Stages[w.Stage-1].ScrollScript) > 0 {
		w.advanceCamera()
	}
	if w.weaponTables() != nil {
		w.captureOriginalTerrain()
		w.updateOriginalWeaponsPreActors()
	}
	if w.weaponTables() != nil {
		w.updateOriginalActorPass()
		w.updateBullets()
		w.collide()
	} else {
		w.updateBossActors()
		w.updateEnemies()
		w.updateBullets()
		w.updatePowerUps()
		if w.ControlsEnabled() {
			w.collide()
		} else {
			w.updateOriginalAutomaticFire()
			w.updateOriginalHostileShots()
			w.compactEntities()
		}
	}
	if w.weaponTables() != nil {
		weaponInput := gameInput
		if !combatAtEntry {
			weaponInput.Fire = false
		}
		w.updateOriginalWeaponsPostActors(weaponInput)
		// Core 0x3BA probes the ship's terrain point after hostile shots and
		// missiles, before the stage-specific map hook.
		w.collideOriginalPlayerTerrain()
	}
	w.updateBossMapHook()
	if w.weaponTables() != nil {
		w.syncOriginalActors()
	}
	w.updateExtraLives()
	if !w.Player.Alive && !nativeScenes {
		return
	}
	stage := w.Stages[w.Stage-1]
	if w.Boss.ReadyToAdvance {
		w.advanceStage()
		return
	}
	w.StageTick++
	if w.StageTick < stage.LengthTicks && len(stage.ScrollScript) == 0 {
		w.Scroll += stage.ScrollSpeed
	}
	for index, checkpoint := range stage.Checkpoints {
		if nativeScenes {
			break
		}
		if checkpoint.Tick <= w.StageTick {
			w.Checkpoint = index
		}
	}
	if stage.Boss == nil && w.StageTick >= stage.LengthTicks && !w.hasBoss() {
		w.stageClearTicks++
		if w.stageClearTicks >= TickRate {
			w.advanceStage()
		}
	}
}

func (w *World) updatePlayerMovement(input Input) {
	p := &w.Player
	p.Age++
	if w.updateOriginalPlayerIntro() {
		return
	}
	if w.Boss.Exiting {
		w.updateBossExit()
		return
	}
	if w.updateOriginalPlayerMovement(input) {
		return
	}
	dx, dy := 0.0, 0.0
	if input.Left {
		dx--
	}
	if input.Right {
		dx++
	}
	if input.Up {
		dy--
	}
	if input.Down {
		dy++
	}
	// The Amiga joystick supplies independent directions. Diagonal movement is
	// intentionally not normalized.
	// Core 0x4A6 initializes speed to two; 0x17D6 increases it up to five.
	speed := 2 + float64(p.SpeedLevel)
	p.VX, p.VY = dx*speed, dy*speed
	p.X = clamp(p.X+p.VX, p.W/2, Width-p.W/2)
	p.Y = clamp(p.Y+p.VY, p.H/2, PlayHeight-p.H/2)
}

func (w *World) updatePlayerWeapons(input Input) {
	if w.Boss.VictoryInitialized {
		return
	}
	p := &w.Player
	if input.Fire {
		if !w.gamePrevious.Fire {
			// Source 0x1874 reads CIA Fire on the main pass. A press or release
			// entirely between two polls does not change its charge latch.
			w.fireNormal()
		} else if p.Charge < MaxCharge {
			p.Charge++
		}
	} else if p.Charge > 0 {
		if p.Charge > 6 {
			w.fireBeam(p.Charge)
		}
		p.Charge = 0
	}
}

func (w *World) updateForce(input Input, forcePressed bool) {
	w.updateOriginalForce(input, forcePressed)
}

func (w *World) updateForceProvisional(input Input, forcePressed bool) {
	f, p := &w.Force, &w.Player
	if f.Mode == ForceNone {
		return
	}
	f.Age++
	if input.Force && (forcePressed || !w.gamePrevious.Force) {
		switch f.Mode {
		case ForceFront:
			f.Mode = ForceFree
			f.VX = 5
			f.VY = 0
		case ForceBack:
			f.Mode = ForceFree
			f.VX = -5
			f.VY = 0
		case ForceFree, ForceReturning:
			f.Mode = ForceReturning
		}
		w.Events = append(w.Events, Event{Kind: EventForce, X: f.X, Y: f.Y})
	}
	switch f.Mode {
	case ForceFront:
		f.X, f.Y = p.X+23, p.Y
	case ForceBack:
		f.X, f.Y = p.X-23, p.Y
	case ForceFree:
		f.X = clamp(f.X+f.VX, 10, Width-10)
		f.VX *= 0.94
		// A detached Force trails vertical ship movement; both the lag and launch
		// decay are provisional values to be calibrated from emulator captures.
		f.Y += clamp(p.Y-f.Y, -1.2, 1.2)
		if f.Bounds().Overlaps(p.Bounds()) {
			w.attachForce()
		}
	case ForceReturning:
		f.X += clamp(p.X-f.X, -3, 3)
		f.Y += clamp(p.Y-f.Y, -2.5, 2.5)
		if f.Bounds().Overlaps(p.Bounds()) {
			w.attachForce()
		}
	}
}

func (w *World) attachForce() {
	if w.Force.X < w.Player.X {
		w.Force.Mode = ForceBack
		w.Force.X = w.Player.X - 23
	} else {
		w.Force.Mode = ForceFront
		w.Force.X = w.Player.X + 23
	}
	w.Force.Y = w.Player.Y
	w.Force.VX, w.Force.VY = 0, 0
}

func (w *World) updateEnemies() {
	// Native actor updates can allocate children. A value copy survives slice
	// growth, and the entry snapshot defers those children until the next tick.
	count := len(w.Enemies)
	for index := 0; index < count; index++ {
		enemy := w.Enemies[index]
		w.updateEnemy(&enemy)
		enemy.automaticFireEligible = enemy.Health > 0
		w.Enemies[index] = enemy
	}
}

func (w *World) updateEnemy(e *Enemy) {
	if e.Health <= 0 && e.Descriptor != 0 {
		return
	}
	if e.bossRole != bossNone {
		return
	}
	e.Age++
	// Core 0x123C consumes the pending mask after drawing. The previous
	// collision's flag affects this update's art, before the new hurt gate.
	e.MaskFlash = e.maskFlashPending
	e.maskFlashPending = false
	e.OriginalCollisionPaused = e.Descriptor != 0 && e.ContactCooldown > 0
	if !w.actorPassRunning && e.ContactCooldown > 0 && w.ControlsEnabled() {
		e.ContactCooldown--
		if e.Descriptor != 0 && e.ContactCooldown&1 != 0 {
			e.maskFlashPending = true
			w.Events = append(w.Events, Event{Kind: EventSound, Value: 6, X: e.X, Y: e.Y})
		}
	}
	if w.updateRecoveredBehavior(e) {
		return
	}
	if e.MoveDelay > 0 {
		if e.DriftDuringDelay {
			e.X += e.VX
			e.Y += e.VY
		}
		e.MoveDelay--
		return
	}
	if len(e.MotionX) > 0 || len(e.MotionY) > 0 {
		age := e.MotionTick
		e.MotionTick++
		e.X += e.VX
		e.Y += e.VY
		if len(e.MotionX) > 0 {
			e.X += float64(e.MotionX[motionIndex(age, len(e.MotionX), e.MotionLoop)])
		}
		if len(e.MotionY) > 0 {
			e.Y += float64(e.MotionY[motionIndex(age, len(e.MotionY), e.MotionLoop)])
		}
		// The stage 1/4/7 formation routines subtract four, then retire at
		// unsigned raw X <= 32 before drawing, contacts and automatic fire.
		if e.Behavior == BehaviorSineFormation && uint16(int(e.X-e.W/2)+GameOriginX) <= 32 {
			e.Health = 0
		}
		return
	}
	switch e.Kind {
	case EnemyFighter, EnemyDrifter:
		e.X += e.VX
		e.Y += e.VY
		if e.Pattern != 0 {
			// A bounded triangle path avoids platform-dependent trigonometric
			// approximations in deterministic replay collision decisions.
			phase := float64(e.Age % 100)
			if phase > 50 {
				phase = 100 - phase
			}
			e.Y = e.BaseY + (phase-25)*0.55*float64(e.Pattern)
		}
		if e.Descriptor == 0 && e.Age%100 == 0 && e.X < Width-16 {
			w.enemyAimedShot(*e, 1.8)
		}
	case EnemyTurret:
		e.X += e.VX
		if e.Descriptor == 0 && e.Age%80 == 0 && e.X < Width {
			w.enemyAimedShot(*e, 1.6)
		}
	case EnemyCarrier:
		e.X += e.VX
		e.Y += e.VY
	case EnemyBoss:
		if e.X > Width-62 {
			e.X += e.VX
		}
		phase := float64(e.Age % 200)
		if phase > 100 {
			phase = 200 - phase
		}
		e.Y = e.BaseY + (phase-50)*0.38
		if e.Descriptor == 0 && e.Age%45 == 0 {
			w.enemyAimedShot(*e, 2)
			if e.Pattern > 1 {
				for _, vy := range []float64{-0.9, 0.9} {
					w.addBullet(EnemyShot, false, e.X-e.W/2, e.Y, -2, vy, 6, 6, 1, 200, false)
				}
			}
		}
	}
}

func (w *World) updatePowerUps() {
	for index := range w.PowerUps {
		p := &w.PowerUps[index]
		w.updatePowerUp(p)
	}
}

func (w *World) updatePowerUp(p *PowerUp) {
	p.Age++
	if !w.Boss.CameraPaused {
		p.X += p.VX
		p.Y += p.VY
	}
	if !w.Boss.CameraPaused && p.Original != nil && p.X-p.W/2+GameOriginX <= 16 {
		p.Kind = PowerNone
	}
}

func (w *World) advanceEffects() {
	w.advanceEffectsForPass(false)
}

func (w *World) advanceEffectsForPass(actorPass bool) {
	out := w.Effects[:0]
	for _, effect := range w.Effects {
		if actorPass && effect.Sprite != "" {
			if effect.Age <= effect.Lifetime {
				out = append(out, effect)
			}
		} else if w.advanceEffect(&effect) {
			out = append(out, effect)
		}
	}
	w.Effects = out
}

func (w *World) advanceEffect(effect *Effect) bool {
	if effect.Delay > 0 {
		effect.Delay--
		if effect.Delay > 0 {
			return true
		}
	}
	if effect.Age == 0 && effect.Sound > 0 {
		w.Events = append(w.Events, Event{Kind: EventSound, Value: effect.Sound, X: effect.X, Y: effect.Y})
	}
	effect.Age++
	return effect.Age < effect.Lifetime || effect.Sprite != "" && effect.Age == effect.Lifetime
}

func (w *World) resetCheckpoint() {
	w.actorList = originalActorList{}
	w.PeakOriginalActorLinks = 0
	w.ActorImages, w.ActorImagesValid = nil, false
	w.bitInitialized = false
	w.gamePrevious = Input{}
	w.pendingForce = false
	checkpoint := w.Stages[w.Stage-1].Checkpoints[w.Checkpoint]
	w.StageTick, w.Scroll = checkpoint.Tick, checkpoint.Scroll
	if w.sceneRules() != nil {
		w.StageTick = 0
		w.EventTick = 0
	}
	// Bound original profiles retain live cells except for stage four's
	// pristine copy. Manifest-only eighth-stage fixtures retain their already
	// verified policy; provisional fixtures keep their existing full reset.
	boss := w.Stages[w.Stage-1].Boss
	retainMap := w.Scene.DeathInitialized && boss != nil && boss.Eighth != nil
	reset := w.Stages[w.Stage-1].CheckpointReset
	if w.Scene.DeathInitialized && reset != nil {
		retainMap = reset.MapRestore.Retain
		// Core's point restores precede the stage header initializer.
		for _, patch := range reset.Patches {
			w.SetTileAt(patch.Column, patch.Row, patch.Word)
		}
	}
	w.resetLevelDataWithTiles(retainMap)
	w.Enemies, w.Bullets, w.PowerUps, w.Effects = nil, nil, nil, nil
	w.Player = Player{Entity: w.newEntity(87, 81, 0, 0, 32, 14), Alive: true, Invulnerable: 100}
	w.redrawLives()
	w.resetOriginalForceHardware()
	// Checkpoint core 0x608 clears the physical Force-button latch after
	// retaining hidden animation through spawn's separate power reset.
	w.forceHardware.ButtonDown = false
	w.Force = Force{}
	w.missileCooldown, w.forceCooldown, w.stageClearTicks = 0, 0, 0
	w.originalMissileCooldown = 0
	w.WeaponImages = nil
	w.resetOriginalScene()
}

func (w *World) hasBoss() bool {
	for _, enemy := range w.Enemies {
		if enemy.Kind == EnemyBoss && enemy.Health > 0 {
			return true
		}
	}
	return false
}

func (w *World) advanceStage() {
	w.actorList = originalActorList{}
	w.PeakOriginalActorLinks = 0
	w.ActorImages, w.ActorImagesValid = nil, false
	w.StageResult = nil
	w.redrawLives()
	w.Events = append(w.Events, Event{Kind: EventStage, Value: w.Stage + 1})
	if w.Stage >= len(w.Stages) {
		w.State = Complete
		return
	}
	w.Stage++
	w.StageTick, w.Checkpoint, w.stageClearTicks = 0, 0, 0
	w.Scroll, w.ScrollY = 0, 0
	w.resetLevelData()
	w.Enemies, w.Bullets, w.PowerUps, w.Effects = nil, nil, nil, nil
	w.WeaponImages = nil
	if w.sceneRules() == nil {
		w.Player.X, w.Player.Y = 87, 81
	}
	// The next stage's 0x596 clears the held/charge latch retained through
	// victory, its blocking score printer and automatic ship departure.
	w.gamePrevious = Input{}
	w.pendingForce = false
	w.Player.Charge = 0
	w.Player.Invulnerable = 100
	if w.Force.Mode != ForceNone {
		if w.sceneRules() != nil {
			w.resetOriginalScene()
			return
		}
		w.Force.Mode = ForceFront
		w.Force.X, w.Force.Y = w.Player.X+23, w.Player.Y
	}
	w.resetOriginalScene()
}

func (w *World) addScore(points int) {
	w.Score += points
	w.StageScore += points
	if w.Score > w.HighScore {
		w.HighScore = w.Score
	}
}
