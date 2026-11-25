package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestMostProminentType: ExMostProminentCreatureType is the size of the largest group
// of matches that share a creature type (gorge mostProminentCreatureType): a
// lone creature reads 1, an Equipment subtype is no creature type, another
// player's creatures do not match Creature.YouCtrl, and a Changeling joins
// every group on top of the largest one.
func TestMostProminentType(t *testing.T) {
	tbl := *vanilla.Table
	tbl.Cards = append([]kop.CardDef(nil), vanilla.Table.Cards...)
	tbl.Exprs = make([]kop.Expr, 1) // a table with Exprs prepares the gen tables
	tbl.Subtypes = append(append([]string(nil), vanilla.Table.Subtypes...), "Bear", "Elf", "Equipment")
	n := len(vanilla.Table.Subtypes)
	tbl.TypeVocab = make([]uint8, 50)
	subs := func(is ...int) (m kop.SubMask) {
		for _, i := range is {
			m = m.With(n + i)
		}
		return m
	}
	bear, elfBear, gear, shifter := CardID(len(tbl.Cards)), CardID(len(tbl.Cards)+1), CardID(len(tbl.Cards)+2), CardID(len(tbl.Cards)+3)
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 1, Toughness: 1, Subtypes: subs(0)},
		kop.CardDef{Name: "Test Elf Bear", Types: kop.TCreature, Power: 1, Toughness: 1, Subtypes: subs(0, 1)},
		kop.CardDef{Name: "Test Gear", Types: kop.TCreature | kop.TArtifact, Power: 1, Toughness: 1, Subtypes: subs(2)},
		kop.CardDef{Name: "Test Shifter", Types: kop.TCreature, Power: 1, Toughness: 1, AllCreatureTypes: true})
	tbl.Filters = append(append([]kop.Filter(nil), vanilla.Table.Filters...), kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlYou})
	f := kop.FilterID(len(tbl.Filters) - 1)
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	count := func() int {
		x := kop.Expr{Kind: kop.ExMostProminentCreatureType, F: f}
		return sc.s.exprCold(sc.k, &x, evalCtx{ctrl: 0, src: NoObj, srcSl: NoSlot})
	}
	if got := count(); got != 0 {
		t.Fatalf("no match reads %d", got)
	}
	sc.perm(gear, 0)
	if got := count(); got != 0 {
		t.Fatalf("a creature with only an Equipment subtype reads %d, want 0", got)
	}
	sc.perm(bear, 0)
	if got := count(); got != 1 {
		t.Fatalf("a lone Bear reads %d, want 1", got)
	}
	sc.perm(elfBear, 0)
	if got := count(); got != 2 {
		t.Fatalf("Bear and Elf Bear read %d, want 2 (the Bears)", got)
	}
	sc.perm(bear, 1)
	if got := count(); got != 2 {
		t.Fatalf("the opponent's Bear moved the count to %d", got)
	}
	sc.perm(shifter, 0)
	if got := count(); got != 3 {
		t.Fatalf("a Changeling reads %d, want the largest group 2 plus 1", got)
	}
}
