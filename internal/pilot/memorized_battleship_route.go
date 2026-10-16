package pilot

import (
	"math"

	"github.com/olivierh59500/r-type/internal/sim"
	"github.com/olivierh59500/r-type/internal/source"
)

const battleshipRouteBlocked float32 = 1e20

const rehearsalExposure float32 = 8

// rehearsedBattleshipRoute is a data-only, time-expanded navigation memory.
// Authored terrain, camera, patch, body and attack bounds guide own-ship motor
// gestures. No cloned world, live enemy HP/RNG or engine stepping is involved.
type rehearsedBattleshipRoute struct {
	speed, width, height, end, minX, minY int
	cost                                  []float32
	shots                                 []bool
}

func rehearseBattleshipRoute(stage sim.StageSpec, camera *rehearsedCamera, iris, hull *memorizedIrisMovie, speed int) *rehearsedBattleshipRoute {
	d, player := stage.Boss.Third.Core, stage.Weapons.Player
	r := &rehearsedBattleshipRoute{speed: speed, end: d.Trigger + d.CycleCount*(d.PatchReload+1) + d.CycleHold - 3}
	r.end += 2*(1+(d.CycleCount-1)*(d.PatchReload+1)) + d.CycleHold + 1 + d.CycleWait + 1
	r.minX, r.minY = 157, 113
	maxX, maxY := 157, 113
	for r.minX+113 > player.LeftLimit {
		r.minX -= speed
	}
	for maxX+113 <= player.RightLimit {
		maxX += speed
	}
	for r.minY+37 > player.TopLimit {
		r.minY -= speed
	}
	for maxY+37 < player.BottomLimit {
		maxY += speed
	}
	r.width, r.height = (maxX-r.minX)/speed+1, (maxY-r.minY)/speed+1
	area := r.width * r.height
	r.cost = make([]float32, (r.end+1)*area)
	tiles := append([]uint16(nil), stage.TileMap...)
	for _, patch := range stage.Boss.Third.InitialPatches {
		for i, word := range patch.Words {
			tiles[(patch.Row+i/patch.Width)*stage.MapWidth+patch.Column+i%patch.Width] = word
		}
	}
	irisSolids := memorizedIrisSolidUnion(stage, iris)
	type mountedEnvelope struct {
		birth int
		world sim.Rect
	}
	var turrets []mountedEnvelope
	for _, wave := range stage.Waves {
		d := wave.BattleshipTurret
		if d == nil && wave.Behavior != sim.BehaviorMountedTurret {
			continue
		}
		birth := wave.Trigger + 43
		point := camera.points[birth-1]
		left, top, right, bottom := wave.W, wave.H, 0.0, 0.0
		for frame, box := range wave.CollisionFrames {
			a := [2]int{}
			if d != nil {
				if frame >= len(d.AimOffsets) {
					break
				}
				a = d.AimOffsets[frame]
			}
			left, top = min(left, float64(a[0]+box.X)), min(top, float64(a[1]+box.Y))
			right, bottom = max(right, float64(a[0]+box.X+box.Width+1)), max(bottom, float64(a[1]+box.Y+box.Height+1))
		}
		turrets = append(turrets, mountedEnvelope{birth, sim.Rect{X: wave.X - wave.W/2 + float64(point.x) + left - 2, Y: wave.Y - wave.H/2 + float64(point.y) + top - 2, W: right - left + 4, H: bottom - top + 4}})
	}
	m := memorizedObservation{stage: stage, camera: camera}
	shotCells := rehearsedCoreShotCells(stage, camera, hull, r, tiles)
	r.shots = shotCells
	for main := r.end; main >= 0; main-- {
		m.cameraAge = main
		point := camera.points[main]
		eye, eyeActive := m.irisEnvelope(0)
		birth, now := camera.projection[d.Trigger], camera.projection[main]
		core := sim.Rect{X: float64(d.X - sim.GameOriginX + birth.x - now.x), Y: float64(d.Y - sim.GameOriginY + birth.y - now.y), W: 31, H: 19}
		for row := 0; row < r.height; row++ {
			for col := 0; col < r.width; col++ {
				i := main*area + row*r.width + col
				r.cost[i] = battleshipRouteBlocked
				if shotCells[i] {
					continue
				}
				x, y := r.minX+col*speed, r.minY+row*speed
				mx, my := (x+point.x-12)/16, (y+point.y-1)/16
				word := tiles[my*stage.MapWidth+mx]
				if next, ok := iris.word(mx, my, max(0, main-1)); ok {
					word = next
					if main >= stage.Boss.Third.Iris.Trigger {
						word |= irisSolids[(my-iris.row)*iris.width+mx-iris.column]
					}
				}
				if next, ok := hull.word(mx, my, max(0, main-1)); ok {
					word = next
				}
				body := sim.Rect{X: float64(x - 9), Y: float64(y - 3), W: 17, H: 8}
				if word&0x8000 != 0 || eyeActive && body.Overlaps(eye) || main >= d.Trigger && body.Overlaps(core) {
					continue
				}
				blocked := false
				exposure := float32(0)
				for _, turret := range turrets {
					if main >= turret.birth {
						box := turret.world
						box.X -= float64(point.x)
						box.Y -= float64(point.y)
						if body.Overlaps(box) {
							blocked = true
							break
						}
						distance := rectDistance(body, box)
						if distance < 100 {
							exposure += rehearsalExposure * float32((100-distance)/100)
						}
					}
				}
				if blocked {
					continue
				}
				if main == r.end {
					if float64(x+20) < core.X && math.Abs(float64(y)-core.Y-9) <= 6 {
						r.cost[i] = float32(math.Abs(float64(x-37)) + 2*math.Abs(float64(y)-core.Y-9))
					}
					continue
				}
				for _, move := range actions {
					nx, ny := max(0, min(r.width-1, col+2*move.dx)), max(0, min(r.height-1, row+2*move.dy))
					next := r.cost[(main+1)*area+ny*r.width+nx]
					if next < battleshipRouteBlocked {
						penalty := float32(0)
						if move.dx != 0 || move.dy != 0 {
							penalty = 1
						}
						r.cost[i] = min(r.cost[i], next+penalty+exposure)
					}
				}
			}
		}
	}
	return r
}

