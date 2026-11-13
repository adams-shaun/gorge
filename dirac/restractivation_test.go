package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// synthStdMshScen is an empty scenario over the std-msh synthetic pool with
// the named cards (and Islands) in both decks, and the card ids by name.
func synthStdMshScen(t *testing.T, names ...string) (*scen, map[string]CardID) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Island", Count: 16}}
	for _, n := range names {
		deck = append(deck, cardset.Entry{Name: n, Count: 1})
	}
	tbl, da, _, err := cardsettest.Synth(t, "std-msh").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	return newScenTable(t, tbl, Config{NoElide: true}), id
}

// helperFloatScen is a board with Hydraulic Helper tapped for its {U}, which
// can't be spent to cast a nonartifact spell and so floats for an ability.
func helperFloatScen(t *testing.T, names ...string) (*scen, map[string]CardID) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Hydraulic Helper", Count: 1}, {Name: "Island", Count: 16}}
	for _, n := range names {
		deck = append(deck, cardset.Entry{Name: n, Count: 1})
	}
	set := cardsettest.Synth(t, "std-msh")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	sl := sc.slot(sc.perm(id["Hydraulic Helper"], 0))
	var held [kop.NumManaKinds]uint8
	sc.s.tapGap(sc.k, 0, sl, slotBit(sl), &held)
	if sc.s.restrN(0) != 1 || sc.s.pl[0].pool != ([kop.NumManaKinds]uint8{}) {
		t.Fatalf("after the Helper: restricted %v pool %v, want one restricted {U} and an empty pool", sc.s.rb[0], sc.s.pl[0].pool)
	}
	return sc, id
}

// Floating restricted mana counts toward an activated ability's {X} when its
// restriction admits the ability (CR 106.6, CR 107.3). Stature entered this turn,
// so its power-up costs {U} less, {X}{U}: two Islands and the Helper's {U} reach
// X=2, where the Islands alone reach X=1.
func TestFloatingRestrictedManaBoundsAnActivationsX(t *testing.T) {
	sc, id := helperFloatScen(t, "Stature, Size Shifter")
	stature := sc.perm(id["Stature, Size Shifter"], 0)
	sc.perm(id["Island"], 0)
	sc.perm(id["Island"], 0)
	xs := sc.xChoices(t, stature, 0)
	if len(xs) == 0 || xs[len(xs)-1] != 2 {
		t.Fatalf("X choices %v, want a bound of 2", xs)
	}
}

// A filter's fee spends the floating restricted mana that may pay it before any
// ordinary unit, as the payment carves the admitted batches first: Surveillance
// Room's {1} takes the Helper's {U}, and the ordinary {R} stays in the pool.
func TestFilterFeeSpendsRestrictedManaFirst(t *testing.T) {
	sc, id := helperFloatScen(t, "Surveillance Room")
	s := sc.s
	room := sc.slot(sc.perm(id["Surveillance Room"], 0))
	if fee := s.filtFee(sc.k, room); fee != 1 {
		t.Fatalf("the Room's fee is %d, want 1", fee)
	}
	s.pl[0].pool[kop.R] = 1
	s.filtDo(sc.k, 0, filtStep{sl: room, gap: true, ans: 1 + uint8(kop.G)})
	if s.rbAny(0) {
		t.Fatalf("restricted %v, want the restricted {U} spent on the fee", s.rb[0])
	}
	if s.pl[0].pool[kop.R] != 1 || s.pl[0].pool[kop.G] != 1 {
		t.Fatalf("pool %v, want the {R} kept and the filter's {G} added", s.pl[0].pool)
	}
}

