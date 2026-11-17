package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestExtraBlockNoDamageBlockerNoDivision: a blocker of two attackers whose
// combat damage assignment is zeroed (AssignNoCombatDamage) is not asked to
// divide it; gorge's appendBandBlockerDivisions skips a blocker whose
// combatDamageAmount is 0. The control blocker (no static) is asked.
func TestExtraBlockNoDamageBlockerNoDivision(t *testing.T) {
	for _, tc := range []struct {
		name   string
		noDmg  bool
		wantAk bool
	}{{"control", false, true}, {"noDamage", true, false}} {
		twins := kop.CardDef{Name: "Twins", Types: kop.TCreature, Power: 7, Toughness: 7, ExtraBlocks: 1}
		if tc.noDmg {
			twins.Abilities = []kop.Ability{{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatNoCombatDamage, Filter: 1}}}
		}
		tbl := &kop.Table{
			Cards: []kop.CardDef{
				{Name: "Bear", Types: kop.TCreature, Power: 2, Toughness: 2},
				twins,
				{Name: "Three", Types: kop.TCreature, Power: 3, Toughness: 3},
				{Name: "Four", Types: kop.TCreature, Power: 4, Toughness: 4},
			},
			Filters: []kop.Filter{{}, {Types: kop.TCreature}},
			Conds:   []kop.Cond{},
		}
		sc := newScenTableLib(t, tbl, 0, Config{NoElide: true})
		if sc.k.gen == nil || !sc.k.gen.anyExtraBlock || sc.k.anyNoDmg != tc.noDmg {
			t.Fatalf("%s: gen prep anyExtraBlock/anyNoDmg not as built", tc.name)
		}
		a3, a4 := sc.perm(2, 0), sc.perm(3, 0)
		tw := sc.perm(1, 1)
		xbAttack(t, sc, a3, a4)
		sc.do(t, AkBlock, TCode(tw), TCode(a4))
		sc.do(t, AkBlock, TCode(tw), TCode(a3))
		sc.do(t, AkDone, scAny, scAny)
		asked := false
		for i := 0; i < 60; i++ {
			d := sc.s.Step()
			if d.Kind == DkChoose && d.Sub == ChDivide {
				asked = true
				break
			}
			if d.Kind != DkPriority {
				break
			}
			if sc.s.step == kop.StepMain2 {
				break
			}
			sc.s.Apply(MakeAction(AkPass, 0, 0, 0))
		}
		if asked != tc.wantAk {
			t.Errorf("%s: blocker division asked=%v, want %v", tc.name, asked, tc.wantAk)
		}
	}
}
