package sim

import "fmt"

const (
	OriginalActorSlots       = 41
	OriginalActorRecordWords = 31
	OriginalActorPrefixWords = 9
)

// OriginalActorHandle identifies native record storage. Pool handles are 1..41;
// external records explicitly represent caller-selected scratch or aliases.
// Handles are data references, never executable pointers or Amiga addresses.
type OriginalActorHandle uint32

// OriginalActorRecord is a bounded view retaining data cells across reuse.
// Copying a record shares its view; Snapshot copies the values independently.
// Native callbacks interpret private fields; no cell executes source code.
type OriginalActorRecord struct {
	words []uint16
}

func (r *OriginalActorRecord) ensureWords() {
	if r.words == nil {
		r.words = make([]uint16, OriginalActorRecordWords)
	}
}

// Snapshot copies the bounded record, preserving the storage view itself.
func (r *OriginalActorRecord) Snapshot() (words [OriginalActorRecordWords]uint16) {
	copy(words[:], r.words)
	return words
}

// SetWords copies values into the existing view. It never replaces aliased
// storage or expands the record into an adjacent actor.
func (r *OriginalActorRecord) SetWords(words [OriginalActorRecordWords]uint16) {
	r.ensureWords()
	copy(r.words, words[:])
}

func (r *OriginalActorRecord) Word(index int) (uint16, error) {
	if index < 0 || index >= OriginalActorRecordWords {
		return 0, fmt.Errorf("actor word index %d is outside its record", index)
	}
	r.ensureWords()
	return r.words[index], nil
}

func (r *OriginalActorRecord) SetWord(index int, value uint16) error {
	if index < 0 || index >= OriginalActorRecordWords {
		return fmt.Errorf("actor word index %d is outside its record", index)
	}
	r.ensureWords()
	r.words[index] = value
	return nil
}

func (r *OriginalActorRecord) Next() OriginalActorHandle {
	r.ensureWords()
	return OriginalActorHandle(uint32(r.words[0])<<16 | uint32(r.words[1]))
}

func (r *OriginalActorRecord) SetNext(handle OriginalActorHandle) {
	r.ensureWords()
	r.words[0], r.words[1] = uint16(uint32(handle)>>16), uint16(handle)
}

func (r *OriginalActorRecord) Priority() uint8 { r.ensureWords(); return uint8(r.words[2] >> 8) }
func (r *OriginalActorRecord) SetPriority(priority uint8) {
	r.ensureWords()
	r.words[2] = uint16(priority)<<8 | r.words[2]&255
}

// OriginalActorPredecessor distinguishes the global live-head link from a
// record link, including an explicitly modeled external record numbered zero.
type OriginalActorPredecessor struct {
	Head   bool
	Record OriginalActorHandle
}

// OriginalActorPool preserves allocation identity, LIFO reuse and the source's
// caller-dependent exhaustion. It remains separate from World's ID list until
// every spawner's caller and retained private fields have native mappings.
type OriginalActorPool struct {
	storage    []uint16
	records    [OriginalActorSlots]OriginalActorRecord
	external   map[OriginalActorHandle]*OriginalActorRecord
	free, live OriginalActorHandle
}

func (p *OriginalActorPool) ensureStorage() {
	if p.storage != nil {
		return
	}
	p.storage = make([]uint16, OriginalActorPrefixWords+OriginalActorSlots*OriginalActorRecordWords)
	for index := range p.records {
		start := OriginalActorPrefixWords + index*OriginalActorRecordWords
		p.records[index].words = p.storage[start : start+OriginalActorRecordWords : start+OriginalActorRecordWords]
	}
}

func (p *OriginalActorPool) PrefixSnapshot() (words [OriginalActorPrefixWords]uint16) {
	p.ensureStorage()
	copy(words[:], p.storage[:OriginalActorPrefixWords])
	return words
}

func (p *OriginalActorPool) SetPrefix(words [OriginalActorPrefixWords]uint16) {
	p.ensureStorage()
	copy(p.storage[:OriginalActorPrefixWords], words[:])
}

// RegisterPoolView maps a logical caller to a bounded overlapping data view.
// Byte offsets are relative to the first actor; -18 represents the native
// history tail used by intro/death. No source address or executable memory is
// involved. Partial stores immediately affect every intersecting record.
func (p *OriginalActorPool) RegisterPoolView(handle OriginalActorHandle, offsetBytes int) (*OriginalActorRecord, error) {
	if offsetBytes%2 != 0 || offsetBytes < -OriginalActorPrefixWords*2 || offsetBytes > (OriginalActorSlots-1)*OriginalActorRecordWords*2 {
		return nil, fmt.Errorf("actor data view offset %d is unaligned or outside native storage", offsetBytes)
	}
	p.ensureStorage()
	start := OriginalActorPrefixWords + offsetBytes/2
	record := &OriginalActorRecord{words: p.storage[start : start+OriginalActorRecordWords : start+OriginalActorRecordWords]}
	if err := p.RegisterExternal(handle, record); err != nil {
		return nil, err
	}
	return record, nil
}

func NewOriginalActorPool() *OriginalActorPool {
	p := &OriginalActorPool{}
	p.Reset()
	return p
}

func (p *OriginalActorPool) FreeHead() OriginalActorHandle { return p.free }
func (p *OriginalActorPool) LiveHead() OriginalActorHandle { return p.live }

