package sim

import "sort"

type originalActorNode struct {
	ID       uint64
	Priority int
}

// OriginalActorImage preserves a BOB pose after motion/animation and before
// contacts. A hit later in the pass cannot erase an already drawn body.
type OriginalActorImage struct {
	Entity
	Sprite    string
	Frame     int
	MaskFlash bool
}

// BlitterVisible preserves 0x11D8's whole-BOB rejection before drawing. In
// particular, Gomander's route waits retain raw X=-1 and still run the animator.
func (image OriginalActorImage) BlitterVisible(fineX int) bool {
	x := int(image.X-image.W/2) + GameOriginX + (fineX & 15)
	return x >= 0 && x <= 368
}

func (w *World) selectedActor(enemy *Enemy) bool {
	return w.actorUpdateID == 0 || enemy.ID == w.actorUpdateID
}

type originalActorList struct {
	Nodes       []originalActorNode
	Initialized bool
}

// OriginalActorLinkCount reports current registered links for retrospective
// comparison with original RAM; it does not include an in-flight reservation.
func (w *World) OriginalActorLinkCount() int { return len(w.actorList.Nodes) }

// Insert reproduces 0x12A6 for original chip-RAM links. A value at most the
// current head priority prepends. A greater value walks link high bytes (zero
// in the original actor pool), so it appends rather than performing a sort.
func (list *originalActorList) Insert(id uint64, priority int) {
	for _, node := range list.Nodes {
		if node.ID == id {
			return
		}
	}
	node := originalActorNode{id, priority}
	if len(list.Nodes) == 0 || priority <= list.Nodes[0].Priority {
		list.Nodes = append(list.Nodes, originalActorNode{})
		copy(list.Nodes[1:], list.Nodes[:len(list.Nodes)-1])
		list.Nodes[0] = node
	} else {
		list.Nodes = append(list.Nodes, node)
	}
}

func (list *originalActorList) Next(id uint64) uint64 {
	if id == 0 {
		if len(list.Nodes) > 0 {
			return list.Nodes[0].ID
		}
		return 0
	}
	for index, node := range list.Nodes {
		if node.ID == id && index+1 < len(list.Nodes) {
			return list.Nodes[index+1].ID
		}
	}
	return 0
}

func (list *originalActorList) Remove(id uint64) {
	for index, node := range list.Nodes {
		if node.ID == id {
			copy(list.Nodes[index:], list.Nodes[index+1:])
			list.Nodes = list.Nodes[:len(list.Nodes)-1]
			return
		}
	}
}

// syncOriginalActors keeps native storage independent of original traversal.
// Birth IDs retain allocation order even in families stored in reverse. An
// explosion replacing an enemy retains that enemy's node through ActorOwner.
func (w *World) syncOriginalActors() {
	present := make(map[uint64]int)
	for _, enemy := range w.Enemies {
		if enemy.Descriptor != 0 && enemy.Health > 0 {
			present[enemy.ID] = enemy.Priority
		}
	}
	for _, power := range w.PowerUps {
		if power.Original != nil && power.Kind != PowerNone {
			present[power.ID] = power.Priority
		}
	}
	for _, effect := range w.Effects {
		if effect.Sprite != "" && effect.Lifetime > 0 && effect.Age <= effect.Lifetime {
			id := effect.ID
			if effect.ActorOwner != 0 {
				id = effect.ActorOwner
			}
			present[id] = effect.Priority
		}
	}
	w.PeakOriginalActorLinks = max(w.PeakOriginalActorLinks, len(present))
	known := make(map[uint64]bool)
	nodes := w.actorList.Nodes[:0]
	for _, node := range w.actorList.Nodes {
		if priority, ok := present[node.ID]; ok {
			node.Priority = priority
			nodes = append(nodes, node)
			known[node.ID] = true
		}
	}
	w.actorList.Nodes = nodes
	var added []uint64
	for id := range present {
		if !known[id] {
			added = append(added, id)
		}
	}
	sort.Slice(added, func(a, b int) bool { return added[a] < added[b] })
	for _, id := range added {
		w.actorList.Insert(id, present[id])
	}
	w.actorList.Initialized = true
}

func (w *World) orderedEnemyIndices() []int {
	var order []int
	if !w.actorList.Initialized {
		for index := range w.Enemies {
			order = append(order, index)
		}
		return order
	}
	indices := make(map[uint64]int, len(w.Enemies))
	for index, enemy := range w.Enemies {
		indices[enemy.ID] = index
	}
	for _, node := range w.actorList.Nodes {
		if index, ok := indices[node.ID]; ok {
			order = append(order, index)
		}
	}
	return order
}
