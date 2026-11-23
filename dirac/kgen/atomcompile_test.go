package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestAtomCompilerLowers: the exported compiler lowers a filter word and a
// Count head to the kernel forms a card would carry.
func TestAtomCompilerLowers(t *testing.T) {
	a := NewAtomCompiler()
	id, err := a.Filter("Creature.tapped", kop.ZoneBattlefield)
	if err != nil {
		t.Fatal(err)
	}
	fs, _ := a.Tables()
	if id == 0 || fs[id].Flags&kop.FTapped == 0 || fs[id].Types == 0 {
		t.Errorf("Creature.tapped lowered to %+v", fs[id])
	}
	v, err := a.Count("Count$Valid Creature.YouCtrl")
	if err != nil {
		t.Fatal(err)
	}
	fs, _ = a.Tables()
	if v.K != kop.VCountFilter || fs[v.N].Types&kop.TCreature == 0 {
		t.Errorf("Count$Valid Creature.YouCtrl lowered to %+v", v)
	}
	if _, err := a.Filter("Card.Frobnitz", kop.ZoneBattlefield); err == nil {
		t.Errorf("an unknown word lowered")
	}
}
