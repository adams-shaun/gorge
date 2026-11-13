package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Forage and ExileFromGrave cost objects (CR 701.61a, 601.2h;
// act_forage.go) on BLB's table: Feed the Cycle's "forage or pay {B}",
// Bonebind Orator's graveyard ability exiling itself.

func forageScen(t *testing.T) (*scen, map[string]CardID) {
	t.Helper()
	return giftScen(t, "Feed the Cycle", "Bonebind Orator", "Heaped Harvest", "Jolly Gerbils", "Swamp", "Forest")
}

// grave puts n cards c into p's graveyard.
func (sc *scen) grave(c CardID, p uint8, n int) []Obj {
	var out []Obj
	for i := 0; i < n; i++ {
		o := sc.obj(c, p)
		sc.s.place(o, kop.ZoneGraveyard, false)
		out = append(out, o)
	}
	return out
}

// expect steps to the next decision and fails unless it is kind/sub.
func (sc *scen) expect(t *testing.T, kind DecisionKind, sub ChoiceKind) Decision {
	t.Helper()
	d := sc.s.Step()
	if d.Kind != kind || kind == DkChoose && d.Sub != sub {
		t.Fatalf("decision kind %d sub %d, want kind %d sub %d (opts %v)", d.Kind, d.Sub, kind, sub, d.Options[:d.N])
	}
	return d
}

func (sc *scen) altAddParts(t *testing.T) []uint8 {
	t.Helper()
	d := sc.expect(t, DkChoose, ChAltAdd)
	var parts []uint8
	for i := 0; i < int(d.N); i++ {
		parts = append(parts, uint8(d.Options[i].Target()))
	}
	return parts
}

func (sc *scen) chooseAltAdd(t *testing.T, part uint8) {
	t.Helper()
	d := sc.expect(t, DkChoose, ChAltAdd)
	for i := 0; i < int(d.N); i++ {
		if d.Options[i].Target() == TCode(part) {
			sc.s.Apply(d.Options[i])
			return
		}
	}
	t.Fatalf("either-or part %d not offered", part)
}

// feedSetup: Feed the Cycle in hand, the lands, an opposing Jolly Gerbils.
func feedSetup(t *testing.T, swamps int) (*scen, map[string]CardID, Obj, Obj) {
	sc, id := forageScen(t)
	for i := 0; i < swamps; i++ {
		sc.perm(id["Swamp"], 0)
	}
	gerbils := sc.perm(id["Jolly Gerbils"], 1)
	ftc := sc.hand(id["Feed the Cycle"], 0)
	return sc, id, ftc, gerbils
}

func TestForageByExile(t *testing.T) {
	sc, id, ftc, gerbils := feedSetup(t, 2)
	gy := sc.grave(id["Forest"], 0, 4)
	sc.castCard(t, ftc)
	sc.chooseAltAdd(t, 0)
	sc.do(t, AkTarget, scAny, TCode(gerbils))
	d := sc.expect(t, DkChoose, ChCostForage)
	if d.N != 1 || d.Options[0].Mode() != 0 {
		t.Fatalf("forage options %v, want the exile arm only (no Food)", d.Options[:d.N])
	}
	sc.s.Apply(d.Options[0])
	for i := 0; i < 3; i++ {
		d := sc.expect(t, DkChoose, ChCostExile)
		if int(d.N) != 4-i {
			t.Fatalf("exile pick %d offers %d cards, want %d", i, d.N, 4-i)
		}
		sc.s.Apply(d.Options[0])
	}
	sc.resolveAll(t)
	if sc.onBF(gerbils) {
		t.Fatal("Feed the Cycle did not destroy its target")
	}
	ex := 0
	for _, o := range gy {
		if sc.s.objs[o].zone == kop.ZoneExile {
			ex++
		}
	}
	if ex != 3 || sc.s.pl[0].graveN != 2 { // the fourth Forest and Feed the Cycle
		t.Fatalf("%d graveyard cards exiled, graveyard %d; want 3 exiled, 2 left", ex, sc.s.pl[0].graveN)
	}
}

func TestForageBySacrificingFood(t *testing.T) {
	sc, id, ftc, gerbils := feedSetup(t, 2)
	food := sc.perm(id["Heaped Harvest"], 0)
	sc.grave(id["Forest"], 0, 2) // too few to exile
	sc.castCard(t, ftc)
	sc.chooseAltAdd(t, 0)
	sc.do(t, AkTarget, scAny, TCode(gerbils))
	d := sc.expect(t, DkChoose, ChCostForage)
	if d.N != 1 || d.Options[0].Mode() != 1 || Obj(d.Options[0].Source()) != food {
		t.Fatalf("forage options %v, want the Food only", d.Options[:d.N])
	}
	sc.s.Apply(d.Options[0])
	if sc.s.objs[food].zone != kop.ZoneBattlefield {
		t.Fatal("the Food left before the cost was paid")
	}
	sc.resolveAll(t)
	if sc.onBF(food) || sc.onBF(gerbils) {
		t.Fatalf("food on battlefield %v, target %v; want both gone", sc.onBF(food), sc.onBF(gerbils))
	}
	if sc.s.pl[0].graveN != 2+1+1 { // the Forests, the Food, Feed the Cycle
		t.Fatalf("graveyard %d, want 4", sc.s.pl[0].graveN)
	}
}

func TestForageNotOffered(t *testing.T) {
	// two Swamps: Feed the Cycle's {1}{B} with no forage, and no {B} more
	sc, id, ftc, _ := feedSetup(t, 2)
	sc.grave(id["Forest"], 0, 2)
	d := sc.s.Step()
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkCast && Obj(a.Source()) == ftc {
			t.Fatal("Feed the Cycle offered with neither forage nor {B} payable")
		}
	}
	// a third Swamp: castable, only by paying {B}
	sc.perm(id["Swamp"], 0)
	sc.castCard(t, ftc)
	if parts := sc.altAddParts(t); len(parts) != 1 || parts[0] != 1 {
		t.Fatalf("either-or parts %v, want only {B} (part 1)", parts)
	}
}

