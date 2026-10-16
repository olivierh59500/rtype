// Command buildmedia creates README screenshots and portfolio presentation files.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/olivierh59500/r-type/internal/attract"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

type report struct {
	FPS, Width, Height, SampleRate           int
	Frames, DemoFrames, Score, Lives, Deaths int
	DurationSeconds                          float64
	Complete, BossDefeated                   bool
	InputSHA256, AssetManifestSHA256         string
	Markers                                  map[string]float64
}

type cue struct {
	Start, End float64
	Text       string
}

type typography struct {
	heading, subtitle, small font.Face
}

func main() {
	capture := flag.String("capture", ".cache/media/capture", "lossless native capture directory")
	output := flag.String("out", "dist/portfolio", "excluded portfolio video directory")
	images := flag.String("images", "docs/images", "tracked README screenshot directory")
	skipCapture := flag.Bool("skip-capture", false, "reuse an already verified lossless capture")
	encoderThreads := flag.Int("encoder-threads", 2, "CPU threads for each of the two video encoders")
	fontPath := flag.String("font", "", "presentation TrueType font; system Arial or DejaVu Sans is discovered")
	ffmpeg := flag.String("ffmpeg", "ffmpeg", "FFmpeg executable")
	ffprobe := flag.String("ffprobe", "ffprobe", "FFprobe executable")
	flag.Parse()
	if *encoderThreads < 1 || *encoderThreads > 64 {
		log.Fatal("encoder threads must be between 1 and 64")
	}
	if !*skipCapture {
		run("go", "build", "-o", "bin/recordmedia", "./cmd/recordmedia")
		run(filepath.Join("bin", "recordmedia"), "-out", *capture, "-ffmpeg", *ffmpeg)
	}
	data, err := os.ReadFile(filepath.Join(*capture, "capture.json"))
	check(err)
	var recording report
	check(json.Unmarshal(data, &recording))
	if !recording.Complete || !recording.BossDefeated || recording.Deaths != 0 || recording.Lives != 3 || recording.DemoFrames != attract.FrameCount() || recording.InputSHA256 != attract.InputSHA256() || recording.AssetManifestSHA256 != attract.Metadata().AssetManifestSHA256 {
		log.Fatal("capture must match the current complete, deathless first-stage demo")
	}
	for _, path := range []string{*output, *images} {
		check(os.MkdirAll(path, 0755))
	}
	// Select a settled fight frame rather than the original hurt-mask flash.
	bossTime := recording.Markers["boss_start"] + math.Min(4, (recording.Markers["boss_defeated"]-recording.Markers["boss_start"])/2)
	run(*ffmpeg, "-hide_banner", "-loglevel", "error", "-y", "-ss", fmt.Sprintf("%.3f", bossTime), "-i", filepath.Join(*capture, "native-lossless.mkv"), "-frames:v", "1", "-update", "1", filepath.Join(*capture, "boss.png"))
	for _, item := range []struct{ source, name string }{{"title", "rtype-title"}, {"gameplay", "rtype-stage1"}, {"boss", "rtype-boss1"}, {"touch", "rtype-touch"}} {
		img := readPNG(filepath.Join(*capture, item.source+".png"))
		writePNG(filepath.Join(*images, item.name+".png"), scaleNearest(img, 4))
	}
	typefaces := loadTypography(*fontPath)
	defer typefaces.heading.Close()
	defer typefaces.subtitle.Close()
	defer typefaces.small.Close()
	cues := presentationCues(recording)
	writeSubtitles(filepath.Join(*output, "rtype-stage1.en.srt"), cues, false)
	writeSubtitles(filepath.Join(*output, "rtype-stage1.en.vtt"), cues, true)
	overlays := filepath.Join(*capture, "overlays")
	check(os.MkdirAll(overlays, 0755))
	concatPath := buildOverlaySequence(overlays, recording.DurationSeconds, cues, typefaces)
	poster := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	draw.Draw(poster, poster.Bounds(), &image.Uniform{C: color.RGBA{10, 15, 24, 255}}, image.Point{}, draw.Src)
	title := scaleNearest(readPNG(filepath.Join(*capture, "title.png")), 4)
	draw.Draw(poster, image.Rect(320, 64, 1600, 896), title, image.Point{}, draw.Src)
	draw.Draw(poster, poster.Bounds(), overlay(typefaces, "Complete Stage 1 expert demo\nNo lives lost - original Amiga artwork and sound"), image.Point{}, draw.Over)
	writePNG(filepath.Join(*output, "rtype-poster.png"), poster)
	check(os.WriteFile(filepath.Join(*output, "index.html"), []byte(portfolioHTML), 0644))
	metadata := map[string]any{"capture": recording, "video_width": 1920, "video_height": 1080, "fps": 50, "english_captions": "burned into both videos", "mp4": "H.264 / AAC, yuv420p, faststart", "webm": "VP9 / Opus", "gameplay_speed": "original PAL timing", "subtitles": cues}
	encoded, err := json.MarshalIndent(metadata, "", "  ")
	check(err)
	check(os.WriteFile(filepath.Join(*output, "presentation.json"), append(encoded, '\n'), 0644))
	base := []string{"-hide_banner", "-loglevel", "warning", "-stats", "-stats_period", "5", "-y", "-i", filepath.Join(*capture, "native-lossless.mkv"), "-f", "concat", "-safe", "0", "-i", concatPath, "-i", filepath.Join(*capture, "native-audio.wav"), "-filter_complex_threads", "2", "-filter_complex", "[0:v]scale=1280:832:flags=neighbor,format=rgba,pad=1920:1080:320:64:color=0x0a0f18[game];[game][1:v]overlay=0:0:format=rgb:eof_action=repeat:repeatlast=1[composed];[composed]scale=1920:1080:out_color_matrix=bt709:out_range=tv,format=yuv420p[v]", "-map", "[v]", "-map", "2:a:0", "-frames:v", strconv.Itoa(recording.Frames), "-r", "50", "-fps_mode", "cfr", "-af", fmt.Sprintf("afade=t=out:st=%.3f:d=1", math.Max(0, recording.Markers["demo_end"]-1)), "-ar", "48000", "-ac", "2", "-color_primaries", "bt709", "-color_trc", "bt709", "-colorspace", "bt709", "-color_range", "tv", "-metadata", "title=R-Type - Native Go and Ebitengine - Stage 1 Expert Demo", "-metadata", "comment=English captions are included in the picture. Recorded from the native Go renderer and sound sequencers."}
	outputs := []struct {
		name string
		args []string
	}{
		{"rtype-stage1.mp4", []string{"-c:v", "libx264", "-preset", "medium", "-crf", "18", "-threads", strconv.Itoa(*encoderThreads), "-c:a", "aac", "-b:a", "192k", "-movflags", "+faststart"}},
		{"rtype-stage1.webm", []string{"-c:v", "libvpx-vp9", "-b:v", "0", "-crf", "28", "-deadline", "good", "-cpu-used", "4", "-row-mt", "1", "-threads", strconv.Itoa(*encoderThreads), "-c:a", "libopus", "-b:a", "160k"}},
	}
	var wait sync.WaitGroup
	errors := make(chan error, len(outputs))
	for _, target := range outputs {
		wait.Add(1)
		go func(name string, profile []string) {
			defer wait.Done()
			args := append(append(append([]string(nil), base...), profile...), filepath.Join(*output, name))
			fmt.Printf("Encoding %s\n", name)
			cmd := exec.Command(*ffmpeg, args...)
			cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
			if err := cmd.Run(); err != nil {
				errors <- fmt.Errorf("encode %s: %w", name, err)
			}
		}(target.name, target.args)
	}
	wait.Wait()
	close(errors)
	for err := range errors {
		check(err)
	}
	for _, target := range outputs {
		path := filepath.Join(*output, target.name)
		probe := exec.Command(*ffprobe, "-v", "error", "-show_entries", "stream=codec_name,codec_type,width,height,r_frame_rate,sample_rate,channels,nb_frames:format=duration,size", "-of", "json", path)
		result, err := probe.Output()
		check(err)
		check(verifyProbe(result, target.name, recording))
		check(os.WriteFile(path+".json", result, 0644))
		fmt.Printf("Verified media metadata: %s\n", path)
	}
	fmt.Printf("Presentation: %s (%.2f seconds, 1080p50, English captions)\nREADME images: %s\n", *output, recording.DurationSeconds, *images)
}

