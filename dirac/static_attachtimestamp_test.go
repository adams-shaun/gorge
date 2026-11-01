package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestEquipAfterLossRestampsTheEquipment: CR 613.7e, gorge
// TestEquipAfterAbilityLossRestampsTheEquipment. Celestial Armor entered
// before Witness Protection, but equipping it after the Aura attached gives
// it a newer timestamp, so its flying grant survives the loss (pair 143 of
// FDN 1500: Strix Lookout kept flying in gorge).
func TestEquipAfterLossRestampsTheEquipment(t *testing.T) {
	deck := []cardset.Entry{{Name: "Witness Protection", Count: 1}, {Name: "Celestial Armor", Count: 1},
		{Name: "Ambush Wolf", Count: 1}, {Name: "Plains", Count: 10}}
	set := cardsettest.Set(t, "fdn")
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the FDN table", e.Name)
		}
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	wolf := sc.perm(id["Ambush Wolf"], 0)
	armor := sc.perm(id["Celestial Armor"], 0)
	wp := sc.perm(id["Witness Protection"], 0)
	sc.s.attachTo(sc.k, sc.slot(wp), sc.slot(wolf))
	if m, _ := sc.s.FeatKeywords(wolf); m&kop.KwFlying != 0 {
		t.Fatalf("unequipped wolf has flying: %#x", m)
	}
	sc.s.attachTo(sc.k, sc.slot(armor), sc.slot(wolf))
	if m, _ := sc.s.FeatKeywords(wolf); m&kop.KwFlying == 0 {
		t.Fatalf("armor equipped after the loss: keywords %#x, want flying", m)
	}
	// a re-attach to the same creature is no move: no new stamp
	ts := sc.s.lstamp[sc.slot(armor)]
	sc.s.attachTo(sc.k, sc.slot(armor), sc.slot(wolf))
	if sc.s.lstamp[sc.slot(armor)] != ts {
		t.Fatal("re-attach to the same host restamped")
	}
}

// TestControlChangeKeepsLayerTs: CR 613.7d, a control change gives the
// permanent no new layer timestamp (gorge changes Controller only). The
// kernel restamps perm.ts on a control change (the battlefield list
// order), which must not move the layer timestamp:
// Celestial Armor stolen after Witness Protection attached would have
// outlived the loss.
func TestControlChangeKeepsLayerTs(t *testing.T) {
	deck := []cardset.Entry{{Name: "Witness Protection", Count: 1}, {Name: "Celestial Armor", Count: 1},
		{Name: "Ambush Wolf", Count: 1}, {Name: "Plains", Count: 10}}
	set := cardsettest.Set(t, "fdn")
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the FDN table", e.Name)
		}
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	wolf := sc.perm(id["Ambush Wolf"], 0)
	armor := sc.perm(id["Celestial Armor"], 0)
	sc.s.perms[sc.slot(armor)].attached = sc.slot(wolf) // equipped at its entry stamp
	wp := sc.perm(id["Witness Protection"], 0)
	sc.s.attachTo(sc.k, sc.slot(wp), sc.slot(wolf))
	if m, _ := sc.s.FeatKeywords(wolf); m&kop.KwFlying != 0 {
		t.Fatalf("older armor grant survived the loss: %#x", m)
	}
	p := &sc.s.perms[sc.slot(armor)]
	ts0, lts0 := p.ts, sc.s.layerTs(sc.slot(armor))
	sc.s.joined(p)
	if p.ts == ts0 {
		t.Fatal("joined did not restamp the list order")
	}
	if got := sc.s.layerTs(sc.slot(armor)); got != lts0 {
		t.Fatalf("control change moved the layer timestamp %d -> %d", lts0, got)
	}
	if m, _ := sc.s.FeatKeywords(wolf); m&kop.KwFlying != 0 {
		t.Fatalf("control change revived the older grant: %#x", m)
	}
}
