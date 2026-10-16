package source

import (
	"encoding/binary"
	"fmt"
)

// extractEndingMusic normalizes the separate four-channel credits score.
// Its original driver at 0x5F244 uses 130-byte patterns and one order list.
func extractEndingMusic(memory []byte, graphicsEnd int, write func(string, []byte) error) (*LevelMusic, error) {
	driver := int(binary.BigEndian.Uint32(memory[0x4a32c:]))
	if driver != 0x5f244 || graphicsEnd <= 0x6040c || graphicsEnd > len(memory) {
		return nil, fmt.Errorf("unsupported original ending music layout")
	}
	start := 0x5f570
	music := &LevelMusic{File: "audio/ending-music.bin", DriverAddress: uint32(driver), DataStart: start - driver, Main: 0, Boss1: 0, Boss2: 0, Jingles: []int{0}, MainPatterns: 0x5f642 - start, BossPatterns: 0x5f642 - start, MainStride: 130, BossStride: 130, SampleTable: 0x60376 - start, Periods: 0x603b6 - start, Samples: 0x6040c - start, SampleCount: 8}
	data, err := trimMusicData(memory[start:graphicsEnd], music)
	if err != nil {
		return nil, err
	}
	if err := write(music.File, data); err != nil {
		return nil, err
	}
	return music, nil
}
