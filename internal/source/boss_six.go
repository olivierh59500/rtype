package source

import (
	"encoding/binary"
	"fmt"
)

// BossSixData describes the original stopped-camera crawler queue. Its route
// direction bytes are game data; the native engine supplies all behavior.
type BossSixData struct {
	MusicTrigger   int                `json:"music_trigger"`
	FreezeScroll   int                `json:"freeze_scroll"`
	SpawnX         int                `json:"spawn_x"`
	SpawnY         int                `json:"spawn_y"`
	Descriptor     uint32             `json:"descriptor"`
	SpawnReload    int                `json:"spawn_reload"`
	LastReload     int                `json:"last_reload"`
	Queues         []RouteCrawlerData `json:"queues"`
	Animation      []int              `json:"animation"`
	AnimationTicks int                `json:"animation_ticks"`
}

func extractBossSix(memory []byte) (*Boss, error) {
	if len(memory) < 0x4cb00 {
		return nil, fmt.Errorf("stage-six header is unavailable")
	}
	u16 := func(address int) int { return int(binary.BigEndian.Uint16(memory[address:])) }
	i16 := func(address int) int { return int(int16(binary.BigEndian.Uint16(memory[address:]))) }
	u32 := func(address int) int { return int(binary.BigEndian.Uint32(memory[address:])) }
	animation := func(address int) []int {
		var frames []int
		for index := 0; index < 64 && address >= 0x4a2e4 && address+index < 0x4cb00; index++ {
			value := int(int8(memory[address+index]))
			if value < 0 {
				break
			}
			frames = append(frames, value)
		}
		return frames
	}
	d := &BossSixData{MusicTrigger: u16(0x4a94e), FreezeScroll: u16(0x4a95a), SpawnX: u16(0x4a996), SpawnY: 0, Descriptor: uint32(u32(0x4a99e)), SpawnReload: u16(0x4a970), LastReload: u16(0x4a986)}
	if d.FreezeScroll != 2304 || d.Descriptor != 0x4adbc {
		return nil, fmt.Errorf("unsupported original stage-six boss profile")
	}
	d.Animation = animation(u32(int(d.Descriptor) + 20))
	d.AnimationTicks = int(memory[int(d.Descriptor)+7]) + 1
	rowBytes := u16(0x41b4 + 5*10 + 8)
	for cursor := 0x4ad50; cursor+4 <= 0x4cb00; cursor += 4 {
		pointer := int32(binary.BigEndian.Uint32(memory[cursor:]))
		if pointer < 0 {
			break
		}
		address := int(pointer)
		if address < 0x4a2e4 || address+3 >= 0x4cb00 {
			return nil, fmt.Errorf("invalid boss crawler route at %#x", address)
		}
		queue := RouteCrawlerData{FacingSide: int(memory[address]), InitialDirection: int(memory[address+1]), DamageBudget: 4, ContactHealth: u16(0x4a9e6), DeathKind: int(memory[0x4a9e1]), ShieldOffset: 48, MaxX: 350, MaxY: 225, LeftAnimation: animation(0x4ae19)}
		for index := 2; index < 258 && address+index < 0x4cb00; index++ {
			value := int(int8(memory[address+index]))
			queue.Directions = append(queue.Directions, value)
			if value < 0 {
				break
			}
		}
		for direction := 0; direction < 4; direction++ {
			position := 0x4ad94 + direction*10
			offset := i16(position + 8)
			dy := 0
			if offset >= rowBytes {
				dy = 1
			} else if offset <= -rowBytes {
				dy = -1
			}
			queue.Routes = append(queue.Routes, RouteCrawlerStep{DX: i16(position), DY: i16(position + 2), ProbeX: i16(position + 4), ProbeY: i16(position + 6), NeighborX: (offset - dy*rowBytes) / 2, NeighborY: dy})
		}
		d.Queues = append(d.Queues, queue)
	}
	if len(d.Queues) == 0 {
		return nil, fmt.Errorf("stage-six boss has no original route queues")
	}
	return &Boss{Kind: "stage6_crawler_queue", Six: d, VictoryCounter: 36, VictoryReload: 2, VictoryTicks: 106}, nil
}
