package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// The script search's route on a board whose gaps are a depletion land's
// rider (a Taps-deferred gap) beside a counter-cost source: gorge's ONE
// prefix search (scriptSearch.visit) visits every census gap, so a board the
// tap-another search cannot fund still gets its cast offered (scriptOffers'
// tx arm, script_gen.go). Pair 39 (dr-mbc-bb) dropped gorge's Cast Relic
// Crush {4}{G} exactly there: the tx arm took the prefix, whose gaps
// (rs.gaps, mana_tapanother.go) hold no Taps-deferred source, so Peat Bog's
// {B}{B} stayed unfundable and the play was silently dropped.
func riderCtrGapScen(t *testing.T) (*scen, map[string]Obj) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Peat Bog", Count: 1}, {Name: "Wall of Roots", Count: 1},
		{Name: "Relic Crush", Count: 1}, {Name: "Blasting Station", Count: 1}, {Name: "Island", Count: 16}}
	tbl, _, _, err := cardsettest.Synth(t, "dr-mbc-bb").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for c := range tbl.Cards {
		if _, ok := id[tbl.Cards[c].Name]; !ok {
			id[tbl.Cards[c].Name] = CardID(c)
		}
	}
	sc := newScenTableLib(t, tbl, id["Island"], Config{NoElide: true, GorgeLowering: true})
	objs := map[string]Obj{}
	for _, n := range []string{"Island", "Island", "Peat Bog", "Wall of Roots", "Blasting Station"} {
		objs[n] = sc.perm(id[n], 0)
	}
	objs["Relic Crush"] = sc.hand(id["Relic Crush"], 0)
	bog, wall := sc.slot(objs["Peat Bog"]), sc.slot(objs["Wall of Roots"])
	a, ok := sc.s.riderAbilityOf(sc.k, bog)
	if !ok {
		t.Fatal("precondition: Peat Bog is not the depletion-rider shape")
	}
	if n := sc.s.perms[bog].ctrN(sc.s, int(a.ManaRiderCtr)); n != 2 {
		t.Fatalf("precondition: Peat Bog holds %d depletion counters after its ETB, want 2", n)
	}
	sc.s.perms[bog].flags &^= pfTapped // its ETB tapped it; it has untapped since
	sc.s.perms[wall].flags |= pfSick   // it entered this turn: only the counter route pays it
	return sc, objs
}

func TestRiderBesideCtrGapOffersCast(t *testing.T) {
	sc, objs := riderCtrGapScen(t)
	s, k := sc.s, sc.k
	bog, wall := sc.slot(objs["Peat Bog"]), sc.slot(objs["Wall of Roots"])
	if !s.scriptTapsDefer(k, 0).has(bog) {
		t.Fatal("precondition: Peat Bog is not in the Taps-deferred gap mask")
	}
	if !s.scriptCtr(k, 0).has(wall) {
		t.Fatal("precondition: the sick Wall of Roots is not in the counter-cost gap mask")
	}
	var a avail
	s.availFor(k, 0, &a)
	if a.total != 2 {
		t.Fatalf("precondition: plain availability %d, want the two Islands' 2", a.total)
	}
	if !s.tapXBoard(k, 0) || !s.scriptCtr(k, 0).any() {
		t.Fatal("precondition: this is not a tap-another board over a counter-cost gap")
	}
	d := s.Step()
	offered := false
	for i := 0; i < int(d.N); i++ {
		if d.Options[i].Kind() == AkCast && d.Options[i].Source() == TCode(objs["Relic Crush"]) {
			offered = true
		}
	}
	if !offered {
		t.Fatalf("the cast of Relic Crush is not offered beside the pass; opts %v", d.Options[:d.N])
	}
}

// TestDepletionLastCounterNeedle is the ticket's literal state: a seat with
// Mountain, Mountain, Island and a Sandstone Needle on its last counter. The
// ticket names the seat's spell as Arrow Storm {3}{R}{R}, but kgen refuses
// Arrow Storm in every pool (its raid body reads NoPrevention), so the spell
// here is the pool's {4}{R}{R} Volcanic Dragon, with the fourth Mountain its
// extra generic needs (two Mountains, an Island and the Needle's {R} are only
// 4 mana). The cast is priced through the Needle's activation, which removes
// the last counter and sacrifices the land (CR 605.3b).
func TestDepletionLastCounterNeedle(t *testing.T) {
	deck := []cardset.Entry{{Name: "Sandstone Needle", Count: 1}, {Name: "Volcanic Dragon", Count: 1}, {Name: "Mountain", Count: 18}}
	tbl, _, _, err := cardsettest.Synth(t, "dr-mbc-bb").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for c := range tbl.Cards {
		if _, ok := id[tbl.Cards[c].Name]; !ok {
			id[tbl.Cards[c].Name] = CardID(c)
		}
	}
	sc := newScenTableLib(t, tbl, id["Mountain"], Config{NoElide: true, GorgeLowering: true})
	var needle Obj
	for _, n := range []string{"Mountain", "Mountain", "Island", "Mountain", "Mountain", "Sandstone Needle"} {
		o := sc.perm(id[n], 0)
		if n == "Sandstone Needle" {
			needle = o
		}
	}
	arrow := sc.hand(id["Volcanic Dragon"], 0)
	sl := sc.slot(needle)
	a, ok := sc.s.riderAbilityOf(sc.k, sl)
	if !ok {
		t.Fatal("precondition: Sandstone Needle is not the depletion-rider shape")
	}
	if n := sc.s.perms[sl].ctrN(sc.s, int(a.ManaRiderCtr)); n != 2 {
		t.Fatalf("precondition: the Needle holds %d depletion counters after its ETB, want 2", n)
	}
	sc.s.perms[sl].flags &^= pfTapped // it entered tapped; it has untapped since
	sc.s.perms[sl].subCtrN(sc.s, int(a.ManaRiderCtr), 1)
	if !sc.s.scriptTapsDefer(sc.k, 0).has(sl) {
		t.Fatal("precondition: the Needle on its last counter is not in the Taps-deferred gap mask")
	}
	d := sc.s.Step()
	offered := false
	for i := 0; i < int(d.N); i++ {
		if d.Options[i].Kind() == AkCast && d.Options[i].Source() == TCode(arrow) {
			offered = true
		}
	}
	if !offered {
		t.Fatalf("the cast of Arrow Storm is not offered beside the pass; opts %v", d.Options[:d.N])
	}
}
