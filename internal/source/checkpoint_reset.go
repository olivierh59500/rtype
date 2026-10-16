package source

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io/fs"
)

// CheckpointResetData contains native reset operands decoded from the disk.
// It is separate from presentation metadata. Original executable addresses,
// instruction words and measured elapsed durations are omitted.
type CheckpointResetData struct {
	Version                                                   int
	ADFSHA256                                                 string
	BufferClearBytes                                          uint32
	ActorSlots, ActorStrideBytes, FriendlySlots, HostileSlots int
	HistorySlots, HUDColumns, HUDRows                         int
	Stages                                                    []StageCheckpointReset
}

// StageCheckpointReset preserves authored record order and descending
// checkpoint thresholds. A nil initializer means its work is not yet decoded;
// callers must reject it rather than assume a no-op or fixed duration.
type StageCheckpointReset struct {
	Number       int
	RowBytes     uint16
	HasTileTable bool
	Patches      []CheckpointTilePatch
	Checkpoints  []CheckpointResetEntry
	Initializer  *CheckpointInitializerData `json:",omitempty"`
	MusicStart   *CheckpointMusicStartData  `json:",omitempty"`
	MapRestore   *CheckpointMapRestoreData  `json:",omitempty"`
}

type CheckpointTilePatch struct {
	Column, Row int
	Word        uint16
}

// EventIndex selects a logical native event, including the end cursor. It
// replaces the source event pointer without changing threshold selection.
type CheckpointResetEntry struct {
	Scroll     uint16
	EventIndex int
}

// CheckpointInitializerData names a translated initializer and its operands.
// The first-stage initializer clears six state bytes and reloads its boss wait.
type CheckpointInitializerData struct {
	Kind        string
	ClearBytes  int
	InitialWait uint16
}

// CheckpointMusicStartData preserves the first main-score row of the supported
// first-stage driver. Sample offsets are relative to the extracted audio file;
// lengths retain Paula's word units. It contains no original driver code.
type CheckpointMusicStartData struct {
	Variant       string
	PatternStride uint16
	Channels      [2]CheckpointMusicStartChannel
}

type CheckpointMusicStartChannel struct {
	PatternIndex, SampleIndex, Note, Transpose, Volume, Control byte
	SampleOffset                                                uint32
	SampleLengthWords, Period                                   uint16
}

func extractCheckpointMusicStart(memory []byte, music *LevelMusic, graphicsEnd int) (*CheckpointMusicStartData, error) {
	if music == nil || music.MainStride != 98 || graphicsEnd > len(memory) {
		return nil, fmt.Errorf("unsupported checkpoint music data")
	}
	begin := int(music.DriverAddress) + music.DataStart
	if begin < 0 || begin >= graphicsEnd {
		return nil, fmt.Errorf("checkpoint music data lies outside its source range")
	}
	data := memory[begin:graphicsEnd]
	span := func(offset, length int) ([]byte, error) {
		if offset < 0 || offset > len(data) || length < 0 || length > len(data)-offset {
			return nil, fmt.Errorf("checkpoint music operand lies outside its data")
		}
		return data[offset : offset+length], nil
	}
	result := &CheckpointMusicStartData{Variant: "stage-one", PatternStride: uint16(music.MainStride)}
	for index := range result.Channels {
		order, err := span(music.Main+index*2, 2)
		if err != nil {
			return nil, err
		}
		row, err := span(music.MainPatterns+int(order[0])*music.MainStride, 4)
		if err != nil {
			return nil, err
		}
		if int(row[0]) >= music.SampleCount || row[1]&0x80 != 0 {
			return nil, fmt.Errorf("unsupported first checkpoint music row")
		}
		sample, err := span(music.SampleTable+int(row[0])*8, 8)
		if err != nil {
			return nil, err
		}
		tuning, err := span(music.Periods+(int(row[1])+int(order[1]))*2, 2)
		if err != nil {
			return nil, err
		}
		offset := uint64(music.Samples) + uint64(binary.BigEndian.Uint32(sample))
		length := binary.BigEndian.Uint16(sample[6:])
		if offset > uint64(len(data)) || uint64(length)*2 > uint64(len(data))-offset {
			return nil, fmt.Errorf("checkpoint music sample lies outside its data")
		}
		result.Channels[index] = CheckpointMusicStartChannel{
			PatternIndex: order[0], Transpose: order[1], SampleIndex: row[0], Note: row[1], Volume: row[2], Control: row[3],
			SampleOffset: uint32(offset), SampleLengthWords: length, Period: binary.BigEndian.Uint16(tuning),
		}
	}
	return result, nil
}

