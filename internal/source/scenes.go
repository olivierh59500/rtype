package source

import "encoding/binary"

// SceneRules preserves the original common palette and checkpoint counters.
// These are instruction immediates decoded once from the source disk, not
// executable instructions consumed by the native game.
type SceneRules struct {
	FadeInCounter, FadeReload, DeathHold, DeathFadeCounter, DeathBlackPAL int
}

func extractSceneRules(memory []byte) *SceneRules {
	return &SceneRules{FadeInCounter: int(memory[0x5e3]), FadeReload: int(memory[0x663]), DeathHold: int(memory[0x721]), DeathFadeCounter: int(memory[0x729]), DeathBlackPAL: int(binary.BigEndian.Uint32(memory[0x79a:]))}
}
