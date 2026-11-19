package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestActScriptPaysHandTableDoesNotPanic: a hand-written table without a
// generated extension has prep.gen nil (prep.go), so the activation-offer
// walk must not hand actScriptPays a filter script that dereferences it. A
// 1/1 with a {1} activation and no mana is unaffordable; the fallback must
// decline, not panic while enumerating p0's priority options.
func TestActScriptPaysHandTableDoesNotPanic(t *testing.T) {
	tbl := &kop.Table{
		Cards: []kop.CardDef{
			{Name: "Land", Types: kop.TLand},
			{Name: "Test Bear", Types: kop.TCreature, Power: 1, Toughness: 1,
				Abilities: []kop.Ability{{Kind: kop.AbActivated,
					Cost: kop.Cost{Mana: kop.ManaCost{Generic: 1}},
					Prog: []kop.Instr{{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(1)}}}}},
		},
	}
	sc := newScenTableLib(t, tbl, 0, Config{NoElide: true})
	if sc.k.gen != nil {
		t.Fatal("precondition: the hand-written table has a generated extension")
	}
	bear := sc.perm(1, 0)
	d := sc.s.Step()
	if d.Kind != DkPriority {
		t.Fatalf("Step kind %d, want priority", d.Kind)
	}
	for i := 0; i < int(d.N); i++ {
		if o := d.Options[i]; o.Kind() == AkActivate && o.Source() == TCode(bear) {
			t.Fatalf("offered an unaffordable activation: %v", o)
		}
	}
}
