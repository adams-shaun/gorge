package divergence

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
)

// TestMaterialTable walks the human-divergence design §5.5's materiality table: each row names
// the one rule that decides it, and asserts the precondition that rule
// reads (same names on distinct objects, differing plans, bare costs), so
// no row passes vacuously.
func TestMaterialTable(t *testing.T) {
	intents := func(cs ...int) decision.Intent {
		return decision.Intent{Choices: cs}
	}

	prioritySameNameLands := &decision.Decision{
		Seq:    1,
		Player: 0,
		Kind:   decision.KPriority,
		Prompt: "priority",
		Options: []decision.Option{
			{Index: 0, Kind: "activate", Label: "Mountain", Obj: 5, Player: 0},
			{Index: 1, Kind: "activate", Label: "Mountain", Obj: 9, Player: 0},
			{Index: 2, Kind: "pass", Label: "Pass"},
		},
	}
	priorityDiffNames := &decision.Decision{
		Seq:    2,
		Player: 0,
		Kind:   decision.KPriority,
		Prompt: "priority",
		Options: []decision.Option{
			{Index: 0, Kind: "activate", Label: "Mountain", Obj: 5, Player: 0},
			{Index: 1, Kind: "activate", Label: "Forest", Obj: 6, Player: 0},
			{Index: 2, Kind: "cast", Label: "Cast Lightning Bolt", Obj: 7, Player: 0},
			{Index: 3, Kind: "pass", Label: "Pass"},
		},
	}
	attackSameNames := &decision.Decision{
		Seq:    3,
		Player: 0,
		Kind:   decision.KAttackers,
		Prompt: "attackers",
		Max:    2,
		Options: []decision.Option{
			{Index: 0, Kind: "attack", Label: "Attack with Grizzly Bears", Obj: 20, Player: 0},
			{Index: 1, Kind: "attack", Label: "Attack with Grizzly Bears", Obj: 21, Player: 0},
			{Index: 2, Kind: "attack", Label: "Attack with Hill Giant", Obj: 22, Player: 0},
		},
	}
	attackDiffNames := &decision.Decision{
		Seq:    4,
		Player: 0,
		Kind:   decision.KAttackers,
		Prompt: "attackers",
		Max:    3,
		Options: []decision.Option{
			{Index: 0, Kind: "attack", Label: "Attack with Grizzly Bears", Obj: 20, Player: 0},
			{Index: 1, Kind: "attack", Label: "Attack with Hill Giant", Obj: 22, Player: 0},
			{Index: 2, Kind: "attack", Label: "Attack with Vigilant Giant", Obj: 23, Player: 0},
		},
	}
	blocks := &decision.Decision{
		Seq:    5,
		Player: 0,
		Kind:   decision.KBlockers,
		Prompt: "blockers",
		Options: []decision.Option{
			{Index: 0, Kind: "block", Label: "Block with Bears", Obj: 30, Attacker: 20, Player: 0},
			{Index: 1, Kind: "block", Label: "Block with Giant", Obj: 31, Attacker: 20, Player: 0},
			{Index: 2, Kind: "block", Label: "Block with Ogre", Obj: 32, Attacker: 20, Player: 0},
		},
	}
	targets := &decision.Decision{
		Seq:    6,
		Player: 0,
		Kind:   decision.KTarget,
		Prompt: "choose a target",
		Min:    1,
		Max:    1,
		Options: []decision.Option{
			{Index: 0, Kind: "creature", Label: "Grizzly Bears", Obj: 40, Player: 1},
			{Index: 1, Kind: "creature", Label: "Hill Giant", Obj: 41, Player: 1},
		},
	}
	modes := &decision.Decision{
		Seq:    7,
		Player: 0,
		Kind:   decision.KModes,
		Prompt: "choose modes",
		Min:    1,
		Max:    2,
		Options: []decision.Option{
			{Index: 0, Kind: "mode", Label: "Deal 2 damage"},
			{Index: 1, Kind: "mode", Label: "Draw a card"},
		},
	}
	mulligans := &decision.Decision{
		Seq:    8,
		Player: 0,
		Kind:   decision.KMulligan,
		Prompt: "keep or mulligan",
		Options: []decision.Option{
			{Index: 0, Kind: "card", Label: "Mountain", Obj: 5, Player: 0},
			{Index: 1, Kind: "card", Label: "Island", Obj: 6, Player: 0},
		},
	}
	arrange := &decision.Decision{
		Seq:    9,
		Player: 0,
		Kind:   decision.KArrange,
		Prompt: "arrange the top of your library",
		Min:    1,
		Max:    2,
		Options: []decision.Option{
			{Index: 0, Kind: "top", Label: "Grizzly Bears", Obj: 50, Player: 0},
			{Index: 1, Kind: "top", Label: "Hill Giant", Obj: 51, Player: 0},
		},
	}

	// A priority intent that pays through the payment extension instead of
	// a choice index.
	payIntent := func(id, planID string) decision.Intent {
		return decision.Intent{Payment: &decision.PaymentSelection{
			ActionID: id,
			Plan:     decision.PaymentPlan{Version: 1, ID: planID},
		}}
	}

	t.Run("same choices are never material", func(t *testing.T) {
		if ok, reason := Material(priorityDiffNames, intents(0, 2), intents(0, 2)); ok {
			t.Fatalf("identical answers judged material (%s)", reason)
		}
		// Unordered kind: the same set in a different offer order.
		if ok, reason := Material(priorityDiffNames, intents(0, 2), intents(2, 0)); ok {
			t.Fatalf("same set, reordered, judged material on an unordered kind (%s)", reason)
		}
	})

	t.Run("two same-name lands are equivalent", func(t *testing.T) {
		if prioritySameNameLands.Options[0].Label != prioritySameNameLands.Options[1].Label ||
			prioritySameNameLands.Options[0].Obj == prioritySameNameLands.Options[1].Obj {
			t.Fatalf("precondition lost: options must share a name and differ in object")
		}
		if ok, reason := Material(prioritySameNameLands, intents(0), intents(1)); ok {
			t.Fatalf("left Mountain vs right Mountain judged material (%s)", reason)
		}
		// The same equivalence on a non-mana kind, where the bare-mana rule
		// cannot be what passes it: attacking with the left Bears instead
		// of the right one.
		if attackSameNames.Options[0].Label != attackSameNames.Options[1].Label ||
			attackSameNames.Options[0].Obj == attackSameNames.Options[1].Obj {
			t.Fatalf("precondition lost: attack options must share a name and differ in object")
		}
		if ok, reason := Material(attackSameNames, intents(0), intents(1)); ok {
			t.Fatalf("left Bears vs right Bears judged material (%s)", reason)
		}
	})

	t.Run("payment-only difference is equivalent", func(t *testing.T) {
		a := payIntent("p1", "plan-a")
		b := payIntent("p1", "plan-b")
		if a.Payment.ActionID != b.Payment.ActionID {
			t.Fatalf("precondition lost: the two answers must name the same action")
		}
		if a.Payment.Plan.ID == b.Payment.Plan.ID {
			t.Fatalf("precondition lost: the two plans must differ for this row")
		}
		if ok, reason := Material(priorityDiffNames, a, b); ok {
			t.Fatalf("plan-only difference judged material (%s)", reason)
		}
		// A different chosen action is material even though neither answer
		// carries choices.
		if ok, _ := Material(priorityDiffNames, payIntent("p1", "p"), payIntent("p2", "p")); !ok {
			t.Fatalf("different payment actions judged equivalent")
		}
	})

	t.Run("bare mana sources are not material", func(t *testing.T) {
		d := priorityDiffNames
		// Precondition: the two answers tap differently NAMED sources, so
		// the bare-mana rule (not name equivalence) is what passes them.
		if d.Options[0].Label == d.Options[1].Label {
			t.Fatalf("precondition lost: the two sources must differ in name")
		}
		if d.Options[0].Kind != "activate" || d.Options[0].Cost != "" ||
			d.Options[1].Kind != "activate" || d.Options[1].Cost != "" {
			t.Fatalf("precondition lost: both answers must be bare mana sources")
		}
		if ok, reason := Material(d, intents(0), intents(1)); ok {
			t.Fatalf("two bare mana taps judged material (%s)", reason)
		}
		// A bare tap against a cast is material; neither side passes, so
		// the reason is "other", not cast-vs-pass.
		if ok, reason := Material(d, intents(0), intents(2)); !ok {
			t.Fatalf("tap vs cast judged equivalent")
		} else if reason != MaterialOther {
			t.Fatalf("tap vs cast reason = %q, want other", reason)
		}
	})

	t.Run("different target is material", func(t *testing.T) {
		if targets.Options[0].Label == targets.Options[1].Label {
			t.Fatalf("precondition lost: the two targets must differ in name")
		}
		ok, reason := Material(targets, intents(0), intents(1))
		if !ok {
			t.Fatalf("different targets judged equivalent")
		}
		if reason != MaterialTarget {
			t.Fatalf("reason = %q, want target", reason)
		}
	})

	t.Run("attack subset is material attack-set", func(t *testing.T) {
		if ok, reason := Material(attackDiffNames, intents(2), intents(0, 2)); !ok {
			t.Fatalf("attack subset judged equivalent")
		} else if reason != MaterialAttackSet {
			t.Fatalf("reason = %q, want attack-set", reason)
		}
	})

	t.Run("cast versus pass is material", func(t *testing.T) {
		ok, reason := Material(priorityDiffNames, intents(), intents(2))
		if !ok {
			t.Fatalf("pass vs cast judged equivalent")
		}
		if reason != MaterialCastVsPass {
			t.Fatalf("reason = %q, want cast-vs-pass", reason)
		}
		// Payment extension against a pass reads the same way.
		if ok, reason := Material(priorityDiffNames, intents(), payIntent("p1", "p")); !ok || reason != MaterialCastVsPass {
			t.Fatalf("pass vs payment action: ok=%v reason=%q, want true cast-vs-pass", ok, reason)
		}
	})

	t.Run("block and mode reasons", func(t *testing.T) {
		if ok, reason := Material(blocks, intents(0), intents(0, 1)); !ok || reason != MaterialBlockSet {
			t.Fatalf("block set difference: ok=%v reason=%q, want true block-set", ok, reason)
		}
		if ok, reason := Material(modes, intents(0), intents(1)); !ok || reason != MaterialMode {
			t.Fatalf("mode difference: ok=%v reason=%q, want true mode", ok, reason)
		}
		if ok, reason := Material(mulligans, intents(0), intents(1)); !ok || reason != MaterialOther {
			t.Fatalf("mulligan difference: ok=%v reason=%q, want true other", ok, reason)
		}
	})

	t.Run("arrange answers compare as sequences", func(t *testing.T) {
		// Same set, same order: equivalent.
		a := decision.Intent{Choices: []int{0, 1}, Rest: []int{}}
		b := decision.Intent{Choices: []int{0, 1}, Rest: []int{}}
		if ok, _ := Material(arrange, a, b); ok {
			t.Fatalf("identical arrange answers judged material")
		}
		// Same set, different order: material.
		c := decision.Intent{Choices: []int{1, 0}}
		if ok, reason := Material(arrange, a, c); !ok {
			t.Fatalf("reordered arrange judged equivalent")
		} else if reason != MaterialOther {
			t.Fatalf("reason = %q, want other", reason)
		}
		// Same named options at the same positions collapse (two same-named
		// cards stacked in a different physical order).
		sameName := &decision.Decision{
			Seq:    10,
			Player: 0,
			Kind:   decision.KArrange,
			Prompt: "arrange",
			Min:    1,
			Max:    2,
			Options: []decision.Option{
				{Index: 0, Kind: "top", Label: "Mountain", Obj: 60, Player: 0},
				{Index: 1, Kind: "top", Label: "Mountain", Obj: 61, Player: 0},
			},
		}
		if sameName.Options[0].Label != sameName.Options[1].Label {
			t.Fatalf("precondition lost: arrange options must share a name")
		}
		if ok, _ := Material(sameName, decision.Intent{Choices: []int{0, 1}}, decision.Intent{Choices: []int{1, 0}}); ok {
			t.Fatalf("reordered same-name arrange judged material")
		}
	})

	t.Run("nil decision is a no-op", func(t *testing.T) {
		if ok, _ := Material(nil, intents(0), intents(1)); ok {
			t.Fatalf("nil decision judged material")
		}
	})

	t.Run("distinguishing fields break name equivalence", func(t *testing.T) {
		// Same name, but one option needs a different player (a redirected
		// or multiplayer choice) and one carries an amount: neither may
		// collapse into the other.
		d := &decision.Decision{
			Seq:    11,
			Player: 0,
			Kind:   decision.KTarget,
			Prompt: "choose a player",
			Min:    1,
			Max:    1,
			Options: []decision.Option{
				{Index: 0, Kind: "player", Label: "Opponent", Player: 1},
				{Index: 1, Kind: "player", Label: "Opponent", Player: 2},
				{Index: 2, Kind: "draw", Label: "Draw two", Amount: 2},
				{Index: 3, Kind: "draw", Label: "Draw two", Amount: 3},
			},
		}
		if ok, _ := Material(d, intents(0), intents(1)); !ok {
			t.Fatalf("same name for different players judged equivalent")
		}
		if ok, _ := Material(d, intents(2), intents(3)); !ok {
			t.Fatalf("same name with different amounts judged equivalent")
		}
		if ok, _ := Material(d, intents(2), intents(2)); ok {
			t.Fatalf("identical amount option judged material")
		}
		if d.Options[0].Player == d.Options[1].Player {
			t.Fatalf("precondition lost: the two player options must name different players")
		}
	})
}
