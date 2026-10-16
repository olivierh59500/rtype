package source

// PlayerData preserves the manual joystick bounds and ship collision poses.
// Positions use hardware sprite coordinates in the original PAL controller.
type PlayerData struct {
	Intro                                        *PlayerIntroData `json:"intro,omitempty"`
	LeftLimit, RightLimit, TopLimit, BottomLimit int
	InitialSpeed, MaximumSpeed                   int
	CollisionY, CollisionHeight                  []int
}

func extractPlayerData(memory []byte) *PlayerData {
	data := &PlayerData{LeftLimit: 150, RightLimit: 416, TopLimit: 48, BottomLimit: 220, InitialSpeed: 2, MaximumSpeed: 5}
	data.Intro = extractPlayerIntro(memory)
	// Core 0x227C divides the sixteen-step tilt by four before indexing the
	// collision offsets at 0x5108 and heights at 0x510D.
	for frame := 0; frame < 5; frame++ {
		data.CollisionY = append(data.CollisionY, int(int8(memory[0x5108+frame])))
		data.CollisionHeight = append(data.CollisionHeight, int(int8(memory[0x510D+frame])))
	}
	return data
}
