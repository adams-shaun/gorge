package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// graveManaBoard builds a gravemana scenario (the pool's lands and spells) with
// Test Jack appended: a graveyard mana source of any colour (kop.CardDef.
// GraveMana), which kgen does not lower yet. A cast's build is live (the
// window a cast's payment opens) and seat 0's pool holds pool.
func graveManaBoard(t *testing.T, pool [kop.NumManaKinds]uint8) (*scen, map[string]CardID, Obj) {
	t.Helper()
	set := cardsettest.Set(t, "gravemana")
	tbl0, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	tbl := *tbl0
	tbl.Cards = append([]kop.CardDef(nil), tbl0.Cards...)
	jackID := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Jack", Types: kop.TArtifact, GraveMana: uint8(kop.ColAny)})
	id := map[string]CardID{}
	for c := range tbl0.Cards {
		if _, ok := id[tbl0.Cards[c].Name]; !ok {
			id[tbl0.Cards[c].Name] = CardID(c)
		}
	}
	sc := newScenTableLib(t, &tbl, id["Plains"], Config{NoElide: true})
	jack := sc.grave(jackID, 0, 1)[0]
	sc.s.pl[0].pool = pool
	if sc.s.objs[jack].zone != kop.ZoneGraveyard {
		t.Fatal("precondition: Test Jack is not in the graveyard")
	}
	if !graveManaOn(sc.k) {
		t.Fatal("precondition: the table's gen prep does not see a graveyard mana source")
	}
	return sc, id, jack
}

// openCastWindow makes the build live: the window a spell's payment opens.
func (sc *scen) openCastWindow(card CardID) {
	*sc.s.bld() = build{e: stackEntry{kind: seSpell, card: card, ctrl: 0}}
	sc.s.ctl = ctlBuild
}

func gravePips(c int) kop.ManaCost {
	var m kop.ManaCost
	m.Pips[c] = 1
	return m
}

