package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// DebugDbManaResolve is the mutation: when set, a resolving
// SPELL's OpAddMana adds nothing, while a triggered ability's still does. The
// oracle's TestDbmanaMutation requires the lockstep to diverge with it.
var DebugDbManaResolve bool

// prepDbMana reports whether some card's spell ability carries an OpAddMana in
// its program (genPrep.anySpellMana). A charm's modes are lowered inline into
// the spell ability's program (OpCharm branches), so a mana mode is a plain
// OpAddMana there and needs no separate mode walk. Triggered and activated
// abilities are not counted: their mana is the trigger path OpAddMana already
// served. Cold, called once from prepareGen.
func prepDbMana(t *kop.Table) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			ab := &t.Cards[i].Abilities[j]
			if ab.Kind != kop.AbSpell {
				continue
			}
			for k := range ab.Prog {
				if ab.Prog[k].Op == kop.OpAddMana {
					return true
				}
			}
		}
	}
	return false
}

// dbManaMut is the mutation's one hot-path hook: a SPELL's AddMana zeroes its
// amount when DebugDbManaResolve is set. The resolving entry is s.res.e, whose
// kind separates a spell (seSpell) from an activated or triggered ability.
//
//go:noinline
func (s *State) dbManaMut(e *stackEntry, n *int) {
	if DebugDbManaResolve && e.kind == seSpell {
		*n = 0
	}
}
