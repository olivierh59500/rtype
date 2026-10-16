package pilot

import "github.com/olivierh59500/r-type/internal/sim"

// bossDamageReport is retrospective telemetry, never pilot perception. The
// fourth encounter stores accepted damage on three separate weak-point actors.
type bossDamageReport struct{ fourth map[uint64]int }

func (r *bossDamageReport) observe(w *sim.World) int {
	if w.Stage < 1 || w.Stage > len(w.Stages) {
		clear(r.fourth)
		return w.Boss.DamageTaken
	}
	spec := w.Stages[w.Stage-1].Boss
	if spec == nil || spec.Fourth == nil || !w.Boss.Active {
		clear(r.fourth)
		return w.Boss.DamageTaken
	}
	if r.fourth == nil {
		r.fourth = make(map[uint64]int)
	}
	for _, enemy := range w.Enemies {
		if enemy.Descriptor == spec.Fourth.WeakpointDescriptor {
			r.fourth[enemy.ID] = max(r.fourth[enemy.ID], min(spec.Fourth.WeakpointHealth, enemy.BehaviorTicks))
		}
	}
	total := 0
	for _, damage := range r.fourth {
		total += damage
	}
	return total
}
