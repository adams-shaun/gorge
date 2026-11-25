package rules

// Decree of Savagery: "When you cycle this card, you MAY put four +1/+1
// counters on target creature." CR 603.5: an optional trigger's controller
// chooses at resolution whether its effect happens. The corpus T: line
// carries OptionalDecider$ You (the fork fix); this pins that the Cycled
// trigger poses a KTriggerOptional election at resolution (after the target is chosen), that
// "no" leaves the chosen creature alone and that "yes" puts four +1/+1
// counters on it (the xproof cluster follower-only:xmage:decree-of-savagery
// put the counters on unconditionally because the script had no decider).

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// decreeCycle cycles the real corpus Decree of Savagery with a creature on
// seat 1's board (the divergent game targeted the opponent's Kobolds),
// chooses that creature when the trigger is put on the stack (CR 603.3d) and
// returns the KTriggerOptional election posed at resolution (CR 603.5) plus
// the target creature. It fails if the trigger resolves as mandatory.
func decreeCycle(t *testing.T) (*Engine, *decision.Decision, state.ObjID) {
	t.Helper()
	e := handEngine(t, corpusAlternativeCard(t, "Decree of Savagery"))
	kobold := onBoard(t, e, 1, "Name:Kobold\nManaCost:R\nTypes:Creature Kobold\nPT:0/1\nOracle:x\n")
	id := e.G.Zone(state.ZHand, 0)[0]
	if o := e.G.Obj(id); o == nil || o.Face() == nil || o.Face().Name != "Decree of Savagery" {
		t.Fatalf("precondition: hand card %d is not Decree of Savagery", id)
	}
	if o := e.G.Obj(kobold); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: target creature %d is not on the battlefield", kobold)
	}

	addMana(t, e, 0, "CCCCGG") // Cycling {4}{G}{G}
	e.beginActivation(0, cyclingAbilityOption(t, e, id))

	targeted := false
	for i := 0; i < 30; i++ {
		d := e.Pending()
		if d == nil {
			if len(e.G.Stack) == 0 {
				t.Fatal("the Cycled trigger resolved without posing KTriggerOptional")
			}
			e.Advance()
			continue
		}
		switch d.Kind {
		case decision.KTriggerOptional:
			if !targeted {
				t.Fatalf("election posed before the target was chosen: %+v", d)
			}
			if d.Player != 0 || len(d.Options) != 2 {
				t.Fatalf("optional election = %+v, want seat 0's yes/no", d)
			}
			return e, d, kobold
		case decision.KTarget:
			idx := -1
			for _, o := range d.Options {
				if o.Obj == kobold {
					idx = o.Index
				}
			}
			if idx < 0 {
				t.Fatalf("target options %+v do not include the kobold %d", d.Options, kobold)
			}
			submitChoices(t, e, idx)
			targeted = true
		case decision.KPriority:
			if got := e.G.Obj(kobold).Counter("P1P1"); got != 0 {
				t.Fatalf("Decree put %d counters on the creature without an election", got)
			}
			passOnce(t, e)
		default:
			t.Fatalf("unexpected decision %+v", d)
		}
	}
	t.Fatal("KTriggerOptional never posed (the trigger resolved as mandatory)")
	return nil, nil, 0
}

func TestDecreeOfSavageryCycleTriggerIsOptional(t *testing.T) {
	t.Parallel()
	t.Run("no", func(t *testing.T) {
		t.Parallel()
		e, d, kobold := decreeCycle(t)
		submitChoices(t, e, d.Options[1].Index)
		passUntilStackEmpty(t, e, 20)
		if got := e.G.Obj(kobold).Counter("P1P1"); got != 0 {
			t.Fatalf("declining still put %d +1/+1 counters on the creature, want 0", got)
		}
	})
	t.Run("yes", func(t *testing.T) {
		t.Parallel()
		e, d, kobold := decreeCycle(t)
		submitChoices(t, e, d.Options[0].Index)
		passUntilStackEmpty(t, e, 20)
		if got := e.G.Obj(kobold).Counter("P1P1"); got != 4 {
			t.Fatalf("accepting put %d +1/+1 counters on the creature, want 4", got)
		}
	})
}
