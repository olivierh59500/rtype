// Package game presents the native Go simulation through Ebitengine.
package game

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/olivierh59500/r-type/internal/assets"
	"github.com/olivierh59500/r-type/internal/attract"
	"github.com/olivierh59500/r-type/internal/mediaaudio"
	"github.com/olivierh59500/r-type/internal/pilot"
	"github.com/olivierh59500/r-type/internal/sim"
	"github.com/olivierh59500/r-type/internal/sound"
	"github.com/olivierh59500/r-type/internal/source"
)

// Game owns presentation resources. World remains independent of the host.
type Game struct {
	resultPrintImages        [2]*ebiten.Image
	resultPrintPixels        [2][]byte
	recordAudio              *mediaaudio.Clock
	attract                  *attract.Controller
	attractSession           attract.Session
	display                  *source.DisplayData
	menuDisplay              *source.DisplayData
	inputReplay              *pilot.Replay
	flashImages              map[string]*ebiten.Image
	chargeImages             []*ebiten.Image
	World                    *sim.World
	bundle                   *assets.Bundle
	images                   map[string]*ebiten.Image
	imageFiles               map[string]string
	paletteImages            map[string]*ebiten.Image
	activePalette            []uint16
	sprites                  map[string]*ebiten.Image
	spriteDetails            map[string]spriteResource
	title                    *ebiten.Image
	canvas                   *ebiten.Image
	touch                    bool
	input                    controls
	frames                   int
	dataDir                  string
	storedScore              int
	storedRecordRevision     uint64
	focused                  bool
	hostFocused              bool
	titleAudio               playback
	fontChars                string
	effects                  *sound.Effects
	effectsAudio             playback
	captureError             error
	stageMusic               []*sound.StageMusic
	stageAudio               []playback
	musicStage               int
	bossMusic                bool
	bossFade                 bool
	musicJingle              int
	jingleCounts             []int
	audioCharge              int
	audioStageTick           int
	endingMusic              *sound.StageMusic
	endingAudio              playback
	endingStarted            bool
	menuMusic                *sound.StageMusic
	menuAudio                playback
	menuStarted, menuFading  bool
	stageDeath, gameOverWait bool
	// FrameLimit and CapturePath support bounded, reproducible visual checks.
	FrameLimit  int
	CapturePath string
	AutoPlay    bool
	captured    bool
}

// New reads regenerated assets from a desktop directory.
func New(assetDir string) (*Game, error) { return NewFS(os.DirFS(assetDir)) }

// SetReplay installs recorded physical button levels before the first update.
// Host focus cannot pause a bounded replay, and no simulation state is injected.
func (g *Game) SetReplay(replay *pilot.Replay) error {
	if replay == nil || g.frames != 0 || g.World.State != sim.Title {
		return fmt.Errorf("input replay must begin at the original title")
	}
	if replay.Metadata.Seed != pilot.DefaultSeed || len(replay.Frames) == 0 {
		return fmt.Errorf("input replay seed or frames do not match this game")
	}
	if err := replay.ValidateSource(source.ADFSHA256, fmt.Sprintf("%x", sha256.Sum256(g.bundle.Manifest))); err != nil {
		return err
	}
	g.inputReplay = replay
	return nil
}

// NewFS also supports resources embedded by the Android build command.
func NewFS(files fs.FS) (*Game, error) { return newFS(files, false) }

func newFS(files fs.FS, recording bool) (*Game, error) {
	bundle, err := assets.Load(files)
	if err != nil {
		return nil, err
	}
	title, err := bundle.Image("title.png")
	if err != nil {
		return nil, fmt.Errorf("original title: %w", err)
	}
	g := &Game{World: sim.NewWorld(pilot.DefaultSeed), bundle: bundle, images: make(map[string]*ebiten.Image), sprites: make(map[string]*ebiten.Image), focused: true, hostFocused: true}
	if recording {
		g.recordAudio, err = mediaaudio.New(sound.SampleRate, sim.TickRate)
		if err != nil {
			return nil, err
		}
	}
	g.title = ebiten.NewImageFromImage(title)
	g.canvas = ebiten.NewImage(sim.Width, sim.Height)
	g.input.reset()
	if err := g.loadGraphics(); err != nil {
		return nil, err
	}
	if err := g.loadAudio(files); err != nil {
		return nil, err
	}
	g.SetAttractDelay(attract.DefaultTimeoutPAL)
	return g, nil
}

