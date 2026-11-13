package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestAzraOddsmakerEffectKeepsChosenCreature pins the Effect trigger whose
// ValidSource$ is Card.ChosenCardStrict: the chain's own ClearChosenCard$
// cleanup empties the source's chosen list, yet the registration keeps the
// creature chosen at creation (Forge copies the chosen cards onto the effect
// card). Combat damage from the chosen creature draws; from another does not.
func TestAzraOddsmakerEffectKeepsChosenCreature(t *testing.T) {
	t.Parallel()
	promise := card(t, "Name:ChosenPromise\nManaCost:U\nTypes:Sorcery\n"+
		"A:SP$ ChooseCard | Choices$ Creature | ChoiceZone$ Battlefield | SubAbility$ DBEffect\n"+
		"SVar:DBEffect:DB$ Effect | Triggers$ TrigDamage | SubAbility$ DBCleanup\n"+
		"SVar:TrigDamage:Mode$ DamageDone | ValidSource$ Card.ChosenCardStrict | ValidTarget$ Player | Execute$ TrigDraw | CombatDamage$ True\n"+
		"SVar:TrigDraw:DB$ Draw | Defined$ You | NumCards$ 2\n"+
		"SVar:DBCleanup:DB$ Cleanup | ClearChosenCard$ True\nOracle:x\n")
	bear := card(t, "Name:Bear\nManaCost:1\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	e := handEngine(t, promise)
	chosen := putOnBattlefield(t, e, 0, bear)
	other := putOnBattlefield(t, e, 0, bear)
	e.G.Players[0].Pool[state.MU] = 1
	e.askPriority(0)
	castFirst(t, e, "cast")
	for i := 0; i < 8 && len(e.G.Stack) > 0; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatal("no decision while resolving")
		}
		if d.Kind == decision.KChoose && d.ResumeKind != "" && len(d.Options) > 0 {
			answerAsk(t, e, optionIdxForObj(t, d, chosen))
			continue
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{0}}); err != nil {
			t.Fatalf("submit: %v", err)
		}
	}
	if len(e.G.Delayed) != 1 || e.G.Delayed[0].EventMode != "DamageDone" {
		t.Fatalf("precondition: Effect did not arm a DamageDone trigger: %+v", e.G.Delayed)
	}
	if got := e.G.Obj(e.G.Delayed[0].Source).Chosen; len(got) != 0 {
		t.Fatalf("precondition: the chain's ClearChosenCard$ left %v chosen", got)
	}
	hit := func(src state.ObjID) int {
		before := len(e.G.Zone(state.ZHand, 0))
		e.damaging, e.combatDamaging = src, true
		e.emit(events.Event{Kind: events.Damage, Player: 1, Amount: 1})
		e.damaging, e.combatDamaging = 0, false
		e.putTriggersOnStack()
		passUntilStackEmpty(t, e, 8)
		return len(e.G.Zone(state.ZHand, 0)) - before
	}
	if got := hit(other); got != 0 {
		t.Fatalf("an unchosen creature's combat damage drew %d cards, want 0", got)
	}
	if got := hit(chosen); got != 2 {
		t.Fatalf("the chosen creature's combat damage drew %d cards, want 2", got)
	}
}
