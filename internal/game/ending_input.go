package game

import "github.com/olivierh59500/r-type/internal/sim"

// endingInput retains the physical Force button while a pending death gates
// the automatic route. The owned Force still runs before main death reset.
func endingInput(world *sim.World, input sim.Input) sim.Input {
	if world.State == sim.Playing && world.Ending() {
		input.Left, input.Right, input.Up, input.Down, input.Fire = false, false, false, false, false
		if !world.Scene.DeathRequested {
			input.Force = false
		}
	}
	return input
}
