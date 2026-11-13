package dirac

import "testing"

// A spell's own RaiseCost Waterbend<N> (Benevolent River Spirit {U}{U} + waterbend
// {5}, CR 701.67a): the offer credit, the pick ask capped at N, the taps made after
// the mana, and the stack object. The lockstep cannot reach this cast: gorge's
// autopay vocabulary offers no waterbend spell without floating mana, which it
// never offers (oracle/raisecostwaterbend_test.go).

func rcwbSetup(t *testing.T, bears, islands int) (sc *scen, id map[string]CardID, spirit Obj, bs, is []Obj) {
	t.Helper()
	tbl, id := wbTable(t, "Benevolent River Spirit", "Grizzly Bears", "Island")
	if tbl.Cards[id["Benevolent River Spirit"]].SpellWaterbend != 5 {
		t.Fatalf("SpellWaterbend = %d, want 5", tbl.Cards[id["Benevolent River Spirit"]].SpellWaterbend)
	}
	sc = newScenTableLib(t, tbl, id["Island"], Config{NoElide: true})
	spirit = sc.hand(id["Benevolent River Spirit"], 0)
	for i := 0; i < bears; i++ {
		bs = append(bs, sc.perm(id["Grizzly Bears"], 0))
	}
	for i := 0; i < islands; i++ {
		is = append(is, sc.perm(id["Island"], 0))
	}
	return sc, id, spirit, bs, is
}

func rcwbOffered(sc *scen, o Obj) bool {
	d := sc.s.Step()
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkCast && a.Source() == TCode(uint8(o)) {
			return true
		}
	}
	return false
}

func TestRaisecostWaterbendCastPaysWithTaps(t *testing.T) {
	sc, _, spirit, bs, is := rcwbSetup(t, 6, 2)
	if !rcwbOffered(sc, spirit) {
		t.Fatal("the cast is not offered with two Islands and six creatures to tap")
	}
	sc.castCard(t, spirit)
	asks := 0
	picked := 0
	for i := 0; i < 40; i++ {
		d := sc.s.Step()
		if d.Kind == DkChoose && d.Sub == ChWaterbend {
			asks++
			if picked >= 5 {
				t.Fatalf("a sixth waterbend pick was offered: Waterbend<5> caps at five")
			}
			sc.do(t, AkChoose, TCode(bs[picked]), scAny)
			picked++
			continue
		}
		break
	}
	if asks != 5 {
		t.Fatalf("%d waterbend asks, want 5", asks)
	}
	for i := 0; i < 5; i++ {
		if !sc.tapped(bs[i]) {
			t.Fatalf("pick %d was not tapped", i)
		}
	}
	if sc.tapped(bs[5]) {
		t.Fatal("the unpicked creature was tapped")
	}
	for _, o := range is {
		if !sc.tapped(o) {
			t.Fatal("an Island was left untapped: {U}{U} is paid by mana")
		}
	}
	if sc.s.sp == 0 {
		t.Fatal("the spell is not on the stack")
	}
}

// With no creature to tap the {5} is unpayable by two Islands: no cast.
func TestRaisecostWaterbendNeedsTheCredit(t *testing.T) {
	sc, _, spirit, _, _ := rcwbSetup(t, 4, 2)
	if rcwbOffered(sc, spirit) {
		t.Fatal("offered with two Islands and four creatures: {U}{U}{5} is seven")
	}
}

// A mana source earns no credit (gorge withWaterbendOfferCredit).
func TestRaisecostWaterbendManaSourceEarnsNoCredit(t *testing.T) {
	tbl, id := wbTable(t, "Benevolent River Spirit", "Grizzly Bears", "Llanowar Elves", "Island")
	sc := newScenTableLib(t, tbl, id["Island"], Config{NoElide: true})
	spirit := sc.hand(id["Benevolent River Spirit"], 0)
	for i := 0; i < 3; i++ {
		sc.perm(id["Grizzly Bears"], 0)
	}
	sc.perm(id["Llanowar Elves"], 0)
	sc.perm(id["Island"], 0)
	sc.perm(id["Island"], 0)
	if rcwbOffered(sc, spirit) {
		t.Fatal("offered: three creatures credit three and the Elves makes mana (six); were it credited too the cast would be payable")
	}
}

// TestRaisecostWaterbendMutation: with the offer credit dropped a cast the six
// creatures pay for is no longer offered.
func TestRaisecostWaterbendMutation(t *testing.T) {
	old := DebugWaterbendNoCredit
	DebugWaterbendNoCredit = true
	defer func() { DebugWaterbendNoCredit = old }()
	sc, _, spirit, _, _ := rcwbSetup(t, 6, 2)
	if rcwbOffered(sc, spirit) {
		t.Fatal("the cast is offered without the waterbend credit")
	}
}
