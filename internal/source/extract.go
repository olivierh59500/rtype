// Package source recreates original art, maps, and audio data from the user's
// Amiga disk. Generated assets and original executable bytes stay outside Git.
package source

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// ADFSHA256 pins every extracted offset to the exact original source disk.
const ADFSHA256 = "5a481e82ce563d0b61faf96d5a41fd2762be79f2336ec3edbf1e704f30fe763d"

// Manifest records generated files and their original addresses.
type Manifest struct {
	Display             *DisplayData `json:"display,omitempty"`
	Menu                *MenuData    `json:"menu,omitempty"`
	Records             *RecordsData `json:"records,omitempty"`
	Version             int          `json:"version"`
	ADFSHA256           string       `json:"adf_sha256"`
	InitialHighScore    int          `json:"initial_high_score"`
	InitialLives        int          `json:"initial_lives"`
	ExtraLifeThresholds []int        `json:"extra_life_thresholds"`
	Title               string       `json:"title"`
	Font                string       `json:"font"`
	FontChars           string       `json:"font_chars"`
	Digits              string       `json:"digits"`
	HUD                 string       `json:"hud"`
	HUDChargeColors     []uint16     `json:"hud_charge_colors"`
	Life                string       `json:"life"`
	TitleMusic          string       `json:"title_music"`
	TitleSamples        string       `json:"title_samples"`
	Sprites             []Sprite     `json:"sprites"`
	Levels              []Level      `json:"levels"`
	Sounds              []Sound      `json:"sounds"`
}

// Sound stores an unchanged signed eight-bit Paula sample and its original
// playback settings. Length is already converted from Amiga words to bytes.
type Sound struct {
	Number   int    `json:"number"`
	File     string `json:"file"`
	Address  uint32 `json:"address"`
	Length   int    `json:"length"`
	Period   int    `json:"period"`
	Volume   int    `json:"volume"`
	Priority int    `json:"priority"`
}

// Sprite identifies one decoded original animation frame.
type Sprite struct {
	Name           string          `json:"name"`
	File           string          `json:"file"`
	Width          int             `json:"width"`
	Height         int             `json:"height"`
	Address        uint32          `json:"address"`
	Frame          int             `json:"frame"`
	Level          int             `json:"level"`
	CollisionFrame *CollisionFrame `json:"collision_frame,omitempty"`
}

// Level preserves the complete original map, including its collision flags.
// The lower twelve tile bits select art; bit fifteen is the collision flag.
type Level struct {
	Number         int          `json:"number"`
	File           string       `json:"file"`
	Width          int          `json:"width"`
	Height         int          `json:"height"`
	MapWidth       int          `json:"map_width"`
	MapHeight      int          `json:"map_height"`
	TileMap        []uint16     `json:"tile_map"`
	Palette        []uint16     `json:"palette"`
	Events         []Event      `json:"events"`
	Checkpoints    []Checkpoint `json:"checkpoints"`
	ScrollSpeed    float64      `json:"scroll_speed"`
	Music          *LevelMusic  `json:"music,omitempty"`
	EndingMusic    *LevelMusic  `json:"ending_music,omitempty"`
	EventClock     string       `json:"event_clock"`
	ScrollScript   []ScrollStep `json:"scroll_script"`
	TileAtlas      string       `json:"tile_atlas"`
	TileCount      int          `json:"tile_count"`
	Boss           *Boss        `json:"boss,omitempty"`
	InitialScrollX int          `json:"initial_scroll_x"`
	InitialScrollY int          `json:"initial_scroll_y"`
	Weapons        *Weapons     `json:"weapons,omitempty"`
}

// LevelMusic identifies original non-executable song data. All content offsets
// are relative to File; DataStart is relative to the original driver address.
type LevelMusic struct {
	File          string `json:"file"`
	DriverAddress uint32 `json:"driver_address"`
	DataStart     int    `json:"data_start"`
	Main          int    `json:"main"`
	Boss1         int    `json:"boss1"`
	Boss2         int    `json:"boss2"`
	Jingles       []int  `json:"jingles"`
	MainPatterns  int    `json:"main_patterns"`
	BossPatterns  int    `json:"boss_patterns"`
	MainStride    int    `json:"main_stride"`
	BossStride    int    `json:"boss_stride"`
	SampleTable   int    `json:"sample_table"`
	Periods       int    `json:"periods"`
	Samples       int    `json:"samples"`
	SampleCount   int    `json:"sample_count"`
}

