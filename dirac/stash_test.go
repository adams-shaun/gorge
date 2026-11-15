package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// stashScen: p0 controls Tinybones, Bauble Burglar, a creature to
// sacrifice and swamps Swamps; p1's Eaten Alive is in exile with a stash
// counter, and p1 controls a creature to target.
func stashScen(t *testing.T, cfg Config, swamps int) (sc *scen, eaten Obj, lands []Obj) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Tinybones, Bauble Burglar", Count: 1}, {Name: "Eaten Alive", Count: 1}, {Name: "Eager Trufflesnout", Count: 2}, {Name: "Swamp", Count: 10}}
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
	if int(v.Plains) >= len(tbl.Cards) {
		t.Skip("table too small for the scenario library")
	}
	sc = newScenTable(t, tbl, cfg)
	sc.perm(id["Tinybones, Bauble Burglar"], 0)
	sc.perm(id["Eager Trufflesnout"], 0)
	sc.perm(id["Eager Trufflesnout"], 1)
	for i := 0; i < swamps; i++ {
		lands = append(lands, sc.perm(id["Swamp"], 0))
	}
	eaten = sc.obj(id["Eaten Alive"], 1)
	sc.s.place(eaten, kop.ZoneExile, false)
	sc.s.mObjSet(sc.k, mechStash, eaten)
	return sc, eaten, lands
}

func tappedN(sc *scen, lands []Obj) int {
	n := 0
	for _, o := range lands {
		if sc.s.perms[sc.slot(o)].flags&pfTapped != 0 {
			n++
		}
	}
	return n
}

// TestMayPlayEitherOrPaysThePart: Eaten Alive cast through Tinybones'
// stash with the {3}{B} part costs {3}{B}{B} (CR 601.2f: the additional
// cost is added however the spell is cast). Before, a may-play cast's cost
// left the part's mana out and only {B} was paid.
func TestMayPlayEitherOrPaysThePart(t *testing.T) {
	sc, eaten, lands := stashScen(t, Config{NoElide: true}, 5)
	sc.do(t, AkCast, TCode(eaten), scAny)
	d := sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChAltAdd {
		t.Fatalf("want the either-or ask, got kind %d sub %d", d.Kind, d.Sub)
	}
	ok := false
	for _, a := range d.Options[:d.N] {
		if a.Target() == 1 {
			sc.s.Apply(a)
			ok = true
			break
		}
	}
	if !ok {
		t.Fatalf("the {3}{B} part is not offered: %v", d.Options[:d.N])
	}
	for i := 0; i < 6 && !(sc.s.objs[eaten].zone == kop.ZoneStack && sc.s.ctl == ctlPriority); i++ {
		d = sc.s.Step()
		sc.s.Apply(d.Options[0])
	}
	if n := tappedN(sc, lands); n != 5 {
		t.Fatalf("%d Swamps tapped for the {3}{B} part, want 5 ({3}{B}{B})", n)
	}
}

// TestStashCastLoweredUnderSurface: under the oracle's autopay surface
// gorge reaches a stash cast its planner does not price (Eaten Alive) by
// lowering the potential play: the witness for the printed {B} is tapped
// as the cast begins, and the card goes to the stack first (CR 601.2a, gorge
// ), a new object without its stash counter. Picking the unpayable
// {3}{B} part reverses the cast (CR 733.1): the card returns to exile still
// without the counter (pairs 14 1228 7587 7713).
func TestStashCastLoweredUnderSurface(t *testing.T) {
	sc, eaten, lands := stashScen(t, Config{NoElide: true, GorgeAutopaySurface: true, GorgeAltAdd: true}, 1)
	sc.do(t, AkCast, TCode(eaten), scAny)
	d := sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChAltAdd {
		t.Fatalf("want the either-or ask, got kind %d sub %d", d.Kind, d.Sub)
	}
	if tappedN(sc, lands) != 1 || sc.s.pl[0].pool[2] != 1 {
		t.Fatalf("the witness Swamp is not tapped into the pool (tapped %d, pool %v)", tappedN(sc, lands), sc.s.pl[0].pool)
	}
	if sc.s.objs[eaten].zone != kop.ZoneStack || sc.s.mObj(sc.k, mechStash, eaten) {
		t.Fatal("the card is not on the stack without its stash counter at the either-or ask")
	}
	for _, a := range d.Options[:d.N] {
		if a.Target() == 1 {
			sc.s.Apply(a)
		}
	}
	if sc.s.objs[eaten].zone != kop.ZoneExile || sc.s.mObj(sc.k, mechStash, eaten) {
		t.Fatalf("after the reversed cast: zone %d stash %v, want exile without the counter", sc.s.objs[eaten].zone, sc.s.mObj(sc.k, mechStash, eaten))
	}
}

