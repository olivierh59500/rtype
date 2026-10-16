// Command attractgen verifies and compacts an authored normal-input demo.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"go/format"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/olivierh59500/r-type/internal/pilot"
	"github.com/olivierh59500/r-type/internal/sim"
	"github.com/olivierh59500/r-type/internal/source"
)

func main() {
	replayPath := flag.String("replay", "", "verified normal-input stage-one replay JSON")
	manifestPath := flag.String("manifest", "assets/runtime/manifest.json", "matching excluded original asset manifest")
	outputPath := flag.String("out", "internal/attract/replay_data.go", "tracked Go input data output")
	flag.Parse()
	if *replayPath == "" {
		fail(fmt.Errorf("-replay is required"))
	}
	replayFile, err := os.Open(*replayPath)
	if err != nil {
		fail(err)
	}
	defer replayFile.Close()
	manifest, err := os.ReadFile(*manifestPath)
	if err != nil {
		fail(err)
	}
	files := manifestAssetFS{FS: os.DirFS(filepath.Dir(*manifestPath)), manifestName: filepath.Base(*manifestPath)}
	replay, report, err := verifyReplay(replayFile, manifest, files)
	if err != nil {
		fail(err)
	}
	generated, err := generate(replay, report)
	if err != nil {
		fail(err)
	}
	if err := os.WriteFile(*outputPath, generated, 0644); err != nil {
		fail(err)
	}
	fmt.Printf("Verified %d PAL inputs: stage one defeated, score %d, ships %d, deaths %d\n", report.Ticks, report.Score, report.Lives, len(report.Deaths))
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

// verifyReplay advances only the recorded input levels. It rejects a different
// source revision, unsupported host seed, early exit, and unverified stage end.
func verifyReplay(reader io.Reader, manifestData []byte, files fs.FS) (*pilot.Replay, pilot.Report, error) {
	replay, err := pilot.ReadReplay(reader)
	if err != nil {
		return nil, pilot.Report{}, err
	}
	if replay.Metadata.Seed != pilot.DefaultSeed {
		return nil, pilot.Report{}, fmt.Errorf("demo seed must match the native host seed")
	}
	if replay.Metadata.AssetManifestSHA256 == "" {
		return nil, pilot.Report{}, fmt.Errorf("demo requires an extracted manifest hash")
	}
	if len(replay.Frames) < 5 {
		return nil, pilot.Report{}, fmt.Errorf("demo does not contain its complete startup")
	}
	for tick := 0; tick < 4; tick++ {
		if replay.Frames[tick].Buttons != pilot.Start {
			return nil, pilot.Report{}, fmt.Errorf("demo requires four recorded Start levels")
		}
	}
	for _, frame := range replay.Frames[4:] {
		if frame.Buttons&(pilot.Start|pilot.Pause) != 0 {
			return nil, pilot.Report{}, fmt.Errorf("demo contains a restart or pause")
		}
	}
	var manifest source.Manifest
	if err := json.Unmarshal(manifestData, &manifest); err != nil {
		return nil, pilot.Report{}, err
	}
	if manifest.Version != 1 || manifest.ADFSHA256 != source.ADFSHA256 {
		return nil, pilot.Report{}, fmt.Errorf("demo requires the supported original asset manifest")
	}
	if err := replay.ValidateSource(manifest.ADFSHA256, fmt.Sprintf("%x", sha256.Sum256(manifestData))); err != nil {
		return nil, pilot.Report{}, err
	}
	stages, _, err := sim.LoadStagesFS(files)
	if err != nil {
		return nil, pilot.Report{}, err
	}
	w := sim.NewWorld(replay.Metadata.Seed)
	for _, configure := range []func() error{
		func() error { return w.SetStages(stages) },
		func() error { return w.SetInitialLives(manifest.InitialLives) },
		func() error { return w.SetExtraLifeThresholds(manifest.ExtraLifeThresholds) },
		func() error { return w.SetRecords(manifest.Records) },
		func() error { return w.SetMenu(manifest.Menu) },
	} {
		if err := configure(); err != nil {
			return nil, pilot.Report{}, err
		}
	}
	report, _ := pilot.Run(w, nil, replay.Metadata, len(replay.Frames), 1, replay)
	if report.Outcome != "stage_completed" || report.Stage != 2 || report.Ticks != len(replay.Frames) || len(report.Stages) != 1 || !report.Stages[0].BossDefeated {
		return nil, report, fmt.Errorf("replay must end exactly when its defeated first stage advances")
	}
	return replay, report, nil
}

// manifestAssetFS preserves the explicit -manifest filename while companion
// operands remain relative to the same asset directory.
type manifestAssetFS struct {
	fs.FS
	manifestName string
}

func (f manifestAssetFS) Open(name string) (fs.File, error) {
	if name == "manifest.json" {
		name = f.manifestName
	}
	return f.FS.Open(name)
}

func compact(replay *pilot.Replay) ([]byte, []byte) {
	levels := make([]byte, len(replay.Frames))
	for tick, frame := range replay.Frames {
		levels[tick] = byte(frame.Buttons)
	}
	var runs []byte
	for tick := 0; tick < len(levels); {
		end := tick + 1
		for end < len(levels) && levels[end] == levels[tick] {
			end++
		}
		runs = binary.AppendUvarint(runs, uint64(end-tick))
		runs = append(runs, levels[tick])
		tick = end
	}
	return runs, levels
}

func generate(replay *pilot.Replay, report pilot.Report) ([]byte, error) {
	runs, levels := compact(replay)
	var output bytes.Buffer
	fmt.Fprintln(&output, "// Code generated by cmd/attractgen; DO NOT EDIT.")
	fmt.Fprintln(&output, "// This file contains authored pilot input commands and verification metadata only.")
	fmt.Fprintln(&output, "package attract")
	fmt.Fprintln(&output, `import "github.com/olivierh59500/r-type/internal/pilot"`)
	fmt.Fprintf(&output, "const replayFrameCount = %d\n", len(replay.Frames))
	fmt.Fprintf(&output, "const replayInputSHA256 = %q\n", fmt.Sprintf("%x", sha256.Sum256(levels)))
	fmt.Fprintf(&output, "const replayFinalDigest = %q\n", report.FinalDigest)
	fmt.Fprintf(&output, "var replayMetadata = %#v\n", replay.Metadata)
	fmt.Fprintln(&output, "var replayRuns = []byte{")
	for index, value := range runs {
		if index%16 == 0 {
			fmt.Fprint(&output, "\t")
		}
		fmt.Fprintf(&output, "0x%02x,", value)
		if index%16 == 15 {
			fmt.Fprintln(&output)
		}
	}
	fmt.Fprintln(&output, "\n}")
	return format.Source(output.Bytes())
}