func TestFeedTheCyclePaysB(t *testing.T) {
	sc, id, ftc, gerbils := feedSetup(t, 3)
	sc.grave(id["Forest"], 0, 3)
	sc.castCard(t, ftc)
	if parts := sc.altAddParts(t); len(parts) != 2 {
		t.Fatalf("either-or parts %v, want forage and {B}", parts)
	}
	sc.chooseAltAdd(t, 1)
	sc.do(t, AkTarget, scAny, TCode(gerbils))
	sc.resolveAll(t)
	if sc.onBF(gerbils) {
		t.Fatal("Feed the Cycle did not destroy its target")
	}
	if sc.s.pl[0].graveN != 4 {
		t.Fatalf("graveyard %d, want 4 (no forage: nothing exiled)", sc.s.pl[0].graveN)
	}
	for m := sc.s.ctrlM[0]; m.any(); m = m.rest() {
		if sc.s.perms[m.first()].flags&pfTapped == 0 {
			t.Fatal("an untapped Swamp: {1}{B}{B} is three")
		}
	}
}

func TestExileFromGraveSelf(t *testing.T) {
	sc, id := forageScen(t)
	for i := 0; i < 4; i++ {
		sc.perm(id["Swamp"], 0)
	}
	bo := sc.grave(id["Bonebind Orator"], 0, 1)[0]
	activatable := func() bool {
		d := sc.s.Step()
		for i := 0; i < int(d.N); i++ {
			if a := d.Options[i]; a.Kind() == AkActivate && Obj(a.Source()) == bo {
				return true
			}
		}
		return false
	}
	if activatable() {
		t.Fatal("Bonebind Orator offered with no other creature card to return")
	}
	g := sc.grave(id["Jolly Gerbils"], 0, 1)[0]
	if !activatable() {
		t.Fatal("Bonebind Orator not offered")
	}
	sc.do(t, AkActivate, TCode(bo), scAny)
	d := sc.expect(t, DkTarget, 0)
	if d.N != 1 || Obj(d.Options[0].Target()) != g {
		t.Fatalf("targets %v, want the Gerbils only (another creature card)", d.Options[:d.N])
	}
	sc.s.Apply(d.Options[0])
	if d := sc.s.Step(); d.Kind == DkChoose && d.Sub == ChCostExile {
		t.Fatal("the CARDNAME exile was asked; gorge exAsk pays its singleton unasked")
	}
	sc.resolveAll(t)
	if sc.s.objs[bo].zone != kop.ZoneExile || sc.s.objs[g].zone != kop.ZoneHand {
		t.Fatalf("orator zone %d, gerbils zone %d; want exile and hand", sc.s.objs[bo].zone, sc.s.objs[g].zone)
	}
}

