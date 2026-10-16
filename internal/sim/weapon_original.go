package sim

import (
	"sort"

	"github.com/olivierh59500/r-type/internal/source"
)

type OriginalWeaponKind uint8

const (
	OriginalSpread OriginalWeaponKind = iota
	OriginalCannon
	OriginalWaveSmall
	OriginalWaveLarge
	OriginalRicochet
	OriginalGroundWave
	OriginalMissile
)

// OriginalWeaponMotion contains music-independent projectile rule state. Raw
// positions are the original blitter anchors, not executable memory addresses.
type OriginalWeaponMotion struct {
	ProbeColumn                    int
	Kind                           OriginalWeaponKind
	X, Y, VX, VY                   int
	Direction, Frame, Phase, Delay int
	Stream, Slot, Group            int
	RouteBank                      int
	Budget, Wait                   int
	Upper                          bool
	Target                         uint64
	Collision                      source.WeaponBounds
	Frames                         []string
	Width, Height                  int
}

// OriginalWeaponImage is one original blitter write retained for presentation
// across both PAL display frames. Layer zero precedes actors; one follows them.
type OriginalWeaponImage struct {
	Entity
	Sprite string
	Layer  int
}

func (w *World) recordOriginalWeaponImage(m *OriginalWeaponMotion, frame int) {
	layer := 0
	if m.Kind == OriginalMissile {
		layer = 1
		frame = m.Direction*2 + frame
	}
	if frame < 0 || frame >= len(m.Frames) {
		return
	}
	w.WeaponImages = append(w.WeaponImages, OriginalWeaponImage{Entity: Entity{X: float64(m.X-GameOriginX) + float64(m.Width)/2, Y: float64(m.Y-GameOriginY) + float64(m.Height)/2, W: float64(m.Width), H: float64(m.Height)}, Sprite: m.Frames[frame], Layer: layer})
}

func (w *World) updateOriginalWeaponsPreActors() {
	w.WeaponImages = w.WeaponImages[:0]
	var updates []int
	for index := range w.Bullets {
		b := &w.Bullets[index]
		if b.Original != nil && b.Original.Kind != OriginalMissile && b.Lifetime > 0 {
			updates = append(updates, index)
		}
	}
	// Source 0x19AC visits spread words in descending physical order;
	// 0x1A6C then visits the cannon, small waves, large wave and route pool.
	// Allocation interleaves route slots 0,4,1,5,..., but 0x1C40 draws 0..7.
	sort.SliceStable(updates, func(a, b int) bool {
		return originalWeaponUpdateOrder(w.Bullets[updates[a]].Original) < originalWeaponUpdateOrder(w.Bullets[updates[b]].Original)
	})
	for _, index := range updates {
		w.updateOriginalBullet(&w.Bullets[index])
	}
}

func originalWeaponUpdateOrder(m *OriginalWeaponMotion) int {
	switch m.Kind {
	case OriginalSpread:
		return 9 - m.Slot
	case OriginalCannon:
		return 10
	case OriginalWaveSmall:
		return 11 + m.Group - 20
	case OriginalWaveLarge:
		return 13
	case OriginalRicochet, OriginalGroundWave:
		return 20 + m.Slot
	}
	return 100
}

func (w *World) updateOriginalWeaponsPostActors(input Input) {
	active := w.originalWeaponCount(30)
	for index := range w.Bullets {
		b := &w.Bullets[index]
		if b.Original != nil && b.Original.Kind == OriginalMissile && b.Lifetime > 0 {
			w.updateOriginalBullet(b)
		}
	}
	if active == 0 {
		w.updateOriginalMissilesMain(input)
	}
}

func (w *World) spawnOriginalWeapon(kind BulletKind, m OriginalWeaponMotion, damage int, piercing bool) bool {
	if m.Kind == OriginalSpread {
		low, high := originalSpreadSlots(m.Group)
		m.Slot = -1
		for slot := high; slot >= low; slot-- {
			used := false
			for _, bullet := range w.Bullets {
				used = used || bullet.Lifetime > 0 && bullet.Original != nil && bullet.Original.Kind == OriginalSpread && bullet.Original.Slot == slot
			}
			if !used {
				m.Slot = slot
				break
			}
		}
		if m.Slot < 0 {
			return false
		}
	}
	width, height := float64(m.Width), float64(m.Height)
	w.addBullet(kind, true, float64(m.X-GameOriginX)+width/2, float64(m.Y-GameOriginY)+height/2, float64(m.VX), float64(m.VY), width, height, damage, 1000, piercing)
	b := &w.Bullets[len(w.Bullets)-1]
	b.Original = &m
	if piercing {
		b.PierceBudget = damage
	}
	if len(m.Frames) > 0 {
		frame := m.Frame
		if frame < 0 || frame >= len(m.Frames) {
			frame = 0
		}
		b.Sprite = m.Frames[frame]
	}
	return true
}

