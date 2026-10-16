package source

import "encoding/binary"

// DeathEffectData describes one of the original four generic destruction
// styles. The original selector is data; native Go owns its animation timer.
type DeathEffectData struct {
	Descriptor                                 uint32
	Width, Height, FrameTicks, Lifetime, Sound int
	Animation                                  []int
}

func extractDeathEffects(memory []byte) []DeathEffectData {
	var effects []DeathEffectData
	for kind := 0; kind < 4; kind++ {
		entry := 0x491a + kind*8
		descriptor := int(binary.BigEndian.Uint32(memory[entry:]))
		size := int(binary.BigEndian.Uint16(memory[descriptor:]))
		period := int(memory[descriptor+7]) + 1
		animation := int(binary.BigEndian.Uint32(memory[descriptor+20:]))
		e := DeathEffectData{Descriptor: uint32(descriptor), Width: (size&63 - 1) * 16, Height: (size >> 6) / 4, FrameTicks: period, Lifetime: (int(memory[entry+7]) + 1) * period, Sound: 10 + kind}
		for offset := 0; offset < 64; offset++ {
			frame := int(int8(memory[animation+offset]))
			if frame < 0 {
				break
			}
			e.Animation = append(e.Animation, frame)
		}
		effects = append(effects, e)
	}
	return effects
}
