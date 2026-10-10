//go:build race

package tests

// the fiona binary under test is built with the race detector too
const raceEnabled = true