// forageResScen: Treetop Sentries ("When this enters, you may forage. If
// you do, draw a card.") entering under player 0 with g graveyard cards and
// f Foods, its trigger on the stack and both players passed to resolve it;
// returns the first decision of the resolution.
func forageResScen(t *testing.T, g, f int) (*scen, Obj, []Obj, Decision) {
	sc, id := newBLBScen(t, []cardset.Entry{{Name: "Treetop Sentries", Count: 1}, {Name: "Bakersbane Duo", Count: 1}, {Name: "Plains", Count: 8}})
	food := CardID(0xFFFF)
	for i := range sc.k.t.Cards {
		if n := sc.k.t.Cards[i].Name; n == "c_a_food_sac" {
			food = CardID(i)
		}
	}
	if food == 0xFFFF {
		t.Fatal("no Food token in the table")
	}
	pl := id["Plains"]
	for i := 0; i < g; i++ {
		o := sc.obj(pl, 0)
		sc.s.place(o, kop.ZoneGraveyard, false)
	}
	var foods []Obj
	for i := 0; i < f; i++ {
		foods = append(foods, sc.perm(food, 0))
	}
	o := sc.obj(id["Treetop Sentries"], 0)
	sc.s.enter(sc.k, o, 0, false, kop.ZoneNone)
	for i := 0; i < 6; i++ {
		d := sc.s.Step()
		if d.Kind != DkPriority {
			return sc, o, foods, d
		}
		sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
	}
	t.Fatal("the trigger never asked")
	return nil, 0, nil, Decision{}
}

func payOffered(d Decision) bool {
	for i := 0; i < int(d.N); i++ {
		if d.Options[i].Mode() == 1 {
			return true
		}
	}
	return false
}

// TestForageResolutionCost: gorge trigger_cost_forage.go. The
// trigger_cost ask offers pay only when the forage is payable; with both
// arms live the either-or (forage_exile, then each Food) is posed; a lone
// arm settles unasked; a decline skips the "if you do" body.
func TestForageResolutionCost(t *testing.T) {
	// nothing to forage with: decline only, the draw skipped
	sc, _, _, d := forageResScen(t, 2, 0)
	if d.Kind != DkChoose || d.Sub != ChGenPay || payOffered(d) {
		t.Fatalf("unpayable forage: kind %d sub %d n %d, want a decline-only ChGenPay", d.Kind, d.Sub, d.N)
	}
	hand := sc.s.pl[0].handN
	sc.s.Apply(d.Options[0])
	if sc.s.pl[0].handN != hand {
		t.Fatal("declined forage still drew")
	}

	// decline with a payable forage: nothing moves, no draw
	sc, _, foods, d := forageResScen(t, 3, 1)
	if !payOffered(d) {
		t.Fatal("payable forage: pay not offered")
	}
	hand = sc.s.pl[0].handN
	sc.s.Apply(MakeAction(AkChoose, 0, 0, 0))
	if sc.s.pl[0].handN != hand || sc.s.pl[0].graveN != 3 || !sc.onBF(foods[0]) {
		t.Fatal("decline moved something or drew")
	}

	// both arms: the election offers exile and every Food
	sc, _, foods, d = forageResScen(t, 3, 2)
	sc.s.Apply(MakeAction(AkChoose, 0, 0, 1))
	d = sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChCostForage || d.N != 3 || d.Options[0].Mode() != 0 {
		t.Fatalf("election: kind %d sub %d n %d, want ChCostForage exile + 2 Foods", d.Kind, d.Sub, d.N)
	}
	hand = sc.s.pl[0].handN
	sc.s.Apply(MakeAction(AkChoose, TCode(foods[1]), 0, 1))
	if sc.onBF(foods[1]) || !sc.onBF(foods[0]) || sc.s.pl[0].graveN < 3 || sc.s.pl[0].handN != hand+1 {
		t.Fatalf("the chosen Food was not sacrificed alone, or no draw: %v %v grave %d hand %d/%d", sc.onBF(foods[1]), sc.onBF(foods[0]), sc.s.pl[0].graveN, sc.s.pl[0].handN, hand)
	}

	// lone exile arm: no election, the first three graveyard cards exiled
	sc, _, _, d = forageResScen(t, 4, 0)
	first := sc.s.pl[0].grave[0]
	hand = sc.s.pl[0].handN
	sc.s.Apply(MakeAction(AkChoose, 0, 0, 1))
	if sc.s.pl[0].graveN != 1 || sc.s.objs[first].zone != kop.ZoneExile || sc.s.pl[0].handN != hand+1 {
		t.Fatalf("lone exile arm: grave %d, first in zone %d", sc.s.pl[0].graveN, sc.s.objs[first].zone)
	}

	// lone Food: sacrificed unasked
	sc, _, foods, d = forageResScen(t, 1, 1)
	hand = sc.s.pl[0].handN
	sc.s.Apply(MakeAction(AkChoose, 0, 0, 1))
	if sc.onBF(foods[0]) || sc.s.pl[0].handN != hand+1 {
		t.Fatal("lone Food not sacrificed, or no draw")
	}
	_ = d
}
