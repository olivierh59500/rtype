package sim

// OriginalPlayerLatch retains the two player coordinates consumed by aiming
// and collision routines. They update together when the sprite-output phase
// accepts a visible ship coordinate; PAL movement and bank extents are separate.
// The caller owns that phase's scheduling, which this data-only type does not
// infer from a PAL tick or a main update.
type OriginalPlayerLatch struct {
	X, Y int
}

// Update samples raw hardware sprite words. Disabled controls blink on counter
// bit one. A blink or raw X=95 preserves both old coordinates, including Y.
// Accepted subtraction wraps as signed sixteen-bit coordinate arithmetic.
func (l *OriginalPlayerLatch) Update(rawX, rawY int, controls bool, palCounter uint32) bool {
	if !controls && palCounter&2 != 0 {
		return false
	}
	x := int16(uint16(rawX) - 95)
	if x == 0 {
		return false
	}
	l.X, l.Y = int(x), int(int16(uint16(rawY)-12))
	return true
}
