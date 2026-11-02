package kgen

import "testing"

// TestDestroyReplacementRefused: the kernel has no destruction proposal, so every
// R:Event$ Destroy carrier refuses by name. Pyramids carries its line on an Effect.
func TestDestroyReplacementRefused(t *testing.T) {
	const why = "repl-hard: Destroy replacement"
	checkCarrier(t, nil, map[string]string{
		"Knight of the Holy Nimbus": why,
		"Clergy of the Holy Nimbus": why,
		"Mossbridge Troll":          why,
		"Crackling Emergence":       why,
		"Harmonious Emergence":      why,
		"Pyramids":                  "",
	}, nil)
}
