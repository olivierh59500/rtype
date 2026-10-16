package sim

import "github.com/olivierh59500/r-type/internal/source"

func (w *World) spawnBossTwoComponents(spec *BossSpec) {
	data := spec.Second
	if data == nil || w.Boss.Active || int(w.Scroll) < data.Trigger {
		return
	}
	w.Boss.Active = true
	for component, record := range data.Components {
		enemy := w.newBossActor(record.Descriptor, spec.SecondArt[component], 352, 60, bossTwoPart)
		enemy.bossSegment = component
		enemy.Priority = len(data.Components) - 1 - component
		enemy.bossDelay = component * data.DelayStep
		if component < len(data.DeathDelays) {
			enemy.bossDeathDelay = data.DeathDelays[component]
		}
		enemy.Animation = record.Animation
		enemy.AnimationTicks = max(1, record.AnimationTicks)
		enemy.AnimationOverride = true
		if len(record.Animation) > 0 {
			enemy.AnimationFrame = record.Animation[0]
		}
		w.Enemies = append(w.Enemies, enemy)
	}
}

// updateBossTwoActors translates 0x4AA08. Component priority reverses allocation
// order, putting the leader last so its shared-route change follows all others.
func (w *World) updateBossTwoActors(spec *BossSpec) {
	w.updateCaveActors()
	if spec.Second == nil {
		return
	}
	if w.actorUpdateID == 0 || w.actorUpdateID == w.Boss.ControllerID {
		w.Boss.HitFlash = false
	}
	for index := range w.Enemies {
		enemy := &w.Enemies[index]
		if enemy.bossRole != bossController || enemy.Health <= 0 || !w.selectedActor(enemy) {
			continue
		}
		enemy.Age++
		if !w.actorPassRunning && enemy.ContactCooldown > 0 {
			enemy.ContactCooldown--
		}
		if enemy.DamageTaken != w.Boss.DamageTaken {
			w.Boss.HitFlash = true
			w.Boss.DamageTaken = enemy.DamageTaken
			w.Boss.Health = spec.ControllerHealth - enemy.DamageTaken
			w.Boss.OrganWait = spec.Second.ClosedWait
			enemy.ContactCooldown = 3
			if enemy.DamageTaken >= spec.ControllerHealth && !w.Boss.Defeated {
				w.Boss.HitFlash = false
				w.Boss.Defeated = true
				w.Boss.Health = 0
				w.addScore(spec.Score)
				w.startBossVictory(spec)
				w.Events = append(w.Events, Event{Kind: EventBossDefeated, Value: w.Stage})
			}
		}
		enemy.AnimationOverride = true
		enemy.AnimationFrame = w.Boss.OrganFrame
	}
	for component := len(spec.Second.Components) - 1; component >= 0; component-- {
		for index := range w.Enemies {
			enemy := &w.Enemies[index]
			if enemy.bossRole != bossTwoPart || enemy.bossSegment != component || enemy.Health <= 0 || !w.selectedActor(enemy) {
				continue
			}
			enemy.Age++
			w.updateBossTwoComponent(enemy, spec)
			if enemy.Health > 0 {
				w.animateBossTwoComponent(enemy, spec)
			}
		}
	}
}

func (w *World) updateBossTwoComponent(enemy *Enemy, spec *BossSpec) {
	data := spec.Second
	if w.Boss.Defeated {
		enemy.bossDeathDelay--
		if enemy.bossDeathDelay < 0 {
			enemy.Health = 0
			w.bossImmediateDeath(enemy.Entity, 0)
			return
		}
	}
	if enemy.bossAdvance {
		enemy.bossAdvance = false
		w.Boss.SnakeRoute++
		if w.Boss.SnakeRoute >= len(data.Routes) {
			w.Boss.SnakeRoute = 1
			w.startBossVictory(spec)
		}
	}
	if enemy.bossDelay >= 0 {
		enemy.bossDelay--
		return
	}
	if !enemy.bossInitialized {
		enemy.bossRoute = w.Boss.SnakeRoute
		route := data.Routes[enemy.bossRoute]
		enemy.X = float64(route.StartX) - GameOriginX + enemy.W/2
		enemy.Y = float64(route.StartY) - GameOriginY + enemy.H/2
		enemy.MotionTick = 0
		enemy.DamageTaken = 0
		enemy.bossInitialized = true
		enemy.Hidden, enemy.NoCollidable = false, false
	}
	route := data.Routes[enemy.bossRoute]
	phase := enemy.MotionTick
	enemy.X += float64(route.MotionX[phase])
	enemy.Y += float64(route.MotionY[phase])
	component := data.Components[enemy.bossSegment]
	if !component.Animated {
		enemy.bossAnimationIndex = route.Frames[phase]
	} else if enemy.DamageTaken > 0 && !enemy.bossBroken {
		enemy.Animation = data.DamagedAnimation
		enemy.AnimationStart = 0
		enemy.Age = 1
		enemy.bossBroken = true
		enemy.bossAnimationIndex, enemy.bossAnimationTimer = 0, 0
		w.addScore(data.PartHitScore)
		// Source 0x4AAA4 allocates independent priority-thirty debris while
		// keeping the damaged body alive. It animates when its node is reached.
		if effect, ok := w.originalDeathEffect(enemy.Entity, 0); ok {
			effect.Priority = 30
			w.Effects = append(w.Effects, effect)
		}
	}
	if phase+1 < len(route.MotionX) {
		enemy.MotionTick++
		return
	}
	enemy.X = -1 - GameOriginX + enemy.W/2
	enemy.Hidden, enemy.NoCollidable = true, true
	enemy.bossInitialized = false
	enemy.bossDelay = data.RouteWait
	if enemy.bossBroken {
		enemy.Animation = component.Animation
		enemy.bossBroken = false
		enemy.AnimationStart = 0
	}
	if enemy.bossSegment == 0 {
		enemy.bossAdvance = true
	}
}

