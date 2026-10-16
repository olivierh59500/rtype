package source

import (
	"encoding/binary"
	"fmt"
)

// diskImage describes the standard 80-cylinder, two-sided Amiga DD disk.
// The original loader reads tracks directly; it does not use AmigaDOS files.
type diskImage []byte

const trackBytes = 11 * 512

func (disk diskImage) tracks(first, count int) ([]byte, error) {
	start, end := first*trackBytes, (first+count)*trackBytes
	if first < 0 || count < 0 || end > len(disk) {
		return nil, fmt.Errorf("track range %d+%d is outside the disk", first, count)
	}
	return disk[start:end], nil
}

// unpackTitle translates the three original backward decompression passes.
// The first pass follows the loader's Huffman tree, the second expands LZ
// references, and the third expands byte runs. No original CPU code is run.
func unpackTitle(disk []byte) ([]byte, error) {
	const header = 0x396
	if len(disk) < 0x5800 {
		return nil, fmt.Errorf("disk is too short for the title payload")
	}
	u32 := func(offset int) int { return int(binary.BigEndian.Uint32(disk[offset:])) }
	outputSize, lzSize, huffmanSize := u32(header), u32(header+4), u32(header+12)
	packedSize, treeSize := u32(header+16), u32(header+20)
	if outputSize != 0xb800 || lzSize != 0x6f34 || huffmanSize != 0x5902 || treeSize != 0x7f0 {
		return nil, fmt.Errorf("unsupported title compression header")
	}
	flags := disk[header+8 : header+12]
	markers := disk[header+32 : header+36]
	tree, position := header+36, 0x7ba+packedSize
	buffer := make([]byte, outputSize)
	var word uint32
	bits := 0
	readBit := func() (int, error) {
		if bits == 0 {
			position -= 4
			if position < 0 || position+4 > len(disk) {
				return 0, fmt.Errorf("title Huffman stream is truncated")
			}
			word, bits = binary.BigEndian.Uint32(disk[position:]), 32
		}
		bit := int(word & 1)
		word >>= 1
		bits--
		return bit, nil
	}
	for index := huffmanSize - 1; index >= 0; index-- {
		node := tree + treeSize/2
		for depth := 0; ; depth++ {
			if depth > 256 {
				return nil, fmt.Errorf("invalid title Huffman tree")
			}
			bit, err := readBit()
			if err != nil {
				return nil, err
			}
			node += (1 ^ bit) * 2
			if node < tree || node+2 > tree+treeSize {
				return nil, fmt.Errorf("title Huffman node is outside the tree")
			}
			value := int(binary.BigEndian.Uint16(disk[node:]))
			if value >= 0x400 {
				buffer[index] = byte(value)
				break
			}
			node = tree + value
		}
	}
	src, dst := huffmanSize, lzSize
	take := func() (byte, error) {
		src--
		if src < 0 {
			return 0, fmt.Errorf("title compressed stream is truncated")
		}
		return buffer[src], nil
	}
	put := func(value byte) error {
		dst--
		if dst < 0 {
			return fmt.Errorf("title decompression exceeds the destination")
		}
		buffer[dst] = value
		return nil
	}
	copyBack := func(offset, count int) error {
		for range count {
			if dst+offset-1 >= len(buffer) || offset <= 0 {
				return fmt.Errorf("invalid title LZ reference")
			}
			if err := put(buffer[dst+offset-1]); err != nil {
				return err
			}
		}
		return nil
	}
	for src > 0 {
		value, err := take()
		if err != nil {
			return nil, err
		}
		switch {
		case value == markers[2]:
			other, err := take()
			if err != nil {
				return nil, err
			}
			if other == 0 {
				err = put(value)
			} else {
				value, err = take()
				if err == nil {
					for _, v := range []byte{value, other, value, other} {
						if err = put(v); err != nil {
							break
						}
					}
				}
			}
			if err != nil {
				return nil, err
			}
		case value == markers[1] || value == markers[3] || value == markers[0] || value == flags[3]:
			count := 4
			if value == markers[1] {
				count = 3
			}
			if value == flags[3] {
				v, err := take()
				if err != nil {
					return nil, err
				}
				count = int(v) + 1
				if v == 0 {
					if err := put(value); err != nil {
						return nil, err
					}
					continue
				}
			}
			low, err := take()
			if err != nil {
				return nil, err
			}
			if low == 0 && value != flags[3] {
				if err := put(value); err != nil {
					return nil, err
				}
				continue
			}
			offset := int(low)
			if value == markers[0] || value == flags[3] {
				high, err := take()
				if err != nil {
					return nil, err
				}
				offset |= int(high) << 8
			}
			if err := copyBack(offset, count); err != nil {
				return nil, err
			}
		default:
			if err := put(value); err != nil {
				return nil, err
			}
		}
	}
	if dst != 0 {
		return nil, fmt.Errorf("title LZ size mismatch: %d bytes remain", dst)
	}
	src, dst = lzSize, outputSize
	value, err := take()
	if err != nil {
		return nil, err
	}
	for dst > 0 {
		if value == flags[0] || value == flags[1] {
			count, err := take()
			if err != nil {
				return nil, err
			}
			run := value
			if count >= 3 {
				if value == flags[1] {
					run = flags[2]
				} else {
					run, err = take()
					if err != nil {
						return nil, err
					}
				}
			}
			// The original routine prefetches once past the final compressed byte.
			// Keep that harmless final read out of the translated implementation.
			if int(count)+1 < dst {
				value, err = take()
				if err != nil {
					return nil, err
				}
			}
			for range int(count) + 1 {
				if err := put(run); err != nil {
					return nil, err
				}
			}
		} else {
			run := value
			if dst > 1 {
				value, err = take()
				if err != nil {
					return nil, err
				}
			}
			if err := put(run); err != nil {
				return nil, err
			}
		}
	}
	return buffer, nil
}