func (g *Game) loadAudio(files fs.FS) error {
	mdat, err := fs.ReadFile(files, "audio/title.mdat")
	if err != nil {
		return fmt.Errorf("original title music: %w", err)
	}
	samples, err := fs.ReadFile(files, "audio/title.smpl")
	if err != nil {
		return err
	}
	periods, err := fs.ReadFile(files, "audio/title.periods")
	if err != nil {
		return err
	}
	stream, err := sound.NewTFMX(mdat, samples, periods, 0)
	if err != nil {
		return err
	}
	player, err := g.newAudioPlayer(stream, .7)
	if err != nil {
		return err
	}
	g.titleAudio = player
	manifest, err := source.ReadManifest(files)
	if err != nil {
		return err
	}
	effects := make([]sound.Effect, len(manifest.Sounds))
	for _, original := range manifest.Sounds {
		if original.Number < 0 || original.Number >= len(effects) {
			return fmt.Errorf("invalid effect number %d", original.Number)
		}
		data, err := fs.ReadFile(files, original.File)
		if err != nil {
			return err
		}
		effects[original.Number] = sound.Effect{Samples: data, Address: original.Address, Period: original.Period, Volume: original.Volume, Priority: original.Priority}
	}
	g.effects, err = sound.NewEffects(effects)
	if err != nil {
		return err
	}
	g.effectsAudio, err = g.newAudioPlayer(g.effects, .7)
	if err != nil {
		return err
	}
	g.stageMusic = make([]*sound.StageMusic, len(manifest.Levels))
	g.stageAudio = make([]playback, len(manifest.Levels))
	g.jingleCounts = make([]int, len(manifest.Levels))
	for _, level := range manifest.Levels {
		if level.Number < 1 || level.Number > len(manifest.Levels) || level.Music == nil {
			return fmt.Errorf("missing original music metadata for stage %d", level.Number)
		}
		data, err := fs.ReadFile(files, level.Music.File)
		if err != nil {
			return err
		}
		stream, err := sound.NewStageMusic(data, *level.Music)
		if err != nil {
			return fmt.Errorf("stage %d music: %w", level.Number, err)
		}
		g.stageMusic[level.Number-1] = stream
		g.jingleCounts[level.Number-1] = len(level.Music.Jingles)
		g.stageAudio[level.Number-1], err = g.newAudioPlayer(stream, .6)
		if err != nil {
			return err
		}
		if level.EndingMusic != nil {
			data, err := fs.ReadFile(files, level.EndingMusic.File)
			if err != nil {
				return err
			}
			g.endingMusic, err = sound.NewStageMusic(data, *level.EndingMusic)
			if err != nil {
				return err
			}
			g.endingAudio, err = g.newAudioPlayer(g.endingMusic, .6)
			if err != nil {
				return err
			}
		}
	}
	if manifest.Menu != nil && manifest.Menu.Music != nil {
		data, err := fs.ReadFile(files, manifest.Menu.Music.File)
		if err != nil {
			return err
		}
		g.menuMusic, err = sound.NewStageMusic(data, *manifest.Menu.Music)
		if err != nil {
			return err
		}
		g.menuAudio, err = g.newAudioPlayer(g.menuMusic, .6)
		if err != nil {
			return err
		}
	}
	return nil
}

// SetTouchEnabled adds controls beside the original game viewport.
func (g *Game) SetTouchEnabled(enabled bool) { g.touch = enabled; g.CancelInput() }

// SetDataDir selects platform-private storage for the player's high score.
func (g *Game) SetDataDir(directory string) {
	g.dataDir = directory
	data, err := os.ReadFile(filepath.Join(directory, "highscore.json"))
	if err != nil {
		return
	}
	var value struct {
		Score   int                     `json:"score"`
		Records []source.HighScoreEntry `json:"records"`
	}
	if json.Unmarshal(data, &value) == nil && value.Score >= 0 && value.Score <= 99999999 {
		g.World.HighScore = max(g.World.HighScore, value.Score)
		g.storedScore = value.Score
		if len(value.Records) == len(g.World.Records) && len(value.Records) == 10 {
			valid := true
			for index, entry := range value.Records {
				if entry.Score < 0 || entry.Score > 99999999 || len(entry.Name) > 16 || (index > 0 && entry.Score > value.Records[index-1].Score) {
					valid = false
					break
				}
			}
			if valid {
				for index, entry := range value.Records {
					g.World.Records[index].Name, g.World.Records[index].Score = entry.Name, entry.Score
				}
			}
		}
	}
}

