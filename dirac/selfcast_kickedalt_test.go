package dirac

import (
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestSelfCastKickedAlt: a SpellCast self trigger filtered on PredKickedAlt
// ("when you cast this spell, if it was kicked with its {G} kicker",
// Wastescape Battlemage's Card.Self+kicked 1 / kicked 2) reads the spell's own
// kicker-alternative byte, written at beginCast before the cast triggers fire
// (setKickAlt). Bit N-1 fires trigger N alone; an unkicked cast and a
// single-cost kicked cast (no bit) fire neither; both bits fire both.
func TestSelfCastKickedAlt(t *testing.T) {
	tbl := v.NewTable()
	var fids [2]kop.FilterID
	for i := range fids {
		tbl.Filters = append(tbl.Filters, kop.Filter{Pred: kop.PredKickedAlt, PredN: int16(i + 1)})
		fids[i] = kop.FilterID(len(tbl.Filters) - 1)
	}
	gain := []kop.Instr{{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(1)}}
	ab := func(f kop.FilterID) kop.Ability {
		return kop.Ability{Kind: kop.AbTriggered, Prog: gain,
			Trig: kop.Trigger{Event: kop.TrigSpellCast, Flags: kop.TfSelf, Filter: f}}
	}
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Dual Kicker Cast", Types: kop.TCreature, Power: 1, Toughness: 1,
		Cost: kop.ManaCost{Generic: 1}, Abilities: []kop.Ability{ab(fids[0]), ab(fids[1])}})
	dual := CardID(len(tbl.Cards) - 1)
	for _, tc := range []struct {
		name   string
		kicked bool
		bits   uint8
		want   []uint8
	}{
		{"unkicked", false, 0, nil},
		{"single-cost kicked, no alternative bit", true, 0, nil},
		{"first alternative", true, 1, []uint8{0}},
		{"second alternative", true, 2, []uint8{1}},
		{"both alternatives", true, 3, []uint8{0, 1}},
	} {
		sc := newScenTable(t, tbl, Config{NoElide: true})
		if !sc.k.selfCast {
			t.Fatal("the table's self cast triggers did not set prep.selfCast")
		}
		o := sc.obj(dual, 0)
		sc.s.setKickAlt(o, tc.bits)
		e := stackEntry{kind: seSpell, obj: o, card: dual, ctrl: 0}
		if tc.kicked {
			e.flags |= sfKicked
		}
		sc.s.npend = 0
		sc.s.fireSelfCast(sc.k, &e)
		var got []uint8
		for i := 0; i < int(sc.s.npend); i++ {
			got = append(got, sc.s.pend[i].ab)
		}
		if !sameAbs(got, tc.want...) {
			t.Errorf("%s: queued abilities %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestWastescapeKickedCastTriggers: Wastescape Battlemage cast for each
// kicker alternative (the hostedcmdrstackcast pool, kgen's "Card.Self+kicked
// N" lowering) asks and resolves exactly that alternative's trigger -- {G}
// exiles an artifact or enchantment an opponent controls, {1}{U} returns an
// opposing creature -- and a plain cast asks neither. Lockstep never poses the
// per-alternative casts (twoaltkicker_lockstep_test.go), so the kernel read of
// the spell's byte is pinned here against gorge's FlagKicked1 / FlagKicked2.
func TestWastescapeKickedCastTriggers(t *testing.T) {
	deck := []cardset.Entry{
		{Name: "Wastescape Battlemage", Count: 1}, {Name: "Wastes", Count: 1}, {Name: "Forest", Count: 1},
		{Name: "Darksteel Relic", Count: 1}, {Name: "Hill Giant", Count: 1}, {Name: "Mountain", Count: 1},
	}
	set := cardsettest.Set(t, "hostedcmdrstackcast")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatalf("matchup: %v", err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	if cd := &tbl.Cards[id["Wastescape Battlemage"]]; !hasKicker2(cd) {
		t.Fatal("Wastescape Battlemage lowered without CardDef.Kicker2 (kgen regression)")
	}
	for _, tc := range []struct {
		name              string
		tgt               uint8
		asked             bool
		relic, giantStays bool // the relic and the giant still on the battlefield
	}{
		{"plain cast", 0, false, true, true},
		{"kicked with {G}", castKickAlt1Tgt, true, false, true},
		{"kicked with {1}{U}", castKickAlt2Tgt, true, true, false},
	} {
		sc := newScenTableLib(t, tbl, id["Forest"], Config{NoElide: true})
		relic, giant := sc.perm(id["Darksteel Relic"], 1), sc.perm(id["Hill Giant"], 1)
		o := sc.hand(id["Wastescape Battlemage"], 0)
		sc.s.pl[0].pool = [kop.NumManaKinds]uint8{kop.C: 2, kop.G: 2, kop.U: 1}
		d := sc.s.Step()
		a := twoAltCastOf(d, o, tc.tgt)
		if a == 0 {
			t.Fatalf("%s: the cast is not offered", tc.name)
		}
		if asked := twoAltResolve(t, sc, a, o); asked != tc.asked {
			t.Errorf("%s: target ask = %v, want %v", tc.name, asked, tc.asked)
		}
		if !sc.onBF(o) {
			t.Errorf("%s: Wastescape Battlemage did not resolve onto the battlefield", tc.name)
		}
		if got := sc.onBF(relic); got != tc.relic {
			t.Errorf("%s: Darksteel Relic on the battlefield = %v, want %v", tc.name, got, tc.relic)
		}
		if got := sc.onBF(giant); got != tc.giantStays {
			t.Errorf("%s: Hill Giant on the battlefield = %v, want %v", tc.name, got, tc.giantStays)
		}
	}
}
