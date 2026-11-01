package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// A modal triggered ability whose chosen mode's targets all became illegal
// does not resolve (CR 608.2b), so none of the mode's untargeted riders run.
// gorge rechecked the Charm root, whose own ValidTgts$ is empty, and resolved
// Kykar, Zephyr Awakener's exile mode after its target had left (kernel
// lockstep pair 726: an empty delayed return was registered).
func TestModalTriggerWithAllTargetsIllegalFizzles(t *testing.T) {
	const kykarLike = "Name:Modal Bird\nManaCost:2 W U\nTypes:Creature Bird\nPT:3/4\n" +
		"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigCharm\n" +
		"SVar:TrigCharm:DB$ Charm | Choices$ DBBlink,DBGain\n" +
		"SVar:DBBlink:DB$ ChangeZone | ValidTgts$ Creature.YouCtrl+Other | TgtPrompt$ Select another target creature you control | Origin$ Battlefield | Destination$ Exile | SubAbility$ DBRider | SpellDescription$ Exile another target creature you control.\n" +
		"SVar:DBRider:DB$ GainLife | Defined$ You | LifeAmount$ 5\n" +
		"SVar:DBGain:DB$ GainLife | Defined$ You | LifeAmount$ 1 | SpellDescription$ Gain 1 life.\n" +
		"Oracle:x\n"
	// twoModes chooses two targeted modes, both of whose targets leave.
	const twoModes = "Name:Twin Modal Bird\nManaCost:2 W U\nTypes:Creature Bird\nPT:3/4\n" +
		"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigCharm\n" +
		"SVar:TrigCharm:DB$ Charm | CharmNum$ 2 | Choices$ DBTapA,DBTapB\n" +
		"SVar:DBTapA:DB$ Tap | ValidTgts$ Creature.YouCtrl+Other | TgtPrompt$ Select target | SubAbility$ DBRider | SpellDescription$ Tap another target creature you control.\n" +
		"SVar:DBTapB:DB$ Tap | ValidTgts$ Creature.YouCtrl+Other | TgtPrompt$ Select target | SpellDescription$ Tap another target creature you control.\n" +
		"SVar:DBRider:DB$ GainLife | Defined$ You | LifeAmount$ 5\n" +
		"Oracle:x\n"
	const bear = "Name:Plain Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
	for _, script := range []string{kykarLike, twoModes} {
		e := mayPlayBase(t)
		victim := onBoardGrant(t, e, 0, bear)
		other := onBoardGrant(t, e, 0, bear)
		onBoardGrant(t, e, 0, script)
		e.priorityRound()
		life0 := e.G.Players[0].Life
		targeted := false
		for i := 0; i < 8; i++ {
			d := e.Pending()
			if d == nil || d.Kind == decision.KPriority {
				break
			}
			var picks []int
			usedGroup, usedObj := map[string]bool{}, map[state.ObjID]bool{}
			for _, o := range d.Options {
				if d.Kind == decision.KTarget && (o.Obj == victim || o.Obj == other) && len(picks) < d.Max &&
					!usedObj[o.Obj] && (o.Group == "" || !usedGroup[o.Group]) {
					picks = append(picks, o.Index)
					usedGroup[o.Group], usedObj[o.Obj] = true, true
					targeted = true
				}
			}
			if len(picks) == 0 {
				for i := 0; i < d.Min || i == 0; i++ {
					picks = append(picks, d.Options[i].Index)
				}
			}
			submitChoices(t, e, picks...)
		}
		if !targeted {
			t.Fatalf("the targeted mode never asked for its target; pending %+v", e.Pending())
		}
		// Every target leaves before the trigger resolves.
		e.emit(events.Event{Kind: events.MoveZone, Obj: victim, From: state.ZBattlefield, To: state.ZGraveyard})
		e.emit(events.Event{Kind: events.MoveZone, Obj: other, From: state.ZBattlefield, To: state.ZGraveyard})
		passUntilStackEmpty(t, e, 20)
		if got := e.G.Players[0].Life; got != life0 {
			t.Fatalf("%s: life %d -> %d: the modal trigger resolved with no legal target (its rider ran)", script[5:20], life0, got)
		}
	}
}
