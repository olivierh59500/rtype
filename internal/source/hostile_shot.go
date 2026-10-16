package source

// HostileShotData stores the common original sixteen-slot projectile pool's
// directional velocities and five indexed art frames. The first frame flies;
// the remaining four frames play when terrain or the Force intercepts a shot.
type HostileShotData struct {
	VelocityX   []int    `json:"velocity_x"`
	VelocityY   []int    `json:"velocity_y"`
	Frames      []string `json:"frames"`
	Width       int      `json:"width"`
	Height      int      `json:"height"`
	PoolSize    int      `json:"pool_size"`
	RandomRange int      `json:"random_range"`
}
