package rules

// Charm mode labels follow the printed SpellDescription$ on the trigger
// placement path. Varchild's War-Riders is one of the corpus's three
// sub-chain-only carriers and the only one whose Charm is a TRIGGER body: at
// the beginning of its controller's upkeep its cumulative-upkeep trigger
// poses the placement KModes ask (askTriggerModes, CR 603.3c), whose two
// modes (OnAgeSurvivor, OnAgeSacrifice) carry no SpellDescription$ of
// their own — theirs ride the SurvivorDistribution and Sacrifice subs, one
// hop down. Neither option is labelled by its raw SVar name.
//
// The script follows Varchild's War-Riders, without the deck-related rider or
// the Oracle line, inlined so the
// corpus carrier's exact parameter spellings are what the engine defends
// without committing GPL data.

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

const varchildsWarRidersScript = "Name:Varchild's War-Riders\nManaCost:1 R\nTypes:Creature Human Warrior\nPT:3/4\n" +
	"K:Trample\nK:Rampage:1\n" +
	"T:Mode$ Phase | Phase$ Upkeep | ValidPlayer$ You | TriggerZones$ Battlefield | Execute$ OnCumUpkeep\n" +
	"SVar:OnCumUpkeep:DB$ Charm | Choices$ OnAgeSurvivor,OnAgeSacrifice\n" +
	"SVar:OnAgeSurvivor:DB$ PutCounter | Defined$ Self | CounterType$ AGE | CounterNum$ 1 | SubAbility$ SurvivorDistribution\n" +
	"SVar:SurvivorDistribution:DB$ Repeat | RepeatSubAbility$ PickOpponent | MaxRepeat$ X | SpellDescription$ An opponent gets a red Survivor for each Age Counter.\n" +
	"SVar:PickOpponent:DB$ ChoosePlayer | Defined$ You | Choices$ Player.Opponent | SubAbility$ MakeToken\n" +
	"SVar:MakeToken:DB$ Token | TokenScript$ r_1_1_survivor | TokenOwner$ ChosenPlayer\n" +
	"SVar:X:Count$CardCounters.AGE\n" +
	"SVar:OnAgeSacrifice:DB$ PutCounter | Defined$ Self | CounterType$ AGE | CounterNum$ 1 | SubAbility$ Sacrifice\n" +
	"SVar:Sacrifice:DB$ Sacrifice | Defined$ Self | SpellDescription$ Sacrifice this.\n" +
	"Oracle:x\n"

// TestVarchildsWarRidersPlacementLabelsFollowThePrintedDescriptions drives a
// battlefield Varchild's War-Riders to its upkeep trigger's placement
// KModes ask and asserts the options carry the subs' printed descriptions,
// never the raw SVar names.
func TestVarchildsWarRidersPlacementLabelsFollowThePrintedDescriptions(t *testing.T) {
	e := combatEngine(t)
	riders := onBoardCard(t, e, 0, card(t, varchildsWarRidersScript))
	if o := e.G.Obj(riders); o == nil || o.Zone != state.ZBattlefield {
		t.Fatal("precondition: Varchild's War-Riders is not on the battlefield")
	}

	e.pending = nil
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepUpkeep})
	if len(e.pendingTriggers) != 1 {
		t.Fatalf("precondition: queued triggers = %d, want the War-Riders' upkeep trigger",
			len(e.pendingTriggers))
	}
	e.putTriggersOnStack()
	d := e.Pending()
	if d == nil || d.Kind != decision.KModes {
		t.Fatalf("pending = %+v, want the placement KModes ask", d)
	}
	if len(d.Options) != 2 {
		t.Fatalf("precondition: options = %+v, want the two Choices$ modes", d.Options)
	}

	for _, want := range []string{
		"a red Survivor for each Age Counter",
		"Sacrifice this.",
	} {
		found := false
		for _, o := range d.Options {
			if strings.Contains(o.Label, want) {
				found = true
			}
		}
		if !found {
			t.Fatalf("printed description %q not offered: %+v", want, d.Options)
		}
	}
	for _, o := range d.Options {
		if o.Label == "OnAgeSurvivor" || o.Label == "OnAgeSacrifice" {
			t.Fatalf("mode option still labelled by its raw SVar name: %+v", d.Options)
		}
	}
}
