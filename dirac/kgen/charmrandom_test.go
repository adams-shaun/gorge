package kgen

import "testing"

// TestRandomCharmRefused: a Random$ Charm draws as it goes on the stack in gorge,
// and the kernel reads no Random$.
func TestRandomCharmRefused(t *testing.T) {
	const why = "unread params [Random"
	checkCarrier(t, nil, map[string]string{
		"Cult of Skaro":           why,
		"Outlaws' Merriment":      why,
		"Umaro, Raging Yeti":      why,
		"Typhoid Mary, Fractured": why,
		"Summon: Magus Sisters":   why,
	}, nil)
}