// Event preserves one original eighteen-byte scripted spawn record and the
// data referenced by its descriptor. Go gameplay can consume this data without
// loading or interpreting the original executable routines.
type Event struct {
	Priority         int                   `json:"priority"`
	HuntingEmitter   *HuntingEmitterData   `json:"hunting_emitter,omitempty"`
	ShieldVehicle    *ShieldVehicleData    `json:"shield_vehicle,omitempty"`
	SpiralSentinel   *SpiralSentinelData   `json:"spiral_sentinel,omitempty"`
	MiniTail         *MiniTailData         `json:"mini_tail,omitempty"`
	GroundWalker     *GroundWalkerData     `json:"ground_walker,omitempty"`
	BattleshipTurret *BattleshipTurretData `json:"battleship_turret,omitempty"`
	FireWorm         *FireWormData         `json:"fire_worm,omitempty"`
	Reactor          *ReactorData          `json:"reactor,omitempty"`
	Address          uint32                `json:"address"`
	Trigger          int                   `json:"trigger"`
	X                int                   `json:"x"`
	Y                int                   `json:"y"`
	Parameter        uint32                `json:"parameter"`
	Count            int                   `json:"count"`
	Aux              int                   `json:"aux"`
	Descriptor       uint32                `json:"descriptor"`
	SpawnAddress     uint32                `json:"spawn_address"`
	UpdateAddress    uint32                `json:"update_address"`
	Health           int                   `json:"health"`
	AutomaticFire    int                   `json:"automatic_fire,omitempty"`
	Score            int                   `json:"score"`
	Width            int                   `json:"width"`
	Height           int                   `json:"height"`
	AnimationTicks   int                   `json:"animation_ticks"`
	Animation        []int                 `json:"animation"`
	Formation        []Formation           `json:"formation,omitempty"`
	MotionX          []int                 `json:"motion_x,omitempty"`
	MotionY          []int                 `json:"motion_y,omitempty"`
	MotionAnimation  []int                 `json:"motion_animation,omitempty"`
	MotionLoop       bool                  `json:"motion_loop"`
	Behavior         string                `json:"behavior,omitempty"`
	Variant          int                   `json:"variant"`
	LungeVX          int                   `json:"lunge_vx,omitempty"`
	LungeVY          int                   `json:"lunge_vy,omitempty"`
	AimMap           []int                 `json:"aim_map,omitempty"`
	AimThresholds    []int                 `json:"aim_thresholds,omitempty"`
	RelativeScroll   bool                  `json:"relative_scroll"`
	SpawnDelay       int                   `json:"spawn_delay,omitempty"`
	TurnTicks        int                   `json:"turn_ticks,omitempty"`
	HomingTurns      int                   `json:"homing_turns,omitempty"`
	ShootMin         int                   `json:"shoot_min,omitempty"`
	ShootMax         int                   `json:"shoot_max,omitempty"`
	AimAnimation     []int                 `json:"aim_animation,omitempty"`
	AimOffsetsX      []int                 `json:"aim_offsets_x,omitempty"`
	AimOffsetsY      []int                 `json:"aim_offsets_y,omitempty"`
	ShotVX           []int                 `json:"shot_vx,omitempty"`
	ShotVY           []int                 `json:"shot_vy,omitempty"`
	CollisionFrames  []CollisionFrame      `json:"collision_frames,omitempty"`
	Carrier          *CarrierData          `json:"carrier,omitempty"`
	FixedVX          int                   `json:"fixed_vx,omitempty"`
	FixedVY          int                   `json:"fixed_vy,omitempty"`
	StampWord        uint16                `json:"stamp_word,omitempty"`
	StampProbeX      int                   `json:"stamp_probe_x,omitempty"`
	StampProbeY      int                   `json:"stamp_probe_y,omitempty"`
	Hopper           *HopperData           `json:"hopper,omitempty"`
	RouteCrawler     *RouteCrawlerData     `json:"route_crawler,omitempty"`
	Stalker          *StalkerData          `json:"stalker,omitempty"`
	TilePulse        *TilePulseData        `json:"tile_pulse,omitempty"`
	QuarterTurner    *QuarterTurnerData    `json:"quarter_turner,omitempty"`
	Spitter          *SpitterData          `json:"spitter,omitempty"`
	WallCrawler      *WallCrawlerData      `json:"wall_crawler,omitempty"`
	GroundGunner     *GroundGunnerData     `json:"ground_gunner,omitempty"`
}

// Checkpoint preserves the original restart scroll position and event cursor.
type Checkpoint struct {
	Scroll       int    `json:"scroll"`
	EventAddress uint32 `json:"event_address"`
}

// Formation describes a timed member of an original scripted enemy group.
type Formation struct {
	Y         int `json:"y"`
	Delay     int `json:"delay"`
	Animation int `json:"animation"`
}

// ReadManifest loads the runtime index from either disk or embedded assets.
func ReadManifest(files fs.FS) (Manifest, error) {
	data, err := fs.ReadFile(files, "manifest.json")
	if err != nil {
		return Manifest{}, err
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, err
	}
	if manifest.Version != 1 || manifest.ADFSHA256 != ADFSHA256 {
		return Manifest{}, fmt.Errorf("unsupported asset manifest")
	}
	return manifest, nil
}

