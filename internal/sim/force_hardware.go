package sim

import "github.com/olivierh59500/r-type/internal/source"

// advanceAnimation is also called for an invisible, inactive Force. The source
// decrements a signed byte and treats a zero direction byte as forward motion.
// A rank change selects an out-of-range sentinel, then wraps to the last image.
func (f *OriginalForceMotion) advanceAnimation(ranks []source.ForceWeapon) {
	if len(ranks) != 4 || f.Rank < 0 || f.Rank >= len(ranks) {
		return
	}
	changedRank := f.ShownRank != f.Rank
	if changedRank {
		f.ShownRank, f.FrameDelay = f.Rank, 0
	}
	shape := ranks[f.Rank]
	f.FrameDelay = int(int8(f.FrameDelay) - 1)
	if f.FrameDelay >= 0 {
		return
	}
	f.FrameDelay = max(0, shape.FrameTicks-1)
	if changedRank {
		f.Frame = len(shape.Frames) - 1
		return
	}
	if f.Spin < 0 {
		f.Frame--
	} else {
		f.Frame++
	}
	if f.Frame < 0 {
		f.Frame = len(shape.Frames) - 1
	} else if f.Frame >= len(shape.Frames) {
		f.Frame = 0
	}
}

// resetOriginalForceHardware follows the power reset without discarding the
// inactive hardware state. Core 0x56A preserves Y, displayed rank, animation
// timer, direction and image; the next Copper update handles a rank change.
func (w *World) resetOriginalForceHardware() {
	if w.Force.Original != nil {
		w.forceHardware = *w.Force.Original
	}
	w.forceHardware.Rank, w.forceHardware.X = 0, 0
}

// advanceInactiveOriginalForce retains animation during death hold, fade and
// blackout. The zero state represents boot's first rank-zero image, Y=0,
// timer=0 and forward direction=0. Visible acquisition has its own controller
// initialization; this state never makes an invisible Force collide or draw.
func (w *World) advanceInactiveOriginalForce() {
	if w.Force.Mode != ForceNone {
		return
	}
	if tables := w.weaponTables(); tables != nil {
		w.forceHardware.advanceAnimation(tables.Force)
		w.forceHardware.DrawX, w.forceHardware.DrawY = w.forceHardware.X, w.forceHardware.Y
	}
}