// animateBossTwoComponent follows core 0x117C independently of motion waits.
// Changing streams retains the last bitmap until the existing timer expires;
// body damage explicitly clears that timer and index before its first pose.
func (w *World) animateBossTwoComponent(enemy *Enemy, spec *BossSpec) {
	enemy.bossAnimationTimer--
	if enemy.bossAnimationTimer >= 0 {
		return
	}
	enemy.bossAnimationTimer = max(1, enemy.AnimationTicks) - 1
	stream := spec.Second.Components[enemy.bossSegment].Animation
	if enemy.bossBroken {
		stream = spec.Second.DamagedAnimation
	}
	if len(stream) == 0 {
		return
	}
	index := enemy.bossAnimationIndex
	if index < 0 || index >= len(stream) {
		index = 0
	}
	enemy.AnimationFrame = stream[index]
	if spec.Second.Components[enemy.bossSegment].Animated {
		index++
	}
	enemy.bossAnimationIndex = index
}

func (w *World) updateBossTwoMap(spec *BossSpec) {
	if !w.Boss.ControllerActive && int(w.Scroll) < spec.FreezeScroll {
		w.updateCaveMap()
	}
	data := spec.Second
	if data == nil {
		return
	}
	if w.Boss.Defeated {
		y := data.ExplosionYBase + int(w.nextRandom()%uint64(max(1, data.ExplosionYRange)))
		x := data.ExplosionXBase + int(w.nextRandom()%uint64(max(1, data.ExplosionXRange)))
		if len(data.ExplosionKinds) > 0 {
			kind := data.ExplosionKinds[int(w.nextRandom()%uint64(len(data.ExplosionKinds)))]
			w.bossDeferredDeath(Entity{X: float64(x) - GameOriginX, Y: float64(y) - GameOriginY}, kind)
		}
		return
	}
	if !w.Boss.ControllerActive && int(w.Scroll) >= spec.FreezeScroll {
		w.Boss.ControllerActive = true
		w.Boss.CameraPaused = true
		w.Boss.OrganWait = data.ClosedWait
		w.Boss.OrganTimer = 0
		enemy := w.newBossActor(spec.ControllerDescriptor, spec.ControllerArt, data.ControllerX, data.ControllerY, bossController)
		enemy.Hidden = true
		enemy.AnimationOverride = true
		for _, box := range data.ControllerCollisions {
			enemy.CollisionFrames = append(enemy.CollisionFrames, CollisionFrame{X: box.X, Y: box.Y, Width: box.Width, Height: box.Height})
		}
		w.Enemies = append(w.Enemies, enemy)
		w.Boss.ControllerID = enemy.ID
		return
	}
	if w.Boss.ControllerActive && !w.Boss.Defeated {
		w.Boss.OrganTimer--
		switch w.Boss.OrganPhase {
		case 0:
			if w.Boss.OrganTimer < 0 {
				w.Boss.OrganPhase = 1
				w.Boss.OrganSteps = 3
			}
		case 1:
			if w.Boss.OrganTimer < 0 {
				if w.Boss.OrganSteps == 0 {
					w.Boss.OrganPhase = 2
					w.Boss.OrganSteps = 3
				} else {
					w.Boss.OrganTimer = data.PatchReload
					w.Boss.OrganFrame++
					w.patchBossTwoFrame(data)
					w.Boss.OrganSteps--
					if w.Boss.OrganSteps == 0 {
						w.Boss.OrganTimer = data.OpenHold
					}
				}
			}
		case 2:
			if w.Boss.OrganTimer < 0 {
				w.Boss.OrganTimer = data.PatchReload
				w.Boss.OrganFrame--
				w.patchBossTwoFrame(data)
				w.Boss.OrganSteps--
				if w.Boss.OrganSteps == 0 {
					if w.Boss.OrganWait > data.MinimumWait {
						w.Boss.OrganWait -= data.WaitReduction
					}
					w.Boss.OrganTimer = w.Boss.OrganWait
					w.Boss.OrganPhase = 0
				}
			}
		}
	}
}

func (w *World) patchBossTwoFrame(data *source.BossTwoData) {
	words := data.Patches[w.Boss.OrganFrame]
	for index, word := range words {
		w.SetTileAt(data.PatchColumn+index%4, data.PatchRow+index/4, word)
	}
}
