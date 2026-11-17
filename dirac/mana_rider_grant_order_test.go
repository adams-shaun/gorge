package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// A rider-first source beside a grant is two answers of one "choose a mana
// ability" ask in gorge's search: the printed rider (a Taps-deferred gap)
// first, the granted ability after — at every node, including one the rider's
// own tap reached, where the tapped source's sacrifice-only grant is still
// activatable (grantSacLive). This is the probe shape: Sandstone Needle (two
// depletion counters, untapped) beside Rain of Filth's granted sacrifice
// ("Sacrifice this land: Add {B}"), a Swamp whose printed {T} paid the Rain,
// and a {B} cast the rider's {R}{R} cannot pay. gorge's plan taps the
// Needle's rider and then sacrifices the NEEDLE over its grant; routing the
// tdef candidate to tapsDeferBranch alone (gapBranch's is=true) skips the
// grant side, so the payment sacrificed the SWAMP over its grant instead and
// the wrong land died (CR 601.2g, 605.1a).
func riderGrantScen(t *testing.T) (*scen, Obj, Obj, Obj, Obj) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Sandstone Needle", Count: 1}, {Name: "Rain of Filth", Count: 1},
		{Name: "Dark Ritual", Count: 1}, {Name: "Swamp", Count: 17}}
	tbl, _, _, err := cardsettest.Set(t, "mbc").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for c := range tbl.Cards {
		if _, ok := id[tbl.Cards[c].Name]; !ok {
			id[tbl.Cards[c].Name] = CardID(c)
		}
	}
	for _, n := range []string{"Sandstone Needle", "Rain of Filth", "Dark Ritual", "Swamp"} {
		if _, ok := id[n]; !ok {
			t.Fatalf("precondition: %s is not in the table", n)
		}
	}
	sc := newScenTableLib(t, tbl, id["Swamp"], Config{NoElide: true, GorgeLowering: true})
	needle := sc.perm(id["Sandstone Needle"], 0) // first in battlefield order
	swamp := sc.perm(id["Swamp"], 0)
	ns := sc.slot(needle)
	a, ok := sc.s.riderAbilityOf(sc.k, ns)
	if !ok {
		t.Fatal("precondition: Sandstone Needle is not the depletion-rider shape")
	}
	if n := sc.s.perms[ns].ctrN(sc.s, int(a.ManaRiderCtr)); n != 2 {
		t.Fatalf("precondition: the Needle holds %d depletion counters after its ETB, want 2", n)
	}
	sc.s.perms[ns].flags &^= pfTapped // it entered tapped; it has untapped since
	rain := sc.hand(id["Rain of Filth"], 0)
	ritual := sc.hand(id["Dark Ritual"], 0)
	return sc, needle, swamp, rain, ritual
}

// resolveStack passes priority until card o has left the stack.
func (sc *scen) resolveStack(t *testing.T, o Obj) {
	t.Helper()
	for i := 0; i < 12 && sc.s.objs[o].zone == kop.ZoneStack; i++ {
		d := sc.s.Step()
		if d.Kind != DkPriority {
			t.Fatalf("decision kind %d while resolving, want priority", d.Kind)
		}
		sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
	}
	if sc.s.objs[o].zone == kop.ZoneStack {
		t.Fatal("precondition: the spell never resolved")
	}
}

func TestRiderBesideGrantGrantSideFollowsTheRider(t *testing.T) {
	sc, needle, swamp, rain, ritual := riderGrantScen(t)
	s, k := sc.s, sc.k
	ns, ss := sc.slot(needle), sc.slot(swamp)

	// the Rain's own {B} leaves the Swamp the only source: its printed tap
	// pays, which taps it out
	sc.do(t, AkCast, TCode(rain), scAny)
	// a printed tap's payment runs when the entry's (empty) build completes
	for i := 0; i < 6 && !sc.tapped(swamp); i++ {
		s.Step()
	}
	if !sc.tapped(swamp) {
		t.Fatal("precondition: the Swamp's printed tap did not pay the Rain")
	}
	sc.resolveStack(t, rain)
	if !s.liveGrantGap(k, 0) {
		t.Fatal("precondition: the Rain's granted sacrifice is not live")
	}
	if !s.scriptTapsDefer(k, 0).has(ns) {
		t.Fatal("precondition: the Needle is not in the Taps-deferred gap mask")
	}
	if !s.tdBesideGrant(k, 0) {
		t.Fatal("precondition: this is not a Taps-deferred source beside a live grant")
	}
	if !sc.onBF(needle) || !s.grantSacOK(k, ns) || !s.grantSacOK(k, ss) {
		t.Fatal("precondition: a land on the battlefield holds no live sacrifice grant")
	}

	// the rider's {R}{R} cannot pay {B}: the payment is [the Needle's rider,
	// then the NEEDLE's grant]. The Swamp's grant would be the rider-first
	// routing's skip (the wrong land dies). The payment's steps run inside
	// the cast's Apply, which parks the entry at its build stage (no modes,
	// X or targets), so the zones already hold the payment.
	sc.do(t, AkCast, TCode(ritual), scAny)
	if s.ctl != ctlBuild {
		t.Fatalf("precondition: the cast of Dark Ritual is not at its build stage (ctl %d)", s.ctl)
	}
	if sc.onBF(needle) {
		t.Fatalf("the Needle's grant side was skipped: the Needle is still on the battlefield (tapped %v, Swamp sacrificed %v)",
			sc.tapped(needle), !sc.onBF(swamp))
	}
	if !sc.onBF(swamp) {
		t.Fatal("the Swamp was sacrificed: the payment took the Swamp's grant over the Needle's")
	}
}
