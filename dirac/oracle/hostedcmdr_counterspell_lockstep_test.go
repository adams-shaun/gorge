package oracle

import (
	"errors"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Disrupting Shoal (vivi-ornitier-cedh's DB$ Counter blocker, dirac gap
// cli-20261127T171908Z) played in lockstep against gorge:
//
//	A:SP$ TargetOnly | TargetType$ Spell | TgtZone$ Stack | ValidTgts$ Card
//	SVar:DBCounter:DB$ Counter | Defined$ Targeted
//	  | ConditionCheckSVar$ T | ConditionSVarCompare$ EQX
//
// The head declares the stack-spell slot (pumpSpellTarget already lowers it);
// the Counter sub takes the cast's own spell-only slot as its object (gorge
// effCounter's Defined → Targeted, engine/effects/resolve.go), gated on the
// SVar condition "targeted spell's mana value is X" (SVar T =
// Targeted$CardManaCost, the kernel's targetMV read). X is announced by the
// AlternativeCost static's Announce$ X and paid by exiling a blue card of
// mana value X from hand (the announcex plumbing, hostedcmdr_announcex_lockstep_test.go).
// The pool: seat 1 casts Shock (mana value 2), seat 0 responds with the
// Shoal announcing X = 2 against a hand Cancel.
func TestHostedCmdrCounterTargetedLockstep(t *testing.T) {
	t.Parallel()
	_ = corpusDirOrSkip(t)
	decks := [2][]cardset.Entry{
		{
			{Name: "Disrupting Shoal", Count: 12}, {Name: "Cancel", Count: 10},
			{Name: "Merfolk of the Pearl Trident", Count: 6}, {Name: "Island", Count: 12},
		},
		{
			{Name: "Shock", Count: 12}, {Name: "Hill Giant", Count: 8},
			{Name: "Mountain", Count: 20},
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
	if hostedCmdrTargetedCounterInstrs(tbl) == nil {
		t.Fatal("Disrupting Shoal has no OpCounter over its cast spell target: the spelling did not lower")
	}
	lockstepSet(t, hostedCmdrStackPool, decks, 200, "KHOSTEDCMDRCTRTGT_SEEDS")
	if lockstepRun(t, hostedCmdrStackPool, decks, 200, false, func(tbl *kop.Table) {
		for _, in := range hostedCmdrTargetedCounterInstrs(tbl) {
			in.Obj = kop.RefNone // the counter lands on nothing
		}
	}) == 0 {
		t.Fatal("no seed diverged with the Shoal's OpCounter pointed at nothing: the pool never reaches its counter")
	}
}

// hostedCmdrTargetedCounterInstrs are Disrupting Shoal's spell program's
// OpCounter instructions (its DBCounter sub over the cast's spell target).
func hostedCmdrTargetedCounterInstrs(tbl *kop.Table) []*kop.Instr {
	cd := mbcCard(tbl, "Disrupting Shoal")
	if cd == nil {
		return nil
	}
	var out []*kop.Instr
	for j := range cd.Abilities {
		prog := cd.Abilities[j].Prog
		for k := range prog {
			if prog[k].Op == kop.OpCounter {
				out = append(out, &prog[k])
			}
		}
	}
	return out
}
