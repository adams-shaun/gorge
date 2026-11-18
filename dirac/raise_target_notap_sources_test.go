package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The CR 601.2g window's untapped-source gate reads every available mana
// ability's payability, not the source's tapped flag (gorge mana_available.go
// rejects only mf.Tap && o.Tapped): an ability whose cost needs no tap makes
// its source one however tapped the permanent is. Two shapes on the preysac
// pool:
//
//   - a tapped Blood Vassal ("Sacrifice this: Add {B}{B}") under Dragon's
//     Prey's target-dependent raise ({2} more when the spell targets a
//     Dragon), under Twin Bolt's payment and under Kolaghan's Command's
//     charm ask, whose cost the pool alone cannot pay;
//   - a tapped Birchlore Rangers ("Tap two untapped Elves you control: Add
//     one mana of any color"), whose cost has no {T} of its own, beside two
//     plain Elves.

// preysacScen seats seat 0's board from the preysac pool table (each name
// once, untapped) and returns the scenario and the name index.
func preysacScen(t *testing.T, board ...string) (*scen, map[string]CardID, []Obj) {
	t.Helper()
	set := cardsettest.Set(t, "preysac")
	tbl, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for c := range tbl.Cards {
		if _, ok := id[tbl.Cards[c].Name]; !ok {
			id[tbl.Cards[c].Name] = CardID(c)
		}
	}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	var objs []Obj
	for _, n := range board {
		c, ok := id[n]
		if !ok {
			t.Fatalf("%s not in the preysac table", n)
		}
		objs = append(objs, sc.perm(c, 0))
	}
	return sc, id, objs
}

// preysacAsk casts Dragon's Prey with three {B} and one {G} floating (the
// plain {2}{B} settles from the pool, a Dragon target raises to {4}{B})
// and returns the cast's target ask.
func preysacAsk(t *testing.T, sc *scen, id map[string]CardID) Decision {
	t.Helper()
	dp := sc.hand(id["Dragon's Prey"], 0)
	sc.s.pl[0].pool[2] = 3 // three {B}
	sc.s.pl[0].pool[4] = 1 // one {G}
	sc.do(t, AkCast, TCode(dp), scAny)
	return sc.s.Step()
}

// preysacPay drives the game through the cast's payment: a priority ask
// passes, every other ask takes its first option (the botpolicy clamp's
// fewest, first options), until src (a sacrifice-only mana source) has left
// the battlefield and the pool is empty.
func preysacPay(t *testing.T, sc *scen, src Obj) {
	t.Helper()
	for i := 0; i < 24; i++ {
		if !sc.onBF(src) && sc.s.pl[0].pool == ([kop.NumManaKinds]uint8{}) {
			return
		}
		d := sc.s.Step()
		switch d.Kind {
		case DkNone:
			return
		case DkPriority:
			sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
		default:
			if d.N == 0 {
				t.Fatalf("empty ask kind %d", d.Kind)
			}
			sc.s.Apply(d.Options[0])
		}
	}
	t.Fatal("the cast's payment never finished")
}

func preysacGone(t *testing.T, sc *scen, o Obj, what string) {
	t.Helper()
	if sc.onBF(o) || sc.s.objs[o].zone != kop.ZoneGraveyard {
		t.Errorf("%s on bf %v zone %d, want sacrificed to the graveyard", what, sc.onBF(o), sc.s.objs[o].zone)
	}
	if pool := sc.s.pl[0].pool; pool != ([kop.NumManaKinds]uint8{}) {
		t.Errorf("pool %v after the cast, want empty", pool)
	}
}

// TestRaiseTargetNotapSacSource: beside the Dragon's Prey cast the payer's
// only mana source is a TAPPED Blood Vassal, whose sacrifice-only ability
// needs no tap (CR 602.2b), so the gate counts it: the Dragon target raised
// to {4}{B} stays on the ask, and the cast answered on it completes (the
// Vassal is sacrificed at payment; Scavenger Regent's ward--discard a card
// asks the empty-handed seat 1, whose first option does not pay it).
func TestRaiseTargetNotapSacSource(t *testing.T) {
	sc, id, objs := preysacScen(t, "Blood Vassal")
	vassal := objs[0]
	sc.s.perms[sc.slot(vassal)].flags |= pfTapped
	dragon := sc.perm(id["Scavenger Regent"], 1)
	bear := sc.perm(id["Lasyd Prowler"], 1)
	d := preysacAsk(t, sc, id)
	if raiseTapXOffers(&d, bear) == 0 {
		t.Fatalf("the plain creature's target vanished from the ask: %v", d.Options[:d.N])
	}
	if n := raiseTapXOffers(&d, dragon); n == 0 {
		t.Fatalf("the Dragon target vanished with a tapped sacrifice-only source on the board: %v", d.Options[:d.N])
	}
	// the cast on the Dragon target completes: the pool and the Vassal's
	// {B}{B} pay the raised {4}{B} (CR 601.2g-h)
	for i := 0; i < int(d.N); i++ {
		if TCode(dragon) == d.Options[i].Target() {
			sc.s.Apply(d.Options[i])
			preysacPay(t, sc, vassal)
			preysacGone(t, sc, vassal, "Blood Vassal")
			return
		}
	}
	t.Fatal("no Dragon-target option to answer the ask with")
}