func rehearsedCoreShotCells(stage sim.StageSpec, camera *rehearsedCamera, hull *memorizedIrisMovie, r *rehearsedBattleshipRoute, tiles []uint16) []bool {
	d := stage.Boss.Third.Core
	area := r.width * r.height
	blocked := make([]bool, len(r.cost))
	var births []int
	phase, timer, cursor, wait, fire := 0, 0, 0, 0, 0
	for main := 1; main <= r.end; main++ {
		switch phase {
		case 0:
			if main >= d.AnimationTrigger {
				if main >= d.ShotTrigger {
					fire--
					if fire < 0 {
						births = append(births, main)
						fire = d.FireReload
					}
				}
				timer--
				if timer < 0 {
					timer = d.IdleReload
					cursor++
				}
			}
			if main >= d.Trigger {
				wait--
				if wait < 0 {
					phase, cursor, timer, wait = 1, 0, 0, d.CycleCount
				}
			}
		case 1, 3:
			timer--
			if timer < 0 {
				cursor++
				timer = d.PatchReload
				wait--
				if wait == 0 {
					if phase == 1 {
						phase, timer = 2, d.CycleHold
					} else {
						phase, cursor, timer, wait = 0, 0, 0, d.CycleWait
					}
				}
			}
		case 2:
			timer--
			if timer < 0 {
				phase, wait = 3, d.CycleCount
			}
		}
	}
	m := memorizedObservation{stage: stage, camera: camera}
	for _, birth := range births {
		m.cameraAge = birth
		core := m.rehearsedCorePose()
		for start := 0; start < d.ShotStartRange; start++ {
			for _, vx := range []int{-1, 1, 2} {
				x, y := int(core.X)+d.ShotOffsetX, int(core.Y)+d.ShotOffsetY
				for main := birth + 1; main <= r.end; main++ {
					cam := camera.points[main]
					col, row := (x+cam.x-8)/16, (y+cam.y+8)/16
					word := uint16(0)
					if col >= 0 && col < stage.MapWidth && row >= 0 && row < stage.MapHeight {
						word = tiles[row*stage.MapWidth+col]
						if v, ok := hull.word(col, row, main-1); ok {
							word = v
						}
					}
					if word&0x8000 != 0 {
						break
					}
					x += vx
					y += int(d.ShotMotionY[min(start+main-birth-1, len(d.ShotMotionY)-1)])
					if x+sim.GameOriginX < 0 || x+sim.GameOriginX > 368 || y+sim.GameOriginY < 0 || y+sim.GameOriginY > 224 {
						break
					}
					box := sim.Rect{X: float64(x + 1), Y: float64(y + 1), W: 9, H: 9}
					left, right := max(0, (x-8-r.minX)/r.speed-1), min(r.width-1, (x+19-r.minX)/r.speed+1)
					top, bottom := max(0, (y-3-r.minY)/r.speed-1), min(r.height-1, (y+14-r.minY)/r.speed+1)
					for sy := top; sy <= bottom; sy++ {
						for sx := left; sx <= right; sx++ {
							body := sim.Rect{X: float64(r.minX + sx*r.speed - 9), Y: float64(r.minY + sy*r.speed - 3), W: 17, H: 8}
							if body.Overlaps(box) {
								blocked[main*area+sy*r.width+sx] = true
							}
						}
					}
				}
			}
		}
	}
	return blocked
}

