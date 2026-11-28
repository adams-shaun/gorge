package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// kingpinProbeScript is Ob Nixilis, Captive Kingpin's LifeLostAll trigger on a
// {1} 1/1 body: whenever the opponents each lose exactly one life (gorge sums
// each player's loss over a simultaneous group: a combat damage pass, one
// LoseLife), a +1/+1 counter, the top card exiled and playable until the
// controller's next end step.
const kingpinProbeScript = "Name:Kingpin Probe\nManaCost:1\nTypes:Creature Demon\nPT:1/1\n" +
	"T:Mode$ LifeLostAll | ValidPlayer$ Opponent | ValidAmountEach$ EQ1 | TriggerZones$ Battlefield | Execute$ ProbeGrow | TriggerDescription$ Whenever opponents each lose exactly one life, grow this, exile your top card and you may play it until your next end step.\n" +
	"SVar:ProbeGrow:DB$ PutCounter | CounterType$ P1P1 | SubAbility$ ProbeExileTop\n" +
	"SVar:ProbeExileTop:DB$ Dig | DigNum$ 1 | ChangeNum$ All | DestinationZone$ Exile | RememberChanged$ True | SubAbility$ ProbePlayEffect\n" +
	"SVar:ProbePlayEffect:DB$ Effect | StaticAbilities$ ProbeMayPlay | RememberObjects$ Remembered | Duration$ UntilYourNextEndStep | SubAbility$ ProbeForget | ForgetOnMoved$ Exile\n" +
	"SVar:ProbeMayPlay:Mode$ Continuous | MayPlay$ True | Affected$ Card.IsRemembered | AffectedZone$ Exile | Description$ You may play the exiled card until your next end step.\n" +
	"SVar:ProbeForget:DB$ Cleanup | ClearRemembered$ True\n" +
	"Oracle:Whenever opponents each lose exactly one life, grow this, exile your top card and you may play it until your next end step.\n"

// drainProbeScript makes each opponent lose one life: one LoseLife group.
const drainProbeScript = "Name:Drain Probe\nManaCost:1\nTypes:Sorcery\n" +
	"A:SP$ LoseLife | Defined$ Player.Opponent | LifeAmount$ 1 | SpellDescription$ Each opponent loses one life.\n" +
	"Oracle:Each opponent loses one life.\n"

// lossWatcherProbeScript keeps a LifeLost listener live, so the kernel deals
// a combat pass's player damage source by source (combatEventsGen) rather
// than as one summed loss: the per-source path is where the group matters.
const lossWatcherProbeScript = "Name:Loss Watcher Probe\nManaCost:1\nTypes:Creature Bat\nPT:0/1\n" +
	"T:Mode$ LifeLost | ValidPlayer$ Opponent | TriggerZones$ Battlefield | Execute$ WatcherGain | TriggerDescription$ Whenever an opponent loses life, gain one life.\n" +
	"SVar:WatcherGain:DB$ GainLife | Defined$ You | LifeAmount$ 1\n" +
	"Oracle:Whenever an opponent loses life, gain one life.\n"

// kingpinGrewAndSummed: over the whole game a Kingpin Probe got a +1/+1
// counter (the trigger fired and resolved), and seat 1 took two one-damage
// hits in one step while a Kingpin Probe was on the battlefield (a combat
// pass gorge sums to 2, so the line must not fire for it: a kernel matching
// each hit alone would queue it twice).
func kingpinGrewAndSummed(g *GorgeSide) bool {
	probes := map[state.ObjID]bool{}
	grew, summed := false, false
	hits := 0
	for i := range g.E.L.Events {
		e := &g.E.L.Events[i]
		switch e.Kind {
		case events.StepChange, events.TurnChange:
			hits = 0
		case events.MoveZone:
			if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && o.Face().Name == "Kingpin Probe" {
				if e.To == state.ZBattlefield {
					probes[e.Obj] = true
				} else if e.From == state.ZBattlefield {
					delete(probes, e.Obj)
				}
			}
		case events.CounterChange:
			if probes[e.Obj] && e.Amount > 0 {
				grew = true
			}
		case events.Damage:
			if e.Obj == 0 && e.Player == 1 && e.Amount == 1 && len(probes) > 0 {
				if hits++; hits >= 2 {
					summed = true
				}
			}
		}
	}
	return grew && summed
}

// TestHostedCmdrLifeLostAll plays the LifeLostAll group trigger decision by
// decision against gorge: seat 0 attacks with Kingpin Probes and Memnites,
// casts Drain Probe and holds Loss Watcher Probes, seat 1 blocks with Grizzly
// Bears at random.
func TestHostedCmdrLifeLostAll(t *testing.T) {
	probeDeck := append(deckOf(12, "Kingpin Probe"), deckOf(8, "Memnite")...)
	probeDeck = append(append(probeDeck, deckOf(4, "Drain Probe")...), deckOf(4, "Loss Watcher Probe")...)
	probeDeck = append(probeDeck, deckOf(12, "Swamp")...)
	oppDeck := append(deckOf(6, "Grizzly Bears"), deckOf(34, "Forest")...)
	runHostedCmdrShape(t, hostedCmdrShape{
		hostedCmdrCase: hostedCmdrCase{
			name:     "kingpin_probe",
			carrier:  "Kingpin Probe",
			seats:    [2]string{"hcs-kingpin-a", "hcs-kingpin-b"},
			decks:    [2][]string{probeDeck, oppDeck},
			fixtures: map[string]string{"Kingpin Probe": kingpinProbeScript, "Drain Probe": drainProbeScript, "Loss Watcher Probe": lossWatcherProbeScript},
		},
		real:    []string{"Swamp", "Forest", "Grizzly Bears", "Memnite"},
		reached: kingpinGrewAndSummed,
	})
}
