package sim

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"strings"

	"github.com/olivierh59500/r-type/internal/source"
)

// LoadStagesFS binds the original manifest and its separate checkpoint and cave
// operands. Both desktop and Android use this path without changing replay
// inputs or the manifest bytes that identify their extracted asset revision.
func LoadStagesFS(files fs.FS) ([]StageSpec, []RecoveryReport, error) {
	if files == nil {
		return nil, nil, fmt.Errorf("original stage assets are unavailable")
	}
	manifest, err := fs.ReadFile(files, "manifest.json")
	if err != nil {
		return nil, nil, fmt.Errorf("read original stage assets: %w", err)
	}
	stages, reports, err := LoadStages(bytes.NewReader(manifest))
	if err != nil {
		return nil, nil, err
	}
	reset, err := source.ReadCheckpointReset(files)
	if err != nil {
		return nil, nil, fmt.Errorf("read original checkpoint operands: %w", err)
	}
	var identity struct {
		Version   int    `json:"version"`
		ADFSHA256 string `json:"adf_sha256"`
	}
	if err := json.Unmarshal(manifest, &identity); err != nil {
		return nil, nil, err
	}
	if identity.Version != 1 || identity.ADFSHA256 != reset.ADFSHA256 {
		return nil, nil, fmt.Errorf("original stage and checkpoint asset revisions differ")
	}
	if len(stages) != len(reset.Stages) {
		return nil, nil, fmt.Errorf("original stage and checkpoint metadata counts differ")
	}
	for index := range stages {
		if reports[index].Stage != reset.Stages[index].Number {
			return nil, nil, fmt.Errorf("stage %d checkpoint metadata belongs to another stage", index+1)
		}
		stages[index].CheckpointReset = &reset.Stages[index]
		if err := validateStageCheckpointReset(stages[index], index+1); err != nil {
			return nil, nil, err
		}
	}
	if len(stages) >= 2 && reports[1].Stage == 2 {
		cave, err := source.ReadCave(files)
		if err != nil {
			return nil, nil, err
		}
		stages[1].Cave = cave
		var images struct {
			Sprites []struct {
				Name                        string `json:"name"`
				Address                     uint32 `json:"address"`
				Level, Frame, Width, Height int
			}
		}
		if err := json.Unmarshal(manifest, &images); err != nil {
			return nil, nil, err
		}
		frames := make(map[int]string)
		for _, sprite := range images.Sprites {
			if sprite.Level != 2 || sprite.Address != cave.ShotDescriptor {
				continue
			}
			if sprite.Frame < 0 || sprite.Frame >= len(cave.ShotCollisions) || sprite.Width <= 0 || sprite.Height <= 0 {
				return nil, nil, fmt.Errorf("invalid original cave projectile art")
			}
			if _, exists := frames[sprite.Frame]; exists {
				return nil, nil, fmt.Errorf("duplicate original cave projectile frame")
			}
			frames[sprite.Frame] = sprite.Name
			if sprite.Frame == 0 {
				base := strings.TrimSuffix(sprite.Name, "-0")
				if base == "" || base == sprite.Name {
					return nil, nil, fmt.Errorf("invalid original cave projectile name")
				}
				stages[1].CaveShotArt = BossArt{Sprite: base, Width: sprite.Width, Height: sprite.Height}
			}
		}
		if stages[1].CaveShotArt.Sprite == "" || len(frames) != len(cave.ShotCollisions) {
			return nil, nil, fmt.Errorf("original cave projectile art is missing")
		}
		for frame, name := range frames {
			if name != fmt.Sprintf("%s-%d", stages[1].CaveShotArt.Sprite, frame) {
				return nil, nil, fmt.Errorf("incompatible original cave projectile frame name")
			}
		}
		for _, box := range cave.ShotCollisions {
			stages[1].CaveShotArt.CollisionFrames = append(stages[1].CaveShotArt.CollisionFrames, CollisionFrame{X: box.X, Y: box.Y, Width: box.Width, Height: box.Height})
		}
		if err := validateStageCave(stages[1], 2); err != nil {
			return nil, nil, err
		}
	}
	return stages, reports, nil
}

func validateStageCave(stage StageSpec, number int) error {
	if stage.Cave == nil {
		return nil
	}
	if err := stage.Cave.Validate(); err != nil {
		return fmt.Errorf("stage %d cave operands: %w", number, err)
	}
	if number != 2 || stage.MapWidth < 4 || stage.MapHeight < 3 || stage.MapWidth > 4096 || stage.MapHeight > 256 || len(stage.TileMap) != stage.MapWidth*stage.MapHeight || stage.CaveShotArt.Sprite == "" || stage.CaveShotArt.Width <= 0 || stage.CaveShotArt.Height <= 0 || len(stage.CaveShotArt.CollisionFrames) != len(stage.Cave.ShotCollisions) {
		return fmt.Errorf("stage %d cave assets are incomplete or incompatible", number)
	}
	for _, creature := range stage.Cave.Creatures {
		if creature.Column > stage.MapWidth-4 || creature.Row > stage.MapHeight-3 {
			return fmt.Errorf("stage %d cave map rectangle is outside its map", number)
		}
	}
	return nil
}

func validateStageCheckpointReset(stage StageSpec, number int) error {
	data := stage.CheckpointReset
	if data == nil {
		return nil
	}
	if data.Number != number || data.MapRestore == nil || stage.MapWidth <= 0 || stage.MapWidth > 4096 || stage.MapHeight <= 0 || stage.MapHeight > 256 || len(stage.TileMap) != stage.MapWidth*stage.MapHeight || int(data.RowBytes) != stage.MapWidth*2 || !data.HasTileTable && len(data.Patches) != 0 {
		return fmt.Errorf("stage %d checkpoint map operands are incomplete or incompatible", number)
	}
	for _, patches := range [][]source.CheckpointTilePatch{data.Patches, data.MapRestore.ExtraPatches} {
		for _, patch := range patches {
			if patch.Column < 0 || patch.Column >= stage.MapWidth || patch.Row < 0 || patch.Row >= stage.MapHeight {
				return fmt.Errorf("stage %d checkpoint tile patch is outside its map", number)
			}
		}
	}
	return nil
}
