package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestConvokeStrandedCensusHidesPickedManaCreature (CR 601.2g-h, 702.51a,
// 733.1): a convoke cast whose picked creature can also tap for mana is
// stranded when the lands cannot pay what the pick leaves, whatever paid
// {1},{T} fee filter the board holds. Halo Hopper ({3}) with Llanowar
// Elves convoked for {1} leaves {2}; Rootrider Faun ({T}: {G}; {1},{T}: any colour, a filter that
// nets no mana) alone makes one mana, so
// the cast is undone before any mana ability. The picked Elves' own {T}: {G}
// must not count as a second source (gorge's census skips the
// convoke-committed creature).
func TestConvokeStrandedCensusHidesPickedManaCreature(t *testing.T) {
	deck := []cardset.Entry{
		{Name: "Halo Hopper", Count: 1},
		{Name: "Llanowar Elves", Count: 1},
		{Name: "Rootrider Faun", Count: 1},
		{Name: "Plains", Count: 2},
		{Name: "Forest", Count: 4},
	}
	set := cardsettest.Set(t, "convokefilter")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Forest"], Config{NoElide: true})
	hopper := sc.hand(id["Halo Hopper"], 0)
	elves := sc.perm(id["Llanowar Elves"], 0)
	faun := sc.perm(id["Rootrider Faun"], 0)
	if !sc.k.gen.anyFilt || !sc.s.scriptFilt(sc.k, 0).has(sc.slot(faun)) {
		t.Fatalf("precondition: Rootrider Faun is no live fee filter (anyFilt %v)", sc.k.gen.anyFilt)
	}

	sc.do(t, AkCast, TCode(hopper), scAny)
	d := sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChGenConvoke {
		t.Fatalf("first decision %v/%d, want the convoke ask", d.Kind, d.Sub)
	}
	picked := false
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkChoose && Obj(a.Source()) == elves && a.Mode() == 5 {
			sc.s.Apply(a)
			picked = true
			break
		}
	}
	if !picked {
		t.Fatalf("Llanowar Elves' convoke pick not offered: %v", d.Options[:d.N])
	}
	sc.do(t, AkDone, scAny, scAny)
	sc.s.Step()

	if sc.s.objs[hopper].zone != kop.ZoneHand {
		t.Fatalf("Halo Hopper is in zone %d, want its hand: the cast is stranded and undone", sc.s.objs[hopper].zone)
	}
	for _, o := range []Obj{faun, elves} {
		if sc.s.perms[sc.slot(o)].flags&pfTapped != 0 {
			t.Fatalf("object %d is tapped: the window opened and tapped sources for a stranded payment", o)
		}
	}
}
