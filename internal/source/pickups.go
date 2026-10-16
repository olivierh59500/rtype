package source

import "encoding/binary"

// PickupData stores the released item's own art and contact envelope. Its six
// frames select Blue, Red, Yellow, Speed, Missiles and Bit in that exact order.
type PickupData struct {
	Width, Height int
	Collision     []CollisionFrame
}

func extractPickupData(memory []byte) (*PickupData, error) {
	size := int(binary.BigEndian.Uint16(memory[0x5288:]))
	frames, err := collisionFrames(memory, int(binary.BigEndian.Uint32(memory[0x5288+24:])), 6)
	if err != nil {
		return nil, err
	}
	return &PickupData{Width: (size&63 - 1) * 16, Height: (size >> 6) / 4, Collision: frames}, nil
}

// CarrierData preserves the original floating-container data, independently of
// its translated Go movement and tile-contact rules.
type CarrierData struct {
	Kind                   int    `json:"kind"`
	Velocity               int    `json:"velocity"`
	MotionY                []int8 `json:"motion_y"`
	AnimationFloat         []int  `json:"animation_float"`
	AnimationGround        []int  `json:"animation_ground"`
	AnimationFloatReverse  []int  `json:"animation_float_reverse"`
	AnimationGroundReverse []int  `json:"animation_ground_reverse"`
}

// annotateCarrier recognizes the common container spawner at original RAM
// 0x2D90. Its sine steps and animation bytes are assets, never instructions.
func annotateCarrier(memory []byte, event *Event) bool {
	if event.SpawnAddress != 0x2d90 || event.UpdateAddress != 0x2dde {
		return false
	}
	kind := int(byte(event.Parameter >> 16))
	if kind > 5 || len(memory) < 0x1c650+58 {
		return false
	}
	velocity := int(int8(byte(event.Parameter)))
	floatAddress, groundAddress := 0x5233, 0x5229
	if velocity < 0 {
		floatAddress, groundAddress = 0x522e, 0x5224
	}
	animation := func(address int) []int {
		var frames []int
		for count := 0; count < 32 && address+count < len(memory); count++ {
			value := int(int8(memory[address+count]))
			if value < 0 {
				break
			}
			frames = append(frames, value)
		}
		return frames
	}
	data := &CarrierData{Kind: kind, Velocity: velocity, AnimationFloat: animation(floatAddress), AnimationGround: animation(groundAddress)}
	data.AnimationFloatReverse = animation(0x5233 + 0x522e - floatAddress)
	data.AnimationGroundReverse = animation(0x5229 + 0x5224 - groundAddress)
	for _, value := range memory[0x1c650 : 0x1c650+58] {
		data.MotionY = append(data.MotionY, int8(value))
	}
	event.Carrier = data
	event.Behavior = "power_carrier"
	// This spawner explicitly passes one hit point to the common allocator.
	event.Health = 1
	event.AnimationTicks = int(memory[int(event.Descriptor)+7]) + 1
	// Preserve the actual sprite's animation base rather than inferring its
	// color from the item it releases after being shot.
	if len(data.AnimationFloat) > 0 {
		event.Animation = append([]int(nil), data.AnimationFloat...)
	}
	return true
}