// CancelInput prevents held keys or touches from surviving a focus change.
func (g *Game) CancelInput() {
	g.stopAttract()
	if g.attract != nil {
		g.attract.Reset()
	}
	g.input.reset()
	g.World.CancelInput()
}

// Pause is idempotent so repeated Android lifecycle callbacks cannot resume play.
func (g *Game) Pause() {
	g.stopAttract()
	if g.World.State == sim.Playing {
		g.World.State = sim.Paused
	}
	g.CancelInput()
}

// Back pauses a running game, then returns a paused game to the title.
func (g *Game) Back() {
	if g.AttractDemo() {
		g.CancelInput()
		return
	}
	switch g.World.State {
	case sim.Playing:
		g.Pause()
	case sim.Paused, sim.GameOver, sim.Complete, sim.ScoreEntry, sim.ScoreTable:
		if g.World.State == sim.ScoreEntry {
			g.World.FinishRecordEntry()
		}
		g.saveScore()
		g.World.State = sim.Title
	}
	g.CancelInput()
}

// AtTitle reports whether platform Back may leave the application.
func (g *Game) AtTitle() bool { return g.World.State == sim.Title }

// Update consumes one PAL input tick and synchronizes the original audio roles.
func (g *Game) Update() error {
	if g.captureError != nil {
		return g.captureError
	}
	if g.FrameLimit > 0 && g.frames >= g.FrameLimit && (g.CapturePath == "" || g.captured) {
		g.saveScore()
		return ebiten.Termination
	}
	focused := ebiten.IsFocused() && g.hostFocused
	// Bounded deterministic checks must advance while their window is behind
	// another development tool. Normal play still pauses on actual focus loss.
	if g.AutoPlay || g.inputReplay != nil || g.recordAudio != nil {
		focused = true
	}
	if !focused && g.focused {
		g.Pause()
	}
	g.focused = focused
	in := sim.Input{}
	if g.recordAudio == nil {
		in = g.readInput()
	}
	if g.AutoPlay {
		in = sim.Input{Start: g.frames == 0, Fire: g.frames%110 < 70, Right: g.World.Player.X < 85}
	}
	if g.inputReplay != nil {
		in = g.inputReplay.InputAt(g.frames)
	}
	in, err := g.attractInput(in, focused)
	if err != nil {
		return err
	}
	in = endingInput(g.World, in)
	g.World.HoldStageAdvance = g.World.Boss.Exiting && !g.World.Ending() && (g.musicJingle != 2 || !g.stageMusic[g.World.Stage-1].JingleDone())
	if g.gameOverWait && (g.World.Scene.Phase != sim.SceneGameOverMusic || g.World.State != sim.Playing && g.World.State != sim.Paused) {
		g.gameOverWait = false
	}
	if finishGameOverMusicWait(g.World, g.stageMusic[g.World.Stage-1], g.gameOverWait) {
		g.gameOverWait = false
	}
	g.World.Step(in)
	if g.AttractDemo() && (g.World.Stage != 1 || g.World.State != sim.Playing) {
		// Stage two and record-entry screens belong only to human play.
		g.stopAttract()
	}
	if err := g.updateMusic(focused); err != nil {
		return err
	}
	if g.titleAudio != nil {
		if g.AtTitle() && focused {
			g.titleAudio.Play()
		} else {
			g.titleAudio.Pause()
		}
	}
	if g.effectsAudio != nil {
		if g.World.State == sim.Playing && focused && g.musicJingle == 0 {
			g.effectsAudio.Play()
		} else {
			g.effectsAudio.Pause()
		}
		if g.AtTitle() || g.musicJingle != 0 {
			g.effects.Stop()
		}
		for _, event := range g.World.Events {
			// Original jingles own all four Paula channels and suppress effects.
			if g.musicJingle != 0 {
				continue
			}
			switch event.Kind {
			case sim.EventSound:
				g.effects.Play(event.Value)
			case sim.EventShot:
				g.effects.Play(0)
			case sim.EventBeam:
				g.effects.Play(1)
			case sim.EventPlayerDestroyed:
				g.effects.Play(7)
			case sim.EventPowerUp:
				g.effects.Play(8)
			case sim.EventEnemyDestroyed:
				if event.Value >= 0 {
					g.effects.Play(10 + event.Value%4)
				}
			}
		}
		if g.World.State == sim.Playing && g.World.ControlsEnabled() && g.musicJingle == 0 && g.World.Player.Charge == 6 && g.audioCharge < 6 {
			g.effects.Play(9)
		}
		if g.World.State == sim.Playing && g.World.ControlsEnabled() && g.musicJingle == 0 && g.World.Player.Charge == sim.MaxCharge && g.audioStageTick != g.World.StageTick {
			g.effects.Play(13)
		}
		g.audioCharge = g.World.Player.Charge
		g.audioStageTick = g.World.StageTick
	}
	g.frames++
	if g.World.State == sim.GameOver || g.World.State == sim.Complete || g.scoreDisplay() {
		g.saveScore()
	}
	return nil
}