// originalSpreadSlots reproduces the physical words used by 0x1874/0x1DE0.
// The ship owns slots 0..3. Force straight/up/down patterns share slots 4..7;
// the full-rank diagonal patterns occupy slots 9 and 8 respectively.
func originalSpreadSlots(group int) (low, high int) {
	switch group {
	case 2:
		return 4, 7
	case 3:
		return 4, 5
	case 4:
		return 6, 7
	case 5:
		return 9, 9
	case 6:
		return 8, 8
	}
	return 0, 3
}

func (w *World) originalWeaponCount(group int) int {
	count := 0
	for index := range w.Bullets {
		b := &w.Bullets[index]
		if b.Original != nil && b.Original.Group == group && b.Lifetime > 0 {
			count++
		}
	}
	return count
}

func (w *World) fireOriginalNormal() {
	data := w.weaponTables()
	if data == nil || len(data.Shots) < 5 {
		return
	}
	// Source 0x1898 dispatches the Force burst before allocating the ship shot.
	if w.Force.Mode != ForceNone {
		w.fireOriginalForceWeapon()
	}
	if w.originalWeaponCount(0) < 4 {
		shot := data.Shots[0]
		if w.spawnOriginalWeapon(Shot, OriginalWeaponMotion{Kind: OriginalSpread, X: int(w.Player.X) + 45, Y: int(w.Player.Y) + 31, VX: shot.VX, VY: shot.VY, Frames: []string{shot.Frame}, Width: shot.Width, Height: shot.Height, Group: 0, Collision: source.WeaponBounds{Width: 12, Height: 4}}, 1, false) {
			w.Events = append(w.Events, Event{Kind: EventShot, X: w.Player.X, Y: w.Player.Y})
		}
	}
}

func (w *World) fireOriginalBeam(charge int) {
	data := w.weaponTables()
	if data == nil {
		return
	}
	for _, beam := range data.Beams {
		if charge < beam.Threshold {
			continue
		}
		// The original wave-cannon slot is singular; a new discharge replaces
		// its remaining wave rather than creating simultaneous charge beams.
		for i := range w.Bullets {
			if b := &w.Bullets[i]; b.Original != nil && b.Original.Kind == OriginalCannon {
				b.Lifetime = 0
			}
		}
		frames := beam.Frames
		if len(frames) == 0 {
			frames = []string{beam.Frame}
		}
		w.spawnOriginalWeapon(Beam, OriginalWeaponMotion{Kind: OriginalCannon, X: int(w.Player.X) + 18 + beam.X, Y: int(w.Player.Y) + 25 + beam.Y, VX: 14, ProbeColumn: beam.ProbeColumn, Frames: frames, Width: beam.Width, Height: beam.Height, Group: 1, Collision: beam.Collision}, beam.Damage, true)
		w.Events = append(w.Events, Event{Kind: EventBeam, X: w.Player.X, Y: w.Player.Y, Value: charge})
		return
	}
}

func (w *World) fireOriginalForceWeapon() {
	data := w.weaponTables()
	if data == nil || w.Force.Original == nil {
		return
	}
	f := w.Force.Original
	if f.Side == 0 {
		// Free Force bursts use the separate five-entry spread table.
		indices := []int{0}
		if f.Rank != 0 {
			indices = []int{1, 2}
		}
		if f.Rank == 3 {
			indices = append(indices, 3, 4)
		}
		for _, number := range indices {
			group, capacity := 2+number, 4
			if number == 1 || number == 2 {
				capacity = 2
			}
			if number >= 3 {
				capacity = 1
			}
			if w.originalWeaponCount(group) >= capacity {
				continue
			}
			shot := data.Shots[number]
			w.spawnOriginalWeapon(ForceShot, OriginalWeaponMotion{Kind: OriginalSpread, X: f.X - 85, Y: f.Y - 8, VX: shot.VX, VY: shot.VY, Width: shot.Width, Height: shot.Height, Frames: []string{shot.Frame}, Group: group, Collision: source.WeaponBounds{X: 2, Y: 2, Width: 8, Height: 9}}, 1, false)
		}
		return
	}
	if f.Rank == 0 {
		return
	}
	switch w.Player.Weapon {
	case WeaponRed:
		w.fireOriginalWave()
	case WeaponBlue:
		w.fireOriginalRicochet()
	case WeaponYellow:
		w.fireOriginalGround()
	}
}

