package oracle

import (
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Alchemist's Gambit's ExtraTurnDelayedTrigger$ rider (hostedCmdrCases'
// alchemists_gambit_upkeep_rider): a Static$ True Mode$ Phase | Phase$
// Upkeep delayed trigger gorge registers at the granted turn (events.Apply
// foldExtraTurn, MinTurn the granted turn) whose Execute is a DB$ Effect
// carrying a global CantPreventDamage static and, on the uncleaved face, the
// EndLose end-step trigger. "Gambit Probe" is the uncleaved face's rider on
// a {2}{R} sorcery, "Gambit Cleave Probe" the cleave face's (static only);
// "Pyre Sweep Probe" is Pyroclasm's body and "Red Ward Probe" a creature with
// protection from red, whose damage the sweep can deal only while damage
// can't be prevented. The case FAILS unless some seed sweeps a warded
// creature for real damage during a granted turn.

const gambitProbeScript = "Name:Gambit Probe\nManaCost:2 R\nTypes:Sorcery\n" +
	"A:SP$ AddTurn | NumTurns$ 1 | ExtraTurnDelayedTrigger$ ProbeUpkeep | ExtraTurnDelayedTriggerExecute$ ProbeNoPrev | SubAbility$ ProbeExile | SpellDescription$ Probe: one more turn; no prevention then, and you lose at its end step.\n" +
	"SVar:ProbeUpkeep:Mode$ Phase | Phase$ Upkeep | ThisTurn$ True | Static$ True | TriggerDescription$ Probe: no prevention this turn.\n" +
	"SVar:ProbeNoPrev:DB$ Effect | StaticAbilities$ ProbeStatic | Triggers$ ProbeEnd | Defined$ You\n" +
	"SVar:ProbeStatic:Mode$ CantPreventDamage | Description$ Probe: prevention is off.\n" +
	"SVar:ProbeEnd:Mode$ Phase | Phase$ End of Turn | Execute$ ProbeLose | TriggerDescription$ Probe: lose at this end step.\n" +
	"SVar:ProbeLose:DB$ LosesGame | Defined$ You\n" +
	"SVar:ProbeExile:DB$ ChangeZone | Origin$ Stack | Destination$ Exile | SpellDescription$ Probe: exile it.\n" +
	"Oracle:Probe: one more turn; no prevention then, and you lose at its end step.\n"

const gambitCleaveProbeScript = "Name:Gambit Cleave Probe\nManaCost:2 R\nTypes:Sorcery\n" +
	"A:SP$ AddTurn | NumTurns$ 1 | ExtraTurnDelayedTrigger$ ProbeUpkeep | ExtraTurnDelayedTriggerExecute$ ProbeNoPrev | SubAbility$ ProbeExile | SpellDescription$ Probe: one more turn with no prevention.\n" +
	"SVar:ProbeUpkeep:Mode$ Phase | Phase$ Upkeep | ThisTurn$ True | Static$ True | TriggerDescription$ Probe: no prevention this turn.\n" +
	"SVar:ProbeNoPrev:DB$ Effect | StaticAbilities$ ProbeStatic | Defined$ You\n" +
	"SVar:ProbeStatic:Mode$ CantPreventDamage | Description$ Probe: prevention is off.\n" +
	"SVar:ProbeExile:DB$ ChangeZone | Origin$ Stack | Destination$ Exile | SpellDescription$ Probe: exile it.\n" +
	"Oracle:Probe: one more turn with no prevention.\n"

// hubProbeScript is Eon Hub's printed upkeep skip on a {1} artifact: with it
// on the battlefield no upkeep happens, so gorge's Phase$ Upkeep rider never
// fires during the granted turn (no restriction, no end-step loss).
const hubProbeScript = "Name:Hub Probe\nManaCost:1\nTypes:Artifact\n" +
	"R:Event$ BeginPhase | ActiveZones$ Battlefield | Phase$ Upkeep | Skip$ True | Description$ Probe: no upkeep steps.\n" +
	"Oracle:Probe: no upkeep steps.\n"

const pyreSweepProbeScript = "Name:Pyre Sweep Probe\nManaCost:1 R\nTypes:Sorcery\n" +
	"A:SP$ DamageAll | ValidCards$ Creature | NumDmg$ 2 | ValidDescription$ every creature. | SpellDescription$ Probe: 2 damage to every creature.\n" +
	"Oracle:Probe: 2 damage to every creature.\n"

const redWardProbeScript = "Name:Red Ward Probe\nManaCost:1 R\nTypes:Creature Human Knight\nPT:2/2\n" +
	"K:Protection from red\n" +
	"Oracle:Protection from red\n"

func gambitRiderCases() []hostedCmdrCase {
	probeDeck := append(append(deckOf(2, "Gambit Probe"), deckOf(2, "Gambit Cleave Probe")...),
		append(deckOf(12, "Pyre Sweep Probe"), deckOf(24, "Mountain")...)...)
	wardDeck := append(deckOf(16, "Red Ward Probe"), deckOf(24, "Mountain")...)
	cleaveDeck := append(append(deckOf(4, "Gambit Cleave Probe"), deckOf(12, "Pyre Sweep Probe")...), deckOf(24, "Mountain")...)
	hubDeck := append(append(deckOf(4, "Gambit Probe"), deckOf(4, "Hub Probe")...), deckOf(32, "Mountain")...)
	gambitDeck := append(deckOf(2, "Alchemist's Gambit"), deckOf(38, "Mountain")...)
	creatureDeck := append(deckOf(8, "Grizzly Bears"), deckOf(32, "Forest")...)
	return []hostedCmdrCase{
		{
			name:    "alchemists_gambit_upkeep_rider",
			carrier: "Gambit Probe",
			seats:   [2]string{"hck-gambit-a", "hck-gambit-b"},
			decks:   [2][]string{probeDeck, wardDeck},
			fixtures: map[string]string{
				"Gambit Probe": gambitProbeScript, "Gambit Cleave Probe": gambitCleaveProbeScript,
				"Pyre Sweep Probe": pyreSweepProbeScript, "Red Ward Probe": redWardProbeScript,
			},
			reached: wardedCreatureDamaged,
		},
		{
			// the cleave face's rider: the static alone, no end-step loss
			name:    "alchemists_gambit_cleave_rider",
			carrier: "Gambit Cleave Probe",
			seats:   [2]string{"hck-gambit-cleave-a", "hck-gambit-cleave-b"},
			decks:   [2][]string{cleaveDeck, wardDeck},
			fixtures: map[string]string{
				"Gambit Cleave Probe": gambitCleaveProbeScript,
				"Pyre Sweep Probe":    pyreSweepProbeScript, "Red Ward Probe": redWardProbeScript,
			},
			reached: wardedCreatureDamaged,
		},
		{
			// the granted turn's upkeep is skipped: the rider waits for the
			// first upkeep that happens, so the caster survives that end step
			name:     "alchemists_gambit_upkeep_skipped",
			carrier:  "Gambit Probe",
			seats:    [2]string{"hck-gambit-hub-a", "hck-gambit-hub-b"},
			decks:    [2][]string{hubDeck, creatureDeck},
			fixtures: map[string]string{"Gambit Probe": gambitProbeScript, "Hub Probe": hubProbeScript},
			reached:  grantedTurnUpkeepSkipped,
		},
		{
			name:    "alchemists_gambit_printed",
			carrier: "Alchemist's Gambit",
			seats:   [2]string{"hck-gambit-real-a", "hck-gambit-real-b"},
			decks:   [2][]string{gambitDeck, creatureDeck},
			reached: grantConsumed("Alchemist's Gambit"),
		},
	}
}

// wardedCreatureDamaged: a seat-1 Red Ward Probe was dealt damage. Every
// damage source seat 0 has is red (Pyre Sweep Probe) and seat 1 never
// attacks, so protection prevents every such hit unless damage can't be
// prevented: only the granted turn's CantPreventDamage static lets it land.
func wardedCreatureDamaged(g *GorgeSide) bool {
	for i := range g.E.L.Events {
		e := &g.E.L.Events[i]
		if e.Kind != events.Damage || e.Amount <= 0 || e.Obj == 0 {
			continue
		}
		if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Owner == 1 && o.Face().Name == "Red Ward Probe" {
			return true
		}
	}
	return false
}

// grantedTurnUpkeepSkipped: a Gambit Probe grant was consumed (its granted
// turn began) and that turn reached its end step with no upkeep StepChange in
// between: the upkeep skip held the rider back, so in gorge that end step
// costs the caster nothing (the lockstep checks the kernel agrees).
func grantedTurnUpkeepSkipped(g *GorgeSide) bool {
	granted := false
	for i := range g.E.L.Events {
		e := &g.E.L.Events[i]
		switch {
		case e.Kind == events.ExtraTurn && e.Amount == -1:
			o := g.E.G.Obj(e.Obj)
			granted = o != nil && o.Face() != nil && o.Face().Name == "Gambit Probe"
		case e.Kind == events.StepChange && e.Step == state.StepUpkeep:
			granted = false
		case e.Kind == events.StepChange && e.Step == state.StepEnd && granted:
			return true
		}
	}
	return false
}

// grantConsumed: a grant from the named card was consumed, so its granted
// turn began with the rider registered.
func grantConsumed(name string) func(g *GorgeSide) bool {
	return func(g *GorgeSide) bool {
		for i := range g.E.L.Events {
			e := &g.E.L.Events[i]
			if e.Kind != events.ExtraTurn || e.Amount != -1 {
				continue
			}
			if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == name {
				return true
			}
		}
		return false
	}
}