// TestStashCastPlansUnderRider: gorge's cast planner funds a stash cast's pips
// as generic under the "mana of any type" rider (rules payment_plan.go
// planCastPaymentChecked -> pay.PlanManaUnderRider), so Slumbering Cerberus's
// {1}{R} is {2} to the plan and taps both Plains, the first two sources, leaving
// the Mountain.
func TestStashCastPlansUnderRider(t *testing.T) {
	deck := []cardset.Entry{{Name: "Tinybones, Bauble Burglar", Count: 1}, {Name: "Slumbering Cerberus", Count: 1}, {Name: "Plains", Count: 5}, {Name: "Mountain", Count: 5}}
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
	if int(v.Plains) >= len(tbl.Cards) {
		t.Skip("table too small for the scenario library")
	}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	sc.perm(id["Tinybones, Bauble Burglar"], 0)
	pl := []Obj{sc.perm(id["Plains"], 0), sc.perm(id["Plains"], 0)}
	mtn := sc.perm(id["Mountain"], 0)
	cer := sc.obj(id["Slumbering Cerberus"], 1)
	sc.s.place(cer, kop.ZoneExile, false)
	sc.s.mObjSet(sc.k, mechStash, cer)
	sc.do(t, AkCast, TCode(cer), scAny)
	for i := 0; i < 6 && !(sc.s.objs[cer].zone == kop.ZoneStack && sc.s.ctl == ctlPriority); i++ {
		d := sc.s.Step()
		sc.s.Apply(d.Options[0])
	}
	if sc.s.objs[cer].zone != kop.ZoneStack {
		t.Fatalf("Slumbering Cerberus zone %d, want the stack", sc.s.objs[cer].zone)
	}
	if sc.tapped(mtn) || !sc.tapped(pl[0]) || !sc.tapped(pl[1]) {
		t.Fatalf("tapped Mountain=%v Plains=%v,%v; want both Plains and not the Mountain", sc.tapped(mtn), sc.tapped(pl[0]), sc.tapped(pl[1]))
	}
}

// TestStashCastAnyTypeWitness: under the autopay surface without
// Config.GorgeUnpriced, a stash cast whose
// printed symbols p0's sources cannot pay is still offered (the any-type
// rider pays it), and the lowered cast's witness prices the any-type cost
// when the printed one has no plan: Slumbering Cerberus's {1}{R} from two
// Plains. Before, the witness planned {1}{R} and panicked "autopay found no
// plan" (47 of 3,000 self-play games, pair 241).
func TestStashCastAnyTypeWitness(t *testing.T) {
	// (Imprisoned in the Moon's granted {C} source in the table makes the
	// planner gate read the ungranted sources, as a pool table does)
	deck := []cardset.Entry{{Name: "Tinybones, Bauble Burglar", Count: 1}, {Name: "Slumbering Cerberus", Count: 1}, {Name: "Imprisoned in the Moon", Count: 1}, {Name: "Plains", Count: 5}, {Name: "Mountain", Count: 5}}
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
	if int(v.Plains) >= len(tbl.Cards) {
		t.Skip("table too small for the scenario library")
	}
	sc := newScenTable(t, tbl, Config{NoElide: true, GorgeAutopaySurface: true, GorgeAltAdd: true})
	sc.perm(id["Tinybones, Bauble Burglar"], 0)
	pl := []Obj{sc.perm(id["Plains"], 0), sc.perm(id["Plains"], 0)}
	cer := sc.obj(id["Slumbering Cerberus"], 1)
	sc.s.place(cer, kop.ZoneExile, false)
	sc.s.mObjSet(sc.k, mechStash, cer)
	sc.do(t, AkCast, TCode(cer), scAny)
	for i := 0; i < 6 && !(sc.s.objs[cer].zone == kop.ZoneStack && sc.s.ctl == ctlPriority); i++ {
		d := sc.s.Step()
		sc.s.Apply(d.Options[0])
	}
	if sc.s.objs[cer].zone != kop.ZoneStack || !sc.tapped(pl[0]) || !sc.tapped(pl[1]) {
		t.Fatalf("Slumbering Cerberus zone %d, Plains tapped %v,%v: want it cast from both Plains", sc.s.objs[cer].zone, sc.tapped(pl[0]), sc.tapped(pl[1]))
	}
}

// TestStashCounterLeavesWithLand: a land played from Tinybones' stash keeps
// its stash counter on the battlefield (gorge), but leaving the
// battlefield makes a new object without it (CR 400.7): a Forest bounced
// to the hand and played again has no counter (pair 10760). Before, the
// kernel's stash mark survived the move.
func TestStashCounterLeavesWithLand(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Forest", Count: 8}})
	f := sc.perm(id["Forest"], 0)
	sc.s.stashSet(sc.k, f)
	sc.s.leave(sc.k, sc.slot(f), kop.ZoneHand, false)
	if sc.s.mObj(sc.k, mechStash, f) {
		t.Fatalf("the Forest kept its stash mark in the hand")
	}
}