func (r *rehearsedBattleshipRoute) at(x, y float64, main int) float32 {
	if r == nil || main < 0 || main > r.end {
		return 0
	}
	col, row := int(math.Round((x-float64(r.minX))/float64(r.speed))), int(math.Round((y-float64(r.minY))/float64(r.speed)))
	if col < 0 || col >= r.width || row < 0 || row >= r.height {
		return battleshipRouteBlocked
	}
	return r.cost[(main*r.height+row)*r.width+col]
}

// Fixed motor preference selected during ordinary-input route rehearsal.
var rehearsalActionOrder = []action{{1, 0, 0}, {1, 1, 0}, {-1, -1, 0}, {-1, 1, 0}, {0, -1, 0}, {0, 0, 0}, {-1, 0, 0}, {1, -1, 0}, {0, 1, 0}}

func (r *rehearsedBattleshipRoute) waypoint(x, y float64, main, horizon int) (float64, float64, bool) {
	col, row := int(math.Round((x-float64(r.minX))/float64(r.speed))), int(math.Round((y-float64(r.minY))/float64(r.speed)))
	if col < 0 || col >= r.width || row < 0 || row >= r.height || r.at(x, y, main) >= battleshipRouteBlocked {
		return x, y, false
	}
	area := r.width * r.height
	for t := main; t < min(r.end, main+horizon); t++ {
		best := battleshipRouteBlocked
		bx, by := col, row
		for _, a := range rehearsalActionOrder {
			nx, ny := max(0, min(r.width-1, col+2*a.dx)), max(0, min(r.height-1, row+2*a.dy))
			v := r.cost[(t+1)*area+ny*r.width+nx]
			if a.dx != 0 || a.dy != 0 {
				v++
			}
			if v < best {
				best, bx, by = v, nx, ny
			}
		}
		col, row = bx, by
	}
	return float64(r.minX + col*r.speed), float64(r.minY + row*r.speed), true
}

// memorizedIrisSolidUnion is deliberately independent of hit timing. An eye
// can retain an earlier opening pose or display its authored damage/flash
// patches, so every possibly solid cell stays blocked in long-range routing.
func memorizedIrisSolidUnion(stage sim.StageSpec, iris *memorizedIrisMovie) []uint16 {
	mask := make([]uint16, iris.width*iris.height)
	for i, word := range iris.words {
		mask[i%len(mask)] |= word & 0x8000
	}
	add := func(p *source.MapPatch) {
		if p == nil {
			return
		}
		for i, word := range p.Words {
			x, y := p.Column+i%p.Width-iris.column, p.Row+i/p.Width-iris.row
			if x >= 0 && x < iris.width && y >= 0 && y < iris.height {
				mask[y*iris.width+x] |= word & 0x8000
			}
		}
	}
	for _, f := range stage.Boss.Third.Iris.DamageSteps {
		add(f.Patch)
	}
	for _, bank := range stage.Boss.Third.Iris.Flash {
		for _, f := range bank {
			add(f.Patch)
		}
	}
	return mask
}
