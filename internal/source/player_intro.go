package source

import "encoding/binary"

// PlayerIntroData preserves the cold-start automatic ship route. The original
// 0x4204 flag prevents this route from repeating on human restarts or respawns.
type PlayerIntroData struct {
	Arrow                                   *IntroArrowData
	StartX, StartY, StartTilt               int
	OutwardTarget, ReturnTarget, Step       int
	HoldReload, ReturnTiltDelay, TiltReload int
}

type IntroArrowData struct {
	Descriptor                                                uint32
	X, Y, Width, Height, WaitReload, VX, MaximumX, FrameTicks int
	Animation                                                 []int
	Collision                                                 []CollisionFrame
}

func extractPlayerIntro(memory []byte) *PlayerIntroData {
	u16 := func(a int) int { return int(binary.BigEndian.Uint16(memory[a:])) }
	d := &PlayerIntroData{StartX: u16(0x512), StartY: u16(0x4b0), StartTilt: int(memory[0x51b]), OutwardTarget: u16(0x22d4), ReturnTarget: u16(0x2302), Step: 2, HoldReload: int(memory[0x22e7]), ReturnTiltDelay: int(memory[0x232d]), TiltReload: int(memory[0x233f])}
	arrow := &IntroArrowData{Descriptor: 0x4252, X: int(memory[0x4d7]), Y: int(memory[0x4d9]), WaitReload: u16(0x4fa), VX: 4, MaximumX: u16(0xea0), FrameTicks: int(memory[0x4259]) + 1}
	size := u16(0x4252)
	arrow.Width, arrow.Height = (size&63-1)*16, (size>>6)/4
	anim := int(binary.BigEndian.Uint32(memory[0x4266:]))
	for i := 0; i < 32; i++ {
		v := int(int8(memory[anim+i]))
		if v < 0 {
			break
		}
		arrow.Animation = append(arrow.Animation, v)
	}
	arrow.Collision, _ = collisionFrames(memory, int(binary.BigEndian.Uint32(memory[0x426a:])), 2)
	d.Arrow = arrow
	return d
}
