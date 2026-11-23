package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestCantBeCast: a StatCantBeCast with StRaiseOpp and the your-turn
// condition (Dragonlord Dromoka: your opponents can't cast spells during
// your turn) forbids the opponent's casts during its controller's turn
// only, and never the controller's own.
func TestCantBeCast(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	tbl.Conds = append(tbl.Conds, kop.Cond{Kind: kop.CondYourTurn})
	yt := kop.CondID(len(tbl.Conds) - 1)
	lock, spell := CardID(len(tbl.Cards)), CardID(len(tbl.Cards)+1)
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Lock", Types: kop.TCreature, Power: 5, Toughness: 7, Abilities: []kop.Ability{
			{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatCantBeCast, StFlags: kop.StRaiseOpp, Cond2: yt}}}},
		kop.CardDef{Name: "Test Spell", Types: kop.TInstant})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	if !sc.k.gen.anyCantCast {
		t.Fatal("anyCantCast not set")
	}
	sc.perm(lock, 0)
	sc.s.active = 0
	if !sc.s.castForbidden(sc.k, 1, spell, NoObj) || sc.s.castForbidden(sc.k, 0, spell, NoObj) {
		t.Fatal("during its controller's turn: the opponent forbidden, the controller not")
	}
	sc.s.active = 1
	if sc.s.castForbidden(sc.k, 1, spell, NoObj) {
		t.Fatal("during the opponent's own turn the opponent is forbidden")
	}
}

// TestCantBeCastXLookahead is the kernel side of Brisela, Voice of Nightmares'
// "your opponents can't cast spells with mana value 3 or less": an X spell
// whose mana value is 1 at X = 0 (bound by the restriction) escapes at a
// payable X whose mana value is above the bound (CR 601.3a), and the CR
// 601.2e recheck binds it again at the announced X (CR 202.3e). The mutation
// switch removes the look-ahead and must withhold the cast the offer allows.
func TestCantBeCastXLookahead(t *testing.T) {
	set := cardsettest.Set(t, "fdn")
	deck := []cardset.Entry{{Name: "Forest", Count: 20}}
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	forest := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Forest", Types: kop.TLand | kop.TBasic, Timing: kop.TimingLand,
		Abilities: []kop.Ability{{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColG, ManaN: 1}}})
	tbl.Filters = append(tbl.Filters, kop.Filter{Zone: kop.ZoneStack, Pred: kop.PredCMCLE, PredN: 3})
	f := kop.FilterID(len(tbl.Filters) - 1)

	lock := CardID(len(tbl.Cards))
	spell := lock + 1
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Brisela", Types: kop.TCreature, Power: 9, Toughness: 10, Abilities: []kop.Ability{
			{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatCantBeCast, StFlags: kop.StRaiseOpp, Filter: f}}}},
		kop.CardDef{Name: "Test X Spell", Cost: kop.ManaCost{Pips: [kop.NumManaKinds]uint8{0, 0, 0, 0, 1, 0}, X: 1},
			Types: kop.TInstant, Timing: kop.TimingInstant, Colors: kop.ColG,
			Abilities: []kop.Ability{{Kind: kop.AbSpell}}})

	sc := newScenTableLib(t, tbl, forest, Config{NoElide: true})
	if !sc.k.gen.anyCantCast {
		t.Fatal("anyCantCast not set")
	}
	sc.perm(lock, 0) // the restriction source, controlled by seat 0
	o := sc.hand(spell, 1)
	cost := sc.k.cards[spell].cost
	if cost.X != 1 || sc.k.cards[spell].total != 1 {
		t.Fatalf("test spell cost = %+v total %d, want one {X} and one {G}", cost, sc.k.cards[spell].total)
	}

	// X = 0: the printed mana value 1 is under the bound, so the plain
	// offer binds (the look-ahead has no cost to price).
	if !sc.s.castForbidden(sc.k, 1, spell, o) {
		t.Fatal("X = 0: the offer does not bind at the printed mana value")
	}
	// Two Forests pay only X <= 0 (the {G} pip leaves one generic), so every
	// payable announcement is still under the bound.
	sc.perm(forest, 1)
	sc.perm(forest, 1)
	if !sc.s.castForbiddenCost(sc.k, 1, spell, o, &cost, nil) {
		t.Fatal("X <= 1: the offer escapes though no payable X lifts the mana value above 3")
	}
	// Four Forests pay X = 3, mana value 4: the offer escapes (CR 601.3a).
	sc.perm(forest, 1)
	sc.perm(forest, 1)
	if sc.s.castForbiddenCost(sc.k, 1, spell, o, &cost, nil) {
		t.Fatal("X = 3 payable: the offer binds though the announced X lifts the mana value above 3")
	}
	// The CR 601.2e recheck reads the announced X on the stack: X = 0 is
	// illegal, X = 3 is not.
	push := func(x objWord) *stackEntry {
		sc.s.stack[sc.s.sp] = stackEntry{kind: seSpell, obj: o, card: spell, ctrl: 1, x: x}
		sc.s.sp++
		sc.s.objs[o].zone = kop.ZoneStack
		return &sc.s.stack[sc.s.sp-1]
	}
	if e := push(0); !sc.s.castXRestrictionRecheck(sc.k, e) {
		t.Fatal("recheck at the announced X = 0: the cast is legal")
	}
	sc.s.sp--
	if e := push(3); sc.s.castXRestrictionRecheck(sc.k, e) {
		t.Fatal("recheck at the announced X = 3: the cast is illegal")
	}
	sc.s.sp--
	sc.s.objs[o].zone = kop.ZoneHand

	// Mutation: without the look-ahead the four-Forest offer is withheld.
	DebugCantCastNoXLookahead = true
	defer func() { DebugCantCastNoXLookahead = false }()
	if !sc.s.castForbiddenCost(sc.k, 1, spell, o, &cost, nil) {
		t.Fatal("DebugCantCastNoXLookahead: the offer still escapes")
	}
}
