package game

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"math"
	"strings"
	"unicode"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/olivierh59500/r-type/internal/sim"
	"github.com/olivierh59500/r-type/internal/source"
)

type spriteResource struct {
	Name    string `json:"name"`
	File    string `json:"file"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
	Address uint32 `json:"address"`
	Frame   int    `json:"frame"`
	Level   int    `json:"level"`
}
type levelResource struct {
	Number    int      `json:"number"`
	File      string   `json:"file"`
	Width     int      `json:"width"`
	Height    int      `json:"height"`
	MapWidth  int      `json:"map_width"`
	MapHeight int      `json:"map_height"`
	TileMap   []uint16 `json:"tile_map"`
	TileAtlas string   `json:"tile_atlas"`
}

func (g *Game) loadGraphics() error {
	g.spriteDetails = make(map[string]spriteResource)
	g.imageFiles = make(map[string]string)
	var manifest struct {
		Display             *source.DisplayData `json:"display"`
		FontChars           string              `json:"font_chars"`
		InitialHighScore    int                 `json:"initial_high_score"`
		ExtraLifeThresholds []int               `json:"extra_life_thresholds"`
		InitialLives        int                 `json:"initial_lives"`
		HUDChargeColors     []uint16            `json:"hud_charge_colors"`
		Records             *source.RecordsData `json:"records"`
		Menu                *source.MenuData    `json:"menu"`
		Sprites             []spriteResource    `json:"sprites"`
		Levels              []levelResource     `json:"levels"`
	}
	if err := json.Unmarshal(g.bundle.Manifest, &manifest); err != nil {
		return err
	}
	g.fontChars = manifest.FontChars
	g.display = manifest.Display
	if manifest.Menu != nil {
		g.menuDisplay = manifest.Menu.Display
	}
	stages, _, err := sim.LoadStagesFS(g.bundle.Files)
	if err != nil {
		return fmt.Errorf("original level data: %w", err)
	}
	if err := g.World.SetStages(stages); err != nil {
		return err
	}
	g.World.HighScore = manifest.InitialHighScore
	if err := g.World.SetRecords(manifest.Records); err != nil {
		return err
	}
	if err := g.World.SetMenu(manifest.Menu); err != nil {
		return err
	}
	if manifest.Menu != nil {
		for _, file := range []string{manifest.Menu.FontImage, manifest.Menu.FooterImage} {
			img, err := g.bundle.Image(file)
			if err != nil {
				return err
			}
			g.images[file] = ebiten.NewImageFromImage(img)
		}
	}
	if manifest.Records != nil {
		for key, file := range map[string]string{"records-row": manifest.Records.RowImage, "records-font": manifest.Records.FontImage} {
			if file != "" {
				img, err := g.bundle.Image(file)
				if err != nil {
					return err
				}
				g.images[key] = ebiten.NewImageFromImage(img)
			}
		}
	}
	if err := g.World.SetExtraLifeThresholds(manifest.ExtraLifeThresholds); err != nil {
		return err
	}
	if err := g.World.SetInitialLives(manifest.InitialLives); err != nil {
		return err
	}
	for _, resource := range manifest.Sprites {
		img, err := g.bundle.Image(resource.File)
		if err != nil {
			return fmt.Errorf("sprite %s: %w", resource.Name, err)
		}
		g.sprites[resource.Name] = ebiten.NewImageFromImage(img)
		g.spriteDetails[resource.Name] = resource
	}
	for _, level := range manifest.Levels {
		img, err := g.bundle.Image(level.File)
		if err != nil {
			return fmt.Errorf("stage %d: %w", level.Number, err)
		}
		g.images[fmt.Sprintf("level-%d", level.Number)] = ebiten.NewImageFromImage(img)
		g.imageFiles[fmt.Sprintf("level-%d", level.Number)] = level.File
		if level.TileAtlas != "" {
			tiles, err := g.bundle.Image(level.TileAtlas)
			if err != nil {
				return err
			}
			g.images[fmt.Sprintf("tiles-%d", level.Number)] = ebiten.NewImageFromImage(tiles)
			g.imageFiles[fmt.Sprintf("tiles-%d", level.Number)] = level.TileAtlas
		}
	}
	for _, name := range []string{"hud", "digits", "font"} {
		if img, err := g.bundle.Image(name + ".png"); err == nil {
			g.images[name] = ebiten.NewImageFromImage(img)
		}
	}
	if len(manifest.HUDChargeColors) != 5 {
		return fmt.Errorf("original HUD charge colors are missing; recreate runtime assets with cmd/extract")
	}
	g.loadChargeImages(manifest.HUDChargeColors)
	return nil
}

func (g *Game) sprite(screen *ebiten.Image, name string, x, y float64, flash ...bool) bool {
	// Common blitter objects use each stage's palette; hardware sprites retain
	// their separate fixed palette. Every variant comes from original planar data.
	details, found := g.spriteDetails[name]
	if !found {
		details, found = g.spriteDetails[name+"-0"]
	}
	if found && details.Address < 0x6000 {
		variant := fmt.Sprintf("object-%d-%05x-%d", g.World.Stage, details.Address, details.Frame)
		if g.sprites[variant] != nil {
			name = variant
		}
	}
	img := g.sprites[name]
	if img == nil {
		img = g.sprites[name+"-0"]
	}
	if img == nil {
		return false
	}
	if resource, ok := g.spriteDetails[name]; ok {
		img = g.paletteImage(resource.File, img)
		if len(flash) > 0 && flash[0] {
			img = g.flashImage(resource.File, img)
		}
	}
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(math.Round(x-float64(img.Bounds().Dx())/2), math.Round(y-float64(img.Bounds().Dy())/2))
	screen.DrawImage(img, op)
	return true
}

// blitterOffset preserves the original sum of 0x48C8 and the complementary
// BPLCON1 delay: fine + (15-fine). Relative to raw-minus-32 simulation
// coordinates and the raw-X=48 fetch, every gameplay BOB is one pixel left.
func (g *Game) blitterOffset() float64 {
	offset, _ := g.presentationGeometry()
	return float64(offset + 15)
}

func (g *Game) bob(screen *ebiten.Image, name string, x, y float64, flash ...bool) bool {
	return g.sprite(screen, name, x+g.blitterOffset(), y, flash...)
}

func (g *Game) drawWorld(screen *ebiten.Image) {
	w := g.World
	g.updatePalette()
	mapOriginX := 16 + g.blitterOffset()
	backgroundKey := fmt.Sprintf("level-%d", w.Stage)
	if background := g.paletteImage(g.imageFiles[backgroundKey], g.images[backgroundKey]); background != nil {
		start := max(0, int(w.Scroll))
		top := max(0, int(w.ScrollY))
		portion := background.SubImage(image.Rect(start, top, min(start+sim.Width, background.Bounds().Dx()), min(top+sim.PlayHeight, background.Bounds().Dy()))).(*ebiten.Image)
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(mapOriginX+math.Max(0, -w.Scroll), math.Max(0, -w.ScrollY))
		screen.DrawImage(portion, op)
	}
	tilesKey := fmt.Sprintf("tiles-%d", w.Stage)
	if tiles := g.paletteImage(g.imageFiles[tilesKey], g.images[tilesKey]); tiles != nil && w.Stage >= 1 && w.Stage <= len(w.Stages) {
		stage := w.Stages[w.Stage-1]
		changes := w.TileChanges
		if w.TerrainImageValid {
			changes = w.TerrainImageChanges
		}
		for index, word := range changes {
			x := float64(index%stage.MapWidth*16) + mapOriginX - w.Scroll
			y := float64(index/stage.MapWidth*16) - w.ScrollY
			if x+16 <= 16 || x >= sim.Width || y+16 <= 0 || y >= sim.PlayHeight {
				continue
			}
			id := int(word & 0xfff)
			columns := tiles.Bounds().Dx() / 16
			x0, y0 := id%columns*16, id/columns*16
			if y0+16 > tiles.Bounds().Dy() {
				continue
			}
			cell := tiles.SubImage(image.Rect(x0, y0, x0+16, y0+16)).(*ebiten.Image)
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(math.Round(x), math.Round(y))
			screen.DrawImage(cell, op)
		}
	}
	g.drawOriginalWeapons(screen, 0)
	if w.ActorImagesValid {
		for _, actor := range w.ActorImages {
			if actor.BlitterVisible(int(w.Scroll)) {
				g.bob(screen, actor.ActorImageName(), actor.X, actor.Y, actor.MaskFlash)
			}
		}
	}
	for _, enemy := range w.Enemies {
		if w.ActorImagesValid {
			break
		}
		if enemy.Hidden || enemy.Health <= 0 || enemy.PendingOriginalPass() {
			continue
		}
		name := []string{"fighter", "drifter", "turret", "carrier", "boss"}[enemy.Kind]
		if enemy.Sprite != "" {
			name = fmt.Sprintf("%s-%d", enemy.Sprite, enemy.Frame())
		}
		g.bob(screen, name, enemy.X, enemy.Y, enemy.MaskFlash)
	}
	for _, power := range w.PowerUps {
		if w.ActorImagesValid {
			break
		}
		if power.Original != nil && power.Age == 0 {
			continue
		}
		name := fmt.Sprintf("object-0-05288-%d", power.Frame())
		g.bob(screen, name, power.X, power.Y)
	}
	for _, bullet := range w.Bullets {
		if bullet.Original != nil || bullet.Hostile != nil {
			continue
		}
		name := "shot"
		if !bullet.Friendly {
			name = "enemy-shot"
		} else {
			switch bullet.Kind {
			case sim.Beam:
				name = fmt.Sprintf("beam-%d", min(3, max(0, (bullet.Damage-5)/2)))
			case sim.Missile:
				name = "missile"
			case sim.RedLaser:
				name = "red-laser"
			case sim.BlueLaser:
				name = "blue-laser"
			case sim.YellowLaser:
				name = "yellow-laser"
			case sim.ForceShot:
				name = "force-shot"
			}
		}
		if bullet.Sprite != "" {
			name = bullet.Sprite
		}
		g.bob(screen, name, bullet.X, bullet.Y)
	}
	// Core 0x1354 draws the sixteen hostile slots in ascending address order.
	// A reused low slot can overlap an older shot, so birth order is different.
	for slot := 0; slot < 16; slot++ {
		for _, bullet := range w.Bullets {
			if bullet.Hostile != nil && bullet.Hostile.Slot == slot && bullet.Lifetime > 0 {
				g.bob(screen, bullet.Sprite, bullet.X, bullet.Y)
			}
		}
	}
	g.drawOriginalWeapons(screen, 1)
	g.drawHardwareSprites(screen)
	for _, effect := range w.Effects {
		if w.ActorImagesValid && effect.Sprite != "" {
			continue
		}
		if effect.Delay > 0 {
			continue
		}
		name := fmt.Sprintf("explosion-%d", effect.Age/4%5)
		if effect.Sprite != "" {
			frame := 0
			if len(effect.Animation) > 0 {
				index := max(0, effect.Age-1) / max(1, effect.FrameTicks)
				frame = effect.Animation[min(index, len(effect.Animation)-1)]
			}
			name = fmt.Sprintf("%s-%d", effect.Sprite, frame)
		}
		g.bob(screen, name, effect.X, effect.Y, effect.MaskFlash && effect.Age == 1)
	}
	// Menu and gameplay hide the same first sixteen fetched pixels. The boot
	// illustration uses the earlier DIW start and displays the full fetch.
	_, clip := g.presentationGeometry()
	vector.DrawFilledRect(screen, 0, 0, float32(clip), sim.PlayHeight, color.Black, false)
	g.drawHUD(screen)
	if w.State == sim.Paused {
		vector.DrawFilledRect(screen, 0, 0, sim.Width, sim.PlayHeight, color.NRGBA{0, 0, 0, 150}, false)
		vector.DrawFilledRect(screen, 151, 83, 6, 24, color.White, false)
		vector.DrawFilledRect(screen, 163, 83, 6, 24, color.White, false)
	}
	if w.State == sim.GameOver || w.State == sim.Complete {
		vector.DrawFilledRect(screen, 0, 0, sim.Width, sim.PlayHeight, color.NRGBA{0, 0, 0, 160}, false)
		// All letters come from the recovered original game font when available.
		message := "GAME OVER"
		if w.State == sim.Complete {
			message = "COMPLETE"
		}
		g.text(screen, message, 160-len(message)*4, 90)
	}
}

// drawHardwareSprites follows the independent DMA sprite layer, which remains
// active while the main loop clears and prints the completion playfield.
func (g *Game) drawHardwareSprites(screen *ebiten.Image) {
	w := g.World
	if w.SpriteVisible() && (w.Player.Invulnerable == 0 || w.Tick%6 < 3) {
		frame := w.Player.SpriteFrame()
		// Paired original DMA headers place the visible center one pixel
		// after the preserved simulation's hardware coordinate convention.
		g.sprite(screen, fmt.Sprintf("ship-%d", frame), w.Player.X+1, w.Player.Y)
	}
	if w.Force.Mode != sim.ForceNone {
		if w.Force.Sprite != "" {
			g.sprite(screen, w.Force.Sprite, w.Force.DrawX-1, w.Force.DrawY)
		} else {
			g.sprite(screen, fmt.Sprintf("force-%d", min(4, max(0, w.Force.Level-1)*2)+int(w.Tick/8%2)), w.Force.X, w.Force.Y)
		}
	}
	for index, bit := range w.BitEntities() {
		position := w.BitDrawPosition(index)
		frame := int(w.Tick / 4 % 12)
		if position.W > 0 {
			bit = position
			bit.X--
			frame = w.BitFrame(index)
		}
		g.sprite(screen, fmt.Sprintf("bit-%d", frame), bit.X, bit.Y)
	}
}

func (g *Game) drawHUD(screen *ebiten.Image) {
	// The copper list leaves one blank line before the fifteen-row score strip.
	y := float64(sim.PlayHeight + 1)
	vector.DrawFilledRect(screen, 0, sim.PlayHeight, sim.Width, sim.HUDHeight, color.Black, false)
	if hud := g.images["hud"]; hud != nil {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(0, y)
		screen.DrawImage(hud, op)
	}
	if g.images["hud"] == nil {
		g.text(screen, "BEAM", 88, int(y))
		g.text(screen, "1P-", 40, int(y)+8)
		g.text(screen, "HI-", 136, int(y)+8)
		vector.StrokeRect(screen, 124, float32(y+1), 98, 7, 1, color.NRGBA{170, 221, 238, 255}, false)
	}
	// Dynamic charge and score are native state drawn over the original HUD.
	if len(g.chargeImages) == 25 {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(128, y+1)
		screen.DrawImage(g.chargeImages[max(0, min(24, g.World.DisplayedCharge()))], op)
	} else {
		vector.DrawFilledRect(screen, 128, float32(y+1), 96, 5, color.Black, false)
		vector.DrawFilledRect(screen, 128, float32(y+1), float32(96*g.World.ChargeFraction()), 5, color.NRGBA{170, 204, 221, 255}, false)
	}
	g.number(screen, g.World.Score, 6, 72, int(y)+8)
	g.number(screen, max(g.World.HighScore, g.World.Score), 6, 168, int(y)+8)
	vector.DrawFilledRect(screen, 16, float32(y+1), 40, 6, color.Black, false)
	for n := 0; n < min(5, max(0, g.World.DisplayLives)); n++ {
		if ship := g.sprites["life"]; ship != nil {
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(float64(16+n*8), y+1)
			screen.DrawImage(ship, op)
		}
	}
}

func (g *Game) number(screen *ebiten.Image, value, digits, x, y int) {
	font := g.images["digits"]
	if font == nil {
		return
	}
	limit := 1
	for range digits {
		limit *= 10
	}
	// Core 0xB3A blanks leading zero cells, retaining the final zero for an
	// empty score. The unchanged HUD image provides the cleared cell pixels.
	text := fmt.Sprintf("%*d", digits, value%limit)
	for i, r := range text {
		n := int(r - '0')
		if n < 0 || n > 9 {
			continue
		}
		cell := font.Bounds().Dx() / 10
		glyph := font.SubImage(image.Rect(n*cell, 0, (n+1)*cell, font.Bounds().Dy())).(*ebiten.Image)
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(x+i*cell), float64(y))
		screen.DrawImage(glyph, op)
	}
}

func (g *Game) text(screen *ebiten.Image, text string, x, y int) {
	font := g.images["font"]
	if font == nil {
		return
	}
	for i, r := range text {
		// Uppercase codes in this font select icons; letters use lowercase codes.
		index := strings.IndexRune(g.fontChars, unicode.ToLower(r))
		if index < 0 {
			index = strings.IndexRune(g.fontChars, r)
		}
		if index < 0 {
			continue
		}
		cell := 8
		glyph := font.SubImage(image.Rect(index*cell, 0, (index+1)*cell, font.Bounds().Dy())).(*ebiten.Image)
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(x+i*cell), float64(y))
		screen.DrawImage(glyph, op)
	}
}

func (g *Game) drawControls(screen *ebiten.Image) {
	border := color.NRGBA{102, 153, 187, 200}
	fill := color.NRGBA{25, 42, 57, 220}
	vector.StrokeCircle(screen, 40, 140, 29, 1, border, false)
	vector.DrawFilledCircle(screen, 40+float32(g.input.stickX), 140+float32(g.input.stickY), 12, fill, false)
	fire := fill
	if g.input.fire {
		fire = border
	}
	vector.DrawFilledCircle(screen, 440, 148, 27, fire, false)
	vector.StrokeCircle(screen, 440, 148, 27, 1, border, false)
	vector.StrokeCircle(screen, 440, 148, 8, 2, border, false)
	force := fill
	if g.input.force {
		force = border
	}
	vector.DrawFilledCircle(screen, 440, 76, 21, force, false)
	vector.StrokeCircle(screen, 440, 76, 21, 1, border, false)
	g.sprite(screen, "force-0", 440, 76)
	vector.DrawFilledRect(screen, 433, 17, 4, 12, border, false)
	vector.DrawFilledRect(screen, 443, 17, 4, 12, border, false)
	if g.AtTitle() {
		vector.DrawFilledRect(screen, 20, 25, 40, 40, fill, false)
		g.sprite(screen, "ship-2", 40, 45)
	}
}
