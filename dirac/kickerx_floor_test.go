package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Thieving Skydiver (K:Kicker:XMin1 X): the kicker announces an {X} that
// cannot be 0, so the kicked cast is offered only when the pool pays the
// printed cost plus {X} = 1 and the X ask starts at 1 (gorge cast_subcounter.go
// xAsk, pay ComposeFeasibleP; engine/rules xmin_cost_test.go). The floor rides
// CardDef.KickerCost.XMin.

// kickerXScen: p0 holds the Skydiver in hand and lands Islands/Plains; p1
// controls artifacts of mana value 0, 1 and 2 the kicked ETB may take.
func kickerXScen(t *testing.T, islands, plains int) (sc *scen, id map[string]CardID, sky Obj, lands []Obj) {
	t.Helper()
	var deck []cardset.Entry
	for _, n := range []string{"Thieving Skydiver", "Ornithopter", "Sol Ring", "Arcane Signet", "Island", "Plains"} {
		deck = append(deck, cardset.Entry{Name: n, Count: 1})
	}
	tbl, da, _, err := cardsettest.Set(t, "kickerx").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id = map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc = newScenTable(t, tbl, Config{NoElide: true})
	cd := &sc.k.t.Cards[id["Thieving Skydiver"]]
	if cd.KickerCost.XMin != 1 || cd.KickerCost.Flags != 0 || cd.Kicker.X != 1 || cd.Cost.X != 0 {
		t.Fatalf("precondition: Skydiver's table entry Kicker %+v KickerCost %+v Cost.X %d, want a Flags-0 XMin-1 floor beside one kicker {X}", cd.Kicker, cd.KickerCost, cd.Cost.X)
	}
	for i := 0; i < islands; i++ {
		lands = append(lands, sc.perm(id["Island"], 0))
	}
	for i := 0; i < plains; i++ {
		lands = append(lands, sc.perm(id["Plains"], 0))
	}
	for _, n := range []string{"Ornithopter", "Sol Ring", "Arcane Signet"} {
		sc.perm(id[n], 1)
	}
	sky = sc.hand(id["Thieving Skydiver"], 0)
	return sc, id, sky, lands
}

// castOf finds o's cast option, kicked or plain.
func castOf(t *testing.T, d Decision, o Obj, kicked bool) (Action, bool) {
	t.Helper()
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		if a.Kind() == AkCast && Obj(a.Source()) == o && (a.Mode()&1 != 0) == kicked { // seat-ok: a cast-mode bit
			return a, true
		}
	}
	return 0, false
}

// TestKickerXMinOfferGate: the kicked cast is offered only when the pool pays
// the printed {1}{U} plus {X} at the floor 1: two lands do not, three do; the
// plain cast is offered either way.
func TestKickerXMinOfferGate(t *testing.T) {
	for _, tc := range []struct {
		lands int
		want  bool
	}{{2, false}, {3, true}} {
		sc, _, sky, _ := kickerXScen(t, 1, tc.lands-1)
		kicked, plain := offersKickedCast(sc.s, sky)
		if !plain {
			t.Fatalf("%d lands: the plain cast must be offered (precondition)", tc.lands)
		}
		if kicked != tc.want {
			t.Errorf("%d lands: kicked cast offered = %v, want %v", tc.lands, kicked, tc.want)
		}
	}
}

// TestKickerXMinAskStartsAtFloor: a kicked cast on exactly {2}{U} asks X with
// the one answer X = 1, pays the whole pool and the spell carries X = 1, which
// the ETB trigger's Count$xPaid reads: Sol Ring (mana value 1) is a legal
// target, Arcane Signet (2) is not.
func TestKickerXMinAskStartsAtFloor(t *testing.T) {
	sc, id, sky, lands := kickerXScen(t, 1, 2)
	a, ok := castOf(t, sc.s.Step(), sky, true)
	if !ok {
		t.Fatal("precondition: the kicked cast is not offered")
	}
	sc.s.Apply(a)
	d := sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChX {
		t.Fatalf("decision kind %d sub %d, want the X ask", d.Kind, d.Sub)
	}
	if d.N != 1 || d.Options[0].Mode() != 1 {
		t.Fatalf("X ask offers %d values (first %d), want exactly X = 1", d.N, d.Options[0].Mode())
	}
	sc.s.Apply(d.Options[0])
	// the payment spends the three lands; the spell reaches the stack with X = 1
	for i := 0; i < 8; i++ {
		d = sc.s.Step()
		if d.Kind == DkPriority {
			break
		}
		t.Fatalf("decision kind %d sub %d after the X answer, want the spell on the stack", d.Kind, d.Sub)
	}
	if got := sc.s.spellX(sky); got != 1 {
		t.Fatalf("the spell's announced X = %d, want 1", got)
	}
	if sc.s.pl[0].pool != [kop.NumManaKinds]uint8{} {
		t.Errorf("pool %v after the payment, want empty", sc.s.pl[0].pool)
	}
	for _, o := range lands {
		if !sc.tapped(o) {
			t.Errorf("land %d untapped: the payment must spend printed + X = 3 mana", o)
		}
	}
	// both pass: the Skydiver resolves and its kicked ETB asks its target
	sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
	sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
	d = sc.s.Step()
	var got []string
	for i := 0; i < int(d.N); i++ {
		got = append(got, sc.k.t.Cards[sc.s.cardAt(sc.s.objs[Obj(d.Options[i].Target())].slot)].Name)
	}
	if d.Kind != DkTarget || len(got) != 2 || got[0] != "Ornithopter" || got[1] != "Sol Ring" {
		t.Errorf("ETB decision kind %d targets %v, want the artifacts of mana value <= 1 (Ornithopter, Sol Ring) and not Arcane Signet", d.Kind, got)
	}
	_ = id
}

// TestKickerXMinUnkickedAsksNothing: the plain cast pays the printed {1}{U}
// and announces no X (the floor is the kicker's, not the card's).
func TestKickerXMinUnkickedAsksNothing(t *testing.T) {
	sc, _, sky, lands := kickerXScen(t, 1, 2)
	a, ok := castOf(t, sc.s.Step(), sky, false)
	if !ok {
		t.Fatal("precondition: the plain cast is not offered")
	}
	sc.s.Apply(a)
	for i := 0; i < 8; i++ {
		d := sc.s.Step()
		if d.Kind == DkPriority {
			break
		}
		if d.Kind == DkChoose && d.Sub == ChX {
			t.Fatalf("the plain cast asked for X (%d options)", d.N)
		}
		sc.s.Apply(d.Options[0])
	}
	if sc.s.objs[sky].zone != kop.ZoneStack {
		t.Fatalf("the Skydiver is in zone %d, want the stack", sc.s.objs[sky].zone)
	}
	n := 0
	for _, o := range lands {
		if sc.tapped(o) {
			n++
		}
	}
	if n != 2 {
		t.Errorf("%d lands tapped, want the printed cost's 2", n)
	}
}
