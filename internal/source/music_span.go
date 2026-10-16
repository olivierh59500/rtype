package source

import (
	"encoding/binary"
	"fmt"
)

// trimMusicData keeps the contiguous original score/table/sample region while
// excluding later graphic banks and independent executable player routines.
// All score tables precede Samples in these verified original layouts.
func trimMusicData(data []byte, spec *LevelMusic) ([]byte, error) {
	end := spec.Samples
	if spec.SampleCount < 1 || spec.SampleTable < 0 || spec.SampleTable+spec.SampleCount*8 > len(data) || spec.Samples < 0 || spec.Samples > len(data) {
		return nil, fmt.Errorf("original music tables exceed their decoded data")
	}
	for index := 0; index < spec.SampleCount; index++ {
		table := data[spec.SampleTable+index*8:]
		address := spec.Samples + int(binary.BigEndian.Uint32(table))
		length := int(binary.BigEndian.Uint16(table[6:])) * 2
		if address < spec.Samples || address > len(data) || length > len(data)-address {
			return nil, fmt.Errorf("original music sample%d exceeds its decoded data", index)
		}
		end = max(end, address+length)
	}
	if end <= spec.Samples {
		return nil, fmt.Errorf("original music score contains no sample data")
	}
	return data[:end], nil
}