// Extract validates the ADF and recreates assets using translated Go decoders.
// It also writes ignored raw ranges for reproducible asset inspection.
func Extract(adfPath, outputDir string) (Manifest, error) {
	disk, err := os.ReadFile(adfPath)
	if err != nil {
		return Manifest{}, err
	}
	sum := sha256.Sum256(disk)
	if hex.EncodeToString(sum[:]) != ADFSHA256 {
		return Manifest{}, fmt.Errorf("unsupported ADF SHA-256 %x; expected %s", sum, ADFSHA256)
	}
	if len(disk) != 901120 {
		return Manifest{}, fmt.Errorf("expected a 901120-byte Amiga DD disk")
	}
	for _, directory := range []string{"", "sprites", "audio", "raw"} {
		if err := os.MkdirAll(filepath.Join(outputDir, directory), 0755); err != nil {
			return Manifest{}, err
		}
	}
	write := func(name string, data []byte) error {
		return os.WriteFile(filepath.Join(outputDir, name), data, 0644)
	}
	writeImage := func(name string, img image.Image) error {
		file, err := os.Create(filepath.Join(outputDir, name))
		if err != nil {
			return err
		}
		if err := png.Encode(file, img); err != nil {
			file.Close()
			return err
		}
		return file.Close()
	}
	loader, err := unpackTitle(disk)
	if err != nil {
		return Manifest{}, err
	}
	if err := write("raw/title-loader.bin", loader); err != nil {
		return Manifest{}, err
	}
	manifest := Manifest{Version: 1, ADFSHA256: ADFSHA256, Title: "title.png", Font: "font.png", Digits: "digits.png", HUD: "hud.png", Life: "life.png", TitleMusic: "audio/title.mdat", TitleSamples: "audio/title.smpl"}
	titlePalette := words(loader[0x79fea-0x6efa0 : 0x79fea-0x6efa0+64])
	title := image.NewRGBA(image.Rect(0, 0, 320, 200))
	for y := 0; y < 200; y++ {
		for x := 0; x < 320; x++ {
			index := 0
			for plane := 0; plane < 5; plane++ {
				index |= int(loader[plane*8000+y*40+x/8]>>(7-x%8)&1) << plane
			}
			title.SetRGBA(x, y, amigaColor(titlePalette[index]))
		}
	}
	if err := writeImage(manifest.Title, title); err != nil {
		return Manifest{}, err
	}
	// The loader supplies absolute addresses to the TFMX player. Preserve its
	// original sample range, song data, and period table without re-encoding.
	music, _ := diskImage(disk).tracks(4, 24)
	for name, data := range map[string][]byte{
		"audio/title.mdat":    music[0x1da28:0x1f73c],
		"audio/title.smpl":    music[0x1468:0x1da28],
		"audio/title.periods": music[0xf8e:0xfee],
		"raw/music.bin":       music,
	} {
		if err := write(name, data); err != nil {
			return Manifest{}, err
		}
	}
	core, _ := diskImage(disk).tracks(28, 5)
	static, _ := diskImage(disk).tracks(33, 29)
	if err := write("raw/core.bin", core); err != nil {
		return Manifest{}, err
	}
	if err := write("raw/static.bin", static); err != nil {
		return Manifest{}, err
	}
	// Reconstruct only original data addresses. These bytes are never executed
	// by the game engine and are needed solely during offline asset extraction.
	memory := make([]byte, 0x80000)
	copy(memory[0xa4:], core)
	copy(memory[0x6000:], static)
	manifest.Display = extractDisplay(memory, false)
	checkpointReset := extractCheckpointResetCommon(memory)
	manifest.Records = extractRecords(memory)
	presentation, presentationErr := diskImage(disk).tracks(62, 5)
	if presentationErr != nil {
		return Manifest{}, presentationErr
	}
	if err := write("raw/presentation.bin", presentation); err != nil {
		return Manifest{}, err
	}
	menu, menuSprites, menuErr := extractMenu(memory, presentation, writeImage)
	if menuErr != nil {
		return Manifest{}, menuErr
	}
	manifest.Menu = menu
	manifest.Menu.Music, menuErr = extractMenuMusic(presentation, write)
	if menuErr != nil {
		return Manifest{}, menuErr
	}
	manifest.Sprites = append(manifest.Sprites, menuSprites...)
	row, recordsFont := extractRecordImages(memory, presentation)
	manifest.Records.RowImage, manifest.Records.FontImage = "records-row.png", "records-font.png"
	if err := writeImage(manifest.Records.RowImage, row); err != nil {
		return Manifest{}, err
	}
	if err := writeImage(manifest.Records.FontImage, recordsFont); err != nil {
		return Manifest{}, err
	}
	manifest.InitialHighScore = bcdScore(binary.BigEndian.Uint32(memory[0x5764:]))
	// Copper 0x53F8..0x541E changes COLOR01 on each of the five filled
	// charge-bar scanlines, then restores white before the score digits.
	for row := 0; row < 5; row++ {
		address := 0x53f8 + row*8
		if binary.BigEndian.Uint16(memory[address:]) != uint16(0xee01+row*0x100) || binary.BigEndian.Uint16(memory[address+4:]) != 0x182 {
			return Manifest{}, fmt.Errorf("invalid original HUD charge copper row %d", row)
		}
		manifest.HUDChargeColors = append(manifest.HUDChargeColors, binary.BigEndian.Uint16(memory[address+6:]))
	}
	// Human reset writes two at 0x984; spawn 0x55A decrements it to one.
	// Death 0x752 ends only when this signed counter is negative, so values
	// one, zero and minus one each permit an active ship. HUD 0xB96 instead
	// draws counter+1 spare icons, explaining the two initial HUD glyphs.
	manifest.InitialLives = int(binary.BigEndian.Uint16(memory[0x986:])) + 1
	for address := 0x46a2; address+4 <= 0x46ba; address += 4 {
		value := binary.BigEndian.Uint32(memory[address:])
		if value == 0 {
			break
		}
		manifest.ExtraLifeThresholds = append(manifest.ExtraLifeThresholds, bcdScore(value))
	}
	for number := 0; number < 14; number++ {
		table := memory[0x5b08+number*12:]
		address := int(binary.BigEndian.Uint32(table))
		length := int(binary.BigEndian.Uint16(table[4:])) * 2
		if address < 0x6000 || address+length > 0x2de00 {
			return Manifest{}, fmt.Errorf("invalid sound %d sample range", number)
		}
		sound := Sound{Number: number, File: fmt.Sprintf("audio/effect-%d.smpl", number), Address: uint32(address), Length: length, Period: int(binary.BigEndian.Uint16(table[6:])), Volume: int(binary.BigEndian.Uint16(table[8:])), Priority: int(binary.BigEndian.Uint16(table[10:]))}
		if err := write(sound.File, memory[address:address+length]); err != nil {
			return Manifest{}, err
		}
		manifest.Sounds = append(manifest.Sounds, sound)
	}
	spritePalette := words(memory[0x4232:0x4252])
	for frame := 0; frame < 5; frame++ {
		name := fmt.Sprintf("ship-%d", frame)
		file := "sprites/" + name + ".png"
		if err := writeImage(file, attachedSprite(memory, 0x6000+frame*256, 32, 14, 64, spritePalette)); err != nil {
			return Manifest{}, err
		}
		manifest.Sprites = append(manifest.Sprites, Sprite{Name: name, File: file, Width: 32, Height: 14, Address: uint32(0x6000 + frame*256), Frame: frame})
	}
	// The animated orbiting bit uses the same attached-sprite encoding.
	for frame := 0; frame < 12; frame++ {
		name := fmt.Sprintf("bit-%d", frame)
		file := "sprites/" + name + ".png"
		if err := writeImage(file, attachedSprite(memory, 0x6500+frame*112, 16, 12, 56, spritePalette)); err != nil {
			return Manifest{}, err
		}
		manifest.Sprites = append(manifest.Sprites, Sprite{Name: name, File: file, Width: 16, Height: 12, Address: uint32(0x6500 + frame*112), Frame: frame})
	}
	// The text renderer uses a 48-character two-plane 8x8 font. Its planes
	// are 384 bytes apart, as shown by the original routine at RAM 0x3972.
	fontChars := memory[0x570b:0x573b]
	manifest.FontChars = string(fontChars)
	font := image.NewRGBA(image.Rect(0, 0, len(fontChars)*8, 8))
	fontPalette := []color.RGBA{{}, amigaColor(0x999), amigaColor(0xacd), amigaColor(0xfff)}
	for index := range fontChars {
		for y := 0; y < 8; y++ {
			for x := 0; x < 8; x++ {
				value := int(memory[0x2ceb8+index*8+y]>>(7-x)&1) | int(memory[0x2ceb8+0x180+index*8+y]>>(7-x)&1)<<1
				font.SetRGBA(index*8+x, y, fontPalette[value])
			}
		}
	}
	if err := writeImage(manifest.Font, font); err != nil {
		return Manifest{}, err
	}
	digits := image.NewRGBA(image.Rect(0, 0, 80, 7))
	for digit := 0; digit < 10; digit++ {
		for y := 0; y < 7; y++ {
			for x := 0; x < 8; x++ {
				if memory[0x46ba+y*10+digit]>>(7-x)&1 != 0 {
					digits.SetRGBA(digit*8+x, y, color.RGBA{255, 255, 255, 255})
				}
			}
		}
	}
	if err := writeImage(manifest.Digits, digits); err != nil {
		return Manifest{}, err
	}
	// The score strip occupies the final fifteen display rows. Its copper list
	// switches to three planes, each 600 bytes, starting at original RAM 0x2C7B0.
	hudPalette := []uint16{0, 0xfff, 0x999, 0xacd, 0xfff, 0x89f, 0x00c, 0xe00}
	hud := image.NewPaletted(image.Rect(0, 0, 320, 15), indexedPalette(hudPalette, false))
	for y := 0; y < 15; y++ {
		for x := 0; x < 320; x++ {
			value := 0
			for plane := 0; plane < 3; plane++ {
				value |= int(memory[0x2c7b0+plane*600+y*40+x/8]>>(7-x%8)&1) << plane
			}
			hud.SetColorIndex(x, y, uint8(value))
		}
	}
	if err := writeImage(manifest.HUD, hud); err != nil {
		return Manifest{}, err
	}
	// Core 0xB96 draws an eight-by-six icon from three six-byte planes at 0x470E.
	// Its colors are the fixed HUD palette, independent of the active level.
	life := image.NewRGBA(image.Rect(0, 0, 8, 6))
	for y := 0; y < 6; y++ {
		for x := 0; x < 8; x++ {
			value := 0
			for plane := 0; plane < 3; plane++ {
				value |= int(memory[0x470e+plane*6+y]>>(7-x)&1) << plane
			}
			if value != 0 {
				life.SetRGBA(x, y, amigaColor(hudPalette[value]))
			}
		}
	}
	if err := writeImage(manifest.Life, life); err != nil {
		return Manifest{}, err
	}
	manifest.Sprites = append(manifest.Sprites, Sprite{Name: "life", File: manifest.Life, Width: 8, Height: 6, Address: 0x470e})
	var firstPalette []uint16
	commonRequiredFrames := map[uint32]int{}
	for index := 0; index < 8; index++ {
		table := core[0x41b4-0xa4+index*10:]
		track, count := int(binary.BigEndian.Uint16(table)), int(binary.BigEndian.Uint16(table[2:]))
		packedTrack, packedCount := int(binary.BigEndian.Uint16(table[4:])), int(binary.BigEndian.Uint16(table[6:]))
		rowBytes := int(binary.BigEndian.Uint16(table[8:]))
		levelData, err := diskImage(disk).tracks(track, count)
		if err != nil {
			return Manifest{}, err
		}
		packed, err := diskImage(disk).tracks(packedTrack, packedCount)
		if err != nil {
			return Manifest{}, err
		}
		unpacked, err := unpackLevel(packed)
		if err != nil {
			return Manifest{}, fmt.Errorf("level %d: %w", index+1, err)
		}
		if err := write(fmt.Sprintf("raw/level-%d.bin", index+1), levelData); err != nil {
			return Manifest{}, err
		}
		if err := write(fmt.Sprintf("raw/level-%d-unpacked.bin", index+1), unpacked); err != nil {
			return Manifest{}, err
		}
		copy(memory[0x4a2e4:], levelData)
		copy(memory[0x4d300:], unpacked)
		mapAddress := int(binary.BigEndian.Uint32(memory[0x4a30c:]))
		paletteAddress := int(binary.BigEndian.Uint32(memory[0x4a308:]))
		tileAddress := int(binary.BigEndian.Uint32(memory[0x4a310:]))
		mapHeight := 13
		if index == 2 {
			mapHeight = 26
		}
		// Level four keeps a pristine copy before its dynamically modified map.
		// Its initialization routine at 0x4B052 copies 0x8BC words from 0x50F00.
		if index == 3 {
			copy(memory[mapAddress:mapAddress+rowBytes*13], memory[0x50f00:0x50f00+rowBytes*13])
		}
		if mapAddress < 0x4d300 || mapAddress+rowBytes*mapHeight > len(memory) || (index != 3 && mapAddress+rowBytes*mapHeight > 0x4d300+len(unpacked)) || paletteAddress < 0x4a2e4 || paletteAddress+30 > 0x4a2e4+len(levelData) {
			return Manifest{}, fmt.Errorf("level %d has invalid graphic addresses", index+1)
		}
		palette := append([]uint16{0}, words(memory[paletteAddress:paletteAddress+30])...)
		if index == 0 {
			firstPalette = append([]uint16(nil), palette...)
		}
		level := Level{Number: index + 1, File: fmt.Sprintf("level-%d.png", index+1), Width: rowBytes / 2 * 16, Height: mapHeight * 16, MapWidth: rowBytes / 2, MapHeight: mapHeight, Palette: palette, TileMap: words(memory[mapAddress : mapAddress+rowBytes*mapHeight])}
		weapons, weaponSprites, err := extractWeapons(memory, palette, index+1, writeImage)
		if err != nil {
			return Manifest{}, err
		}
		level.Weapons = &weapons
		manifest.Sprites = append(manifest.Sprites, weaponSprites...)
		level.EventClock = "scroll"
		if index == 2 {
			level.EventClock = "tick"
			level.InitialScrollY = 120
		}
		music, err := extractLevelMusic(memory, index+1, 0x4d300+len(unpacked), write)
		if err != nil {
			return Manifest{}, err
		}
		level.Music = music
		if index == 7 {
			level.EndingMusic, err = extractEndingMusic(memory, 0x4d300+len(unpacked), write)
			if err != nil {
				return Manifest{}, err
			}
		}
		if index == 7 {
			level.Boss, err = extractBossEight(memory)
			if err != nil {
				return Manifest{}, err
			}
		}
		level.Events, err = extractEvents(memory, 0x4a2e4, 0x4a2e4+len(levelData), index+1)
		if err != nil {
			return Manifest{}, err
		}
		requiredFrames := map[uint32]int{}
		if index == 7 {
			// Ending actors choose one of four frames directly. Their unused
			// four-byte animation table intentionally has no end marker.
			requiredFrames[0x4aa4c] = 4
		}
		for _, event := range level.Events {
			requiredFrames[event.Descriptor] = max(requiredFrames[event.Descriptor], len(event.CollisionFrames))
			for _, frame := range event.AimAnimation {
				requiredFrames[event.Descriptor] = max(requiredFrames[event.Descriptor], frame+1)
			}
			if event.Descriptor < 0x6000 {
				commonRequiredFrames[event.Descriptor] = max(commonRequiredFrames[event.Descriptor], requiredFrames[event.Descriptor])
			}
			if event.Reactor != nil {
				for _, frame := range event.Reactor.ShotAnimation {
					requiredFrames[event.Reactor.ShotDescriptor] = max(requiredFrames[event.Reactor.ShotDescriptor], frame+1)
				}
			}
			if event.BattleshipTurret != nil {
				for _, stream := range event.BattleshipTurret.ProjectileAnimations {
					for _, frame := range stream {
						requiredFrames[event.BattleshipTurret.ProjectileDescriptor] = max(requiredFrames[event.BattleshipTurret.ProjectileDescriptor], frame+1)
					}
				}
			}
			if event.FireWorm != nil {
				for _, descriptor := range []uint32{event.FireWorm.HeadDescriptor, event.FireWorm.BodyDescriptor, event.FireWorm.TailDescriptor} {
					requiredFrames[descriptor] = 16
				}
			}
		}
		checkpoint := int(binary.BigEndian.Uint32(memory[0x4a304:]))
		for count := 0; count < 128 && checkpoint >= 0x4a2e4 && checkpoint+6 <= 0x4a2e4+len(levelData); count++ {
			scroll := int(binary.BigEndian.Uint16(memory[checkpoint:]))
			level.Checkpoints = append(level.Checkpoints, Checkpoint{Scroll: scroll, EventAddress: binary.BigEndian.Uint32(memory[checkpoint+2:])})
			if scroll == 0 {
				break
			}
			checkpoint += 6
		}
		scrollScript := int(binary.BigEndian.Uint32(memory[0x4a314:]))
		level.ScrollScript, err = extractScrollScript(memory, scrollScript, 0x4a2e4, 0x4a2e4+len(levelData))
		if err != nil {
			return Manifest{}, fmt.Errorf("level %d: %w", index+1, err)
		}
		level.ScrollSpeed = float64(level.ScrollScript[0].XDelta) / float64(level.ScrollScript[0].XPeriod)
		reset, err := extractStageCheckpointReset(memory, level, rowBytes, 0x4a2e4+len(levelData))
		if err != nil {
			return Manifest{}, err
		}
		reset.MapRestore, err = extractCheckpointMapRestore(memory, level, rowBytes, 0x4a2e4+len(levelData))
		if err != nil {
			return Manifest{}, err
		}
		if index == 0 {
			reset.MusicStart, err = extractCheckpointMusicStart(memory, level.Music, 0x4d300+len(unpacked))
			if err != nil {
				return Manifest{}, err
			}
		}
		checkpointReset.Stages = append(checkpointReset.Stages, reset)
		tileEnd := mapAddress
		if index == 3 {
			tileEnd = 0x50f00
		}
		level.TileCount = (tileEnd - tileAddress) / 128
		level.TileAtlas = fmt.Sprintf("tiles-%d.png", index+1)
		atlas, err := extractTileAtlas(memory, tileAddress, level.TileCount, palette)
		if err != nil {
			return Manifest{}, err
		}
		// Preserve the original planar words separately from code-bearing raw
		// ranges so native DMA can read tile assets without an executable image.
		if err := write(fmt.Sprintf("tiles-%d.bin", index+1), memory[tileAddress:tileAddress+level.TileCount*128]); err != nil {
			return Manifest{}, err
		}
		if err := writeImage(level.TileAtlas, atlas); err != nil {
			return Manifest{}, err
		}
		if index == 0 {
			level.Boss, err = extractBossOne(memory, mapAddress, rowBytes)
			if err != nil {
				return Manifest{}, err
			}
		}
		if index == 1 {
			level.Boss, err = extractBossTwo(memory, mapAddress, rowBytes)
			if err != nil {
				return Manifest{}, err
			}
			cave, err := extractCave(memory, rowBytes, 0x4d300+len(unpacked))
			if err != nil {
				return Manifest{}, err
			}
			data, err := json.MarshalIndent(cave, "", "  ")
			if err != nil {
				return Manifest{}, err
			}
			if err := write("cave.json", append(data, '\n')); err != nil {
				return Manifest{}, err
			}
			requiredFrames[cave.ShotDescriptor] = 9
		}
		if index == 2 {
			level.Boss, err = extractBossThree(memory)
			if err != nil {
				return Manifest{}, err
			}
		}
		if index == 3 {
			level.Boss, err = extractBossFour(memory)
			if err != nil {
				return Manifest{}, err
			}
		}
		if index == 5 {
			level.Boss, err = extractBossSix(memory)
			if err != nil {
				return Manifest{}, err
			}
		}
		if index == 6 {
			level.Boss, err = extractBossSeven(memory, level.Events)
			if err != nil {
				return Manifest{}, err
			}
		}
		if index == 4 {
			level.Boss, err = extractBossFive(memory, level.Events)
			if err != nil {
				return Manifest{}, err
			}
		}
		if level.Boss != nil && level.Boss.Second != nil {
			for _, component := range level.Boss.Second.Components {
				sequences := [][]int{component.Animation}
				if component.Animated {
					sequences = append(sequences, level.Boss.Second.DamagedAnimation)
				}
				for _, sequence := range sequences {
					for _, frame := range sequence {
						requiredFrames[component.Descriptor] = max(requiredFrames[component.Descriptor], frame+1)
					}
				}
			}
		}
		if level.Boss != nil && level.Boss.Fourth != nil {
			// The spawner clears animation advancement on all three bodies.
			// Only stream index zero is used; later shared-stream indices refer
			// to other banks and can incorrectly reject the third body's art.
			for _, body := range level.Boss.Fourth.Bodies {
				requiredFrames[body.Descriptor] = 1
			}
		}
		if level.Boss != nil && level.Boss.Fifth != nil {
			// The core's stream index stays zero; the next graphics bank is
			// arm artwork rather than a second independently animated core.
			requiredFrames[level.Boss.Fifth.Descriptor] = 1
		}
		if level.Boss != nil && level.Boss.Eighth != nil {
			for _, stream := range level.Boss.Eighth.CellAnimation {
				for _, frame := range stream {
					requiredFrames[level.Boss.Eighth.CellDescriptor] = max(requiredFrames[level.Boss.Eighth.CellDescriptor], frame+1)
				}
			}
		}
		background := image.NewPaletted(image.Rect(0, 0, level.Width, level.Height), indexedPalette(palette, false))
		for ty := 0; ty < mapHeight; ty++ {
			for tx := 0; tx < level.MapWidth; tx++ {
				tile := int(level.TileMap[ty*level.MapWidth+tx] & 0xfff)
				address := tileAddress + tile*128
				if address < 0x4d300 || address+128 > mapAddress {
					return Manifest{}, fmt.Errorf("level %d references invalid tile %d", index+1, tile)
				}
				for y := 0; y < 16; y++ {
					for x := 0; x < 16; x++ {
						value := planarIndex(memory, address, x, y, 2)
						background.SetColorIndex(tx*16+x, ty*16+y, uint8(value))
					}
				}
			}
		}
		if err := writeImage(level.File, background); err != nil {
			return Manifest{}, err
		}
		manifest.Levels = append(manifest.Levels, level)
		sprites, err := findBobs(memory, 0x4a2e4, min(0x4cb00, 0x4a2e4+len(levelData)), 0x4d300, 0x4d300+len(unpacked), palette, index+1, writeImage, requiredFrames)
		if err != nil {
			return Manifest{}, err
		}
		for _, sprite := range sprites {
			manifest.Sprites = append(manifest.Sprites, sprite)
		}
		// Several native families keep their descriptor in the level header but
		// use common color data, including stage seven's tile-patch pulse. Decode
		// those same validated descriptors against their original static bank.
		shared, err := findBobs(memory, 0x4a2e4, min(0x4cb00, 0x4a2e4+len(levelData)), 0x6000, 0x2de00, palette, index+1, writeImage, requiredFrames)
		if err != nil {
			return Manifest{}, err
		}
		manifest.Sprites = append(manifest.Sprites, shared...)
		// Decode common color indices again with this original stage palette.
		// Never recover indices by reversing an RGB image: duplicate colors in
		// the original palette would make that conversion lose information.
		common, err := findBobs(memory, 0xa4, 0x6000, 0x6000, 0x2de00, palette, index+1, writeImage, commonRequiredFrames)
		if err != nil {
			return Manifest{}, err
		}
		manifest.Sprites = append(manifest.Sprites, common...)
	}
	// Keep level-zero common variants for existing aliases and standalone art.
	// Runtime objects select the stage-specific variants generated above.
	common, err := findBobs(memory, 0xa4, 0x6000, 0x6000, 0x2de00, firstPalette, 0, writeImage, commonRequiredFrames)
	if err != nil {
		return Manifest{}, err
	}
	manifest.Sprites = append(manifest.Sprites, common...)
	// Player projectiles use a separate sixteen-byte blitter table rather than
	// the common object descriptors. Its first five records are the normal shot
	// and the four diagonal spread directions.
	for index := 0; index < 5; index++ {
		address := 0x4c3a + index*16
		size := int(binary.BigEndian.Uint16(memory[address+6:]))
		mask := int(binary.BigEndian.Uint32(memory[address+8:]))
		colors := int(binary.BigEndian.Uint32(memory[address+12:]))
		width, height := (size&63-1)*16, (size>>6)/4
		name := fmt.Sprintf("shot-%d", index)
		file := "sprites/" + name + ".png"
		if err := writeImage(file, blitterSprite(memory, colors, mask, width, height, firstPalette)); err != nil {
			return Manifest{}, err
		}
		manifest.Sprites = append(manifest.Sprites, Sprite{Name: name, File: file, Width: width, Height: height, Address: uint32(address), Frame: index})
	}
	// Charged shots use four thirty-byte records in descending charge order.
	// Their blitter size, color pointer, and color-to-mask distance are explicit.
	for index := 0; index < 4; index++ {
		address := 0x4ca2 + (3-index)*30
		size := int(binary.BigEndian.Uint16(memory[address+18:]))
		colors := int(binary.BigEndian.Uint32(memory[address+22:]))
		mask := colors + int(binary.BigEndian.Uint32(memory[address+26:]))
		width, height := (size&63-1)*16, (size>>6)/4
		name := fmt.Sprintf("beam-%d", index)
		file := "sprites/" + name + ".png"
		if err := writeImage(file, blitterSprite(memory, colors, mask, width, height, firstPalette)); err != nil {
			return Manifest{}, err
		}
		manifest.Sprites = append(manifest.Sprites, Sprite{Name: name, File: file, Width: width, Height: height, Address: uint32(address), Frame: index})
	}
	for frame := 0; frame < 6; frame++ {
		name := fmt.Sprintf("force-%d", frame)
		file := "sprites/" + name + ".png"
		if err := writeImage(file, attachedSprite(memory, 0x6a40+frame*256, 32, 14, 64, spritePalette)); err != nil {
			return Manifest{}, err
		}
		manifest.Sprites = append(manifest.Sprites, Sprite{Name: name, File: file, Width: 32, Height: 14, Address: uint32(0x6a40 + frame*256), Frame: frame})
	}
	// Descriptive aliases retain the original data address and file provenance.
	aliases := map[uint32]string{0x4b99a: "fighter", 0x4baa8: "crawler", 0x4bea0: "turret", 0x4c068: "pickup", 0x4c08e: "weapon", 0x493a: "explosion", 0x4972: "spark", 0x4bd46: "beam-shot", 0x19d90: "enemy-shot"}
	for _, sprite := range append([]Sprite(nil), manifest.Sprites...) {
		if alias, ok := aliases[sprite.Address]; ok && sprite.Level <= 1 {
			sprite.Name = fmt.Sprintf("%s-%d", alias, sprite.Frame)
			manifest.Sprites = append(manifest.Sprites, sprite)
		}
	}
	sort.Slice(manifest.Sprites, func(i, j int) bool { return manifest.Sprites[i].Name < manifest.Sprites[j].Name })
	resetData, err := json.MarshalIndent(checkpointReset, "", "  ")
	if err != nil {
		return Manifest{}, err
	}
	if err := write("checkpoint-reset.json", append(resetData, '\n')); err != nil {
		return Manifest{}, err
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return Manifest{}, err
	}
	if err := write("manifest.json", append(data, '\n')); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

func words(data []byte) []uint16 {
	result := make([]uint16, len(data)/2)
	for index := range result {
		result[index] = binary.BigEndian.Uint16(data[index*2:])
	}
	return result
}

func amigaColor(value uint16) color.RGBA {
	return color.RGBA{byte(value>>8&15) * 17, byte(value>>4&15) * 17, byte(value&15) * 17, 255}
}

// indexedPalette keeps duplicate original color entries distinct. Masked
// blitter pixels use index 16 for transparency, so opaque color zero survives.
func indexedPalette(values []uint16, masked bool) color.Palette {
	result := make(color.Palette, len(values))
	for index, value := range values {
		result[index] = amigaColor(value)
	}
	if masked {
		result = append(result, color.RGBA{})
	}
	return result
}

func maskedIndexedImage(width, height int, palette []uint16) *image.Paletted {
	img := image.NewPaletted(image.Rect(0, 0, width, height), indexedPalette(palette, true))
	for index := range img.Pix {
		img.Pix[index] = uint8(len(palette))
	}
	return img
}

// planarIndex reads four bitplanes interleaved within each source scanline.
func planarIndex(memory []byte, address, x, y, rowBytes int) int {
	value := 0
	for plane := 0; plane < 4; plane++ {
		value |= int(memory[address+y*rowBytes*4+plane*rowBytes+x/8]>>(7-x%8)&1) << plane
	}
	return value
}

// attachedSprite combines paired Amiga hardware sprites into four-bit pixels.
func attachedSprite(memory []byte, address, width, height, stride int, palette []uint16) image.Image {
	result := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			base := address + x/16*stride*2 + 4 + y*4
			value := 0
			for plane := 0; plane < 4; plane++ {
				value |= int(memory[base+plane/2*stride+plane%2*2+x%16/8]>>(7-x%8)&1) << plane
			}
			if value != 0 {
				result.SetRGBA(x, y, amigaColor(palette[value]))
			}
		}
	}
	return result
}