// unpackLevel translates the level loader's PowerPacker backward LZ stream.
// Its first longword is the original in-memory end pointer rather than PP20.
func unpackLevel(packed []byte) ([]byte, error) {
	if len(packed) < 12 {
		return nil, fmt.Errorf("level payload is truncated")
	}
	end := int(binary.BigEndian.Uint32(packed)) - 0x4cb00
	if end < 12 || end > len(packed) {
		return nil, fmt.Errorf("invalid level compressed end pointer")
	}
	trailer := binary.BigEndian.Uint32(packed[end-4:])
	size, skip := int(trailer>>8), int(trailer&255)
	if size <= 0 || size > 512*1024 || skip > 31 {
		return nil, fmt.Errorf("invalid level compression trailer")
	}
	out := make([]byte, size)
	src, dst, bits := end-4, size, 0
	var word uint32
	read := func(count int) (int, error) {
		if count < 0 || count > 31 {
			return 0, fmt.Errorf("invalid level bit count %d", count)
		}
		value := 0
		for range count {
			if bits == 0 {
				src -= 4
				if src < 8 {
					return 0, fmt.Errorf("level compressed bitstream is truncated")
				}
				word, bits = binary.BigEndian.Uint32(packed[src:]), 32
			}
			value = value<<1 | int(word&1)
			word >>= 1
			bits--
		}
		return value, nil
	}
	if _, err := read(skip); err != nil {
		return nil, err
	}
	for dst > 0 {
		bit, err := read(1)
		if err != nil {
			return nil, err
		}
		if bit == 0 {
			count := 1
			for {
				value, err := read(2)
				if err != nil {
					return nil, err
				}
				count += value
				if value != 3 {
					break
				}
			}
			if count > dst {
				return nil, fmt.Errorf("level literal run exceeds output")
			}
			for range count {
				value, err := read(8)
				if err != nil {
					return nil, err
				}
				dst--
				out[dst] = byte(value)
			}
			if dst == 0 {
				break
			}
		}
		index, err := read(2)
		if err != nil {
			return nil, err
		}
		count, bitCount := index+2, int(packed[4+index])
		if index == 3 {
			bit, err := read(1)
			if err != nil {
				return nil, err
			}
			if bit == 0 {
				bitCount = 7
			}
		}
		offset, err := read(bitCount)
		if err != nil {
			return nil, err
		}
		if index == 3 {
			for {
				value, err := read(3)
				if err != nil {
					return nil, err
				}
				count += value
				if value != 7 {
					break
				}
			}
		}
		if count > dst || dst+offset >= len(out) {
			return nil, fmt.Errorf("invalid level LZ reference")
		}
		for range count {
			dst--
			out[dst] = out[dst+offset+1]
		}
	}
	return out, nil
}
