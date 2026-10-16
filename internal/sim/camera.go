package sim

// ScrollStep is one decoded original camera command. Duration is the timer
// reload, so a command lasts Duration+1 updates. Axis phase survives changes.
type ScrollStep struct {
	Address  uint32 `json:"address"`
	XCommand int    `json:"x_command"`
	YCommand int    `json:"y_command"`
	Duration int    `json:"duration"`
	XDelta   int    `json:"x_delta"`
	YDelta   int    `json:"y_delta"`
	XPeriod  int    `json:"x_period"`
	YPeriod  int    `json:"y_period"`
	Repeat   bool   `json:"repeat"`
}

type cameraState struct {
	Cursor, Duration   int
	XCounter, YCounter int
	XDelta, YDelta     int
	XReload, YReload   int
}

// originalCameraCompensation translates 0xF2C after 0xEAE has updated the
// counters and possibly loaded another command. It is intentionally distinct
// from the actual displacement used by saved-camera actors and terrain.
func (w *World) originalCameraCompensation() (dx, dy int) {
	if w.Boss.CameraPaused {
		return 0, 0
	}
	if len(w.Stages[w.Stage-1].ScrollScript) == 0 {
		// Synthetic fixtures without an original command table explicitly
		// provide their camera displacement instead of the source counters.
		return int(w.CameraDeltaX), int(w.CameraDeltaY)
	}
	if w.camera.XCounter == 0 {
		dx = w.camera.XDelta
	}
	if w.camera.YCounter == 0 {
		dy = w.camera.YDelta
	}
	return dx, dy
}

// advanceCamera translates core 0xEAE/0xF06/0xF18. Loading a new command takes
// place after movement, and never resets either axis counter.
func (w *World) advanceCamera() {
	w.CameraDeltaX, w.CameraDeltaY = 0, 0
	if w.Boss.CameraPaused {
		return
	}
	stage := w.Stages[w.Stage-1]
	if len(stage.ScrollScript) == 0 {
		w.CameraDeltaX = stage.ScrollSpeed
		w.Scroll += w.CameraDeltaX
		return
	}
	camera := &w.camera
	axis := func(counter *int, reload, delta int) float64 {
		*counter--
		if *counter < 0 {
			*counter = reload
			return float64(delta)
		}
		return 0
	}
	w.CameraDeltaY = axis(&camera.YCounter, camera.YReload, camera.YDelta)
	w.CameraDeltaX = axis(&camera.XCounter, camera.XReload, camera.XDelta)
	w.Scroll += w.CameraDeltaX
	w.ScrollY += w.CameraDeltaY
	camera.Duration--
	if camera.Duration < 0 {
		step := stage.ScrollScript[camera.Cursor]
		camera.XDelta, camera.YDelta = step.XDelta, step.YDelta
		camera.XReload, camera.YReload = step.XPeriod-1, step.YPeriod-1
		camera.Duration = step.Duration
		if !step.Repeat && camera.Cursor+1 < len(stage.ScrollScript) {
			camera.Cursor++
		}
	}
}

func (w *World) resetLevelData() {
	w.resetLevelDataWithTiles(false)
}

// resetLevelDataWithTiles can retain the live map for a verified checkpoint
// profile while still rerunning its native initializer's authored patches.
func (w *World) resetLevelDataWithTiles(retain bool) {
	stage := w.Stages[w.Stage-1]
	if !retain || len(w.Tiles) != len(stage.TileMap) {
		w.Tiles = append(w.Tiles[:0], stage.TileMap...)
		w.TileChanges = make(map[int]uint16)
	}
	w.TerrainImageChanges, w.TerrainImageValid = nil, false
	w.ScrollY = stage.InitialScrollY
	w.camera = cameraState{}
	w.CameraDeltaX, w.CameraDeltaY = 0, 0
	w.spawnCursor = 0
	checkpoint := stage.Checkpoints[w.Checkpoint]
	if w.sceneRules() != nil && checkpoint.EventAddress != 0 {
		for w.spawnCursor < len(stage.Waves) && stage.Waves[w.spawnCursor].OriginalAddress < checkpoint.EventAddress {
			w.spawnCursor++
		}
	} else {
		for w.spawnCursor < len(stage.Waves) {
			wave := stage.Waves[w.spawnCursor]
			if wave.TriggerClock == "scroll" && float64(wave.Trigger) < w.Scroll {
				w.spawnCursor++
				continue
			}
			if wave.TriggerClock == "tick" && wave.Trigger < w.StageTick {
				w.spawnCursor++
				continue
			}
			break
		}
	}
	previousCave := w.Boss.Cave
	w.Boss = BossState{}
	if stage.Cave != nil {
		w.initializeCave(stage.Cave, previousCave, w.Scene.DeathInitialized && retain)
	}
	if stage.Boss != nil {
		w.Boss.Health = stage.Boss.ControllerHealth
		w.Boss.ArenaTicks = stage.Boss.FreezeTicks
		w.Boss.TailX = stage.Boss.TailBaseX
		if stage.Boss.Third != nil {
			w.Boss.Active = true
			w.Boss.ArenaTicks = stage.Boss.Third.ExitReload
			for _, patch := range stage.Boss.Third.InitialPatches {
				w.applyBattleshipPatch(patch)
			}
			if stage.Boss.Third.Iris != nil {
				w.battleshipIrisState(stage.Boss)
			}
		}
		if stage.Boss.Second != nil {
			data := stage.Boss.Second
			for index, word := range data.Patches[0] {
				w.SetTileAt(data.PatchColumn+index%4, data.PatchRow+index/4, word)
			}
		}
		if stage.Boss.Eighth != nil {
			w.finalBossState(stage.Boss)
		}
		if stage.Boss.Seven != nil {
			w.bossSevenState(stage.Boss)
		}
	}
	if stage.CheckpointReset != nil {
		// Gomander's two closed-creature rectangles follow its organ
		// restore, both at initial entry and at a death checkpoint.
		for _, patch := range stage.CheckpointReset.MapRestore.ExtraPatches {
			w.SetTileAt(patch.Column, patch.Row, patch.Word)
		}
	}
}