// CR 305.6, 605.1a: a land with a granted basic land type has the intrinsic
// ability beside its own. Rumble Arena ({T}: Add {C}; {1},{T}: any colour) and
// Jasmine Dragon Tea Shop ({T}: Add {C}; a restricted any-colour {T} ability)
// each have a plain first ability and a later one the planner census misses or
// counts per payment, so a granted Island is the union view C|U of the first
// ability with the intrinsic {U} (one tap), not a landgrant stall.
func TestLandGrantViewIgnoresLaterGapAbility(t *testing.T) {
	deck := []cardset.Entry{{Name: "The Legend of Kyoshi", Count: 4}, {Name: "Rumble Arena", Count: 8},
		{Name: "Jasmine Dragon Tea Shop", Count: 8}, {Name: "Mountain", Count: 20}}
	set := cardsettest.Synth(t, "std-tla")
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the std-tla table", e.Name)
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
	sc := newScenTableLib(t, tbl, id["Mountain"], Config{})
	g := sc.k.gen
	if !g.blgOn {
		t.Fatal("blgOn unset: nothing in the matchup grants a basic land type")
	}
	island := kop.ColorMask(1) << kop.U
	for _, name := range []string{"Rumble Arena", "Jasmine Dragon Tea Shop"} {
		c := id[name]
		if g.blgIdx[c] < 0 {
			t.Errorf("%s has no granted-type views (it would stall landgrant)", name)
			continue
		}
		v := &g.blgPC[int(g.blgIdx[c])*32+int(island)]
		want := kop.ColorMask(1)<<kop.C | island
		if v.manaCol != want || !v.manaMulti || v.blgFirst != uint8(kop.C)+1 {
			t.Errorf("%s under a granted Island: manaCol %b multi %v first %d; want %b, true, %d", name, v.manaCol, v.manaMulti, v.blgFirst, want, kop.C+1)
		}
	}
}

// CR 305.6: Kyoshi Village ({T}: Add {G} or {W}, Combo G W) under a granted
// Island answers its printed ability first, {G}, not the lowest colour {W};
// an owed {W} still wins the window.
func TestLandGrantComboKeepsTokenOrder(t *testing.T) {
	deck := []cardset.Entry{{Name: "The Legend of Kyoshi", Count: 4}, {Name: "Kyoshi Village", Count: 8}, {Name: "Mountain", Count: 20}}
	set := cardsettest.Synth(t, "std-tla")
	for _, e := range deck {
		if i, ok := set.Lookup(e.Name); !ok || !set.Supported()[i] {
			t.Skipf("%s not in the std-tla table", e.Name)
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
	sc := newScenTableLib(t, tbl, id["Mountain"], Config{})
	g := sc.k.gen
	c := id["Kyoshi Village"]
	if !g.blgOn || g.blgIdx[c] < 0 {
		t.Fatal("no granted-type views for Kyoshi Village")
	}
	island := kop.ColorMask(1) << kop.U
	v := &g.blgPC[int(g.blgIdx[c])*32+int(island)]
	if v.blgFirst != uint8(kop.G)+1 {
		t.Fatalf("blgFirst %d, want %d (G first by the Combo's token order)", v.blgFirst, kop.G+1)
	}
	if v.manaCol&(1<<kop.W) == 0 || v.manaCol&(1<<kop.G) == 0 || v.manaCol&island == 0 {
		t.Fatalf("view manaCol %b lacks G, W or U", v.manaCol)
	}
	sl := Slot(sc.perm(c, 0))
	// give the live state the Island subtype the view models (The Legend of
	// Kyoshi's Earthbend Animate is the table's AddSub override), so
	// manaPCGen hands stepColours the same granted-type view
	islandIdx := g.blgSub[kop.U]
	found := false
	for i := range tbl.Overrides {
		if tbl.Overrides[i].AddSub.Has(int(islandIdx)) {
			sc.s.applyOverride(sc.k, sl, i, c, false)
			found = true
			break
		}
	}
	if !found {
		t.Fatal("no Animate override adds Island: cannot exercise the live granted-type view")
	}
	if got := sc.s.manaPCGen(sc.k, sl); got != v {
		t.Fatalf("live state holds view blgFirst %d, want the granted-type view's %d", got.blgFirst, v.blgFirst)
	}
	var order [kop.NumManaKinds]uint8
	no := sc.s.stepColours(sc.k, sl, -1, uint8(v.manaCol), &order)
	if no < 3 || order[0] != kop.G || order[1] != kop.W || order[2] != kop.U {
		t.Errorf("stepColours %v, want G W U: the Combo's printed order, then the granted intrinsic", order[:no])
	}
	both := cost(0, kop.W, kop.G)
	if col := sc.s.windowCol(sc.k, 0, &both, sl, v, false, false); col != kop.G {
		t.Errorf("{W}{G} owed: window answers %d, want G (token order)", col)
	}
	onlyW := cost(0, kop.W)
	if col := sc.s.windowCol(sc.k, 0, &onlyW, sl, v, false, false); col != kop.W {
		t.Errorf("{W} owed: window answers %d, want W", col)
	}
	if col := sc.s.windowCol(sc.k, 0, &onlyW, sl, v, false, true); col != kop.G {
		t.Errorf("nothing owed: window answers %d, want G", col)
	}
}
