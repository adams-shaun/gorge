package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestMustBlockDutyPrinted: a printed S:Mode$ MustBlock static (CR 509.1a,
// gorge combat.MustBlockCandidates) is a block duty like the Effect-delivered
// mark: while its creature can block, the defender's Done is withheld until
// it blocks; a duty it cannot meet (a tapped duty creature) withholds
// nothing. The precondition each case pins is the block decision's own
// requirements machinery (blockRequiredCore), which the Effect-delivered
// arm already drives.
func TestMustBlockDutyPrinted(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	set := cardsettest.Set(t, "fdn")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	self := kop.FilterID(len(tbl.Filters))
	tbl.Filters = append(tbl.Filters, kop.Filter{Flags: kop.FSelf})
	bear, duty, wall := CardID(len(tbl.Cards)), CardID(len(tbl.Cards)+1), CardID(len(tbl.Cards)+2)
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 2, Toughness: 2},
		kop.CardDef{Name: "Test Duty", Types: kop.TCreature, Power: 1, Toughness: 3,
			Abilities: []kop.Ability{{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatMustBlock, Filter: self}}}},
		kop.CardDef{Name: "Test Wall", Types: kop.TCreature, Power: 0, Toughness: 4})

	for _, c := range []struct {
		name      string
		tapped    bool
		pairs     [][2]Obj
		doneLegal bool
	}{{"duty unmet: no Done", false, nil, false},
		{"duty met: Done", false, nil, true},
		{"duty unmeetable: Done", true, nil, true}} {
		t.Run(c.name, func(t *testing.T) {
			sc := newScenTable(t, tbl, Config{NoElide: true})
			a1 := sc.perm(bear, 0)
			d := sc.perm(duty, 1)
			w := sc.perm(wall, 1)
			if c.tapped {
				sc.s.perms[sc.slot(d)].flags |= pfTapped
			}
			sc.passUntil(t, kop.StepAttackers)
			sc.do(t, AkAttack, TCode(a1), scAny)
			sc.do(t, AkDone, scAny, scAny)
			sc.passUntil(t, kop.StepBlockers)
			if c.doneLegal && !c.tapped {
				// the duty creature is the declaration that meets the duty
				sc.do(t, AkBlock, TCode(d), TCode(a1))
			} else {
				sc.do(t, AkBlock, TCode(w), TCode(a1))
			}
			dd := sc.s.Step()
			offered := false
			for i := 0; i < int(dd.N); i++ {
				if dd.Options[i].Kind() == AkDone {
					offered = true
					break
				}
			}
			if offered != c.doneLegal {
				t.Fatalf("Done offered=%v, want %v (opts %v)", offered, c.doneLegal, dd.Options[:dd.N])
			}
			if c.doneLegal {
				sc.do(t, AkDone, scAny, scAny)
			}
		})
	}
}