func verifyProbe(data []byte, name string, recording report) error {
	// Explicit tags keep the ffprobe field names independent of Go conventions.
	var decoded struct {
		Streams []struct {
			CodecName string `json:"codec_name"`
			CodecType string `json:"codec_type"`
			FPS       string `json:"r_frame_rate"`
			Rate      string `json:"sample_rate"`
			Frames    string `json:"nb_frames"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
			Channels  int    `json:"channels"`
		} `json:"streams"`
		Format struct{ Duration string } `json:"format"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	videoCodec, audioCodec := "h264", "aac"
	if strings.HasSuffix(name, ".webm") {
		videoCodec, audioCodec = "vp9", "opus"
	}
	video, audio := false, false
	for _, stream := range decoded.Streams {
		if stream.CodecType == "video" {
			video = stream.CodecName == videoCodec && stream.Width == 1920 && stream.Height == 1080 && stream.FPS == "50/1" && (stream.Frames == "" || stream.Frames == strconv.Itoa(recording.Frames))
		}
		if stream.CodecType == "audio" {
			audio = stream.CodecName == audioCodec && stream.Rate == "48000" && stream.Channels == 2
		}
	}
	duration, err := strconv.ParseFloat(decoded.Format.Duration, 64)
	if err != nil || !video || !audio || math.Abs(duration-recording.DurationSeconds) > .05 {
		return fmt.Errorf("%s does not match the expected 1080p50 stereo presentation and duration", name)
	}
	return nil
}

func check(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func run(name string, args ...string) {
	command := exec.Command(name, args...)
	command.Stdout, command.Stderr = os.Stdout, os.Stderr
	if os.Getenv("GOCACHE") == "" {
		cache, err := filepath.Abs(".cache/go-build")
		check(err)
		command.Env = append(os.Environ(), "GOCACHE="+cache)
	}
	check(command.Run())
}

func readPNG(path string) image.Image {
	file, err := os.Open(path)
	check(err)
	defer file.Close()
	img, err := png.Decode(file)
	check(err)
	return img
}

func writePNG(path string, img image.Image) {
	file, err := os.Create(path)
	check(err)
	check((&png.Encoder{CompressionLevel: png.BestCompression}).Encode(file, img))
	check(file.Close())
}

func scaleNearest(source image.Image, factor int) *image.RGBA {
	bounds := source.Bounds()
	result := image.NewRGBA(image.Rect(0, 0, bounds.Dx()*factor, bounds.Dy()*factor))
	for y := 0; y < result.Bounds().Dy(); y++ {
		for x := 0; x < result.Bounds().Dx(); x++ {
			result.Set(x, y, source.At(bounds.Min.X+x/factor, bounds.Min.Y+y/factor))
		}
	}
	return result
}

func loadTypography(path string) typography {
	if path == "" {
		for _, candidate := range []string{"/System/Library/Fonts/Supplemental/Arial.ttf", "/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf", "/usr/share/fonts/truetype/liberation2/LiberationSans-Regular.ttf"} {
			if _, err := os.Stat(candidate); err == nil {
				path = candidate
				break
			}
		}
	}
	data, err := os.ReadFile(path)
	check(err)
	parsed, err := opentype.Parse(data)
	check(err)
	face := func(size float64) font.Face {
		result, err := opentype.NewFace(parsed, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
		check(err)
		return result
	}
	return typography{heading: face(32), subtitle: face(38), small: face(20)}
}

func text(img draw.Image, face font.Face, value string, x, baseline int, ink color.Color) {
	drawer := font.Drawer{Dst: img, Src: image.NewUniform(ink), Face: face, Dot: fixed.P(x, baseline)}
	drawer.DrawString(value)
}

func centered(img draw.Image, face font.Face, value string, baseline int, ink color.Color) {
	width := font.MeasureString(face, value).Ceil()
	text(img, face, value, (1920-width)/2, baseline, ink)
}

func overlay(faces typography, caption string) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, 1920, 1080))
	white, muted := color.RGBA{235, 243, 251, 255}, color.RGBA{155, 176, 198, 255}
	text(img, faces.heading, "R-TYPE", 320, 43, white)
	label := "NATIVE GO  /  EBITENGINE  /  AMIGA 1989"
	text(img, faces.small, label, 1600-font.MeasureString(faces.small, label).Ceil(), 38, muted)
	draw.Draw(img, image.Rect(320, 55, 1600, 57), &image.Uniform{C: color.RGBA{54, 89, 119, 255}}, image.Point{}, draw.Src)
	for index, line := range strings.Split(caption, "\n") {
		centered(img, faces.subtitle, line, 967+index*48, white)
	}
	centered(img, faces.small, "Desktop + Android  |  Original artwork and native gameplay", 1053, muted)
	return img
}

func presentationCues(r report) []cue {
	boss, won, end := r.Markers["boss_start"], r.Markers["boss_defeated"], r.Markers["demo_end"]
	cues := []cue{
		{0, 6, "R-Type on Amiga, rebuilt in Go with Ebitengine."},
		{6, 13, "Original Amiga artwork, palettes, music and sound effects."},
		{13, 21, "Stage 1. A complete expert demonstration at the original pace."},
		{26, 34, "Collect the Force orb to shield the ship and upgrade its weapons."},
		{42, 50, "Anticipate enemy waves and charge the wave cannon."},
		{64, 72, "Original formations, scenery and camera movement."},
		{88, 96, "Desktop and Android share the same native Go simulation."},
		{112, 120, "Memorized routes and precise, human-paced control."},
		{boss, won, "Dobkeratops. Hold the firing lane and release charged shots."},
		{won, end, "Boss defeated. The original exit and score sequence follows."},
		{end, r.DurationSeconds, fmt.Sprintf("Stage 1 cleared: %s points, no lives lost.\nAll three ships remain intact.", commas(r.Score))},
	}
	sort.Slice(cues, func(i, j int) bool { return cues[i].Start < cues[j].Start })
	var valid []cue
	for _, item := range cues {
		item.Start, item.End = math.Max(0, item.Start), math.Min(r.DurationSeconds, item.End)
		if item.End > item.Start {
			valid = append(valid, item)
		}
	}
	return valid
}

func commas(value int) string {
	result := strconv.Itoa(value)
	for at := len(result) - 3; at > 0; at -= 3 {
		result = result[:at] + "," + result[at:]
	}
	return result
}

func timestamp(seconds float64, decimal string) string {
	milliseconds := int(math.Round(seconds * 1000))
	return fmt.Sprintf("%02d:%02d:%02d%s%03d", milliseconds/3600000, milliseconds/60000%60, milliseconds/1000%60, decimal, milliseconds%1000)
}

func writeSubtitles(path string, cues []cue, vtt bool) {
	var output strings.Builder
	decimal := ","
	if vtt {
		output.WriteString("WEBVTT\n\n")
		decimal = "."
	}
	for index, item := range cues {
		fmt.Fprintf(&output, "%d\n%s --> %s\n%s\n\n", index+1, timestamp(item.Start, decimal), timestamp(item.End, decimal), item.Text)
	}
	check(os.WriteFile(path, []byte(output.String()), 0644))
}

func buildOverlaySequence(directory string, duration float64, cues []cue, faces typography) string {
	boundaries := []float64{0, duration}
	for _, item := range cues {
		boundaries = append(boundaries, item.Start, item.End)
	}
	sort.Float64s(boundaries)
	var unique []float64
	for _, value := range boundaries {
		if len(unique) == 0 || value > unique[len(unique)-1]+.0001 {
			unique = append(unique, value)
		}
	}
	var concat strings.Builder
	concat.WriteString("ffconcat version 1.0\n")
	var last string
	for index := 0; index+1 < len(unique); index++ {
		caption := ""
		for _, item := range cues {
			if unique[index] >= item.Start && unique[index] < item.End {
				caption = item.Text
			}
		}
		path, err := filepath.Abs(filepath.Join(directory, fmt.Sprintf("overlay-%03d.png", index)))
		check(err)
		writePNG(path, overlay(faces, caption))
		fmt.Fprintf(&concat, "file '%s'\noption framerate 50\nduration %.6f\n", strings.ReplaceAll(filepath.ToSlash(path), "'", "'\\''"), unique[index+1]-unique[index])
		last = path
	}
	fmt.Fprintf(&concat, "file '%s'\noption framerate 50\n", strings.ReplaceAll(filepath.ToSlash(last), "'", "'\\''"))
	path := filepath.Join(directory, "captions.ffconcat")
	check(os.WriteFile(path, []byte(concat.String()), 0644))
	return path
}

const portfolioHTML = `<!doctype html>
<html lang="en">
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>R-Type - Go and Ebitengine</title>
<style>
body{margin:0;background:#0a0f18;color:#ebf3fb;font:16px system-ui,sans-serif}
main{max-width:1100px;margin:48px auto;padding:0 24px}h1{font-size:2rem}
video{display:block;width:100%;height:auto;background:#000;border-radius:8px}
a{color:#9dd8ff}p{line-height:1.6;color:#b7c7d8}
</style>
<main>
<h1>R-Type, rebuilt in Go</h1>
<p>A native Ebitengine reconstruction of the 1989 Amiga release. This full first-stage expert demo loses no ships. English captions are included in the video.</p>
<video controls playsinline preload="metadata" poster="rtype-poster.png">
<source src="rtype-stage1.webm" type="video/webm">
<source src="rtype-stage1.mp4" type="video/mp4">
<p><a href="rtype-stage1.mp4">Download the MP4 presentation</a>.</p>
</video>
<p><a href="rtype-stage1.mp4">MP4</a> · <a href="rtype-stage1.webm">WebM</a> · <a href="rtype-stage1.en.vtt">English WebVTT</a> · <a href="rtype-stage1.en.srt">English SRT</a></p>
</main>
</html>
`
