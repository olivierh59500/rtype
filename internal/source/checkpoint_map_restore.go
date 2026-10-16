package source

import (
	"encoding/binary"
	"fmt"
)

// CheckpointMapRestoreData selects live-map retention or the original full
// pristine copy. ExtraPatches are initializer writes absent from the existing
// native boss data; they follow the initializer's already translated patches.
// Core tile-table writes remain in StageCheckpointReset.Patches.
type CheckpointMapRestoreData struct {
	Retain       bool
	ExtraPatches []CheckpointTilePatch `json:",omitempty"`
}

func extractCheckpointMapRestore(memory []byte, level Level, rowBytes, headerEnd int) (*CheckpointMapRestoreData, error) {
	result := &CheckpointMapRestoreData{Retain: level.Number != 4}
	if level.Number != 2 && level.Number != 4 {
		return result, nil
	}
	span := func(address, length int) ([]byte, error) {
		if address < 0x4a2e4 || length < 0 || address > headerEnd || length > headerEnd-address || address > len(memory) || length > len(memory)-address {
			return nil, fmt.Errorf("stage %d checkpoint map operands exceed the original header", level.Number)
		}
		return memory[address : address+length], nil
	}
	if level.Number == 4 {
		// The original stage-four initializer copies 2,236 words from its
		// pristine source map. The existing tile-map extractor uses that copy.
		instruction, err := span(0x4b052, 4)
		if err != nil {
			return nil, err
		}
		if binary.BigEndian.Uint16(instruction) != 0x303c || int(binary.BigEndian.Uint16(instruction[2:]))+1 != len(level.TileMap) {
			return nil, fmt.Errorf("unsupported stage-four checkpoint map copy")
		}
		return result, nil
	}
	mapPointer, err := span(0x4a30c, 4)
	if err != nil {
		return nil, err
	}
	mapAddress := int(binary.BigEndian.Uint32(mapPointer))
	if rowBytes != level.MapWidth*2 || len(level.TileMap) != level.MapWidth*level.MapHeight || level.MapWidth <= 0 || level.MapHeight <= 0 {
		return nil, fmt.Errorf("stage-two checkpoint map dimensions are inconsistent")
	}
	for _, block := range []struct{ record, words int }{{0x4b38c, 0x4b450}, {0x4b3ac, 0x4b4e0}} {
		pointer, err := span(block.record+0x14, 4)
		if err != nil {
			return nil, err
		}
		words, err := span(block.words, 24)
		if err != nil {
			return nil, err
		}
		offset := int(binary.BigEndian.Uint32(pointer)) - mapAddress
		if offset < 0 || offset&1 != 0 {
			return nil, fmt.Errorf("stage-two initializer target precedes its tile map")
		}
		column, row := (offset%rowBytes)/2, offset/rowBytes
		if column+4 > level.MapWidth || row+3 > level.MapHeight {
			return nil, fmt.Errorf("stage-two initializer rectangle exceeds its tile map")
		}
		for index := 0; index < 12; index++ {
			result.ExtraPatches = append(result.ExtraPatches, CheckpointTilePatch{Column: column + index%4, Row: row + index/4, Word: binary.BigEndian.Uint16(words[index*2:])})
		}
	}
	return result, nil
}
