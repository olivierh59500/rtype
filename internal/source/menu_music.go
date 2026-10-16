package source

import "fmt"

// extractMenuMusic normalizes the separate score-entry player from the five
// presentation tracks. Score tables, font data and executable bytes are omitted.
func extractMenuMusic(presentation []byte, write func(string, []byte) error) (*LevelMusic, error) {
	if len(presentation) != 5*5632 || presentation[0] != 0x48 || presentation[1] != 0xe7 {
		return nil, fmt.Errorf("unsupported original presentation music bank")
	}
	start := 0x32c
	music := &LevelMusic{File: "audio/menu-music.bin", DriverAddress: 0x4a300, DataStart: start, Main: 0, Boss1: 0, Boss2: 0, Jingles: []int{0}, MainPatterns: 0x4a78e - 0x4a62c, BossPatterns: 0x4a78e - 0x4a62c, MainStride: 98, BossStride: 98, SampleTable: 0x4b430 - 0x4a62c, Periods: 0x4b480 - 0x4a62c, Samples: 0x4b4d6 - 0x4a62c, SampleCount: 10}
	data, err := trimMusicData(presentation[start:], music)
	if err != nil {
		return nil, err
	}
	if err = write(music.File, data); err != nil {
		return nil, err
	}
	return music, nil
}
