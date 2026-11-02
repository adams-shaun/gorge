package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// lb-copyperm-sel: a trigger body's own Cost$ Blight<1>, paid in the
// triggered-cost window, records the blighted creature as the body's
// "CounterPut" paid list, and `Defined$ CounterPutCards` copies it. The
// fixture is Grub, Notorious Auntie's attack trigger on a front face
// (the real card is a transformed back face this board cannot reach directly).
func TestBlightPaidListFeedsCounterPutCards(t *testing.T) {
	reg := searchTestRegistry(t)
	auntie := "Name:Fixture Auntie\nManaCost:2 B\nTypes:Creature Goblin Warrior\nPT:2/1\n" +
		"T:Mode$ Attacks | ValidCard$ Card.Self | Execute$ OnCopy | TriggerZones$ Battlefield | TriggerDescription$ blight copy\n" +
		"SVar:OnCopy:AB$ CopyPermanent | Cost$ Blight<1> | TokenTapped$ True | TokenAttacking$ True | Defined$ CounterPutCards | AtEOTTrig$ Sacrifice\n" +
		"Oracle:x\n"
	victim := "Name:Fixture Victim\nManaCost:2 G\nTypes:Creature Elf\nPT:3/3\nOracle:x\n"
	e, cfg := combatTriggerBoard(t, reg, nil, []string{auntie, victim}, nil, nil)
	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: 2})
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepDeclareAttackers})
	src := findBattlefield(t, e, 0, "Fixture Auntie", 0)
	target := findBattlefield(t, e, 0, "Fixture Victim", 0)
	if o := e.G.Obj(target); o == nil || o.Zone != state.ZBattlefield || !o.EffectiveIsCreature() {
		t.Fatalf("precondition: blight candidate %+v is not a battlefield creature", o)
	}

	e.askAttackers()
	submitAttackersOnly(t, e, src)
	payIndex, _ := triggerCostWindowAsk(t, e)
	if payIndex < 0 {
		t.Fatalf("Blight<1> not offered as payable: %+v", e.Pending())
	}
	submitChoices(t, e, payIndex)
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("expected the blight creature choice, got %+v", d)
	}
	pick := -1
	for _, o := range d.Options {
		if o.Obj == target {
			pick = o.Index
		}
	}
	if pick < 0 {
		t.Fatalf("victim %d not offered for blight: %+v", target, d.Options)
	}
	submitChoices(t, e, pick)
	passUntilStackEmpty(t, e, 30)

	if got := e.G.Obj(target).Counter("M1M1"); got != 1 {
		t.Fatalf("precondition: blighted creature has %d -1/-1 counters, want 1", got)
	}
	var copies, auntieCopies int
	for i := range e.G.Objs {
		o := &e.G.Objs[i]
		if !o.IsToken || o.Zone != state.ZBattlefield || o.Face() == nil {
			continue
		}
		switch o.Face().Name {
		case "Fixture Victim":
			copies++
			if !o.Tapped {
				t.Error("the copy of the blighted creature entered untapped")
			}
		case "Fixture Auntie":
			auntieCopies++
		}
	}
	if copies != 1 || auntieCopies != 0 {
		var notes []string
		for _, ev := range e.L.Events {
			if ev.Kind == events.Note {
				notes = append(notes, ev.Text)
			}
		}
		t.Fatalf("copies of the blighted creature = %d, of the source = %d; want 1 and 0; notes %q", copies, auntieCopies, notes)
	}
	replayCheck(t, e, cfg)
}
