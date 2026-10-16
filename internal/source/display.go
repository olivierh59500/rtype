package source

import "encoding/binary"

// DisplayData preserves the original fetch, display and raw framebuffer origin.
// Gameplay BOB coordinates additionally use the scrolling blitter/copper
// compensation. The zero-fine-scroll menu uses this raw origin directly.
type DisplayData struct {
	Width, Height                   int
	BufferOriginX, BufferOriginY    int
	VisibleX, VisibleWidth          int
	PlaneRowBytes, RowBytes, Modulo int
	FetchStart, FetchStop           int
	WindowStart, WindowStop         uint16
	BlitterMinX, BlitterMaxX        int
	SpriteOriginX, SpriteOriginY    int
}

func extractDisplay(memory []byte, menu bool) *DisplayData {
	u16 := func(address int) int { return int(binary.BigEndian.Uint16(memory[address:])) }
	u32 := func(address int) int { return int(binary.BigEndian.Uint32(memory[address:])) }
	// IRQ0x26D2 selects buffer+0x1A06: 32 interleaved rows and six bytes.
	row := u32(0xce4)
	offset := u32(0x26fa) - u32(0x160)
	originY, originX := offset/row, offset%row*8
	start := uint16(u16(0xc60))
	stop := uint16(u16(0xc62))
	if menu {
		stop = uint16(u16(0x1e6))
	}
	fetchStart, fetchStop := u16(0xc68), u16(0xc6a)
	width := ((fetchStop-fetchStart)/8 + 1) * 16
	visibleWidth := (int(stop&255) + 256) - int(start&255)
	modulo := 0
	for address := 0x52e8; address < 0x5360; address += 4 {
		if u16(address) == 0x108 {
			modulo = u16(address + 2)
			break
		}
	}
	return &DisplayData{Width: width, Height: int(stop>>8) - int(start>>8), BufferOriginX: originX, BufferOriginY: originY, VisibleX: width - visibleWidth, VisibleWidth: visibleWidth, PlaneRowBytes: row / 4, RowBytes: row, Modulo: modulo, FetchStart: fetchStart, FetchStop: fetchStop, WindowStart: start, WindowStop: stop, BlitterMinX: 0, BlitterMaxX: u16(0x11e6), SpriteOriginX: int(start&255) - (width - visibleWidth) - 1, SpriteOriginY: int(start >> 8)}
}