func (w *World) fireOriginalRicochet() {
	data := w.weaponTables()
	if w.originalWeaponCount(10) > 0 {
		return
	}
	direction := 1
	start := int(w.Player.X) + 48
	if w.Force.Mode == ForceBack {
		direction = -1
		start = int(w.Player.X) - 2
	}
	count := 4
	if w.Force.Original.Rank == 3 {
		count = 8
	}
	for index := 0; index < count; index++ {
		variant := 0
		if index%2 == 1 {
			variant = 3
		}
		if direction < 0 {
			variant = 1
			if index%2 == 1 {
				variant = 2
			}
		}
		wait := index / 2
		slot := index / 2
		if index%2 == 1 {
			slot += 4
		}
		w.spawnOriginalWeapon(BlueLaser, OriginalWeaponMotion{Kind: OriginalRicochet, X: start, Y: int(w.Player.Y) + 25, Direction: variant, Slot: slot, Wait: wait, Budget: 75, Width: 16, Height: 16, Frames: data.RicochetFrames, Group: 10, Collision: source.WeaponBounds{X: 7, Y: 7, Width: 7, Height: 7}}, 1, false)
	}
}

func (w *World) fireOriginalGround() {
	data := w.weaponTables()
	if w.originalWeaponCount(11) > 0 {
		return
	}
	start, bank := int(w.Player.X)+48, 0
	if w.Force.Mode == ForceBack {
		start = int(w.Player.X) + 2
		bank = 8
	}
	for index := 0; index < 8; index++ {
		v := 0
		rowBank := bank
		if index%2 == 1 {
			v = 2
			rowBank += 4
		}
		wait := index / 2
		slot := index / 2
		if index%2 == 1 {
			slot += 4
		}
		w.spawnOriginalWeapon(YellowLaser, OriginalWeaponMotion{Kind: OriginalGroundWave, X: start, Y: int(w.Player.Y) + 25, Direction: v, RouteBank: rowBank, Slot: slot, Wait: wait, Width: 16, Height: 14, Frames: data.GroundFrames, Group: 11, Collision: source.WeaponBounds{X: 2, Y: 2, Width: 11, Height: 11}}, 1, false)
	}
	w.Events = append(w.Events, Event{Kind: EventSound, Value: 4, X: w.Player.X, Y: w.Player.Y})
}

func (w *World) fireOriginalWave() {
	data := w.weaponTables()
	w.Events = append(w.Events, Event{Kind: EventSound, Value: 3, X: w.Player.X, Y: w.Player.Y})
	direction := 1
	back := 0
	if w.Force.Mode == ForceBack {
		direction = -1
		back = 1
	}
	if w.Force.Original.Rank != 3 {
		for stream := 0; stream < 2; stream++ {
			if w.originalWeaponCount(20+stream) > 0 {
				continue
			}
			first := stream*4 + back*2
			if first+2 > len(data.WaveSmall) {
				continue
			}
			start := int(w.Force.Original.X) - 95 + 15
			if back != 0 {
				start = int(w.Force.Original.X) - 95 - 18
			}
			w.spawnOriginalWeapon(RedLaser, OriginalWeaponMotion{Kind: OriginalWaveSmall, X: start, Y: w.Force.Original.Y - 12 + stream*12, VX: 9 * direction, Width: 32, Height: 4, Frames: data.WaveSmall[first : first+2], Group: 20 + stream, Collision: source.WeaponBounds{Width: 20, Height: 4}}, 2, true)
		}
		return
	}
	// At full rank the two hardware satellites own the small-wave slots.
	// Their sampled sprite positions remain independent of the Force pod.
	for stream := 0; stream < w.Player.Bits && stream < 2; stream++ {
		if w.originalWeaponCount(20+stream) > 0 {
			continue
		}
		first := stream*4 + back*2
		if first+2 > len(data.WaveSmall) {
			continue
		}
		start := int(w.bitDraw[stream].X) + 26
		if back != 0 {
			start -= 30
		}
		w.spawnOriginalWeapon(RedLaser, OriginalWeaponMotion{Kind: OriginalWaveSmall, X: start, Y: int(w.bitDraw[stream].Y) + 28, VX: 9 * direction, Width: 32, Height: 4, Frames: data.WaveSmall[first : first+2], Group: 20 + stream, Collision: source.WeaponBounds{Width: 20, Height: 4}}, 2, true)
	}
	if w.originalWeaponCount(22) > 0 {
		return
	}
	start := int(w.Force.Original.X) - 95 + 16
	initialFrame := 5
	if back != 0 {
		start = int(w.Force.Original.X) - 95 - 48
		initialFrame = 1
	}
	w.spawnOriginalWeapon(RedLaser, OriginalWeaponMotion{Kind: OriginalWaveLarge, X: max(0, start), Y: w.Force.Original.Y - 17, VX: 10 * direction, Frame: initialFrame, Width: 64, Height: 30, Frames: data.WaveLarge, Group: 22, Collision: source.WeaponBounds{X: 2, Y: 2, Width: 58, Height: 26}}, 5, true)
}

