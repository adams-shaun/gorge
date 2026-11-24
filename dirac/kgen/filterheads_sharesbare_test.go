package kgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// filterHeadsSlice1Carriers lower through the bare sharesCreatureTypeWith
// word (gorge reads the source: Kinship's PeekAndReveal, Plane-Merge Elf's
// Kinfall) and the one-type permanent alternative of a DamageDone
// ValidTarget$ beside Player (Player,Battle; Creature,Player).
var filterHeadsSlice1Carriers = []string{
	// bare sharesCreatureTypeWith
	"Ink Dissolver", "Kithkin Zephyrnaut", "Mudbutton Clanger", "Nightshade Schemers", "Plane-Merge Elf",
	"Pyroclast Consul", "Sensation Gorger", "Squeaking Pie Grubfellows", "Wandering Graybeard",
	"Waterspout Weavers", "Winnower Patrol", "Wolf-Skull Shaman",
	// DamageDone ValidTarget$ Player,Battle / Creature,Player
	"Archpriest of Shadows", "Beamtown Beatstick", "Deeproot Wayfinder", "Doomskar Warrior",
	"Furnace Reins", "Thorin, Company's Leader", "Crovax",
}

// filterHeadsSlice1Refused stay refused, each with a fragment of its reason:
// Weight of Conscience's tap cost reads the bare word as a group sharing a
// type (Forge), which gorge does not model; Coat of Arms' AffectedX count
// reads the word relative to each affected creature, which the static's
// count does not lower; an Opponent,Battle recipient is not the Player form.
var filterHeadsSlice1Refused = map[string]string{
	"Weight of Conscience": "shared-creature-type group",
	"Coat of Arms":         "FwSharesCr",
	"Bloodfeather Phoenix": "trigger second filter",
}

func buildNamed(t *testing.T, names []string) map[string][]string {
	t.Helper()
	list := filepath.Join(t.TempDir(), "cards.txt")
	if err := os.WriteFile(list, []byte(strings.Join(names, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := loadCardList(list, ratchetCards(t))
	if err != nil {
		t.Fatal(err)
	}
	all, _, _, err := buildPool(lowerPool(p))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string][]string{}
	for _, c := range all {
		rs := []string{}
		for _, r := range c.Refusals {
			rs = append(rs, r.Reason)
		}
		out[c.Name] = rs
	}
	return out
}

func TestFilterHeadsSlice1(t *testing.T) {
	names := append([]string{}, filterHeadsSlice1Carriers...)
	for n := range filterHeadsSlice1Refused {
		names = append(names, n)
	}
	got := buildNamed(t, names)
	for _, n := range filterHeadsSlice1Carriers {
		rs, ok := got[n]
		if !ok {
			t.Errorf("%s: not lowered (corpus missing?)", n)
			continue
		}
		if len(rs) != 0 {
			t.Errorf("%s: refused: %q", n, rs)
		}
	}
	for n, frag := range filterHeadsSlice1Refused {
		if j := strings.Join(got[n], "; "); !strings.Contains(j, frag) {
			t.Errorf("%s: refusals %q, want one naming %q", n, j, frag)
		}
	}
}