func ReadCheckpointReset(files fs.FS) (CheckpointResetData, error) {
	bytes, err := fs.ReadFile(files, "checkpoint-reset.json")
	if err != nil {
		return CheckpointResetData{}, err
	}
	var data CheckpointResetData
	if err := json.Unmarshal(bytes, &data); err != nil {
		return CheckpointResetData{}, err
	}
	if data.Version != 1 || data.ADFSHA256 != ADFSHA256 || len(data.Stages) != 8 {
		return CheckpointResetData{}, fmt.Errorf("unsupported checkpoint reset metadata")
	}
	for index, stage := range data.Stages {
		if stage.Number != index+1 || stage.RowBytes == 0 || stage.RowBytes&1 != 0 || len(stage.Checkpoints) == 0 || stage.Checkpoints[len(stage.Checkpoints)-1].Scroll != 0 || !stage.HasTileTable && len(stage.Patches) != 0 {
			return CheckpointResetData{}, fmt.Errorf("invalid stage %d checkpoint reset metadata", index+1)
		}
		if stage.MapRestore == nil {
			return CheckpointResetData{}, fmt.Errorf("stage %d checkpoint map profile is missing; recreate runtime assets with cmd/extract", index+1)
		}
	}
	return data, nil
}

func extractCheckpointResetCommon(memory []byte) CheckpointResetData {
	u32 := func(address int) uint32 { return binary.BigEndian.Uint32(memory[address:]) }
	return CheckpointResetData{
		Version: 1, ADFSHA256: ADFSHA256,
		BufferClearBytes: u32(0x3674) - u32(0x366c),
		ActorSlots:       int(memory[0xc07]) + 2, ActorStrideBytes: int(u32(0xc0a)),
		FriendlySlots: int(memory[0xc19]) + 1, HostileSlots: int(memory[0xc27]) + 1,
		HistorySlots: int(memory[0x246f]) + 1, HUDColumns: int(memory[0xb97]) + 1, HUDRows: int(memory[0xb9f]) + 1,
	}
}

func extractStageCheckpointReset(memory []byte, level Level, rowBytes, headerEnd int) (StageCheckpointReset, error) {
	data := StageCheckpointReset{Number: level.Number, RowBytes: uint16(rowBytes)}
	u16 := func(address int) uint16 { return binary.BigEndian.Uint16(memory[address:]) }
	u32 := func(address int) int { return int(binary.BigEndian.Uint32(memory[address:])) }
	if len(memory) < 0x4a328 || headerEnd > len(memory) || headerEnd < 0x4a328 || rowBytes <= 0 || rowBytes > 65535 || rowBytes&1 != 0 || len(level.Checkpoints) == 0 {
		return data, fmt.Errorf("invalid stage checkpoint extraction ranges")
	}
	address := u32(0x4a31c)
	data.HasTileTable = address != 0
	if data.HasTileTable {
		terminated := false
		for count := 0; count < 4096; count++ {
			if address < 0x4a2e4 || address+2 > headerEnd {
				return data, fmt.Errorf("stage %d tile-reset table is outside the header", level.Number)
			}
			column := int(int16(u16(address)))
			if column < 0 {
				terminated = true
				break
			}
			if address+6 > headerEnd {
				return data, fmt.Errorf("stage %d tile-reset record is truncated", level.Number)
			}
			data.Patches = append(data.Patches, CheckpointTilePatch{Column: column, Row: int(u16(address + 2)), Word: u16(address + 4)})
			address += 6
		}
		if !terminated {
			return data, fmt.Errorf("stage %d tile-reset table has no bounded terminator", level.Number)
		}
	}
	for index, checkpoint := range level.Checkpoints {
		if checkpoint.Scroll < 0 || checkpoint.Scroll > 65535 || index > 0 && checkpoint.Scroll >= level.Checkpoints[index-1].Scroll {
			return data, fmt.Errorf("stage %d checkpoints are not descending", level.Number)
		}
		cursor := 0
		for cursor < len(level.Events) && level.Events[cursor].Address < checkpoint.EventAddress {
			cursor++
		}
		data.Checkpoints = append(data.Checkpoints, CheckpointResetEntry{Scroll: uint16(checkpoint.Scroll), EventIndex: cursor})
	}
	if data.Checkpoints[len(data.Checkpoints)-1].Scroll != 0 {
		return data, fmt.Errorf("stage %d checkpoints have no zero terminal", level.Number)
	}
	if level.Number == 1 {
		address := u32(0x4a324)
		if address < 0x4a2e4 || address+46 > headerEnd {
			return data, fmt.Errorf("stage-one initializer is outside the header")
		}
		for index := 0; index < 6; index++ {
			if u16(address+index*6) != 0x4239 {
				return data, fmt.Errorf("stage-one initializer lacks a state-byte reset")
			}
		}
		if u16(address+36) != 0x33fc || u16(address+44) != 0x4e75 {
			return data, fmt.Errorf("unsupported stage-one checkpoint initializer")
		}
		data.Initializer = &CheckpointInitializerData{Kind: "stage-one", ClearBytes: 6, InitialWait: u16(address + 38)}
	}
	return data, nil
}