// updateOriginalMissilesMain reproduces the original level-triggered launcher.
// Its fifty-six-tick cooldown only counts while both missile slots are empty.
func (w *World) updateOriginalMissilesMain(input Input) {
	if w.weaponTables() == nil || !w.Player.Missiles {
		return
	}
	if w.originalWeaponCount(30) > 0 {
		return
	}
	if w.originalMissileCooldown > 0 {
		w.originalMissileCooldown--
		if w.originalMissileCooldown > 0 {
			return
		}
	}
	if !input.Fire {
		return
	}
	data := w.weaponTables()
	for index := 0; index < 2; index++ {
		vy := data.MissileLaunchVY[index]
		if vy == 0 {
			vy = 2
			if index == 0 {
				vy = -2
			}
		}
		w.spawnOriginalWeapon(Missile, OriginalWeaponMotion{Kind: OriginalMissile, X: int(w.Player.X) + 23, Y: int(w.Player.Y) + 25, VY: vy, Direction: 4, Delay: 9, Upper: index == 0, Slot: index, Width: 16, Height: 16, Frames: data.MissileFrames, Group: 30, Collision: data.MissileCollision[4]}, 1, false)
	}
	w.originalMissileCooldown = 56
}

func (w *World) updateOriginalBullet(b *Bullet) {
	m, data := b.Original, w.weaponTables()
	if m == nil || data == nil {
		return
	}
	b.Age++
	if m.Kind == OriginalSpread {
		if w.OriginalSolidAt(m.X, m.Y) {
			w.OriginalSolidHit(m.X, m.Y)
			b.Lifetime = 0
			return
		}
		w.recordOriginalWeaponImage(m, m.Frame)
	} else if m.Kind == OriginalCannon {
		if len(m.Frames) > 1 {
			m.Frame ^= 1
		}
		w.recordOriginalWeaponImage(m, m.Frame)
		col, row := w.originalProjectileTile(m.X, m.Y+7)
		if w.originalCannonTile(col, row) || w.originalCannonTile(col+m.ProbeColumn, row) {
			b.Lifetime = 0
			return
		}
	} else if m.Kind == OriginalWaveSmall {
		probe := 20
		if m.VX < 0 {
			probe = -16
		}
		if w.OriginalSolidHit(m.X+probe, m.Y) {
			b.Lifetime = 0
			return
		}
		m.Frame ^= 1
		w.recordOriginalWeaponImage(m, m.Frame)
	} else if m.Kind == OriginalWaveLarge {
		change := 1
		if m.VX < 0 {
			change = -1
		}
		m.Frame = (m.Frame + change + len(m.Frames)) % len(m.Frames)
		w.recordOriginalWeaponImage(m, m.Frame)
		probe := 46
		if m.VX < 0 {
			probe = 14
		}
		if w.OriginalSolidHit(m.X+probe, m.Y+14) {
			b.Lifetime = 0
			return
		}
	}
	switch m.Kind {
	case OriginalMissile:
		w.moveOriginalMissile(m, data)
		if w.OriginalSolidAt(m.X+15, m.Y+8) {
			b.Lifetime = 0
			anchor := Entity{X: float64(m.X + 8 - GameOriginX), Y: float64(m.Y - GameOriginY)}
			if effect, ok := w.originalDeathEffect(anchor, 1); ok {
				w.Effects = append(w.Effects, effect)
			}
			return
		}
		m.Frame = b.Age % 2
		if m.X >= 0 && m.X <= 368 && m.Y >= 0 && m.Y < 242 {
			w.recordOriginalWeaponImage(m, m.Frame)
		}
	case OriginalRicochet:
		if m.Wait == 0 {
			draw := *m
			dx, dy := w.originalCameraCompensation()
			draw.X -= dx
			draw.Y -= dy
			w.recordOriginalWeaponImage(&draw, m.Direction&1)
		}
		w.moveOriginalRicochet(m, data, b)
	case OriginalGroundWave:
		if m.Wait == 0 {
			draw := *m
			dx, dy := w.originalCameraCompensation()
			draw.X -= dx
			draw.Y -= dy
			w.recordOriginalWeaponImage(&draw, m.Direction)
		}
		w.moveOriginalGround(m, data, b)
	default:
		m.X += m.VX
		m.Y += m.VY
	}
	if m.Kind == OriginalMissile {
		m.Frame = b.Age % 2
	}
	if m.Kind == OriginalMissile && len(data.MissileCollision) > m.Direction {
		m.Collision = data.MissileCollision[m.Direction]
	}
	b.X, b.Y = float64(m.X-GameOriginX)+float64(m.Width)/2, float64(m.Y-GameOriginY)+float64(m.Height)/2
	b.VX, b.VY = float64(m.VX), float64(m.VY)
	frame := m.Frame
	if m.Kind == OriginalMissile {
		frame = m.Direction*2 + m.Frame
	}
	if frame >= 0 && frame < len(m.Frames) {
		b.Sprite = m.Frames[frame]
	}
	if m.X < 0 || m.X > 368 || m.Y < 0 || m.Y >= 242 {
		b.Lifetime = 0
	}
	if m.Kind == OriginalWaveSmall && (m.X < 16 || m.X >= 368) {
		b.Lifetime = 0
	}
	if m.Kind == OriginalSpread && (m.X >= 368 || m.Y < 16 || m.Y >= 240) {
		b.Lifetime = 0
	}
	if (m.Kind == OriginalCannon || m.Kind == OriginalWaveLarge) && m.X >= 368 {
		b.Lifetime = 0
	}
}

