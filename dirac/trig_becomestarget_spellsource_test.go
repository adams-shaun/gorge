package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestBecomesTargetSpellSourceExcludesAbilities pins the dirac half of the
// CR 112.1/113.3 line: a bare ValidSource$ Spell gate admits a card on the
// stack and rejects an ability stack entry, while SpellAbility keeps the
// ability reading through TrigNSrcCtrl/TrigNSrcAbility. The lockstep twin is
// TestBecomesTargetSpellSourceLockstep in dirac/oracle.
func TestBecomesTargetSpellSourceExcludesAbilities(t *testing.T) {
	s := &State{}
	k := &prep{}
	a := &kop.Ability{}
	a.Trig.N = kop.TrigNSrcSpell
	a.Trig.Whose = kop.WhoseAny
	l := trigListener{}
	if !s.btSpellSrcOK(k, l, a, &stackEntry{kind: seSpell}) {
		t.Error("a spell stack entry did not pass the bare Spell gate")
	}
	if s.btSpellSrcOK(k, l, a, &stackEntry{kind: seAbility}) {
		t.Error("an ability stack entry passed the bare Spell gate (CR 112.1, 113.3)")
	}
}
