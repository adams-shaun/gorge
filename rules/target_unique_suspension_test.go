package rules

// The TargetUnique$ accumulator ("every target this resolution chooses must be
// different", CR 601.2c) must ride EVERY suspension, not only the shared
// mid-resolution target pre-ask. These carriers put an intervening ask of a
// DIFFERENT kind -- the row names a dig/scry/arrange ask -- between two
// TargetUnique$ riders: the resumed Ctx rebuilt from the pending ask's ride
// and, before this ticket, an ask that did not stamp that ride dropped the
// accumulator, so the later rider re-offered the earlier rider's pick.
//
// The target-bearing SA lines are the corpus TargetUnique$ parameter
// spellings (see target_unique_test.go's chainUniqueScript); the intervening
// Dig line is dig_ask_test.go's minimal shape. Only ManaCost is simplified so
// the fixture funds with one colour.

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
)

// digBetweenRidersScript is a root Draw followed by two TargetUnique$ player
// riders with a Dig -- a "dig"-kind KChoose suspension, an ask kind OUTSIDE
// the shared target tail -- parked between them.
func digBetweenRidersScript() string {
	return "Name:Dig Between Riders\nManaCost:1\nTypes:Sorcery\n" +
		"A:SP$ Draw | NumCards$ 1 | SubAbility$ R1\n" +
		"SVar:R1:DB$ Draw | NumCards$ 0 | ValidTgts$ Player | TargetUnique$ True | TargetMin$ 0 | TargetMax$ 1 | SubAbility$ R2\n" +
		"SVar:R2:DB$ Dig | Defined$ You | DigNum$ 3 | ChangeNum$ 1 | SubAbility$ R3\n" +
		"SVar:R3:DB$ Draw | NumCards$ 0 | ValidTgts$ Player | TargetUnique$ True | TargetMin$ 0 | TargetMax$ 1\n" +
		"Oracle:x\n"
}

// passPriorityUntilNonPriority answers "pass" to every priority decision until
// a non-priority ask is pending (the same drain the sibling TargetUnique$
// tests inline). It fatals if the stack never reaches an ask.
func passPriorityUntilNonPriority(t *testing.T, e *Engine) *decision.Decision {
	t.Helper()
	for i := 0; i < 30; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatal("no decision while awaiting an ask")
		}
		if d.Kind != decision.KPriority {
			return d
		}
		for _, o := range d.Options {
			if o.Kind == "pass" {
				submitChoices(t, e, o.Index)
				break
			}
		}
	}
	t.Fatal("no non-priority ask after 30 priority passes")
	return nil
}

// TestTargetUniqueSurvivesADifferentAskKindBetweenRiders pins the row's
// remaining gap: a Dig (ResumeKind "dig") parked between two TargetUnique$
// riders must not erase the first rider's pick. Pre-fix the Dig's decision
// carried no accumulator, so the resumed Ctx rebuilt it empty and the third
// ask over-offered seat 0.
func TestTargetUniqueSurvivesADifferentAskKindBetweenRiders(t *testing.T) {
	e, cfg, _ := newFixtureDeck(t, 7008, digBetweenRidersScript())
	addMana(t, e, 0, "C")
	castFirst(t, e, "cast")

	// Rider 1's cast-time TargetUnique$ ask: both players, pick seat 0.
	d := passPriorityUntilNonPriority(t, e)
	if d == nil || d.Kind != decision.KTarget || d.ResumeKind != "cast_sub" {
		t.Fatalf("first rider ask = %+v, want KTarget cast_sub", d)
	}
	first := pendingPlayerIDs(t, e)
	if !first[0] || !first[1] {
		t.Fatalf("precondition: first rider should offer BOTH differing players: %+v", d.Options)
	}
	submitChoices(t, e, 0)

	// The third rider is now asked at announcement too, before Dig resolves.
	d3 := e.Pending()
	if d3 == nil || d3.Kind != decision.KTarget || d3.ResumeKind != "cast_sub" {
		t.Fatalf("third rider ask = %+v, want KTarget cast_sub", d3)
	}
	third := pendingPlayerIDs(t, e)
	if third[0] || !third[1] {
		t.Fatalf("third rider must offer only the other player: %+v", d3.Options)
	}
	submitChoices(t, e, 0)

	// The intervening DIG ask now occurs during resolution.
	dig := passPriorityUntilNonPriority(t, e)
	if dig == nil || dig.Kind != decision.KChoose || dig.ResumeKind != "dig" {
		t.Fatalf("intervening ask = %+v, want the Dig KChoose (resume kind \"dig\")", dig)
	}
	if len(dig.Options) < 2 {
		t.Fatalf("precondition: the Dig must offer a real choice, got %d options: %+v", len(dig.Options), dig.Options)
	}
	submitChoices(t, e, 0)

	// The Dig's remaining window cards offer their own bottom order (the
	// "dig_arrange" KArrange). Answer it in the offered order before the
	// third rider, so the tail is reached.
	if arr := e.Pending(); arr != nil && arr.Kind == decision.KArrange {
		submitArrange(t, e, arr, nil)
	}

	// The answered target must not be re-asked after Dig suspends.
	passUntilStackEmpty(t, e, 30)
	if len(e.G.Stack) != 0 {
		t.Fatalf("stack not empty after resolution: %d", len(e.G.Stack))
	}
	replayCheck(t, e, cfg)
}