// TestGraveManaWindow: a graveyard mana source (Jack-o'-Lantern's shape) is the
// last pick of a cast's payment window and only while mana floats: its {1} fee
// takes the pool's first non-empty kind in W U B R G C order, the card goes to
// exile, and one mana of the colour the cost owes is added (gorge:
// TestGraveManaGorgeProbe Q3 and Q4).
func TestGraveManaWindow(t *testing.T) {
	const W, U, B, R, G = kop.W, kop.U, kop.B, kop.R, kop.G
	pool := func(c ...int) (p [kop.NumManaKinds]uint8) {
		for _, k := range c {
			p[k]++
		}
		return p
	}
	for _, c := range []struct {
		name string
		cost int // the one coloured pip owed
		pool [kop.NumManaKinds]uint8
		want [kop.NumManaKinds]uint8
	}{
		{"R floating pays the fee, the cost owes W", W, pool(R), pool(W)},
		{"RG: the fee is the R", W, pool(R, G), pool(G, W)},
		{"GGU: the fee is the U", W, pool(G, G, U), pool(G, G, W)},
		{"RR: one R stays", W, pool(R, R), pool(R, W)},
		{"B floating, the cost owes G: G is added", G, pool(B), pool(G)},
		{"B floating, the cost owes U: U is added", U, pool(B), pool(U)},
	} {
		sc, id, jack := graveManaBoard(t, c.pool)
		sc.openCastWindow(id["Plains"])
		cost := gravePips(c.cost)
		if sc.s.windowSpellSettled(sc.k, 0, &cost, false) {
			t.Fatalf("%s: precondition: the floating pool already pays the cost", c.name)
		}
		if ok, _ := sc.s.windowTap(sc.k, 0, &cost, true); !ok {
			t.Errorf("%s: the window did not pay", c.name)
		}
		if z := sc.s.objs[jack].zone; z != kop.ZoneExile {
			t.Errorf("%s: Jack is in zone %d, want exile", c.name, z)
		}
		if got := sc.s.pl[0].pool; got != c.want {
			t.Errorf("%s: pool %v, want %v", c.name, got, c.want)
		}
	}

	// the colour is the owed one after the fee: the cost owes {R}{G}, R floats; the R is the fee, so R is
	// owed again and the answer takes the first owed colour in W U B R G order, R, not the G the pool lacked before.
	sc, id, jack := graveManaBoard(t, pool(R))
	sc.openCastWindow(id["Plains"])
	both := kop.ManaCost{}
	both.Pips[R], both.Pips[G] = 1, 1
	sc.s.windowTap(sc.k, 0, &both, true)
	if z := sc.s.objs[jack].zone; z != kop.ZoneExile || sc.s.pl[0].pool[R] != 1 {
		t.Errorf("owed after the fee: Jack in zone %d, pool %v, want exile and an R", z, sc.s.pl[0].pool)
	}

	// an empty pool lists no Jack
	sc, id, jack = graveManaBoard(t, pool())
	sc.openCastWindow(id["Plains"])
	cost := gravePips(W)
	if ok, _ := sc.s.windowTap(sc.k, 0, &cost, true); ok {
		t.Error("an empty pool paid {W}")
	}
	if z := sc.s.objs[jack].zone; z != kop.ZoneGraveyard {
		t.Errorf("empty pool: Jack left the graveyard (zone %d)", z)
	}
	if sc.s.graveManaAny(sc.k, 0) {
		t.Error("empty pool: graveManaAny holds")
	}

	// a battlefield source is listed first: with a Plains the window taps it and leaves Jack
	sc, id, jack = graveManaBoard(t, pool(R))
	plains := sc.perm(id["Plains"], 0)
	sc.openCastWindow(id["Plains"])
	if ok, _ := sc.s.windowTap(sc.k, 0, &cost, true); !ok {
		t.Error("the Plains did not pay {W}")
	}
	if z := sc.s.objs[jack].zone; z != kop.ZoneGraveyard {
		t.Errorf("Plains on the battlefield: Jack left the graveyard (zone %d)", z)
	}
	if sc.s.perms[sc.slot(plains)].flags&pfTapped == 0 || sc.s.pl[0].pool != pool(R, W) {
		t.Errorf("Plains tapped %v, pool %v, want the Plains tapped and R+W", sc.s.perms[sc.slot(plains)].flags&pfTapped != 0, sc.s.pl[0].pool)
	}

	// only a cast's own window lists it: the unless / ward / trigger-cost windows list the battlefield alone
	sc, _, jack = graveManaBoard(t, pool(R))
	sc.s.ctl = ctlResolve
	if ok, _ := sc.s.windowTap(sc.k, 0, &cost, true); ok {
		t.Error("a resolution window paid with the graveyard source")
	}
	if z := sc.s.objs[jack].zone; z != kop.ZoneGraveyard {
		t.Errorf("resolution window: Jack left the graveyard (zone %d)", z)
	}
}

// TestGraveManaGate: the untapped-source gates count a graveyard source only
// while mana floats (graveManaAny).
func TestGraveManaGate(t *testing.T) {
	sc, _, _ := graveManaBoard(t, [kop.NumManaKinds]uint8{kop.R: 1})
	if !sc.s.hasUntappedMana(sc.k, 0) {
		t.Error("R floating and Jack in the graveyard: no untapped mana source")
	}
	sc.s.pl[0].pool = [kop.NumManaKinds]uint8{}
	if sc.s.hasUntappedMana(sc.k, 0) {
		t.Error("empty pool: Jack counts as an untapped mana source")
	}
}

// TestGraveManaPickMutation: DebugNoGraveMana leaves the graveyard source out.
func TestGraveManaPickMutation(t *testing.T) {
	defer func(old bool) { DebugNoGraveMana = old }(DebugNoGraveMana)
	sc, id, jack := graveManaBoard(t, [kop.NumManaKinds]uint8{kop.R: 1})
	sc.openCastWindow(id["Plains"])
	cost := gravePips(kop.W)
	DebugNoGraveMana = true
	if ok, _ := sc.s.windowTap(sc.k, 0, &cost, true); ok {
		t.Error("the window paid with Jack under DebugNoGraveMana")
	}
	if z := sc.s.objs[jack].zone; z != kop.ZoneGraveyard {
		t.Errorf("Jack left the graveyard (zone %d)", z)
	}
	if got := sc.s.pl[0].pool; got != [kop.NumManaKinds]uint8{kop.R: 1} {
		t.Errorf("pool %v, want the R untouched", got)
	}
	if sc.s.hasUntappedMana(sc.k, 0) {
		t.Error("hasUntappedMana counts Jack under DebugNoGraveMana")
	}
}
