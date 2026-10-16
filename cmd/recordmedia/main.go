// Command recordmedia captures the actual Ebitengine renderer and native audio.
package main

import (
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/olivierh59500/r-type/internal/attract"
	"github.com/olivierh59500/r-type/internal/game"
	"github.com/olivierh59500/r-type/internal/sim"
	"github.com/olivierh59500/r-type/internal/sound"
)

type captureReport struct {
	FPS, Width, Height, SampleRate           int
	Frames, DemoFrames, Score, Lives, Deaths int
	DurationSeconds                          float64
	Complete, BossDefeated                   bool
	InputSHA256, AssetManifestSHA256         string
	Markers                                  map[string]float64
}

type recorder struct {
	game        *game.Game
	output      string
	video       io.Writer
	status      io.Writer
	audio       *os.File
	native      *ebiten.Image
	frame       *ebiten.Image
	pixels      []byte
	lastPixels  []byte
	zeroAudio   []byte
	pending     bool
	finished    bool
	finishFrame int
	outroFrames int
	introFrames int
	limit       int
	demoStart   int
	demoWorld   *sim.World
	wasAlive    bool
	err         error
	started     time.Time
	report      captureReport
	shots       map[string]bool
}

func main() {
	assets := flag.String("assets", "assets/runtime", "locally reconstructed original assets")
	output := flag.String("out", ".cache/media/capture", "ignored lossless capture directory")
	ffmpeg := flag.String("ffmpeg", "ffmpeg", "FFmpeg executable")
	rawStdout := flag.Bool("raw-stdout", false, "stream 320x208 RGBA frames at 50 fps to stdout instead of running FFmpeg")
	intro := flag.Int("intro-frames", 300, "original animated-menu PAL frames before the demo")
	outro := flag.Int("outro-frames", 200, "held stage-clear PAL frames after the demo")
	limit := flag.Int("limit", 0, "short development capture limit; zero records the complete stage")
	flag.Parse()
	var status io.Writer = os.Stdout
	if *rawStdout {
		status = os.Stderr
	}
	if *intro < 1 || *outro < 1 || *limit < 0 {
		log.Fatal("capture frame counts must be positive")
	}
	if err := os.MkdirAll(*output, 0755); err != nil {
		log.Fatal(err)
	}
	files := os.DirFS(*assets)
	g, err := game.NewRecordingFS(files)
	if err != nil {
		log.Fatal(err)
	}
	g.SetAttractDelay(*intro + 1)
	// Streaming keeps the actual renderer and PCM clock while allowing a
	// separate host encoder. Stdout contains only frame bytes in this mode.
	var video io.Writer = os.Stdout
	var pipe io.WriteCloser
	var encoder *exec.Cmd
	if !*rawStdout {
		videoPath := filepath.Join(*output, "native-lossless.mkv")
		encoder = exec.Command(*ffmpeg, "-hide_banner", "-loglevel", "error", "-y", "-f", "rawvideo", "-pixel_format", "rgba", "-video_size", "320x208", "-framerate", "50", "-i", "pipe:0", "-an", "-c:v", "ffv1", "-level", "3", "-threads", "2", "-pix_fmt", "bgra", videoPath)
		encoder.Stderr = os.Stderr
		pipe, err = encoder.StdinPipe()
		if err != nil {
			log.Fatal(err)
		}
		if err := encoder.Start(); err != nil {
			log.Fatal(err)
		}
		video = pipe
	}
	wav, err := os.Create(filepath.Join(*output, "native-audio.wav"))
	if err != nil {
		if encoder != nil {
			pipe.Close()
			encoder.Process.Kill()
		}
		log.Fatal(err)
	}
	if _, err := wav.Write(make([]byte, 44)); err != nil {
		log.Fatal(err)
	}
	r := &recorder{game: g, output: *output, video: video, status: status, audio: wav, frame: ebiten.NewImage(320, 208), pixels: make([]byte, 320*208*4), lastPixels: make([]byte, 320*208*4), zeroAudio: make([]byte, sound.SampleRate/sim.TickRate*4), introFrames: *intro, outroFrames: *outro, limit: *limit, demoStart: -1, started: time.Now(), shots: make(map[string]bool)}
	r.report = captureReport{FPS: sim.TickRate, Width: 320, Height: 208, SampleRate: sound.SampleRate, InputSHA256: attract.InputSHA256(), AssetManifestSHA256: attract.Metadata().AssetManifestSHA256, Markers: make(map[string]float64)}
	ebiten.SetTPS(ebiten.SyncWithFPS)
	ebiten.SetVsyncEnabled(false)
	ebiten.SetRunnableOnUnfocused(true)
	ebiten.SetScreenFilterEnabled(false)
	ebiten.SetWindowSize(640, 416)
	ebiten.SetWindowTitle("R-Type media export")
	runErr := ebiten.RunGameWithOptions(r, &ebiten.RunGameOptions{InitUnfocused: true})
	var encodeErr error
	if encoder != nil {
		pipe.Close()
		encodeErr = encoder.Wait()
	}
	if err := writeWAVHeader(wav, uint32(r.report.Frames*len(r.zeroAudio))); err != nil && runErr == nil {
		runErr = err
	}
	wav.Close()
	if runErr != nil {
		log.Fatal(runErr)
	}
	if encodeErr != nil {
		log.Fatal(encodeErr)
	}
	r.report.DurationSeconds = float64(r.report.Frames) / sim.TickRate
	if !r.finished && r.demoWorld != nil {
		r.report.Score, r.report.Lives = r.demoWorld.Score, r.demoWorld.Lives
		r.report.DemoFrames = min(attract.FrameCount(), r.report.Frames-r.demoStart)
	}
	data, err := json.MarshalIndent(r.report, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(*output, "capture.json"), append(data, '\n'), 0644); err != nil {
		log.Fatal(err)
	}
	if *limit == 0 && (!r.report.Complete || !r.report.BossDefeated || r.report.Deaths != 0 || r.report.Lives != 3 || r.report.DemoFrames != attract.FrameCount()) {
		log.Fatalf("capture did not reproduce the verified complete first-stage demo: %+v", r.report)
	}
	fmt.Fprintf(status, "Captured %d frames, %.2f seconds, score %d, ships %d, deaths %d in %s\n", r.report.Frames, r.report.DurationSeconds, r.report.Score, r.report.Lives, r.report.Deaths, *output)
}

