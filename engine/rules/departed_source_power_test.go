package rules

// CR 113.7a / 608.2h: an ability whose source has left the battlefield reads
// the source's characteristics from its last known information. Threefold
// Thunderhulk enters as a 3/3 (three +1/+1 counters); with its "create Gnomes
// equal to its power" enter trigger waiting, its controller sacrifices it to
// Vanguard of the Rose. The trigger still makes three Gnomes -- before this,
// Count$CardPower read the 0/0 graveyard card and made none.
//
// Both fixtures are real Forge scripts written inline (never a .cards/ .txt);
// the Thunderhulk's mana cost is cut to {1} so the fixture deck can cast it.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

const thunderhulkLKISrc = "Name:Threefold Thunderhulk\nManaCost:1\nTypes:Artifact Creature Gnome\nPT:0/0\n" +
	"K:etbCounter:P1P1:3\n" +
	"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigToken | TriggerDescription$ Whenever CARDNAME enters or attacks, create a number of 1/1 colorless Gnome artifact creature tokens equal to its power.\n" +
	"T:Mode$ Attacks | ValidCard$ Card.Self | Execute$ TrigToken | Secondary$ True | TriggerZones$ Battlefield | TriggerDescription$ Whenever CARDNAME enters or attacks, create a number of 1/1 colorless Gnome artifact creature tokens equal to its power.\n" +
	"SVar:TrigToken:DB$ Token | TokenScript$ c_1_1_a_gnome | TokenOwner$ You | TokenAmount$ X\n" +
	"SVar:X:Count$CardPower\n" +
	"Oracle:Threefold Thunderhulk enters with three +1/+1 counters on it.\\nWhenever Threefold Thunderhulk enters or attacks, create a number of 1/1 colorless Gnome artifact creature tokens equal to its power.\n"

const vanguardOfTheRoseSrc = "Name:Vanguard of the Rose\nManaCost:1 W\nTypes:Creature Vampire Knight\nPT:3/1\n" +
	"A:AB$ Pump | Cost$ 1 Sac<1/Creature.Other;Artifact.Other/another creature or artifact> | Defined$ Self | KW$ Indestructible | SubAbility$ DBTap | StackDescription$ SpellDescription | SpellDescription$ CARDNAME gains indestructible until end of turn.\n" +
	"SVar:DBTap:DB$ Tap | Defined$ Self | StackDescription$ SpellDescription | SpellDescription$ Tap it.\n" +
	"Oracle:{1}, Sacrifice another creature or artifact: Vanguard of the Rose gains indestructible until end of turn. Tap it.\n"

func gnomeTokens(e *Engine) int {
	n := 0
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o != nil && o.IsToken && o.Face() != nil && o.Face().Name == "Gnome Token" {
			n++
		}
	}
	return n
}

func TestDepartedSourcePowerIsLastKnown(t *testing.T) {
	t.Parallel()
	e, cfg, find := etbConfig(t, 114, []string{thunderhulkLKISrc, vanguardOfTheRoseSrc}, nil)
	e.G.Tokens["c_1_1_a_gnome"] = card(t, "Name:Gnome Token\nTypes:Artifact Creature Gnome\nPT:1/1\nOracle:x\n")
	hulk, vanguard := find("Threefold Thunderhulk", 0), find("Vanguard of the Rose", 0)
	e.emit(events.Event{Kind: events.MoveZone, Obj: vanguard, From: state.ZHand, To: state.ZBattlefield})
	addMana(t, e, 0, "RR")
	castFirstOf(t, e, hulk)
	// Pass until the Thunderhulk has resolved and its enter trigger waits.
	for i := 0; i < 10; i++ {
		if o := e.G.Obj(hulk); o.Zone == state.ZBattlefield && len(e.G.Stack) == 1 {
			break
		}
		passOnce(t, e)
	}
	if o := e.G.Obj(hulk); o.Zone != state.ZBattlefield || e.Power(hulk) != 3 || len(e.G.Stack) != 1 ||
		e.G.Obj(e.G.Stack[0]).Source != hulk {
		t.Fatalf("precondition: hulk zone %v power %d stack %v, want a 3/3 on the battlefield with its trigger waiting",
			o.Zone, e.Power(hulk), e.G.Stack)
	}
	// Sacrifice it to Vanguard of the Rose with the trigger still waiting.
	submitChoices(t, e, abilityOption(t, e, vanguard, 0).Index)
	for i := 0; i < 5 && e.G.Obj(hulk).Zone == state.ZBattlefield; i++ {
		d := e.Pending()
		if d == nil || d.Kind == decision.KPriority {
			break
		}
		picked := false
		for _, o := range d.Options {
			if o.Obj == hulk {
				submitChoices(t, e, o.Index)
				picked = true
				break
			}
		}
		if !picked {
			t.Fatalf("sacrifice ask %+v offers no Thunderhulk", d)
		}
	}
	if o := e.G.Obj(hulk); o.Zone != state.ZGraveyard || e.Power(hulk) != 0 {
		t.Fatalf("precondition: hulk zone %v power %d, want a 0/0 card in the graveyard", o.Zone, e.Power(hulk))
	}
	passUntilStackEmpty(t, e, 40)
	if got := gnomeTokens(e); got != 3 {
		t.Fatalf("Gnome tokens %d, want 3: X is the Thunderhulk's last-known power (CR 608.2h)", got)
	}
	replayCheck(t, e, cfg)
}
