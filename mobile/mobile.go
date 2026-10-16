//go:build android

// Package mobile connects the shared Go game to Ebitengine's Android view.
package mobile

import (
	"errors"
	"io/fs"
	"sync/atomic"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	enginemobile "github.com/hajimehoshi/ebiten/v2/mobile"
	"github.com/olivierh59500/r-type/internal/game"
	"github.com/olivierh59500/r-type/internal/sim"
)

const (
	pauseCommand uint32 = 1 << iota
	backCommand
	cancelCommand
)

// Android callbacks publish requests; only the update thread mutates the game.
type host struct {
	visualCheck   atomic.Pointer[visualCheck]
	visualHold    bool
	game          *game.Game
	directory     atomic.Pointer[string]
	commands      atomic.Uint32
	acknowledged  atomic.Uint64
	atTitle       atomic.Bool
	windowFocused atomic.Bool
}

type visualCheck struct {
	stage, scroll, endingPhase, ticks int
	hold, records, recordTable        bool
	startup                           bool
	menuTicks, introTicks             int
}

var gameHost host

// The Android builder sets this from a generated file in this package. Keeping
// the declaration here lets a fresh checkout resolve all tracked Go imports.
var assetFiles fs.FS

func init() {
	// Preserve the original Amiga PAL update cadence on Android.
	ebiten.SetTPS(50)
	ebiten.SetScreenFilterEnabled(false)
	gameHost.atTitle.Store(true)
	enginemobile.SetGame(&gameHost)
}

// Configure enables game creation after EbitenView initializes its JNI context.
func Configure(directory string) { gameHost.directory.Store(&directory) }

// SetWindowFocused publishes Android window focus without mutating the game
// from its UI thread. Focus starts false until the Activity becomes interactive.
func SetWindowFocused(focused bool) { gameHost.windowFocused.Store(focused) }

// ConfigureVisualCheck selects an optional local development scene. The
// generated Activity invokes this only for explicitly supplied debug intents.
func ConfigureVisualCheck(stage, scroll, endingPhase, ticks int, hold, records, recordTable bool) {
	if (records || recordTable) && stage == 0 {
		stage = 1
	}
	if stage >= 1 && stage <= 8 && scroll >= 0 && endingPhase >= 0 && endingPhase <= 7 && ticks >= 0 && ticks <= 5000 {
		gameHost.visualCheck.Store(&visualCheck{stage: stage, scroll: scroll, endingPhase: endingPhase, ticks: ticks, hold: hold, records: records, recordTable: recordTable})
	}
}

// ConfigureStartupCheck selects an exact menu or cold-start PAL frame through
// normal native steps. The Activity exposes this only through debug intents.
func ConfigureStartupCheck(menuTicks, introTicks int, hold bool) {
	menu := menuTicks >= 0 && menuTicks <= 5000 && introTicks == -1
	intro := introTicks >= 0 && introTicks <= 5000 && menuTicks == -1
	if menu || intro {
		gameHost.visualCheck.Store(&visualCheck{startup: true, menuTicks: menuTicks, introTicks: introTicks, hold: hold})
	}
}

// RequestPause pauses an active game before Android suspends its render surface.
func RequestPause() {
	previous := gameHost.acknowledged.Load()
	gameHost.commands.Or(pauseCommand | cancelCommand)
	// The UI thread waits briefly for the game thread, then continues when an
	// already suspended surface cannot acknowledge the request immediately.
	deadline := time.Now().Add(250 * time.Millisecond)
	for gameHost.acknowledged.Load() == previous && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
}

// RequestBack routes Android's system Back gesture through the game's menu.
func RequestBack() { gameHost.commands.Or(backCommand | cancelCommand) }

// CancelInput releases unfinished touches after focus or surface changes.
func CancelInput() { gameHost.commands.Or(cancelCommand) }

// IsAtTitle lets the Android activity close when Back is pressed at the title.
func IsAtTitle() bool { return gameHost.atTitle.Load() }