// Reset rebuilds links while retaining every other record field. It does not
// clear private words, change external caller storage or normalize corruption.
func (p *OriginalActorPool) Reset() {
	p.ensureStorage()
	p.free, p.live = 1, 0
	for index := range p.records {
		next := OriginalActorHandle(index + 2)
		if index+1 == len(p.records) {
			next = 0
		}
		p.records[index].SetNext(next)
	}
}

func (p *OriginalActorPool) Record(handle OriginalActorHandle) (*OriginalActorRecord, error) {
	p.ensureStorage()
	if handle >= 1 && handle <= OriginalActorSlots {
		return &p.records[int(handle)-1], nil
	}
	if record, ok := p.external[handle]; ok {
		return record, nil
	}
	return nil, fmt.Errorf("actor handle %X has no native record mapping", handle)
}

// RegisterExternal supplies the actual native storage selected by a caller.
// Unknown handles are never redirected to a spare pool slot or discarded.
func (p *OriginalActorPool) RegisterExternal(handle OriginalActorHandle, record *OriginalActorRecord) error {
	if handle >= 1 && handle <= OriginalActorSlots || record == nil {
		return fmt.Errorf("invalid external actor record %X", handle)
	}
	if p.external == nil {
		p.external = make(map[OriginalActorHandle]*OriginalActorRecord)
	}
	if _, exists := p.external[handle]; exists {
		return fmt.Errorf("external actor record %X already mapped", handle)
	}
	p.external[handle] = record
	return nil
}

// Allocate retains caller when empty. retainedCaller reports that source path;
// it is not a clean failure result and must not be used to drop the spawn.
func (p *OriginalActorPool) Allocate(caller OriginalActorHandle) (handle OriginalActorHandle, retainedCaller bool, err error) {
	if p.free == 0 {
		return caller, true, nil
	}
	handle = p.free
	record, err := p.Record(handle)
	if err != nil {
		return 0, false, err
	}
	p.free = record.Next()
	return handle, false, nil
}

// Construct performs only the common constructor's selective writes. Link,
// callback and uninitialized private cells retain the previous occupant's data.
func (p *OriginalActorPool) Construct(caller OriginalActorHandle, x, y int16, hp uint16) (OriginalActorHandle, bool, error) {
	handle, retained, err := p.Allocate(caller)
	if err != nil {
		return handle, retained, err
	}
	record, err := p.Record(handle)
	if err != nil {
		return handle, retained, err
	}
	record.ensureWords()
	for _, word := range []int{2, 15, 16, 19, 23, 24} {
		record.words[word] = 0
	}
	record.words[17] = 0xff00
	record.words[20] &= 0x00ff
	record.words[18] = hp
	record.words[5], record.words[6] = uint16(x), uint16(y)
	return handle, retained, nil
}

// Release uses the actual predecessor-link location and returns that same
// location for continued traversal. Store order matters when records alias.
func (p *OriginalActorPool) Release(handle OriginalActorHandle, previous OriginalActorPredecessor) (OriginalActorPredecessor, error) {
	record, err := p.Record(handle)
	if err != nil {
		return previous, err
	}
	var predecessor *OriginalActorRecord
	if !previous.Head {
		predecessor, err = p.Record(previous.Record)
		if err != nil {
			return previous, err
		}
	}
	record.ensureWords()
	record.words[5] = 0
	if previous.Head {
		p.live = record.Next()
	} else {
		predecessor.SetNext(record.Next())
	}
	record.SetNext(p.free)
	p.free = handle
	return previous, nil
}

// Insert preserves the source's unusual comparison after the head: it compares
// priority with the high byte of the next record's link, not its priority.
// Reinserting a live handle is allowed and can create a self-linked list.
func (p *OriginalActorPool) Insert(handle OriginalActorHandle) error {
	record, err := p.Record(handle)
	if err != nil {
		return err
	}
	if p.live == 0 {
		p.live = handle
		record.SetNext(0)
		return nil
	}
	head, err := p.Record(p.live)
	if err != nil {
		return err
	}
	priority := record.Priority()
	if priority <= head.Priority() {
		record.SetNext(p.live)
		p.live = handle
		return nil
	}
	current := p.live
	seen := make(map[OriginalActorHandle]bool)
	for {
		if seen[current] {
			return fmt.Errorf("actor insertion reached a preexisting cycle at handle %X", current)
		}
		seen[current] = true
		previous, err := p.Record(current)
		if err != nil {
			return err
		}
		next := previous.Next()
		if next == 0 {
			record.SetNext(0)
			previous.SetNext(handle)
			return nil
		}
		nextRecord, err := p.Record(next)
		if err != nil {
			return err
		}
		if priority > uint8(uint32(nextRecord.Next())>>24) {
			current = next
			continue
		}
		record.SetNext(next)
		previous.SetNext(handle)
		return nil
	}
}

// Walk observes a chain without repairing it. A repeated handle is included,
// matching the bounded original oracle; cycle=true reports the repetition.
func (p *OriginalActorPool) Walk(head OriginalActorHandle) (handles []OriginalActorHandle, cycle bool, err error) {
	seen := make(map[OriginalActorHandle]bool)
	for head != 0 {
		handles = append(handles, head)
		if seen[head] {
			return handles, true, nil
		}
		seen[head] = true
		record, err := p.Record(head)
		if err != nil {
			return handles, false, err
		}
		head = record.Next()
	}
	return handles, false, nil
}