func blitterSprite(memory []byte, colors, mask, width, height int, palette []uint16) image.Image {
	img := maskedIndexedImage(width, height, palette)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if planarIndex(memory, mask, x, y, width/8) != 0 {
				img.SetColorIndex(x, y, uint8(planarIndex(memory, colors, x, y, width/8)))
			}
		}
	}
	return img
}

// findBobs recognizes the original 32-byte blitter object descriptors. It
// validates their geometry, mask sizes, frame stride, and animation pointers
// before accepting them; executable bytes cannot accidentally become art.
func findBobs(memory []byte, start, end, graphicsStart, graphicsEnd int, palette []uint16, level int, writeImage func(string, image.Image) error, required ...map[uint32]int) ([]Sprite, error) {
	var result []Sprite
	for address := start; address+32 <= end; address += 2 {
		data := memory[address:]
		size := int(binary.BigEndian.Uint16(data))
		widthWords, rows := size&63, size>>6
		if widthWords < 2 || widthWords > 26 || rows == 0 || rows%4 != 0 || rows/4 > 128 {
			continue
		}
		width, height := (widthWords-1)*16, rows/4
		mask := int(binary.BigEndian.Uint32(data[16:]))
		stride := int(binary.BigEndian.Uint16(data[4:]))
		base := int(binary.BigEndian.Uint32(data[12:]))
		animation := int(binary.BigEndian.Uint32(data[20:]))
		if int(binary.BigEndian.Uint16(data[2:])) != 52-widthWords*2 || mask != width/8*height*4 || stride != mask*2 || base < graphicsStart || base+stride > graphicsEnd {
			continue
		}
		if animation < start || animation >= end {
			continue
		}
		frames := 0
		for _, counts := range required {
			frames = max(frames, counts[uint32(address)])
		}
		valid := frames > 0
		knownFrames := valid
		for offset := 0; !knownFrames && offset < 64 && animation+offset < end; offset++ {
			frame := memory[animation+offset]
			if frame >= 128 {
				valid = true
				break
			}
			frames = max(frames, int(frame)+1)
		}
		for _, counts := range required {
			frames = max(frames, counts[uint32(address)])
		}
		if !valid || frames == 0 || frames > 32 || base+stride*frames > graphicsEnd {
			continue
		}
		collision, _ := collisionFrames(memory, int(binary.BigEndian.Uint32(data[24:])), frames)
		for frame := 0; frame < frames; frame++ {
			name := fmt.Sprintf("object-%d-%05x-%d", level, address, frame)
			file := "sprites/" + name + ".png"
			img := maskedIndexedImage(width, height, palette)
			for y := 0; y < height; y++ {
				for x := 0; x < width; x++ {
					value := planarIndex(memory, base+stride*frame, x, y, width/8)
					if planarIndex(memory, base+stride*frame+mask, x, y, width/8) != 0 {
						img.SetColorIndex(x, y, uint8(value))
					}
				}
			}
			if err := writeImage(file, img); err != nil {
				return nil, err
			}
			sprite := Sprite{Name: name, File: file, Width: width, Height: height, Address: uint32(address), Frame: frame, Level: level}
			if frame < len(collision) {
				bounds := collision[frame]
				sprite.CollisionFrame = &bounds
			}
			result = append(result, sprite)
		}
	}
	return result, nil
}

