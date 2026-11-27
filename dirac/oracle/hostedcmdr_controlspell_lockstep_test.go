package oracle

import (
	"errors"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Commandeer (vivi-ornitier-cedh's api ControlSpell blocker, dirac gap
// cli-20261127T171908Z) played in lockstep against gorge:
//
//	SP$ ControlSpell | ValidTgts$ Card.nonCreature | TargetType$ Spell | Mode$ Gain
//
// "Gain control of target noncreature spell" — gorge's effControlSpell
// (engine/effects/choose_control.go) moves the target spell's controller on
// the stack (one ControlChange, no duration), so the artifact, enchantment or
// planeswalker it becomes enters under Commandeer's controller; its
// DBChooseTargets sub-ability (DB$ ChangeTargets | Optional$ True) may choose
// new targets and already lowers (ExtChangeTargets). The kernel primitive is
// ExtControlSpell: the stack entry's controller becomes the resolving
// controller. The pool: seat 1 free-casts Darksteel Relic, seat 0's
// Commandeer steals it from the stack.
func TestHostedCmdrControlSpellLockstep(t *testing.T) {
	t.Parallel()
	_ = corpusDirOrSkip(t)
	decks := [2][]cardset.Entry{
		{
			{Name: "Commandeer", Count: 12}, {Name: "Merfolk of the Pearl Trident", Count: 12},
			{Name: "Island", Count: 16},
		},
		{
			{Name: "Darksteel Relic", Count: 14}, {Name: "Hill Giant", Count: 8},
			{Name: "Mountain", Count: 18},
		},
	}
	set, err := cardset.RawByName(hostedCmdrStackPool)
	if errors.Is(err, cardset.ErrNoCorpus) {
		t.Skip("no kernel-synth corpus table: " + err.Error())
	}
	if err != nil {
		t.Fatal(err)
	}
	tbl, _, _, err := set.Matchup(decks[0], decks[1])
	if err != nil {
		t.Fatal(err)
	}
	if hostedCmdrControlSpellOps(tbl) == 0 {
		t.Fatal("Commandeer has no ExtControlSpell on its spell ability: the spelling did not lower")
	}
	lockstepSet(t, hostedCmdrStackPool, decks, 200, "KHOSTEDCMDRCTRLSP_SEEDS")
	if lockstepRun(t, hostedCmdrStackPool, decks, 200, false, func(tbl *kop.Table) {
		for _, in := range hostedCmdrControlSpellInstrs(tbl) {
			in.Obj = kop.RefNone // the flip never lands
		}
	}) == 0 {
		t.Fatal("no seed diverged with ExtControlSpell pointed at nothing: the pool never reaches Commandeer's transfer")
	}
}

// hostedCmdrControlSpellOps counts Commandeer's ExtControlSpell instructions.
func hostedCmdrControlSpellOps(tbl *kop.Table) int {
	return len(hostedCmdrControlSpellInstrs(tbl))
}

// hostedCmdrControlSpellInstrs are Commandeer's spell program's
// OpExt/ExtControlSpell instructions.
func hostedCmdrControlSpellInstrs(tbl *kop.Table) []*kop.Instr {
	cd := mbcCard(tbl, "Commandeer")
	if cd == nil {
		return nil
	}
	var out []*kop.Instr
	for j := range cd.Abilities {
		prog := cd.Abilities[j].Prog
		for k := range prog {
			if prog[k].Op == kop.OpExt && prog[k].A == kop.Const(int(kop.ExtControlSpell)) {
				out = append(out, &prog[k])
			}
		}
	}
	return out
}
