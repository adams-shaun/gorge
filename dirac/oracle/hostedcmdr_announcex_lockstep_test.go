package oracle

import (
	"errors"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The Shoal cycle's Announce$ X alternative cost (vivi-ornitier-cedh's
// Blazing Shoal blocker, dirac gap cli-20261127T171908Z) played in lockstep
// against gorge:
//
//	S:Mode$ AlternativeCost | ValidSA$ Spell.Self | EffectZone$ All
//	  | Cost$ ExileFromHand<1/Card.Red+Other+cmcEQX/red card> | Announce$ X
//	SVar:X:Count$xPaid
//
// gorge reads the static's Announce$ (engine/rules/statics_altcost.go
// altCostView.announce) and, when the exile alternative is picked, asks the
// caster a value for X among the mana values the exilable hand cards still
// match (engine/rules/cast_subcounter.go xAsk's announce arm,
// altCostXCandidates); the settlement's cmcEQX filter then binds that X
// (engine/rules/cast_asks.go exAsk), and Count$xPaid makes the spell's X the
// announced mana value (engine/effects/count_body.go evalCountBodyCostXPaid).
// The kernel takes the same route: the announced X rides the entry (its {X}),
// the ask is bounded by the matching hand cards like the announced
// ExileFromGrave<X/Spec> (cost_exilex.go exileXClamp), and the exile pick
// reads mana value == the announced X through PredCMCLEV. Blazing Shoal's
// body then pumps the target creature +X/+0 for the announced X.
//
// The pool: seat 0 casts Blazing Shoal (announced against its red hand) to
// pump a creature for the attack; seat 1 blocks with bodies.
func TestHostedCmdrAnnounceXLockstep(t *testing.T) {
	t.Parallel()
	_ = corpusDirOrSkip(t)
	decks := [2][]cardset.Entry{
		{
			{Name: "Blazing Shoal", Count: 12}, {Name: "Raging Goblin", Count: 10},
			{Name: "Shock", Count: 6}, {Name: "Mountain", Count: 12},
		},
		{
			{Name: "Hill Giant", Count: 10}, {Name: "Grizzly Bears", Count: 8},
			{Name: "Mountain", Count: 22},
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
	if hostedCmdrShoalPumpInstrs(tbl) == nil {
		t.Fatal("Blazing Shoal has no OpPumpUntilEOT on its spell ability: the spelling did not lower")
	}
	if !hostedCmdrHasCMCBoundX(tbl) {
		t.Fatal("no PredCMCLEV filter in the table: the cmcEQX exile filter is not bound to the announced X")
	}
	lockstepSet(t, hostedCmdrStackPool, decks, 200, "KHOSTEDCMDRANNX_SEEDS")
	if lockstepRun(t, hostedCmdrStackPool, decks, 200, false, func(tbl *kop.Table) {
		for _, in := range hostedCmdrShoalPumpInstrs(tbl) {
			in.A = kop.Const(0) // the announced X never reaches the pump
		}
	}) == 0 {
		t.Fatal("no seed diverged with the Shoal's pump pinned to 0: the pool never reaches its announced X")
	}
}

// hostedCmdrShoalPumpInstrs are Blazing Shoal's spell program's
// OpPumpUntilEOT instructions (its SP$ Pump | NumAtt$ +X node).
func hostedCmdrShoalPumpInstrs(tbl *kop.Table) []*kop.Instr {
	cd := mbcCard(tbl, "Blazing Shoal")
	if cd == nil {
		return nil
	}
	var out []*kop.Instr
	for j := range cd.Abilities {
		prog := cd.Abilities[j].Prog
		for k := range prog {
			if prog[k].Op == kop.OpPumpUntilEOT {
				out = append(out, &prog[k])
			}
		}
	}
	return out
}

// hostedCmdrHasCMCBoundX reports whether the table carries a mana-value
// filter bound to a value (the cmcEQX exile filter read against the
// announced X).
func hostedCmdrHasCMCBoundX(tbl *kop.Table) bool {
	for i := range tbl.Filters {
		if tbl.Filters[i].Pred == kop.PredCMCLEV {
			return true
		}
	}
	return false
}
