package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestSelfExilePauseProjection: a sorcery whose resolution exiles itself
// from the stack (gorge's DB$ ChangeZone | Origin$ Stack | Destination$
// Exile) and then asks a pick (Time Spiral's "untap up to six lands") is in
// exile and off the stack while the pick is pending, as gorge has it: the
// spell leaves the stack at that op (CR 608.2c), so the projection's stack
// excludes the resolving entry. After the answer the stack is empty and the
// card rests in exile, not the graveyard (CR 608.2n is the default the
// explicit exile has already overridden).
func TestSelfExilePauseProjection(t *testing.T) {
	cases := []struct {
		name string
		exit kop.Instr
	}{
		{"plain exile", kop.Instr{Op: kop.OpExile, Obj: kop.RefSelf}},                                  // Time Spiral's lowering
		{"remembered exile", kop.Instr{Op: kop.OpExileEx, Obj: kop.RefSelf, C: int16(kop.MvRemember)}}, // RememberChanged$ (already immediate)
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var spiral CardID
			sc, id := newBLBScenTweak(t, []cardset.Entry{{Name: "Plains", Count: 20}}, func(tbl *kop.Table, _ map[string]CardID) {
				spiral = CardID(len(tbl.Cards))
				lands := kop.FilterID(len(tbl.Filters))
				tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TLand})
				tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Spiral", Types: kop.TSorcery, Abilities: []kop.Ability{{
					Kind: kop.AbSpell,
					Prog: []kop.Instr{c.exit, {Op: kop.OpUntapChoose, A: kop.Const(6), C: 1, F: lands}},
				}}})
			})
			if sc.k.gen == nil {
				t.Fatal("setup: the table has no generated prep, so the exile would not take execGen's moveObj path")
			}
			land := sc.perm(id["Plains"], 0)
			sc.s.perms[sc.slot(land)].flags |= pfTapped
			o := sc.hand(spiral, 0)
			sc.castCard(t, o)
			for i := 0; i < 4 && sc.s.objs[o].zone == kop.ZoneHand; i++ {
				sc.s.Apply(sc.s.Step().Options[0]) // the cast window's payment of {0}
			}
			if sc.s.objs[o].zone != kop.ZoneStack || sc.s.sp != 1 {
				t.Fatalf("setup: after the cast zone %d sp %d, want the spell alone on the stack", sc.s.objs[o].zone, sc.s.sp)
			}
			var d Decision
			for i := 0; i < 4; i++ {
				d = sc.s.Step()
				if d.Kind != DkPriority {
					break
				}
				sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
			}
			if d.Kind != DkChoose || d.Sub != ChGenUntapPick {
				t.Fatalf("decision kind %d sub %d, want the untap pick pending", d.Kind, d.Sub)
			}
			if !sc.s.res.active {
				t.Fatal("setup: the pick is not inside the resolution")
			}
			if z := sc.s.objs[o].zone; z != kop.ZoneExile {
				t.Fatalf("at the pick the spell is in zone %d, want exile %d (gorge exiled it at its ChangeZone)", z, kop.ZoneExile)
			}
			p := new(Projection)
			sc.s.Project(p)
			if p.NStack != 0 {
				t.Fatalf("at the pick NStack %d (Stack[0].Source %d), want 0: gorge's stack no longer holds the spell", p.NStack, p.Stack[0].Source)
			}
			sc.s.Apply(d.Options[0])
			for i := 0; i < 4 && sc.s.res.active; i++ {
				if d = sc.s.Step(); d.Kind == DkPriority {
					break
				}
				sc.s.Apply(d.Options[d.N-1])
			}
			if sc.s.sp != 0 || sc.s.res.active {
				t.Fatalf("after the answer sp %d resolving %v, want the stack empty", sc.s.sp, sc.s.res.active)
			}
			if z := sc.s.objs[o].zone; z != kop.ZoneExile {
				t.Fatalf("after the resolution the spell is in zone %d, want exile %d (not the graveyard)", z, kop.ZoneExile)
			}
		})
	}
}
