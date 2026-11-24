package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestTargetValidTargetingRechecksCurrentTargets pins CR 115.9b at
// resolution: a Counter with TargetValidTargeting$ re-reads its target spell
// or ability's CURRENT targets. The target's creature left the battlefield
// after it was chosen, so the clause matches nothing and the counter's own
// target is illegal (CR 608.2b); the same state after the spell resolved
// would still have been a legal target under the old plain-stack recheck.
// Both lowerings of the clause are covered: a spell target (TgtSpellTgting,
// Fugitive Droid) and an activated/triggered ability target (TgtStackTgting,
// Diplomatic Escort).
func TestTargetValidTargetingRechecksCurrentTargets(t *testing.T) {
	for _, tc := range []struct {
		name    string
		kind    kop.TargetKind
		ability bool
	}{
		{"spell", kop.TgtSpellTgting, false},
		{"ability", kop.TgtStackTgting, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tbl := *v.Table
			tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
			tbl.Exprs = append([]kop.Expr(nil), v.Table.Exprs...)
			tbl.Conds = append([]kop.Cond(nil), v.Table.Conds...)
			targetID := CardID(len(tbl.Cards))
			tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Targeting Spell", Types: kop.TInstant})
			counterID := CardID(len(tbl.Cards))
			tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Targeting Counter", Types: kop.TInstant,
				Abilities: []kop.Ability{{
					Kind: kop.AbSpell,
					Targets: [kop.MaxTargets]kop.TargetSpec{
						{Kind: tc.kind, Min: 1, Max: 1},
					},
					NTargets: 1,
				}}})

			sc := newScenTable(t, &tbl, Config{NoElide: true})
			s, k := sc.s, sc.k
			creature := sc.perm(v.GrizzlyBears, 0)
			counter := sc.obj(counterID, 0)
			s.objs[counter].zone = kop.ZoneStack
			target := sc.obj(targetID, 1)
			s.objs[target].zone = kop.ZoneStack
			s.stack[1] = stackEntry{kind: seSpell, obj: counter, card: counterID, ctrl: 0, nt: 1}
			if tc.ability {
				s.stack[0] = stackEntry{kind: seAbility, obj: target, card: targetID, ctrl: 1, nt: 1, sid: 1}
				s.stack[0].tgt[0] = TCode(creature)
				s.stack[1].tgt[0] = StackTarget0 + 0
				s.stack[1].tver[0] = 1
			} else {
				s.stack[0] = stackEntry{kind: seSpell, obj: target, card: targetID, ctrl: 1, nt: 1}
				s.stack[0].tgt[0] = TCode(creature)
				s.stack[1].tgt[0] = TCode(target)
			}
			s.sp = 2

			// Precondition: the targeting object is on the stack and its
			// own target is the battlefield creature, so the clause matches.
			if !sc.onBF(creature) {
				t.Fatal("precondition: the targeting object's target is not on the battlefield")
			}
			if !s.targetStillLegal(k, &s.stack[1], 0) {
				t.Fatal("precondition: the counter's target is not legal while it still targets the creature")
			}
			// CR 115.9b: the creature left the zone the clause expects, so
			// the targeting object no longer matches and the counter fizzles.
			s.leave(k, sc.slot(creature), kop.ZoneGraveyard, false)
			if s.targetStillLegal(k, &s.stack[1], 0) {
				t.Fatal("the counter's target stayed legal after its target creature left the battlefield (CR 115.9b)")
			}
		})
	}
}
