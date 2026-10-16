package sim

import "github.com/olivierh59500/r-type/internal/source"

// OriginalHostileShot stores one of the independent sixteen slots at RAM
// 0x4A3A. Raw coordinates, collision probes and spark phase are independent
// of the decoded sprite's center and its visible fourteen-row image.
type OriginalHostileShot struct {
	X, Y, VX, VY int
	Slot, Phase  int
}

func (w *World) hostileShotData() *source.HostileShotData {
	if tables := w.weaponTables(); tables != nil {
		return tables.HostileShot
	}
	return nil
}

// initializeOriginalAutomaticFire translates allocator 0x10B6. Its single
// random addition becomes the actor's permanent period, rather than being
// rerolled each time a shot is emitted.
func (w *World) initializeOriginalAutomaticFire(enemy *Enemy) {
	data := w.hostileShotData()
	if data == nil || enemy.AutomaticFire <= 0 {
		return
	}
	if data.RandomRange > 0 {
		enemy.AutomaticFire += int(w.nextRandom() % uint64(data.RandomRange))
	}
	enemy.AutomaticFireTimer = enemy.AutomaticFire
}

// updateOriginalAutomaticFire follows 0x12EE after actor contacts. The common
// pass also runs after motion routines return early or retain a spawn delay.
// A collision death in this pass may still fire once; 0x111E would clear its
// period on the following pass, when the original actor becomes an explosion.
func (w *World) updateOriginalAutomaticFire() {
	data, tables := w.hostileShotData(), w.weaponTables()
	if data == nil || tables == nil {
		return
	}
	for index := range w.Enemies {
		enemy := &w.Enemies[index]
		w.updateOriginalAutomaticFireActor(enemy)
	}
}

func (w *World) updateOriginalAutomaticFireActor(enemy *Enemy) {
	data, tables := w.hostileShotData(), w.weaponTables()
	if data == nil || tables == nil {
		return
	}
	if !enemy.automaticFireEligible || enemy.AutomaticFire <= 0 {
		return
	}
	enemy.AutomaticFireTimer--
	if enemy.AutomaticFireTimer >= 0 {
		return
	}
	enemy.AutomaticFireTimer = enemy.AutomaticFire
	x := int(enemy.X-enemy.W/2) + GameOriginX
	y := int(enemy.Y-enemy.H/2) + GameOriginY
	targetX, targetY := w.originalPlayerAim()
	direction := originalAimDirection(x, y, targetX, targetY, tables)
	if direction < len(data.VelocityX) && direction < len(data.VelocityY) {
		w.spawnOriginalHostileShot(x, y, data.VelocityX[direction], data.VelocityY[direction])
	}
}

func (w *World) spawnOriginalHostileShot(x, y, vx, vy int) bool {
	data := w.hostileShotData()
	if data == nil || x == 0 || data.PoolSize <= 0 {
		return false
	}
	used := make([]bool, data.PoolSize)
	for _, bullet := range w.Bullets {
		if h := bullet.Hostile; h != nil && bullet.Lifetime > 0 && h.Slot >= 0 && h.Slot < len(used) {
			used[h.Slot] = true
		}
	}
	for slot, occupied := range used {
		if occupied {
			continue
		}
		h := &OriginalHostileShot{X: x, Y: y, VX: vx, VY: vy, Slot: slot}
		bullet := Bullet{Hostile: h, Kind: EnemyShot, Damage: 1, Lifetime: 1}
		bullet.Entity = w.newEntity(0, 0, float64(vx), float64(vy), float64(data.Width), float64(data.Height))
		w.syncOriginalHostileShot(&bullet, data)
		w.Bullets = append(w.Bullets, bullet)
		return true
	}
	return false
}

func (w *World) syncOriginalHostileShot(bullet *Bullet, data *source.HostileShotData) {
	h := bullet.Hostile
	bullet.X = float64(h.X-GameOriginX) + float64(data.Width)/2
	bullet.Y = float64(h.Y-GameOriginY) + float64(data.Height)/2
	if h.Phase >= 0 && h.Phase < len(data.Frames) {
		bullet.Sprite = data.Frames[h.Phase]
	}
}

// hostilePlayerBounds preserves 0x1354's EEX+3, width20 probe. Actor contact
// instead uses EEX+5, width16; sharing that narrower rectangle loses real hits.
func (w *World) hostilePlayerBounds() Rect {
	box := w.Player.HitBounds()
	x, _ := w.originalPlayerAim()
	box.X = float64(x + 3 - GameOriginX)
	box.W = 20
	return box
}

func inclusiveOverlap(a, b Rect) bool {
	return a.X <= b.X+b.W && a.X+a.W >= b.X && a.Y <= b.Y+b.H && a.Y+a.H >= b.Y
}

// updateOriginalHostileShots translates 0x1354. New shots participate in the
// current pass; their player/Force probes precede movement, and their original
// slot order remains stable even when a lower free slot is reused.
func (w *World) updateOriginalHostileShots() {
	data := w.hostileShotData()
	if data == nil {
		return
	}
	playerBounds := w.hostilePlayerBounds()
	for slot := 0; slot < data.PoolSize; slot++ {
		for index := range w.Bullets {
			bullet := &w.Bullets[index]
			h := bullet.Hostile
			if h == nil || h.Slot != slot || bullet.Lifetime <= 0 {
				continue
			}
			bullet.Age++
			if h.Phase == 0 {
				probe := Rect{X: float64(h.X + 5 - GameOriginX), Y: float64(h.Y + 4 - GameOriginY), W: 6, H: 6}
				if w.OriginalSolidAt(h.X+8, h.Y+7) {
					h.Phase++
				} else if inclusiveOverlap(probe, playerBounds) {
					if w.ControlsEnabled() && w.Player.Invulnerable == 0 {
						w.killPlayer()
					}
					bullet.Lifetime = 0
					continue
				} else if w.Force.Mode != ForceNone && inclusiveOverlap(probe, w.Force.Bounds()) {
					h.Phase++
				} else if h.X < 32 || h.X > 368 || h.Y < 16 || h.Y > 224 {
					bullet.Lifetime = 0
					continue
				} else {
					h.X = int(int16(h.X + h.VX))
					h.Y = int(int16(h.Y + h.VY))
				}
			} else {
				h.Phase++
			}
			if h.Phase >= len(data.Frames) {
				bullet.Lifetime = 0
				continue
			}
			w.syncOriginalHostileShot(bullet, data)
		}
	}
}
