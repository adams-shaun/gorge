package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Baxter Building's "{4}, {T}: Add four mana in any combination of colours"
// funds Attuma, Atlantean Warlord ({2}{U}{U}) from an empty floating pool,
// with Island, Plains, Fisk Tower ({W} or {B}) and Training Compound ({C})
// untapped: the plain search spends its node budget on the fee-carrying gap's
// own abilities, and gorge's planner prices the cast on the feeOrder run
// (potentialPrefixScript's second build, scriptSearch.feeOrder). The kernel
// must list the cast too (std-msh pair 99, decision 367; the committed
// recording is baxter_feeorder_funded_cast).
func TestBaxterUUCastOffered(t *testing.T) {
	set, err := cardset.SynthByName("std-msh")
	if err != nil {
		t.Skipf("std-msh: %v", err)
	}
	var deck []cardset.Entry
	deck = append(deck, cardset.Entry{Name: "Attuma, Atlantean Warlord", Count: 4},
		cardset.Entry{Name: "Baxter Building", Count: 6},
		cardset.Entry{Name: "Island", Count: 6},
		cardset.Entry{Name: "Plains", Count: 8},
		cardset.Entry{Name: "Fisk Tower", Count: 6},
		cardset.Entry{Name: "Training Compound", Count: 10})
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Plains"], Config{})
	bax := sc.perm(id["Baxter Building"], 0)
	sc.perm(id["Island"], 0)
	sc.perm(id["Plains"], 0)
	ft := sc.perm(id["Fisk Tower"], 0)
	sc.s.perms[sc.slot(ft)].flags &^= pfTapped // the game's untap step ran; the ETB-tapped replacement must not stand
	sc.perm(id["Training Compound"], 0)
	att := sc.hand(id["Attuma, Atlantean Warlord"], 0)

	// The board is the divergence's: Baxter the only filter gap, the four
	// free taps the funding line, the floating pool empty.
	k := sc.k
	if k.gen.filt(id["Baxter Building"]) < 0 {
		t.Fatalf("Baxter Building is not prepped as a filter")
	}
	filt := sc.s.scriptFilt(k, 0)
	if !filt.has(sc.slot(bax)) {
		t.Fatalf("Baxter Building is not a census gap; gaps %v", filt)
	}
	if sc.s.gapOthers(k, 0).any() {
		t.Fatalf("the board has other gaps %v", sc.s.gapOthers(k, 0))
	}
	if !sc.onBF(bax) || sc.s.perms[sc.slot(bax)].flags&pfTapped != 0 {
		t.Fatalf("Baxter Building is not an untapped battlefield permanent")
	}
	var d Decision
	sc.s.priorityOptions(k, &d)
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		if isCastKind(a.Kind()) && a.Source() == TCode(att) {
			return
		}
	}
	t.Fatalf("cast(Attuma) not offered; %d options", d.N)
}
