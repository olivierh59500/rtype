package sim

// OriginalInitialRandomState is the disk core's initial longword at 0x422C.
// Zero is meaningful: the first original query samples the beam register.
const OriginalInitialRandomState uint64 = 0

// nextRandom translates core 0xE58. Original callers reduce the returned low
// word modulo their nonzero range, exactly as DIVU followed by SWAP does.
func (w *World) nextRandom() uint64 {
	if w.rngOverride != nil {
		return w.rngOverride(w)
	}
	previous := uint32(w.randomState)
	next := previous << 1
	if previous&0x80000000 != 0 || next == 0 {
		beam := w.rngBeamSource
		if beam == nil {
			beam = w.nativeBeamWord
		}
		next ^= uint32(beam())<<16 | uint32(beam())
	}
	w.randomState = uint64(next)
	return uint64(uint16(next))
}

// nativeBeamWord is a deterministic PAL-shaped timing surrogate: 312 lines
// and 227 horizontal color-clock positions. Native Go has no 68000 instruction
// clock, so this preserves reproducibility without claiming hardware-cycle
// identity for the beam values sampled by the original random routine.
func (w *World) nativeBeamWord() uint16 {
	line := (44 + w.Tick%312) % 312
	horizontal := (w.Tick*37 + uint64(w.StageTick)) % 227
	return uint16(line&255)<<8 | uint16(horizontal)
}