// The cannon distinguishes penetrable bit-fourteen walls from ordinary
// solids. Core 0x1AC2 clears destructibles before marking those special walls.
func (w *World) originalCannonTile(col, row int) bool {
	tile, col, row := w.originalRouteTile(col, row)
	if tile&0x8000 == 0 {
		return false
	}
	if tile&0x1000 != 0 {
		w.SetTileAt(col, row, 0)
		return false
	}
	if tile&0x4000 != 0 {
		w.SetTileAt(col, row, tile|0x2000)
		return false
	}
	return true
}

// CollisionBounds uses the original per-weapon probe offsets and inclusive
// envelopes independently of the projectile's transparent image dimensions.
func (b Bullet) CollisionBounds() Rect {
	if b.Original == nil {
		return b.Entity.Bounds()
	}
	m := b.Original
	return Rect{float64(m.X - GameOriginX + m.Collision.X), float64(m.Y - GameOriginY + m.Collision.Y), float64(m.Collision.Width), float64(m.Collision.Height)}
}

func (w *World) moveOriginalMissile(m *OriginalWeaponMotion, data *source.Weapons) {
	switch m.Phase {
	case 0:
		m.Y += m.VY
		m.Delay--
		if m.Delay < 0 {
			m.Phase = 1
			m.Delay = 6
		}
	case 1:
		m.X += 6
		m.Delay--
		if m.Delay < 0 {
			m.Phase = 2
		}
	case 2:
		best := 65535
		for index := len(w.Enemies) - 1; index >= 0; index-- {
			e := &w.Enemies[index]
			if e.Health <= 0 || e.Invincible {
				continue
			}
			x, y := int(e.X-e.W/2)+GameOriginX, int(e.Y-e.H/2)+GameOriginY
			if (y < m.Y) != m.Upper {
				continue
			}
			distance := absInt(x-m.X) + absInt(y-m.Y)
			if distance <= best {
				best = distance
				m.Target = e.ID
			}
		}
		if m.Target != 0 {
			m.Phase = 3
			m.Delay = 0
		}
		m.X += 6
	case 3:
		var target *Enemy
		for index := range w.Enemies {
			e := &w.Enemies[index]
			if e.ID == m.Target && e.Health > 0 {
				target = e
				break
			}
		}
		if target != nil {
			m.Delay--
			if m.Delay < 0 {
				m.Delay = 3
				direction := originalAimDirection(m.X, m.Y, int(target.X-target.W/2)+GameOriginX, int(target.Y-target.H/2)+GameOriginY, data)
				m.Direction = originalTurnDirection(direction, m.Direction)
			}
		}
		if m.Direction >= 0 && m.Direction < len(data.MissileVelocity) {
			velocity := data.MissileVelocity[m.Direction]
			m.VX, m.VY = velocity[0], velocity[1]
			m.X += m.VX
			m.Y += m.VY
		}
	}
}