// Dummy provides an exported binding symbol for the Android library generator.
func Dummy() {}

func (h *host) Update() error {
	if h.game == nil {
		directory := h.directory.Load()
		if directory == nil {
			return nil
		}
		if assetFiles == nil {
			return errors.New("Android assets are missing; build with cmd/buildandroid")
		}
		// Graphics and audio are created only after Android initializes the view.
		var err error
		h.game, err = game.NewFS(assetFiles)
		if err != nil {
			return err
		}
		h.game.SetTouchEnabled(true)
		if storage, ok := any(h.game).(interface{ SetDataDir(string) }); ok {
			storage.SetDataDir(*directory)
		}
		if check := h.visualCheck.Swap(nil); check != nil {
			if err = h.prepareVisualCheck(check); err != nil {
				return err
			}
			h.visualHold = check.hold
		}
	}
	h.game.SetHostFocused(h.windowFocused.Load())
	commands := h.commands.Swap(0)
	if commands&cancelCommand != 0 {
		h.game.CancelInput()
	}
	if commands&pauseCommand != 0 {
		h.game.Pause()
		h.acknowledged.Add(1)
	}
	if commands&backCommand != 0 {
		h.visualHold = false
		h.game.Back()
	}
	if h.visualHold {
		h.atTitle.Store(h.game.AtTitle())
		return nil
	}
	if err := h.game.Update(); err != nil {
		return err
	}
	h.atTitle.Store(h.game.AtTitle())
	return nil
}

func (h *host) prepareVisualCheck(check *visualCheck) error {
	w := h.game.World
	if check.startup {
		ticks := check.menuTicks
		if check.introTicks >= 0 {
			w.Step(sim.Input{Start: true})
			ticks = check.introTicks
		}
		for tick := 0; tick < ticks; tick++ {
			w.Step(sim.Input{})
		}
		return nil
	}
	if err := w.StartStage(check.stage); err != nil {
		return err
	}
	w.Player.Invulnerable = 1 << 30
	if check.scroll > 0 {
		w.Scroll = float64(check.scroll)
		stage := &w.Stages[check.stage-1]
		first := 0
		for first < len(stage.Waves) && stage.Waves[first].TriggerClock == "scroll" && stage.Waves[first].Trigger < check.scroll {
			first++
		}
		stage.Waves = stage.Waves[first:]
	}
	if check.endingPhase > 0 {
		if check.stage != 8 || w.Stages[7].Boss == nil || w.Stages[7].Boss.Eighth == nil {
			return errors.New("ending visual check requires original stage-eight metadata")
		}
		d := w.Stages[7].Boss.Eighth
		w.Stages[7].Waves = nil
		w.Scroll = float64(d.FreezeScroll)
		for tick := 0; tick < 15000 && w.Boss.Final.Phase < check.endingPhase; tick++ {
			for index := range w.Enemies {
				enemy := &w.Enemies[index]
				if enemy.Descriptor == d.BrainDescriptor {
					enemy.DamageTaken = d.DamageLimit
				}
			}
			w.Step(sim.Input{})
		}
		if w.Boss.Final.Phase != check.endingPhase {
			return errors.New("native ending did not reach the requested visual-check phase")
		}
	}
	for tick := 0; tick < check.ticks; tick++ {
		w.Step(sim.Input{})
	}
	if check.records {
		w.State = sim.ScoreEntry
		w.RecordEntry = &sim.RecordEntryState{Row: 0, Name: "test"}
		if len(w.Records) > 0 {
			w.Records[0].Name = "test"
		}
	}
	if check.recordTable {
		w.State = sim.ScoreTable
		w.RecordEntry = nil
		w.RecordTimer = 525
	}
	return nil
}

func (h *host) Draw(screen *ebiten.Image) {
	if h.game != nil {
		h.game.Draw(screen)
	}
}

func (h *host) Layout(width, height int) (int, int) {
	if h.game != nil {
		return h.game.Layout(width, height)
	}
	return 320, 208
}
