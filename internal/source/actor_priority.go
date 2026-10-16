package source

import (
	"encoding/binary"
	"fmt"
)

// extractActorPriority reads the immediate source operand observed by the
// all-event original spawner oracle. Unlisted verified spawners retain 0x1080's
// cleared priority. Compound child priorities are kept in their family data.
func extractActorPriority(memory []byte, level int, event Event) (int, error) {
	if event.SpawnAddress == 0x2d90 {
		return priorityOperand(memory, int(event.SpawnAddress)+0x44)
	}
	profiles := map[int]map[uint32]int{
		1: {0x4a83c: 0x22, 0x4b1fc: 0x18},
		3: {0x4ae26: 0x28, 0x4b08c: 0x10},
		4: {0x4ab12: 0x22},
		5: {0x4a928: 0x2e, 0x4ab76: 0x0e, 0x4ac10: 0x18, 0x4ac36: 0x30, 0x4ad00: 0x2a},
		7: {0x4aa28: 0x22, 0x4b3ec: 0x24, 0x4b534: 0x12},
		8: {0x4a8be: 0x1e},
	}
	if offset, ok := profiles[level][event.SpawnAddress]; ok {
		return priorityOperand(memory, int(event.SpawnAddress)+offset)
	}
	if level == 2 && event.SpawnAddress == 0x4a99c {
		// The eleven-part original spawner writes its descending D7 index.
		return 10, nil
	}
	return 0, nil
}

func priorityOperand(memory []byte, address int) (int, error) {
	if address < 2 || address+4 > len(memory) || binary.BigEndian.Uint16(memory[address-2:]) != 0x117c || binary.BigEndian.Uint16(memory[address+2:]) != 4 {
		return 0, fmt.Errorf("invalid original actor priority operand at %#x", address)
	}
	return int(binary.BigEndian.Uint16(memory[address:])), nil
}