func extractEvents(memory []byte, start, end, level int) ([]Event, error) {
	u32 := func(address int) uint32 { return binary.BigEndian.Uint32(memory[address:]) }
	u16 := func(address int) int { return int(binary.BigEndian.Uint16(memory[address:])) }
	i16 := func(address int) int { return int(int16(binary.BigEndian.Uint16(memory[address:]))) }
	checkpoint := int(u32(0x4a304))
	address := 0
	for count := 0; count < 128 && checkpoint >= start && checkpoint+6 <= end; count++ {
		if u16(checkpoint) == 0 {
			address = int(u32(checkpoint + 2))
			break
		}
		checkpoint += 6
	}
	var events []Event
	for count := 0; count < 512 && address >= start && address+18 <= end; count++ {
		trigger := u16(address)
		if trigger == 0xffff {
			break
		}
		descriptor := u32(address + 14)
		if int32(descriptor) < 0 {
			descriptor = 0x51fc
		}
		if int(descriptor) < 0xa4 || int(descriptor)+40 > len(memory) {
			break
		}
		d := int(descriptor)
		event := Event{Address: uint32(address), Trigger: trigger, X: i16(address + 2), Y: i16(address + 4), Parameter: u32(address + 6), Count: i16(address + 10), Aux: i16(address + 12), Descriptor: descriptor, SpawnAddress: u32(d + 32), UpdateAddress: u32(d + 28), Health: i16(d + 8), Width: (u16(d)&63 - 1) * 16, Height: (u16(d) >> 6) / 4, AnimationTicks: int(memory[d+7]) + 1}
		priority, err := extractActorPriority(memory, level, event)
		if err != nil {
			return nil, err
		}
		event.Priority = priority
		event.Score = bcdScore(u32(d + 36))
		// Nonzero-period event descriptors use allocator 0x10B6 through
		// vector 0xCA. Native gameplay adds its random range per actor.
		event.AutomaticFire = u16(d + 10)
		animation := int(u32(d + 20))
		for index := 0; index < 64 && animation >= 0xa4 && animation+index < len(memory); index++ {
			frame := int(int8(memory[animation+index]))
			if frame < 0 {
				break
			}
			event.Animation = append(event.Animation, frame)
		}
		annotateEvent(memory, &event, start, end, level)
		frameCount := 1
		sequences := [][]int{event.Animation, event.MotionAnimation, event.AimAnimation}
		if event.Carrier != nil {
			d := event.Carrier
			sequences = append(sequences, d.AnimationFloat, d.AnimationGround, d.AnimationFloatReverse, d.AnimationGroundReverse)
		}
		if event.SpiralSentinel != nil {
			// The head and broken-body banks follow the initial eight poses.
			// Keep all seventeen art and collision records instead of replacing
			// the annotation with only the initial body animation's eight frames.
			sequences = append(sequences, event.SpiralSentinel.Body, event.SpiralSentinel.HeadStream)
		}
		if event.Hopper != nil {
			data := event.Hopper
			sequences = append(sequences, data.StandingLeft, data.StandingRight, data.JumpLeft, data.JumpRight, []int{data.FacingLeftFrame, data.FacingRightFrame, data.JumpInitialLeftFrame, data.JumpInitialRightFrame})
		}
		if event.Stalker != nil {
			sequences = append(sequences, event.Stalker.LeftAnimation, event.Stalker.RightAnimation)
		}
		if event.QuarterTurner != nil {
			d := event.QuarterTurner
			sequences = append(sequences, d.TurnFrame5, d.TurnFrame7)
			sequences = append(sequences, d.Animations...)
		}
		if event.GroundGunner != nil {
			d := event.GroundGunner
			sequences = append(sequences, d.WalkLeft, d.WalkRight, d.RawLeft, d.RawRight, d.RawFire)
		}
		for _, sequence := range sequences {
			for _, frame := range sequence {
				frameCount = max(frameCount, frame+1)
			}
		}
		event.CollisionFrames, _ = collisionFrames(memory, int(u32(d+24)), frameCount)
		annotateReactor(memory, &event, level)
		annotateBattleshipTurret(memory, &event, level)
		annotateGroundWalker(memory, &event, level)
		events = append(events, event)
		address += 18
	}
	return events, nil
}

