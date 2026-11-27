package kgen

import (
	"strings"
	"testing"
)

// animateShapesKernelSurfacesCarriers are the cards the earlier Animate passes
// left refused on a kernel surface rather than a lowering shape: an Animate
// Replacements$ replacement (Llanowar Greenwidow, Realmbreaker the Invasion
// Tree, Geth Thane of Contracts), a GainsAbilitiesOf$ filter over an artifact
// or land source (Territory Forge, Mirran Safehouse, Koh the Face Stealer), a
// parameterised RemoveKeyword$ landwalk (Mystic Decree), and an Animate AtEOT$
// over an any-number target slot (Depthshaker Titan).
var animateShapesKernelSurfacesCarriers = []string{
	"Llanowar Greenwidow",
	"Realmbreaker, the Invasion Tree",
	"Geth, Thane of Contracts",
	"Territory Forge",
	"Mirran Safehouse",
	"Koh, the Face Stealer",
	"Mystic Decree",
	"Depthshaker Titan",
}

// kernelSurfaceRefusal is the one class each carrier was blocked on: the
// test proves that class is gone even when the card keeps a separate,
// out-of-scope blocker (Realmbreaker's ChangeType$ graveyard return, Koh's
// GainsTriggerAbsOf$, its own triggered-ability grant).
var kernelSurfaceRefusal = map[string]string{
	"Llanowar Greenwidow":             "Replacements",
	"Realmbreaker, the Invasion Tree": "Replacements",
	"Geth, Thane of Contracts":        "Replacements",
	"Territory Forge":                 "GainsAbilitiesOf",
	"Mirran Safehouse":                "GainsAbilitiesOf",
	"Koh, the Face Stealer":           "GainsAbilitiesOf",
	"Mystic Decree":                   "RemoveKeyword",
	"Depthshaker Titan":               "AtEOT on a set",
}

// fullyLowered are the carriers whose only blocker this slice names: with the
// kernel surface lowered they must lower with no refusal at all.
var fullyLowered = map[string]bool{
	"Llanowar Greenwidow":      true,
	"Geth, Thane of Contracts": true,
	"Territory Forge":          true,
	"Mystic Decree":            true,
	"Depthshaker Titan":        true,
}

func TestAnimateShapesKernelSurfacesSlice(t *testing.T) {
	for _, name := range animateShapesKernelSurfacesCarriers {
		c := lowerNamedCard(t, name)
		want := kernelSurfaceRefusal[name]
		if name == "Mirran Safehouse" {
			// its graveyard land lenders' mana abilities diverge from gorge on
			// which source taps (lockstep, the animategainsof pool): the typed
			// artifact/land refusal stays until that payment order is modelled
			found := false
			for _, r := range c.Refusals {
				found = found || strings.Contains(r.Reason, "artifact or land sources")
			}
			if !found {
				t.Errorf("%s refusals = %v, want the artifact or land sources refusal", name, c.Refusals)
			}
			continue
		}
		for _, r := range c.Refusals {
			if strings.Contains(r.Reason, want) {
				t.Errorf("%s still refuses its %s class: %v", name, want, r)
			}
		}
		if fullyLowered[name] && len(c.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, c.Refusals)
		}
		var sb strings.Builder
		dumpLIR(&sb, c)
		got := sb.String()
		switch name {
		case "Llanowar Greenwidow", "Geth, Thane of Contracts":
			if !strings.Contains(overrideLIR(c), "exileonleave") {
				t.Errorf("%s: override lacks the Replacements$ leave-to-exile field:\n%s", name, overrideLIR(c))
			}
		case "Territory Forge":
			if !strings.Contains(got, "gainsof=") {
				t.Errorf("%s: lowered IR lacks gainsof=:\n%s", name, got)
			}
		case "Mystic Decree":
			if !strings.Contains(got, "removelandwalk=") {
				t.Errorf("%s: lowered IR lacks the landwalk removal:\n%s", name, got)
			}
		case "Depthshaker Titan":
			if !strings.Contains(got, "AtEOT obj=Targets0") {
				t.Errorf("%s: lowered IR lacks AtEOT obj=Targets0:\n%s", name, got)
			}
			if !strings.Contains(overrideLIR(c), "power") || !strings.Contains(overrideLIR(c), "tough") {
				// the override must reach every target too
				if !strings.Contains(got, "Override obj=Targets0") {
					t.Errorf("%s: override did not reach every target:\n%s", name, got)
				}
			}
		}
	}
}
