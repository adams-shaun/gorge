package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestRenewTargetWithProtectionNotOffered: an ability worked from the
// graveyard targets with the card there as its source, so a creature with
// protection from that card's colour (Ureni, the Song Unending against
// Alchemist's Assistant's Renew) is not a legal target (CR 115.2, 702.16b).
// With no other creature the ability is not offered; an unprotected creature
// restores the offer.
func TestRenewTargetWithProtectionNotOffered(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	plains := da[0]
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature})
	creature := kop.FilterID(len(tbl.Filters) - 1)
	renew, prot, bear := CardID(len(tbl.Cards)), CardID(len(tbl.Cards)+1), CardID(len(tbl.Cards)+2)
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Renewer", Types: kop.TCreature, Power: 2, Toughness: 1, Colors: kop.ColB,
			Abilities: []kop.Ability{{Kind: kop.AbActivated, ActZone: kop.ZoneGraveyard,
				Cost:    kop.Cost{Mana: kop.ManaCost{Generic: 1}, Flags: kop.CostSorcery},
				Targets: [kop.MaxTargets]kop.TargetSpec{{Kind: kop.TgtObject, Filter: creature}}, NTargets: 1,
				Prog: []kop.Instr{{Op: kop.OpDraw, Obj: kop.RefController, A: kop.Const(1)}}}}},
		kop.CardDef{Name: "Test Protected", Types: kop.TCreature, Power: 2, Toughness: 2, Colors: kop.ColW | kop.ColB, ProtColors: kop.ColW | kop.ColB},
		kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 2, Toughness: 2, Colors: kop.ColG})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	sc.lands(0, plains, 2)
	sc.perm(prot, 1)
	o := sc.obj(renew, 0)
	sc.s.place(o, kop.ZoneGraveyard, false)
	sc.s.ctl = ctlPriority
	if n := sc.graveActivations(o); n != 0 {
		t.Fatalf("offered %d times with only a protection-from-black creature to target, want 0", n)
	}
	sc.perm(bear, 1)
	sc.s.ctl = ctlPriority
	if n := sc.graveActivations(o); n != 1 {
		t.Fatalf("offered %d times with an unprotected creature to target, want 1", n)
	}
}
