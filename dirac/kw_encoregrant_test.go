package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// Araumi of the Dead Tide's pump on the MBC table: the cost is the fixed
// one-card ExileFromGrave (SVar X PlayerCountOpponents$Amount on a two-seat
// card; gorge pay.FixLifeXCost asks no X), so the activation asks the
// graveyard target, then the exile pick, and the resolving GrantEncore
// stamps the encore grant on the card still in the graveyard. gorge offers no activation of a granted encore, and the kernel
// encodes none; the grant ends at the zone change or the cleanup, like a
// granted flashback.
func araumiScen(t *testing.T) (*scen, map[string]CardID) {
	t.Helper()
	var deck []cardset.Entry
	set := cardsettest.Set(t, "mbc")
	for _, n := range []string{"Araumi of the Dead Tide", "Accursed Marauder", "Accorder Paladin"} {
		i, ok := set.Lookup(n)
		if !ok {
			t.Fatalf("%s not in the MBC set list", n)
		}
		if !set.Supported()[i] {
			t.Fatalf("%s not lowered in the MBC table", n)
		}
		deck = append(deck, cardset.Entry{Name: n, Count: 1})
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	if int(v.Plains) >= len(tbl.Cards) {
		t.Fatal("table too small for the scenario library")
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	return newScenTable(t, tbl, Config{NoElide: true}), id
}

func TestAraumiEncoreGrantLandsAndEnds(t *testing.T) {
	sc, id := araumiScen(t)
	ara := sc.perm(id["Araumi of the Dead Tide"], 0)
	tgt := sc.grave(id["Accursed Marauder"], 0, 1)[0]
	fod := sc.grave(id["Accorder Paladin"], 0, 1)[0]

	sc.do(t, AkActivate, TCode(ara), scAny)

	// the target ask: the creature card in the graveyard
	d := sc.expect(t, DkTarget, 0)
	found := false
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkTarget && a.Target() == TCode(tgt) {
			found = true
			sc.s.Apply(a)
		}
	}
	if !found {
		t.Fatalf("the graveyard creature is not a target option: %v", d.Options[:d.N])
	}

	// the exile pick: the one fixed card, either grave card (gorge asks it
	// after the target); pick the fodder so the granted card stays
	d = sc.expect(t, DkChoose, ChCostExile)
	picked := false
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkChoose && a.Source() == TCode(fod) {
			picked = true
			sc.s.Apply(a)
		}
	}
	if !picked {
		t.Fatalf("the fodder is not an exile option: %v", d.Options[:d.N])
	}

	// the ability resolves: the encore grant lands on the card still in the
	// graveyard, the fodder is exiled
	for i := 0; i < 8 && !sc.s.encoreGranted(sc.k, tgt); i++ {
		d = sc.s.Step()
		if d.Kind != DkPriority {
			break
		}
		sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
	}
	if o := sc.s.objs[fod]; o.zone != kop.ZoneExile {
		t.Fatalf("the fodder zone = %d, want exile", o.zone)
	}
	if !sc.s.encoreGranted(sc.k, tgt) {
		t.Fatalf("the graveyard card holds no encore grant")
	}
	// gorge offers nothing for the granted card either: no activation names
	// the graveyard card at priority
	if d := sc.s.Step(); d.Kind == DkPriority {
		for i := 0; i < int(d.N); i++ {
			if a := d.Options[i]; a.Source() == TCode(tgt) || a.Target() == TCode(tgt) {
				t.Fatalf("an action names the granted card: kind %d src %d tgt %d", a.Kind(), a.Source(), a.Target())
			}
		}
	}

	// the grant ends at the cleanup, like a granted flashback: pass to the
	// end of the turn and no further (the grant must still be live before it)
	for i := 0; i < 40; i++ {
		if sc.s.step == kop.StepCleanup && !sc.s.encoreGranted(sc.k, tgt) {
			break
		}
		d = sc.s.Step()
		if d.Kind != DkPriority {
			break
		}
		sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
	}
	if sc.s.step != kop.StepCleanup || sc.s.encoreGranted(sc.k, tgt) {
		t.Fatalf("cleanup step %d, grant live %v; want the cleanup to have ended the grant", sc.s.step, sc.s.encoreGranted(sc.k, tgt))
	}
}

func TestAraumiEncoreGrantEndsOnZoneChange(t *testing.T) {
	sc, id := araumiScen(t)
	ara := sc.perm(id["Araumi of the Dead Tide"], 0)
	tgt := sc.grave(id["Accursed Marauder"], 0, 1)[0]
	fod := sc.grave(id["Accorder Paladin"], 0, 1)[0]

	sc.do(t, AkActivate, TCode(ara), scAny)
	d := sc.expect(t, DkTarget, 0)
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkTarget && a.Target() == TCode(tgt) {
			sc.s.Apply(a)
		}
	}
	d = sc.expect(t, DkChoose, ChCostExile)
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkChoose && a.Source() == TCode(fod) {
			sc.s.Apply(a)
		}
	}
	for i := 0; i < 8 && !sc.s.encoreGranted(sc.k, tgt); i++ {
		d = sc.s.Step()
		if d.Kind != DkPriority {
			break
		}
		sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
	}
	if !sc.s.encoreGranted(sc.k, tgt) {
		t.Fatalf("the graveyard card holds no encore grant")
	}
	// a zone change is a new object: the grant ends (CR 400.7), as gorge's
	// derived keyword does
	sc.s.place(tgt, kop.ZoneHand, false)
	if sc.s.encoreGranted(sc.k, tgt) {
		t.Fatalf("the encore grant survived the zone change")
	}
}
