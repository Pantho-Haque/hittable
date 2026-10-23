//go:build race

package screens

// raceSlowdown scales wall-clock budgets: the race detector makes code
// roughly 5-20x slower, so a frame-time assertion means nothing unscaled.
const raceSlowdown = 10