func originalAimDirection(x, y, targetX, targetY int, data *source.Weapons) int {
	dx, dy := x-targetX, y-targetY
	base := 0
	if dy < 0 {
		dy = -dy
		base += 5
	}
	if dx < 0 {
		dx = -dx
		base += 10
	}
	index := 0
	if dx != 0 {
		ratio := (dy * 512) / dx
		for index < 5 && index < len(data.AimThresholds) && ratio <= data.AimThresholds[index] {
			index++
		}
		if index == 5 {
			index = 4
		}
	}
	if base+index < len(data.AimMap) {
		return data.AimMap[base+index]
	}
	return 4
}

func originalTurnDirection(target, current int) int {
	difference := target - current
	if (difference < 0 && difference < -8) || (difference >= 0 && difference <= 8) {
		current++
	} else {
		current--
	}
	if current < 0 {
		return 15
	}
	if current > 15 {
		return 0
	}
	return current
}

func (w *World) moveOriginalRicochet(m *OriginalWeaponMotion, data *source.Weapons, b *Bullet) {
	dx, dy := w.originalCameraCompensation()
	m.X -= dx
	y := m.Y - dy
	if m.Wait > 0 {
		m.Wait--
		return
	}
	if m.Direction < 0 || m.Direction >= len(data.Ricochet) {
		b.Lifetime = 0
		return
	}
	for _, route := range data.Ricochet[m.Direction] {
		col, row := w.originalProjectileTile(m.X+7, y+7)
		tile, col, row := w.originalRouteTile(col+route.TileX, row+route.TileY)
		if tile&0x8000 != 0 {
			if tile&0x4000 != 0 {
				w.SetTileAt(col, row, tile|0x2000)
				b.Lifetime = 0
				return
			}
			w.Events = append(w.Events, Event{Kind: EventSound, Value: 2, X: b.X, Y: b.Y})
			continue
		}
		m.Budget--
		if m.Budget < 0 {
			b.Lifetime = 0
			return
		}
		m.X += route.DX
		m.Y = y + route.DY
		m.Direction = route.Next
		m.Frame = m.Direction & 1
		w.originalRouteBounds(m, b)
		return
	}
	b.Lifetime = 0
}

func (w *World) moveOriginalGround(m *OriginalWeaponMotion, data *source.Weapons, b *Bullet) {
	dx, dy := w.originalCameraCompensation()
	m.X -= dx
	y := m.Y - dy
	if m.Wait > 0 {
		m.Wait--
		return
	}
	if m.Direction+m.RouteBank < 0 || m.Direction+m.RouteBank >= len(data.Ground) {
		b.Lifetime = 0
		return
	}
	for _, route := range data.Ground[m.Direction+m.RouteBank] {
		if route.Next < 0 {
			b.Lifetime = 0
			return
		}
		col, row := w.originalProjectileTile(m.X+7, y+7)
		tile, _, _ := w.originalRouteTile(col+route.TileX, row+route.TileY)
		if tile&0x8000 != 0 {
			continue
		}
		m.X += route.DX
		m.Y = y + route.DY
		m.Direction = route.Next
		m.Frame = m.Direction
		w.originalRouteBounds(m, b)
		return
	}
	b.Lifetime = 0
}

// Route offsets address the contiguous original map, so stepping left from
// column zero reaches the last word of the preceding row before expiry.
func (w *World) originalRouteTile(col, row int) (uint16, int, int) {
	width := w.Stages[w.Stage-1].MapWidth
	if width <= 0 {
		return 0, col, row
	}
	index := row*width + col
	if index < 0 || index >= len(w.Tiles) {
		return 0, col, row
	}
	col, row = index%width, index/width
	return w.RawTileAt(col, row), col, row
}

// originalRouteBounds preserves core 0x1DBC's inclusive route-weapon limits.
func (w *World) originalRouteBounds(m *OriginalWeaponMotion, b *Bullet) {
	if m.X < 38 || m.X > 368 || m.Y < 16 || m.Y > 224 {
		b.Lifetime = 0
	}
}

func (w *World) originalProjectileTile(x, y int) (int, int) {
	return (x + int(w.Scroll) - GameOriginX - 16) / 16, (y + int(w.ScrollY) - GameOriginY) / 16
}