func (r *recorder) Layout(_, _ int) (int, int) { return 320, 208 }

func (r *recorder) Update() error {
	if r.err != nil {
		return r.err
	}
	if r.pending {
		return nil
	}
	if r.limit > 0 && r.report.Frames >= r.limit || r.finished && r.report.Frames >= r.finishFrame+r.outroFrames {
		return ebiten.Termination
	}
	if !r.finished {
		wasDemo := r.game.AttractDemo()
		if err := r.game.Update(); err != nil {
			return err
		}
		if r.game.AttractDemo() {
			if !wasDemo {
				r.demoStart = r.report.Frames
				r.mark("demo_start")
				r.demoWorld = r.game.World
				r.wasAlive = true
			}
			w := r.game.World
			if r.wasAlive && !w.Player.Alive {
				r.report.Deaths++
			}
			r.wasAlive = w.Player.Alive
			if w.Boss.ControllerActive {
				r.mark("boss_start")
			}
			if w.Boss.Defeated {
				r.report.BossDefeated = true
				r.mark("boss_defeated")
			}
		} else if wasDemo {
			r.finished, r.finishFrame = true, r.report.Frames
			r.report.DemoFrames = min(attract.FrameCount(), r.report.Frames-r.demoStart+1)
			r.report.Score, r.report.Lives = r.demoWorld.Score, r.demoWorld.Lives
			r.report.Complete = r.report.BossDefeated && r.report.DemoFrames == attract.FrameCount()
			r.mark("demo_end")
		}
	}
	pcm := r.zeroAudio
	if !r.finished {
		var err error
		pcm, err = r.game.ReadRecordingAudio()
		if err != nil {
			return err
		}
	}
	if _, err := r.audio.Write(pcm); err != nil {
		return err
	}
	r.pending = true
	return nil
}

func (r *recorder) mark(name string) {
	if _, exists := r.report.Markers[name]; !exists {
		r.report.Markers[name] = float64(r.report.Frames) / sim.TickRate
	}
}