// TestRaiseTargetNotapTapXSource: beside the Dragon's Prey cast the payer's
// only mana source is a TAPPED Birchlore Rangers, whose cost taps two untapped
// Elves and has no {T} of its own, so the gate counts it through the
// payability gate (gorge ManaTapsPayable; the source's own lock is keyed on
// the cost's {T}): the Dragon target stays on the ask. The census cannot
// price the tap-another source, so the menu keeps the target however short
// the window is (gorge's census fallback, castWindowCensusComplete).
func TestRaiseTargetNotapTapXSource(t *testing.T) {
	sc, id, objs := preysacScen(t, "Birchlore Rangers")
	sc.s.perms[sc.slot(objs[0])].flags |= pfTapped
	dragon := sc.perm(id["Scavenger Regent"], 1)
	// the tap-another part's candidates: two untapped Elves without mana
	// abilities, so they are no untapped mana sources of their own
	sc.perm(id["Elvish Warrior"], 0)
	sc.perm(id["Elvish Visionary"], 0)
	bear := sc.perm(id["Lasyd Prowler"], 1)
	d := preysacAsk(t, sc, id)
	if raiseTapXOffers(&d, bear) == 0 {
		t.Fatalf("the plain creature's target vanished from the ask: %v", d.Options[:d.N])
	}
	if n := raiseTapXOffers(&d, dragon); n == 0 {
		t.Fatalf("the Dragon target vanished with a tapped no-{T} tap-another source on the board: %v", d.Options[:d.N])
	}
}

// TestRaiseTargetNotapSacPay: Twin Bolt's {1}{R} with one {R} floating, the
// payer's only mana source a TAPPED Blood Vassal: the target ask is posed and
// the payment completes through the CR 601.2g window (the Vassal's {B}{B}
// funds the {1}, the Vassal is sacrificed).
func TestRaiseTargetNotapSacPay(t *testing.T) {
	sc, id, objs := preysacScen(t, "Blood Vassal")
	vassal := objs[0]
	sc.s.perms[sc.slot(vassal)].flags |= pfTapped
	sc.perm(id["Lasyd Prowler"], 1)
	tb := sc.hand(id["Twin Bolt"], 0)
	sc.s.pl[0].pool[3] = 1 // one {R}
	sc.do(t, AkCast, TCode(tb), scAny)
	d := sc.s.Step()
	if d.Kind != DkTarget || d.N == 0 {
		t.Fatalf("Twin Bolt's target ask is not posed with a tapped sacrifice-only source: kind %d opts %v", d.Kind, d.Options[:d.N])
	}
	sc.s.Apply(d.Options[0])
	preysacPay(t, sc, vassal)
	preysacGone(t, sc, vassal, "Blood Vassal")
}

// TestRaiseTargetNotapCharmAsk: the charm ask's untapped-source gate itself
// (gorge cast_targets.go, charmTargetAskDead). Kolaghan's Command's {1}{B}{R}
// with one {B} and one {R} floating is short of the pool's generic; the
// payer's only mana source is a TAPPED Blood Vassal, so gorge poses the
// choose-two ask and the payment completes through the window (the Vassal's
// {B}{B} funds the {1}); with the Vassal unread the cast reverses unasked.
func TestRaiseTargetNotapCharmAsk(t *testing.T) {
	sc, id, objs := preysacScen(t, "Blood Vassal")
	vassal := objs[0]
	sc.s.perms[sc.slot(vassal)].flags |= pfTapped
	sc.perm(id["Ornithopter"], 1) // a legal artifact target for the destroy mode
	sc.perm(id["Lasyd Prowler"], 1)
	kc := sc.hand(id["Kolaghan's Command"], 0)
	sc.s.pl[0].pool[2] = 1 // one {B}
	sc.s.pl[0].pool[3] = 1 // one {R}
	sc.do(t, AkCast, TCode(kc), scAny)
	d := sc.s.Step()
	if d.Kind != DkModes && d.Kind != DkTarget {
		t.Fatalf("Kolaghan's Command's charm ask is not posed with a tapped sacrifice-only source: kind %d opts %v", d.Kind, d.Options[:d.N])
	}
	preysacPay(t, sc, vassal)
	preysacGone(t, sc, vassal, "Blood Vassal")
}

// TestRaiseTargetNotapSacSourceUntapped: the same Vassal untapped keeps the
// Dragon target on the ask (the gate's untapped arm) -- the shape the gate
// read before the no-tap arms moved ahead of the tapped skip.
func TestRaiseTargetNotapSacSourceUntapped(t *testing.T) {
	sc, id, _ := preysacScen(t, "Blood Vassal")
	dragon := sc.perm(id["Scavenger Regent"], 1)
	bear := sc.perm(id["Lasyd Prowler"], 1)
	d := preysacAsk(t, sc, id)
	if raiseTapXOffers(&d, bear) == 0 {
		t.Fatalf("the plain creature's target vanished from the ask: %v", d.Options[:d.N])
	}
	if n := raiseTapXOffers(&d, dragon); n == 0 {
		t.Fatalf("the Dragon target vanished with an untapped sacrifice-only source on the board: %v", d.Options[:d.N])
	}
}

// TestRaiseTargetNotapNoSource: the same board with no mana source at all.
// The gate reads no source to tap, so the Dragon target stays off the menu.
func TestRaiseTargetNotapNoSource(t *testing.T) {
	sc, id, _ := preysacScen(t)
	dragon := sc.perm(id["Scavenger Regent"], 1)
	bear := sc.perm(id["Lasyd Prowler"], 1)
	d := preysacAsk(t, sc, id)
	if raiseTapXOffers(&d, bear) == 0 {
		t.Fatalf("the plain creature's target vanished from the ask: %v", d.Options[:d.N])
	}
	if n := raiseTapXOffers(&d, dragon); n != 0 {
		t.Fatalf("the Dragon target is offered %d times with no mana source on the board: %v", n, d.Options[:d.N])
	}
}
