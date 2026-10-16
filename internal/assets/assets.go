// Package assets loads only resources reconstructed from the user's original disk.
package assets

import (
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"io/fs"
	"path"

	"github.com/olivierh59500/r-type/internal/source"
)

// Sprite identifies a rectangle in an original graphics sheet.
type Sprite struct {
	Name   string `json:"name"`
	Sheet  string `json:"sheet"`
	X      int    `json:"x"`
	Y      int    `json:"y"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// Bundle keeps decoded resources independent of Ebitengine and the GPU.
type Bundle struct {
	Files         fs.FS
	Manifest      json.RawMessage
	Images        map[string]*image.NRGBA
	Sprites       map[string]Sprite
	IndexedImages map[string]*image.Paletted
}

// Load validates a locally generated manifest before decoding any graphics.
func Load(files fs.FS) (*Bundle, error) {
	if files == nil {
		return nil, fmt.Errorf("original assets are unavailable; reconstruct and bundle the supplied ADF first")
	}
	manifest, err := fs.ReadFile(files, "manifest.json")
	if err != nil {
		return nil, fmt.Errorf("extract the original disk with go run ./cmd/extract first: %w", err)
	}
	if !json.Valid(manifest) {
		return nil, fmt.Errorf("invalid original asset manifest")
	}
	if _, err := source.ReadManifest(files); err != nil {
		return nil, err
	}
	b := &Bundle{Files: files, Manifest: manifest, Images: make(map[string]*image.NRGBA), Sprites: make(map[string]Sprite), IndexedImages: make(map[string]*image.Paletted)}
	return b, nil
}

// Image decodes a PNG lazily and rejects resources outside the bundle.
func (b *Bundle) Image(name string) (*image.NRGBA, error) {
	if !fs.ValidPath(name) || path.Ext(name) != ".png" {
		return nil, fmt.Errorf("invalid image path %q", name)
	}
	if img, found := b.Images[name]; found {
		return img, nil
	}
	file, err := b.Files.Open(name)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	cfg, err := png.DecodeConfig(file)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || cfg.Width > 32768 || cfg.Height > 4096 || int64(cfg.Width)*int64(cfg.Height) > 32<<20 {
		return nil, fmt.Errorf("unreasonable dimensions in %s", name)
	}
	// fs.FS does not require seekable files, including Android's embedded bundle.
	input, err := b.Files.Open(name)
	if err != nil {
		return nil, err
	}
	defer input.Close()
	decoded, err := png.Decode(input)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	if indexed, ok := decoded.(*image.Paletted); ok {
		b.IndexedImages[name] = indexed
	}
	img := image.NewNRGBA(decoded.Bounds())
	for y := 0; y < cfg.Height; y++ {
		for x := 0; x < cfg.Width; x++ {
			img.Set(x, y, decoded.At(x, y))
		}
	}
	b.Images[name] = img
	return img, nil
}
