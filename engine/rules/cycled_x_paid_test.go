package rules

// CR 702.29d / 107.3i: a Mode$ Cycled trigger's Count$xPaid is the X paid for
// the cycling ABILITY, not the discarded card's own X (the card was discarded
// as the ability's cost and never carried the {X}). The two tests below pin
// that binding end to end: once against a real corpus card's target filter
// (Webstrike Elite), and once against a synthetic body that reads
// Count$xPaid directly.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// cyclingAbilityOption returns the "ability" option that activates id from
// hand (the cycling keyword's option).
func cyclingAbilityOption(t *testing.T, e *Engine, id state.ObjID) decision.Option {
	t.Helper()
	for _, o := range e.legalActions(0) {
		if o.Kind == "ability" && o.Obj == id {
			return o
		}
	}
	t.Fatalf("cycling activation not offered for %d: %+v", id, e.legalActions(0))
	return decision.Option{}
}

// answerX asserts the pending decision is the {X} ask and answers it.
func answerX(t *testing.T, e *Engine, x int32) {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || len(d.Options) == 0 || d.Options[0].Kind != "x" {
		t.Fatalf("X decision %+v", d)
	}
	offered := false
	for _, o := range d.Options {
		if o.Kind == "x" && int32(o.Amount) == x {
			offered = true
		}
	}
	if !offered {
		t.Fatalf("X = %d not offered: %+v", x, d.Options)
	}
	submitChoices(t, e, int(x))
}

// TestCycledXPaidFeedsTheCycledTriggerTarget cycles the real corpus card
// Webstrike Elite for two different X values. Its trigger destroys "up to one
// target artifact or enchantment with mana value X", so the target ask itself
// proves the paid X reached the trigger's evaluation: at X=1 only the one-drop
// is offered, at X=3 only the three-drop, and exactly that one is destroyed.
func TestCycledXPaidFeedsTheCycledTriggerTarget(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		x    int32
		mana string
	}{
		{"X=1", 1, "CGG"},
		{"X=3", 3, "CCCGG"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			elite := mshCorpusCard(t, "Webstrike Elite")
			e := handEngine(t, elite)
			relicOne := onBoard(t, e, 0, "Name:Relic One\nManaCost:1\nTypes:Artifact\nOracle:x\n")
			relicThree := onBoard(t, e, 0, "Name:Relic Three\nManaCost:3\nTypes:Artifact\nOracle:x\n")
			if e.G.Obj(relicOne).Zone != state.ZBattlefield || e.G.Obj(relicThree).Zone != state.ZBattlefield {
				t.Fatal("precondition: relics are not on the battlefield")
			}
			mvOne, mvThree := e.G.Obj(relicOne).Face().ManaValue(), e.G.Obj(relicThree).Face().ManaValue()
			if mvOne != 1 || mvThree != 3 || mvOne == mvThree {
				t.Fatalf("precondition: relic mana values %d/%d, want 1/3", mvOne, mvThree)
			}
			if hand := e.G.Zone(state.ZHand, 0); len(hand) != 1 {
				t.Fatalf("precondition: hand = %v, want exactly the Elite", hand)
			}
			id := e.G.Zone(state.ZHand, 0)[0]
			if o := e.G.Obj(id); o == nil || o.Face() == nil || o.Face().Name != "Webstrike Elite" {
				t.Fatalf("precondition: hand card %d is not Webstrike Elite", id)
			}

			addMana(t, e, 0, tc.mana) // {X}{G}{G}
			e.beginActivation(0, cyclingAbilityOption(t, e, id))
			answerX(t, e, tc.x)

			// The cycle cost's discard fires the trigger and the trigger's
			// placement ask is posed before priority.
			if e.Pending() == nil {
				e.Advance()
			}
			d := e.Pending()
			if d == nil || d.Kind != decision.KTarget {
				t.Fatalf("trigger target decision %+v, want KTarget", d)
			}
			want := relicOne
			if tc.x == 3 {
				want = relicThree
			}
			if len(d.Options) != 1 || d.Options[0].Obj != want {
				t.Fatalf("target options %+v, want exactly the mana-value-%d relic", d.Options, tc.x)
			}
			submitChoices(t, e, d.Options[0].Index)
			passUntilStackEmpty(t, e, 20)

			if got := e.G.Obj(want).Zone; got != state.ZGraveyard {
				t.Fatalf("matching relic zone = %s, want graveyard", got)
			}
			other := relicThree
			if tc.x == 3 {
				other = relicOne
			}
			if got := e.G.Obj(other).Zone; got != state.ZBattlefield {
				t.Fatalf("other relic zone = %s, want battlefield", got)
			}
		})
	}
}

// TestCycledXPaidFeedsTheTriggerBody pins the same binding on a synthetic
// card whose trigger body reads Count$xPaid directly: the cycling ability
// draws its own card, then the trigger draws X more, so the final hand size
// is 1+X.
func TestCycledXPaidFeedsTheTriggerBody(t *testing.T) {
	t.Parallel()
	const src = "Name:Cycle X\nManaCost:U\nTypes:Creature\nPT:1/1\n" +
		"K:Cycling:X U\nOracle:x\n" +
		"T:Mode$ Cycled | ValidCard$ Card.Self | Execute$ TrigDraw | TriggerDescription$ When you cycle CARDNAME, draw X cards.\n" +
		"SVar:TrigDraw:DB$ Draw | NumCards$ X\n" +
		"SVar:X:Count$xPaid\n"
	cases := []struct {
		name string
		x    int32
		mana string
	}{
		{"X=1", 1, "CU"},
		{"X=3", 3, "CCCU"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			e := handEngine(t, card(t, src))
			if hand := e.G.Zone(state.ZHand, 0); len(hand) != 1 {
				t.Fatalf("precondition: hand = %v, want exactly the cycler", hand)
			}
			id := e.G.Zone(state.ZHand, 0)[0]

			addMana(t, e, 0, tc.mana) // {X}{U}
			e.beginActivation(0, cyclingAbilityOption(t, e, id))
			answerX(t, e, tc.x)
			passUntilStackEmpty(t, e, 20)

			// 1 card in hand, minus the cycled card, plus the cycling
			// ability's own draw, plus the trigger's X draw.
			if got := len(e.G.Zone(state.ZHand, 0)); got != 1+int(tc.x) {
				t.Fatalf("hand = %d, want %d (cycling draw 1 + trigger draw X=%d)", got, 1+int(tc.x), tc.x)
			}
		})
	}
}
