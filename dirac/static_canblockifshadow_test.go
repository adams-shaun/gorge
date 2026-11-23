package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestCanBlockIfShadowLifts: a printed S:Mode$ CanBlockIfShadow static (CR
// 702.28b, gorge shadowBlockLifted) lifts the shadow rule for its pair: a
// blocker the static's Filter names can block a shadow attacker it could not
// otherwise block, and the attacker's whole damage goes to the blocker (no
// evasion). The precondition each case pins is the shadow branch of the
// block check itself: without the lift the pair is refused at AkBlock.
func TestCanBlockIfShadowLifts(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	self := kop.FilterID(len(tbl.Filters))
	tbl.Filters = append(tbl.Filters, kop.Filter{Flags: kop.FSelf})
	bear, wall, plainWall := CardID(len(tbl.Cards)), CardID(len(tbl.Cards)+1), CardID(len(tbl.Cards)+2)
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 2, Toughness: 2},
		kop.CardDef{Name: "Test Shadow Wall", Types: kop.TCreature, Power: 0, Toughness: 4,
			Abilities: []kop.Ability{{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatCanBlockIfShadow, Filter: self}}}},
		kop.CardDef{Name: "Test Plain Wall", Types: kop.TCreature, Power: 0, Toughness: 4})

	for _, c := range []struct {
		name   string
		wall   CardID
		blocks bool
	}{
		{"lifted: the pair blocks", wall, true},
		{"no lift: the pair is refused", plainWall, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			sc := newScenTable(t, tbl, Config{NoElide: true})
			a1 := sc.perm(bear, 0)
			sc.s.perms[sc.slot(a1)].eotKw |= kop.KwShadow // the attacker is a shadow creature
			w := sc.perm(c.wall, 1)
			sc.passUntil(t, kop.StepAttackers)
			sc.do(t, AkAttack, TCode(a1), scAny)
			sc.do(t, AkDone, scAny, scAny)
			sc.passUntil(t, kop.StepBlockers)
			d := sc.s.Step()
			offered := false
			for i := 0; i < int(d.N); i++ {
				if o := d.Options[i]; o.Kind() == AkBlock && o.Source() == TCode(w) && o.Target() == TCode(a1) {
					offered = true
					break
				}
			}
			if offered != c.blocks {
				t.Fatalf("block pair offered=%v, want %v (opts %v)", offered, c.blocks, d.Options[:d.N])
			}
			if !c.blocks {
				return
			}
			sc.do(t, AkBlock, TCode(w), TCode(a1))
			sc.do(t, AkDone, scAny, scAny)
			sc.passUntil(t, kop.StepMain2)
			if !sc.onBF(w) || sc.s.pl[1].life != 20 {
				t.Fatalf("wall alive=%v life=%d; want blocked with all damage on the wall", sc.onBF(w), sc.s.pl[1].life)
			}
		})
	}
}