// scryBetweenRidersScript is the same shape with a Scry (the "arrange" ask
// kind the row names) between the riders, so the class is covered for a
// second ask kind than Dig.
func scryBetweenRidersScript() string {
	return "Name:Scry Between Riders\nManaCost:1\nTypes:Sorcery\n" +
		"A:SP$ Draw | NumCards$ 1 | SubAbility$ R1\n" +
		"SVar:R1:DB$ Draw | NumCards$ 0 | ValidTgts$ Player | TargetUnique$ True | TargetMin$ 0 | TargetMax$ 1 | SubAbility$ R2\n" +
		"SVar:R2:DB$ Scry | ScryNum$ 2 | SubAbility$ R3\n" +
		"SVar:R3:DB$ Draw | NumCards$ 0 | ValidTgts$ Player | TargetUnique$ True | TargetMin$ 0 | TargetMax$ 1\n" +
		"Oracle:x\n"
}

// TestTargetUniqueSurvivesAScryBetweenRiders is the arrange-kind twin of the
// Dig test.
func TestTargetUniqueSurvivesAScryBetweenRiders(t *testing.T) {
	e, cfg, _ := newFixtureDeck(t, 7009, scryBetweenRidersScript())
	addMana(t, e, 0, "C")
	castFirst(t, e, "cast")

	d := passPriorityUntilNonPriority(t, e)
	if d == nil || d.Kind != decision.KTarget || d.ResumeKind != "cast_sub" {
		t.Fatalf("first rider ask = %+v, want KTarget cast_sub", d)
	}
	if !pendingPlayerIDs(t, e)[0] || !pendingPlayerIDs(t, e)[1] {
		t.Fatalf("precondition: first rider should offer BOTH differing players: %+v", d.Options)
	}
	submitChoices(t, e, 0)

	d3 := e.Pending()
	if d3 == nil || d3.Kind != decision.KTarget || d3.ResumeKind != "cast_sub" {
		t.Fatalf("third rider ask = %+v, want KTarget cast_sub", d3)
	}
	third := pendingPlayerIDs(t, e)
	if third[0] || !third[1] {
		t.Fatalf("third rider must offer only the other player: %+v", d3.Options)
	}
	submitChoices(t, e, 0)

	arrange := passPriorityUntilNonPriority(t, e)
	if arrange == nil || arrange.Kind != decision.KArrange {
		t.Fatalf("intervening ask = %+v, want the Scry KArrange", arrange)
	}
	submitArrange(t, e, arrange, nil)

	// The answered target must not be re-asked after Scry suspends.
	passUntilStackEmpty(t, e, 30)
	if len(e.G.Stack) != 0 {
		t.Fatalf("stack not empty after resolution: %d", len(e.G.Stack))
	}
	replayCheck(t, e, cfg)
}

// submitArrange answers a KArrange by keeping `top` (option indices) on top in
// the given order; a nil top keeps every offered card on top in the offered
// order. The answer must satisfy the decision's own Min/Max, so nil fills
// every option.
func submitArrange(t *testing.T, e *Engine, d *decision.Decision, top []int) {
	t.Helper()
	if top == nil {
		top = make([]int, len(d.Options))
		for i := range d.Options {
			top[i] = d.Options[i].Index
		}
	}
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: top}); err != nil {
		t.Fatalf("submit arrange: %v", err)
	}
}
