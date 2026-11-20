package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestZeroFiltTapChargesOnce: a funding tap beside a zero-amount dynamic
// any-colour filter that both zero-filter gates claim -- a Three Tree City
// beside its plain {T}: {C}, its {2},{T} filter yielding nothing -- charges
// the search's node budget for the tap once: the ability ask, then the {C}
// answer; the filter's own answer is no branch, because it yields nothing
// (mana_zerofilt.go's comment). With the old gate (mana_zerofilt.go) and the
// new one (mana_filtzerofund.go) both claiming the tap, the ask was ticked
// twice and the colour search ran twice: seven nodes a tap where gorge spends
// them once, so at a budget boundary the search exhausted before gorge and
// refused an option gorge offers -- the lockstep class
// TestMBCMinamoSchoolAtWaterSEdgeReprosLockstep fixes, reversed. The pin is a
// failing search (Starfleet Academy's {5} filter gap no board mana funds,
// Three Tree City untapped until its tap's turn): fundTap yields the
// one-answer tap to fundOne's gate, and the search that visited the tap's
// co-firing branch four times spent 92 nodes before the yield, 64 with it and
// 37 once the zero filter's answer is no branch (the Shiv's Embrace pin), and
// 22 once a doomed node's plain funders are no branch (filtSearch.doomed: the
// node whose offer check failed with Starfleet's gap used, nothing left that
// adds mana, spends no node on a funder's tap). The pruning only removes nodes
// below a failed check, so a doubled tick on a tap that is still a branch
// moves the count off 22 just as it moved it off 37.
func TestZeroFiltTapChargesOnce(t *testing.T) {
	deck := []cardset.Entry{
		{Name: "Starfleet Academy", Count: 4}, {Name: "Three Tree City", Count: 4},
		{Name: "Saurian Explorer", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Forest", Count: 12}, {Name: "Island", Count: 12},
	}
	set := cardsettest.Set(t, "dynfilter")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Forest"], Config{NoElide: true})
	sc.perm(id["Island"], 0)
	sc.perm(id["Island"], 0)
	ttc := sc.perm(id["Three Tree City"], 0)
	sfa := sc.perm(id["Starfleet Academy"], 0)
	sc.perm(id["Grizzly Bears"], 0) // a non-Borg creature: Starfleet's X is 1
	saur := sc.hand(id["Saurian Explorer"], 0)
	s, k := sc.s, sc.k

	// Precondition: Three Tree City is the co-firing shape -- a dynamic
	// any-colour filter yielding nothing beside its plain {T}: {C}, usable,
	// out of the search's gaps, its fee {2} unpaid at the root.
	tsl := sc.slot(ttc)
	if !k.gen.has(id["Three Tree City"], gfFilt) || !k.gen.anyDynFilt {
		t.Fatal("want Three Tree City a modelled filter beside a dynamic one")
	}
	a := s.filtAb(k, tsl)
	if a.AFlags&kop.AfManaDynamic == 0 || a.ManaColors != kop.ColAny || s.filtUnits(k, tsl) != 0 ||
		!s.filtUsable(k, tsl) || s.filtFirstOnly(k, tsl) {
		t.Fatalf("want the filter dynamic, any one colour, at zero units, usable, not first-only: dyn=%v colAny=%v units=%d usable=%v firstOnly=%v",
			a.AFlags&kop.AfManaDynamic != 0, a.ManaColors == kop.ColAny, s.filtUnits(k, tsl), s.filtUsable(k, tsl), s.filtFirstOnly(k, tsl))
	}
	if _, pip := s.filtPip(k, tsl); pip {
		t.Fatal("want the filter's fee pipless")
	}
	if _, pip := s.filtRepeatPip(k, tsl); pip {
		t.Fatal("want the filter's fee without a repeat pip")
	}
	if _, _, hyb := s.filtHyb(k, tsl); hyb {
		t.Fatal("want the filter's fee without a hybrid pip")
	}
	cast := MakeAction(AkCast, TCode(saur), 0, 0)
	fs := &filtSearch{}
	fs.a, fs.p, fs.gaps, fs.own, fs.sp0 = cast, 0, s.scriptFilt(k, 0), NoSlot, s.sp
	fs.need = [kop.NumManaKinds]bool{}
	fs.other = s.otherGaps(k, 0, cast)
	for b := fs.gaps; b.any(); b = b.rest() {
		fs.fees += s.filtFee(k, b.first())
	}
	if !fs.gaps.has(sc.slot(sfa)) || fs.fees != 5 || fs.gaps.has(tsl) {
		t.Fatalf("want Starfleet Academy the {5} filter gap and Three Tree City no gap: gaps %v fees %d", fs.gaps, fs.fees)
	}
	// Precondition: with two mana floating, both gates claim the tap -- the
	// co-firing shape this test pins the single charge of.
	tf := s.fork()
	tf.pl[0].pool[kop.C] += 2
	old, new := fs.zeroFiltAsk(k, tf, tsl), fs.zeroFiltAsks(k, tf, tsl)
	s.unfork()
	if !old || !new {
		t.Fatalf("want both zero-filter gates to claim the tap with two mana floating: old=%v new=%v", old, new)
	}
	// The search cannot fund the {5} (two Islands, Three Tree City and
	// Starfleet's own {T}: {C} are four units) and fails, so it explores the
	// whole tree: the tap's co-firing branch is visited four times.
	t0 := s.fork()
	t0.g.noGrant = true
	ok := fs.visit(k, t0, 0, false)
	s.unfork()
	if ok {
		t.Fatal("the search found a script for the {3}{G} cast, want a failed one (a pin over the whole tree)")
	}
	if n := fs.nodes; n != 22 {
		t.Fatalf("the search spent %d nodes, want 22: the tap's ask is charged once and the zero filter's answer is no branch", n)
	}
}
