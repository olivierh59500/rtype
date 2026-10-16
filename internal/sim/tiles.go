package sim

// captureOriginalTerrain preserves the overlays copied by core 0xF5E before
// weapons, actors and the map hook mutate the live collision map. PAL-only
// updates retain this image until the next main pass copies the terrain again.
func (w *World) captureOriginalTerrain() {
	if w.TerrainImageChanges == nil {
		w.TerrainImageChanges = make(map[int]uint16, len(w.TileChanges))
	}
	clear(w.TerrainImageChanges)
	for index, word := range w.TileChanges {
		w.TerrainImageChanges[index] = word
	}
	w.TerrainImageValid = true
}

// OriginalTileAt accepts the original game-buffer coordinates used by core
// 0x832 and returns zero outside the original map.
func (w *World) OriginalTileAt(x, y int) uint16 {
	if w.Stage < 1 || w.Stage > len(w.Stages) {
		return 0
	}
	stage := w.Stages[w.Stage-1]
	column, row := x+int(w.Scroll)-48, y+int(w.ScrollY)-32
	if column < 0 || row < 0 {
		return 0
	}
	column /= 16
	row /= 16
	if column >= stage.MapWidth || row >= stage.MapHeight {
		return 0
	}
	return w.RawTileAt(column, row)
}

// RawTileAt reads a map cell, including cells beyond the visible viewport.
func (w *World) RawTileAt(column, row int) uint16 {
	if w.Stage < 1 || w.Stage > len(w.Stages) {
		return 0
	}
	stage := w.Stages[w.Stage-1]
	if column < 0 || row < 0 || column >= stage.MapWidth || row >= stage.MapHeight {
		return 0
	}
	tiles := w.Tiles
	if len(tiles) == 0 {
		tiles = stage.TileMap
	}
	index := row*stage.MapWidth + column
	if index >= len(tiles) {
		return 0
	}
	return tiles[index]
}

// OriginalSolidAt is the source routine's bit-fifteen tile collision test.
func (w *World) OriginalSolidAt(x, y int) bool { return w.OriginalTileAt(x, y)&0x8000 != 0 }

func (w *World) patchTiles(column, row int, words []uint16) {
	if w.Stage < 1 || w.Stage > len(w.Stages) || len(words) != 4 {
		return
	}
	stage := w.Stages[w.Stage-1]
	if column < 0 || column+1 >= stage.MapWidth || row < 0 || row+1 >= stage.MapHeight {
		return
	}
	if len(w.Tiles) != len(stage.TileMap) {
		w.Tiles = append([]uint16(nil), stage.TileMap...)
	}
	if w.TileChanges == nil {
		w.TileChanges = make(map[int]uint16)
	}
	for index, value := range words {
		w.SetTileAt(column+index%2, row+index/2, value)
	}
}

// SetTileAt changes one original map word and tracks its presentation overlay.
func (w *World) SetTileAt(column, row int, value uint16) bool {
	if w.Stage < 1 || w.Stage > len(w.Stages) {
		return false
	}
	stage := w.Stages[w.Stage-1]
	if column < 0 || row < 0 || column >= stage.MapWidth || row >= stage.MapHeight {
		return false
	}
	if len(w.Tiles) != len(stage.TileMap) {
		w.Tiles = append([]uint16(nil), stage.TileMap...)
	}
	if w.TileChanges == nil {
		w.TileChanges = make(map[int]uint16)
	}
	index := row*stage.MapWidth + column
	w.Tiles[index] = value
	if value == stage.TileMap[index] {
		delete(w.TileChanges, index)
	} else {
		w.TileChanges[index] = value
	}
	return true
}

// OriginalSolidHit translates the player's original solid-tile hit: mark bit
// thirteen and erase a destructible bit-twelve tile. Erasing clears the source
// zero flag and allows Force/laser/wave traversal to continue.
func (w *World) OriginalSolidHit(x, y int) bool {
	word := w.OriginalTileAt(x, y)
	if word&0x8000 == 0 {
		return false
	}
	word |= 0x2000
	if word&0x1000 != 0 {
		word = 0
	}
	column, row := x+int(w.Scroll)-48, y+int(w.ScrollY)-32
	if column >= 0 && row >= 0 {
		w.SetTileAt(column/16, row/16, word)
	}
	return word != 0
}