func (r *recorder) Draw(screen *ebiten.Image) {
	if !r.pending || r.err != nil {
		screen.DrawImage(r.frame, nil)
		return
	}
	if !r.finished {
		width, height := r.game.Layout(320, 208)
		if r.native == nil || r.native.Bounds().Dx() != width || r.native.Bounds().Dy() != height {
			r.native = ebiten.NewImage(width, height)
		}
		r.game.Draw(r.native)
		r.frame.Fill(color.Black)
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(0, float64(208-height)/2)
		r.frame.DrawImage(r.native, op)
		r.frame.ReadPixels(r.pixels)
		copy(r.lastPixels, r.pixels)
	} else {
		copy(r.pixels, r.lastPixels)
		// A completed jingle can release the stage on the same PAL as the
		// last score wait. Preserve the actual final score image even when
		// no separately rendered Ready frame exists before the transition.
		r.snapshot("stage-clear")
	}
	if _, err := r.video.Write(r.pixels); err != nil {
		r.err = err
		return
	}
	if !r.finished {
		w := r.game.World
		if !r.game.AttractDemo() && r.report.Frames == r.gameTitleFrame() {
			r.snapshot("title")
		}
		if r.game.AttractDemo() {
			for name, scroll := range map[string]int{"formations": 500, "gameplay": 1600, "industrial": 2400} {
				if w.Scroll >= float64(scroll) {
					r.snapshot(name)
				}
			}
			if w.Boss.ControllerActive && !w.Boss.Defeated && w.Boss.DamageTaken > 0 {
				r.snapshot("boss")
			}
			if w.Scroll >= 500 && !r.shots["touch"] {
				r.touchSnapshot()
			}
			if w.StageResult != nil && w.StageResult.Ready {
				r.snapshot("stage-clear")
			}
		}
	}
	r.pending = false
	r.report.Frames++
	if r.report.Frames%500 == 0 {
		fmt.Fprintf(r.status, "Capture: %d frames, video %.1fs, wall %s\n", r.report.Frames, float64(r.report.Frames)/sim.TickRate, time.Since(r.started).Round(time.Second))
	}
	screen.DrawImage(r.frame, nil)
}

// The menu has settled by PAL300 and stays unchanged briefly before the demo.
func (r *recorder) gameTitleFrame() int { return r.introFrames - 1 }

func (r *recorder) touchSnapshot() {
	target := ebiten.NewImage(480, 208)
	defer target.Deallocate()
	r.game.DrawRecordingTouchPreview(target)
	pixels := make([]byte, 480*208*4)
	target.ReadPixels(pixels)
	img := &image.RGBA{Pix: pixels, Stride: 480 * 4, Rect: image.Rect(0, 0, 480, 208)}
	file, err := os.Create(filepath.Join(r.output, "touch.png"))
	if err != nil {
		r.err = err
		return
	}
	err = (&png.Encoder{CompressionLevel: png.BestCompression}).Encode(file, img)
	closeErr := file.Close()
	if err != nil {
		r.err = err
	} else if closeErr != nil {
		r.err = closeErr
	}
	r.shots["touch"] = r.err == nil
}

func (r *recorder) snapshot(name string) {
	if r.shots[name] || r.err != nil {
		return
	}
	path := filepath.Join(r.output, name+".png")
	file, err := os.Create(path)
	if err != nil {
		r.err = err
		return
	}
	img := &image.RGBA{Pix: append([]byte(nil), r.pixels...), Stride: 320 * 4, Rect: image.Rect(0, 0, 320, 208)}
	err = (&png.Encoder{CompressionLevel: png.BestCompression}).Encode(file, img)
	closeErr := file.Close()
	if err != nil {
		r.err = err
		return
	}
	if closeErr != nil {
		r.err = closeErr
		return
	}
	r.shots[name] = true
}

func writeWAVHeader(file *os.File, dataBytes uint32) error {
	header := make([]byte, 44)
	copy(header, "RIFF")
	binary.LittleEndian.PutUint32(header[4:], 36+dataBytes)
	copy(header[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(header[16:], 16)
	binary.LittleEndian.PutUint16(header[20:], 1)
	binary.LittleEndian.PutUint16(header[22:], 2)
	binary.LittleEndian.PutUint32(header[24:], sound.SampleRate)
	binary.LittleEndian.PutUint32(header[28:], sound.SampleRate*4)
	binary.LittleEndian.PutUint16(header[32:], 4)
	binary.LittleEndian.PutUint16(header[34:], 16)
	copy(header[36:], "data")
	binary.LittleEndian.PutUint32(header[40:], dataBytes)
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	_, err := file.Write(header)
	return err
}