func (g *Game) updateMusic(focused bool) error {
	if g.scoreDisplay() {
		for _, player := range g.stageAudio {
			player.Pause()
		}
		if g.endingAudio != nil {
			g.endingAudio.Pause()
		}
		g.musicJingle = 4
		if g.menuAudio != nil {
			if g.World.State == sim.ScoreEntry && !g.menuStarted {
				if err := g.menuMusic.StartFull(); err != nil {
					return err
				}
				g.menuStarted = true
				g.menuFading = false
			}
			if g.World.State == sim.ScoreTable && g.menuStarted && !g.menuFading {
				g.menuMusic.FadeOut()
				g.menuFading = true
			}
			if focused && g.menuStarted && !g.menuMusic.FadeDone() {
				g.menuAudio.Play()
			} else {
				g.menuAudio.Pause()
			}
		}
		return nil
	}
	if g.menuAudio != nil {
		g.menuAudio.Pause()
		if g.menuStarted {
			g.menuMusic.Stop()
		}
		g.menuStarted, g.menuFading = false, false
	}
	if g.World.Scene.Phase == sim.SceneGameOverMusic && (g.World.State == sim.Playing || g.World.State == sim.Paused) {
		var err error
		g.gameOverWait, err = syncGameOverMusic(g.World.Scene.GameOverMusicReady, g.stageAudio, g.stageMusic[g.World.Stage-1], g.stageAudio[g.World.Stage-1], g.endingAudio, g.jingleCounts[g.World.Stage-1]-1, g.gameOverWait, focused, g.World.State == sim.Playing)
		g.musicJingle = 0
		if g.gameOverWait {
			g.musicJingle = 3
		}
		return err
	}
	if g.World.Ending() && (g.World.State == sim.Playing || g.World.State == sim.Paused) {
		if !g.World.Player.Alive && !g.stageDeath {
			g.stageMusic[g.World.Stage-1].Stop()
			g.stageDeath = true
		}
		var err error
		g.endingStarted, g.musicJingle, err = syncEndingMusic(g.World.Boss.Final, g.stageAudio, g.endingMusic, g.endingAudio, g.endingStarted, focused, g.World.State == sim.Playing)
		return err
	}
	if g.AtTitle() {
		if g.endingAudio != nil {
			g.endingAudio.Pause()
			g.endingMusic.Stop()
		}
		g.endingStarted = false
		g.musicStage = 0
		g.bossMusic = false
		g.bossFade = false
		g.musicJingle = 0
		g.stageDeath = false
	}
	for index, player := range g.stageAudio {
		state := g.World.State
		if (state != sim.Playing && state != sim.GameOver && state != sim.Complete) || g.World.Stage != index+1 || !focused {
			player.Pause()
			continue
		}
		if state == sim.Playing && !g.World.Player.Alive {
			player.Pause()
			g.stageMusic[index].Stop()
			g.stageDeath = true
			g.musicJingle = 0
			g.bossMusic, g.bossFade = false, false
			continue
		}
		if state == sim.Playing && g.stageDeath && g.World.Player.Alive {
			if err := g.stageMusic[index].Start(false); err != nil {
				return err
			}
			g.stageDeath = false
			g.musicStage = g.World.Stage
			g.musicJingle = 0
			g.bossMusic, g.bossFade = false, false
		}
		if g.musicStage != g.World.Stage {
			var err error
			g.musicJingle = 0
			if index == 0 && g.World.Tick == 0 {
				err = g.stageMusic[index].PlayJingle(0)
				g.musicJingle = 1
			} else {
				err = g.stageMusic[index].Start(false)
			}
			if err != nil {
				return err
			}
			g.musicStage = g.World.Stage
			g.bossMusic = false
			g.bossFade = false
		}
		if state == sim.GameOver && g.musicJingle != 3 {
			if err := g.stageMusic[index].PlayJingle(g.jingleCounts[index] - 1); err != nil {
				return err
			}
			g.musicJingle = 3
		} else if (state == sim.Complete || state == sim.Playing && g.World.Boss.Exiting) && g.musicJingle != 2 {
			if err := g.stageMusic[index].PlayJingle(g.jingleCounts[index] - 2); err != nil {
				return err
			}
			g.musicJingle = 2
		} else if state == sim.Playing && g.musicJingle == 1 && g.stageMusic[index].JingleDone() {
			if err := g.stageMusic[index].Start(false); err != nil {
				return err
			}
			g.musicJingle = 0
		}
		if state == sim.Playing && g.musicJingle == 0 && !g.bossMusic {
			if g.World.Boss.MusicSection == 2 {
				if !g.bossFade {
					g.stageMusic[index].FadeOut()
					g.bossFade = true
				}
				if g.stageMusic[index].FadeDone() {
					if err := g.stageMusic[index].Start(true); err != nil {
						return err
					}
					g.bossMusic = true
					g.bossFade = false
				}
			} else {
				for _, enemy := range g.World.Enemies {
					if enemy.Kind == sim.EnemyBoss {
						if err := g.stageMusic[index].Start(true); err != nil {
							return err
						}
						g.bossMusic = true
						break
					}
				}
			}
		}
		player.Play()
	}
	return nil
}

