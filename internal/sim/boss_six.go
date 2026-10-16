package sim

// updateBossSix translates the original camera hook at RAM 0x4A94C. Each
// queued crawler has native routing, shielding and its original art envelope.
func (w *World) updateBossSix(spec *BossSpec) {
	d := spec.Six
	if d == nil || w.Boss.Clearing {
		return
	}
	if !w.Boss.Active && int(w.Scroll) >= d.MusicTrigger {
		w.Boss.Active = true
		w.Boss.MusicSection = 2
		w.Events = append(w.Events, Event{Kind: EventBossStarted, Value: 2})
	}
	if int(w.Scroll) < d.FreezeScroll {
		return
	}
	w.Boss.CameraPaused = true
	w.Boss.QueueTimer--
	if w.Boss.QueueTimer >= 0 {
		return
	}
	w.Boss.QueueTimer = d.SpawnReload
	if w.Boss.QueueCursor >= len(d.Queues) {
		w.startBossVictory(spec)
		return
	}
	queue := &d.Queues[w.Boss.QueueCursor]
	if w.Boss.QueueCursor == len(d.Queues)-1 {
		w.Boss.QueueTimer = d.LastReload
	}
	w.Boss.QueueCursor++
	art := spec.SixArt
	animation := d.Animation
	if queue.FacingSide == 0 {
		animation = queue.LeftAnimation
	}
	w.Enemies = append(w.Enemies, Enemy{Entity: w.newEntity(float64(d.SpawnX)-GameOriginX+float64(art.Width)/2, float64(d.SpawnY)-GameOriginY+float64(art.Height)/2, 0, 0, float64(art.Width), float64(art.Height)), Behavior: BehaviorRouteCrawler, RouteCrawler: queue, Variant: queue.InitialDirection, Kind: EnemyBoss, Health: queue.ContactHealth, MaxHealth: queue.ContactHealth, Descriptor: d.Descriptor, Sprite: art.Sprite, Animation: animation, AnimationTicks: d.AnimationTicks, CollisionFrames: art.CollisionFrames})
}