func bcdScore(value uint32) int {
	result, multiplier := 0, 1
	for range 8 {
		result += int(value&15) * multiplier
		value >>= 4
		multiplier *= 10
	}
	return result
}

func extractLevelMusic(memory []byte, level, graphicsEnd int, write func(string, []byte) error) (*LevelMusic, error) {
	driver := int(binary.BigEndian.Uint32(memory[0x4a328:]))
	if driver < 0x4d300 || driver+0x400 >= graphicsEnd {
		return nil, fmt.Errorf("level %d has invalid music driver", level)
	}
	data := memory[driver:graphicsEnd]
	music := &LevelMusic{File: fmt.Sprintf("audio/level-%d-music.bin", level), DriverAddress: uint32(driver), MainStride: 98, BossStride: 98}
	// The three original variants relocate their data through PC-relative LEAs.
	// Decode those operands offline instead of running the music driver.
	pc := func(offset int) int { return offset + 2 + int(int16(binary.BigEndian.Uint16(data[offset+2:]))) }
	if level == 1 {
		music.DataStart = 0x2fc
		music.Main, music.Boss1, music.Boss2 = pc(0x7e), pc(0x9c), pc(0x90)
		music.Jingles = []int{pc(0x22), pc(0x2e), pc(0x34)}
		music.MainPatterns, music.BossPatterns = pc(0x208), pc(0x208)
		music.SampleTable, music.Periods, music.Samples = pc(0x236), pc(0x254), pc(0x244)
	} else if level <= 4 {
		music.DataStart = 0x2f0
		music.Main, music.Boss1, music.Boss2 = pc(0x72), pc(0x90), pc(0x84)
		music.Jingles = []int{pc(0x22), pc(0x28)}
		music.MainPatterns, music.BossPatterns = pc(0x1fc), pc(0x1fc)
		music.SampleTable, music.Periods, music.Samples = pc(0x22a), pc(0x248), pc(0x238)
	} else {
		music.DataStart = 0x32a
		music.MainStride = int(binary.BigEndian.Uint16(data[0x98:]))
		music.Main, music.Boss1, music.Boss2 = pc(0x86), pc(0xcc), pc(0xac)
		music.Jingles = []int{pc(0x36), pc(0x3c)}
		music.MainPatterns, music.BossPatterns = pc(0x9a), pc(0xc0)
		music.SampleTable, music.Periods, music.Samples = pc(0x26c), pc(0x28a), pc(0x27a)
	}
	music.SampleCount = (music.Periods - music.SampleTable) / 8
	for _, offset := range []int{music.Main, music.Boss1, music.Boss2, music.MainPatterns, music.BossPatterns, music.SampleTable, music.Periods, music.Samples} {
		if offset < music.DataStart || offset >= len(data) {
			return nil, fmt.Errorf("level %d has invalid music data offset %#x", level, offset)
		}
	}
	music.Main -= music.DataStart
	music.Boss1 -= music.DataStart
	music.Boss2 -= music.DataStart
	music.MainPatterns -= music.DataStart
	music.BossPatterns -= music.DataStart
	music.SampleTable -= music.DataStart
	music.Periods -= music.DataStart
	music.Samples -= music.DataStart
	for index := range music.Jingles {
		music.Jingles[index] -= music.DataStart
	}
	trimmed, err := trimMusicData(data[music.DataStart:], music)
	if err != nil {
		return nil, fmt.Errorf("level%d music: %w", level, err)
	}
	if err = write(music.File, trimmed); err != nil {
		return nil, err
	}
	return music, nil
}