func (g *Game) saveScore() {
	if g.AttractDemo() || g.dataDir == "" || (g.World.HighScore <= g.storedScore && g.World.RecordRevision == g.storedRecordRevision) {
		return
	}
	data, err := json.Marshal(struct {
		Score   int                     `json:"score"`
		Records []source.HighScoreEntry `json:"records"`
	}{g.World.HighScore, g.World.Records})
	if err != nil {
		return
	}
	if os.MkdirAll(g.dataDir, 0755) != nil {
		return
	}
	temporary, err := os.CreateTemp(g.dataDir, "highscore-*.tmp")
	if err != nil {
		return
	}
	name := temporary.Name()
	defer os.Remove(name)
	_, writeErr := temporary.Write(data)
	closeErr := temporary.Close()
	if writeErr == nil && closeErr == nil && os.Rename(name, filepath.Join(g.dataDir, "highscore.json")) == nil {
		g.storedScore = g.World.HighScore
		g.storedRecordRevision = g.World.RecordRevision
	}
}

// Layout preserves the original resolution and reserves space for touch input.
func (g *Game) Layout(_, _ int) (int, int) {
	if g.touch {
		return 480, g.displayHeight()
	}
	return sim.Width, g.displayHeight()
}

// Draw presents original graphics and optionally captures a bounded test frame.
func (g *Game) Draw(screen *ebiten.Image) {
	if g.canvas.Bounds().Dy() != g.displayHeight() {
		g.canvas = ebiten.NewImage(sim.Width, g.displayHeight())
	}
	g.canvas.Fill(color.Black)
	if g.World.State == sim.Title {
		if g.World.Menu != nil {
			g.drawMenu(g.canvas)
		} else {
			g.canvas.DrawImage(g.title, nil)
			g.text(g.canvas, "PRESS FIRE", 120, 200)
		}
	} else if g.scoreDisplay() {
		g.drawRecords(g.canvas)
	} else if g.World.StageResult != nil {
		g.drawStageResult(g.canvas)
	} else {
		g.drawWorld(g.canvas)
	}
	if g.AttractDemo() {
		g.text(g.canvas, "DEMO", 144, 8)
		if g.frames/25%2 == 0 {
			g.text(g.canvas, "PRESS FIRE", 120, 18)
		}
	}
	op := &ebiten.DrawImageOptions{}
	if g.touch {
		op.GeoM.Translate(80, 0)
	}
	screen.Fill(color.Black)
	screen.DrawImage(g.canvas, op)
	if g.touch {
		g.drawControls(screen)
	}
	if g.FrameLimit > 0 && g.frames >= g.FrameLimit && g.CapturePath != "" && !g.captured {
		// Capture the final rendered frame, rather than an uninitialized GPU image.
		bounds := screen.Bounds()
		pixels := make([]byte, bounds.Dx()*bounds.Dy()*4)
		screen.ReadPixels(pixels)
		img := &image.RGBA{Pix: pixels, Stride: bounds.Dx() * 4, Rect: image.Rect(0, 0, bounds.Dx(), bounds.Dy())}
		g.captureError = os.MkdirAll(filepath.Dir(g.CapturePath), 0755)
		if g.captureError == nil {
			file, err := os.Create(g.CapturePath)
			if err != nil {
				g.captureError = err
			} else {
				g.captureError = png.Encode(file, img)
				if err := file.Close(); g.captureError == nil {
					g.captureError = err
				}
			}
		}
		g.captured = true
	}
}
