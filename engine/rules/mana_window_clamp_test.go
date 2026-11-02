package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// The live Fiend Artisan shape (20000-game census seeds
// 15921437281116854304 / 3131652697237171831): the payer announced {B}{B}
// hybrid faces, then the CR 601.2g payment window offered EVERY mana source
// and the colour ask offered EVERY colour; the deterministic first-option
// bot fed a {W} into {B}{B}, stranded the exact payment, and the cast
// legally reversed with "cast aborted: cost no longer payable".
//
// CR 601.2g: inside the payment window the added mana exists only to pay
// the announced composition, so a source (or colour) that can absorb
// nothing is withheld. Both tests assert the payment completes with no
// abort Note.
const hybridBeastSrc = "Name:Hybrid Beast\nManaCost:BG BG\nTypes:Creature Beast\nPT:2/2\nOracle:x\n"
const greenSourceSrc = "Name:Green Rock\nTypes:Land\nA:AB$ Mana | Cost$ T | Produced$ G\nOracle:x\n"
const anySourceSrc = "Name:Any Rock\nTypes:Creature\nPT:1/1\nA:AB$ Mana | Cost$ T | Produced$ Any\nOracle:x\n"

// submitIdx answers d with the option at idx.
func submitIdx(t *testing.T, e *Engine, d *decision.Decision, idx int) {
	t.Helper()
	if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}); err != nil {
		t.Fatalf("submit %q: %v", d.Prompt, err)
	}
}

// findOpt returns the index of d's first option whose label has prefix.
func findOpt(t *testing.T, d *decision.Decision, prefix string) int {
	t.Helper()
	for _, o := range d.Options {
		if strings.HasPrefix(o.Label, prefix) {
			return o.Index
		}
	}
	t.Fatalf("no option %q in %+v", prefix, d.Options)
	return -1
}

// hybridCast drives a hand-built {B/G}{B/G} cast of Hybrid Beast through
// both pip announcements, returning with the payment window pending.
func hybridCast(t *testing.T, e *Engine, beast state.ObjID, pip2Label string) {
	t.Helper()
	e.cast = &pendingCast{player: 0, card: beast, from: state.ZHand, ability: -1,
		cost: ParseCost("BG BG")}
	e.continueCast()
	d := e.Pending()
	if d == nil || !strings.HasPrefix(d.Prompt, "Choose how to pay") {
		t.Fatalf("want the first pip ask, got %+v", d)
	}
	submitIdx(t, e, d, findOpt(t, d, "Pay B"))
	d = e.Pending()
	if d == nil || !strings.HasPrefix(d.Prompt, "Choose how to pay") {
		t.Fatalf("want the second pip ask, got %+v", d)
	}
	submitIdx(t, e, d, findOpt(t, d, pip2Label))
}

// TestManaWindowWithholdsSourceThatCannotPayAnnouncedCost is the window
// half of the Fiend Artisan regression: with {B}{B} announced over one
// floating {B}, the fixed-G sources cannot pay anything and must not be
// offered, while the Any-colour source stays.
func TestManaWindowWithholdsSourceThatCannotPayAnnouncedCost(t *testing.T) {
	t.Parallel()
	e := handEngine(t, card(t, hybridBeastSrc))
	beast := e.G.Zone(state.ZHand, 0)[0]
	battlePerm(t, e, 0, anySourceSrc)
	battlePerm(t, e, 0, greenSourceSrc)
	battlePerm(t, e, 0, greenSourceSrc)
	addMana(t, e, 0, "B")

	hybridCast(t, e, beast, "Pay B")

	d := e.Pending()
	if d == nil || !strings.HasPrefix(d.Prompt, "Activate mana abilities to pay") {
		t.Fatalf("want the payment window, got %+v", d)
	}
	var greens, anys int
	for _, o := range d.Options {
		if o.Kind != "activate" {
			continue
		}
		switch e.G.Obj(o.Obj).Face().Name {
		case "Green Rock":
			greens++
		case "Any Rock":
			anys++
		}
	}
	if greens != 0 || anys != 1 {
		t.Fatalf("window offered %d green sources and %d Any Rock: %+v", greens, anys, d.Options)
	}
	submitIdx(t, e, d, findOpt(t, d, "Activate Any Rock"))

	// The colour ask keeps only the colour the cost still absorbs.
	d = e.Pending()
	if d == nil || !strings.HasPrefix(d.Prompt, "Choose a colour of mana") {
		t.Fatalf("want the colour ask, got %+v", d)
	}
	if len(d.Options) != 1 || d.Options[0].Label != "Add B" {
		t.Fatalf("colour ask options = %+v, want [Add B]", d.Options)
	}
	submitIdx(t, e, d, 0)

	passUntilStackEmpty(t, e, 60)
	if hasNote(e, "aborted") {
		t.Fatal("the cast aborted although the clamped window kept a payable path")
	}
	if o := e.G.Obj(beast); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("Hybrid Beast zone = %v, want battlefield", o.Zone)
	}
}

// TestManaColourAskKeepsOnlyAbsorbableColours is the colour-ask half: with
// {G}{B} announced (pip faces G then B) and no floating mana, the Any
// Rock's colour choice must keep exactly the two colours an unfilled pip
// still needs, not the whole wheel.
func TestManaColourAskKeepsOnlyAbsorbableColours(t *testing.T) {
	t.Parallel()
	e := handEngine(t, card(t, hybridBeastSrc))
	beast := e.G.Zone(state.ZHand, 0)[0]
	battlePerm(t, e, 0, anySourceSrc)
	battlePerm(t, e, 0, greenSourceSrc)

	e.cast = &pendingCast{player: 0, card: beast, from: state.ZHand, ability: -1,
		cost: ParseCost("BG BG")}
	e.continueCast()
	d := e.Pending()
	if d == nil || !strings.HasPrefix(d.Prompt, "Choose how to pay") {
		t.Fatalf("want the first pip ask, got %+v", d)
	}
	// A flexible second pip keeps Pay B feasible ({B}{G} over Any Rock +
	// Green Rock), so both faces are offered; announce G.
	submitIdx(t, e, d, findOpt(t, d, "Pay G"))
	d = e.Pending()
	if d == nil || !strings.HasPrefix(d.Prompt, "Choose how to pay") {
		t.Fatalf("want the second pip ask, got %+v", d)
	}
	// After G, Any Rock keeps Pay G feasible too ({G}{G} = Green Rock +
	// Any Rock); announce B (the first face).
	submitIdx(t, e, d, findOpt(t, d, "Pay B"))

	d = e.Pending()
	if d == nil || !strings.HasPrefix(d.Prompt, "Activate mana abilities to pay") {
		t.Fatalf("want the payment window, got %+v", d)
	}
	var greens, anys int
	for _, o := range d.Options {
		if o.Kind != "activate" {
			continue
		}
		switch e.G.Obj(o.Obj).Face().Name {
		case "Green Rock":
			greens++
		case "Any Rock":
			anys++
		}
	}
	if greens != 1 || anys != 1 {
		t.Fatalf("window offered %d green sources and %d Any Rock, want 1/1: %+v", greens, anys, d.Options)
	}
	submitIdx(t, e, d, findOpt(t, d, "Activate Any Rock"))

	d = e.Pending()
	if d == nil || !strings.HasPrefix(d.Prompt, "Choose a colour of mana") {
		t.Fatalf("want the colour ask, got %+v", d)
	}
	if len(d.Options) != 2 || d.Options[0].Label != "Add B" || d.Options[1].Label != "Add G" {
		t.Fatalf("colour ask options = %+v, want [Add B Add G]", d.Options)
	}
	submitIdx(t, e, d, d.Options[0].Index)

	// The pool {B} settles the B pip; the window re-poses for the {G} and
	// the green source finishes the payment.
	d = e.Pending()
	if d == nil || !strings.HasPrefix(d.Prompt, "Activate mana abilities to pay") {
		t.Fatalf("want the second window, got %+v", d)
	}
	submitIdx(t, e, d, findOpt(t, d, "Activate Green Rock"))

	passUntilStackEmpty(t, e, 60)
	if hasNote(e, "aborted") {
		t.Fatal("the cast aborted although the clamped colour ask kept a payable path")
	}
	if o := e.G.Obj(beast); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("Hybrid Beast zone = %v, want battlefield", o.Zone)
	}
}

// TestColourAskOffersTheOwedColourFirst: a {1}{U} cast paid by a dual and a red
// source. A dual that answers its first colour, W, leaves the {U} to the red
// source and strands the payment (CR 601.2g), so an answerer taking the first
// option must find the colour still owed there, whether the dual offers its
// colours in one ability or one per colour.
func TestColourAskOffersTheOwedColourFirst(t *testing.T) {
	t.Parallel()
	const redSrc = "Name:Red Rock\nTypes:Land\nA:AB$ Mana | Cost$ T | Produced$ R\nOracle:x\n"
	for name, dualSrc := range map[string]string{
		"one ability": "Name:Dual Rock\nTypes:Land\nA:AB$ Mana | Cost$ T | Produced$ Combo W U\nOracle:x\n",
		"one per colour": "Name:Dual Rock\nTypes:Land\nA:AB$ Mana | Cost$ T | Produced$ W\n" +
			"A:AB$ Mana | Cost$ T | Produced$ U\nOracle:x\n",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			e := handEngine(t, card(t, "Name:Blue Bird\nManaCost:1 U\nTypes:Creature Bird\nPT:1/1\nOracle:x\n"))
			bird := e.G.Zone(state.ZHand, 0)[0]
			battlePerm(t, e, 0, dualSrc)
			battlePerm(t, e, 0, redSrc)
			e.cast = &pendingCast{player: 0, card: bird, from: state.ZHand, ability: -1, cost: ParseCost("1 U")}
			e.continueCast()
			for i := 0; i < 8 && e.cast != nil; i++ {
				d := e.Pending()
				if d == nil {
					t.Fatal("no decision while the cast is pending")
				}
				submitIdx(t, e, d, d.Options[0].Index)
			}
			if got := e.G.Obj(bird).Zone; got != state.ZStack {
				t.Fatalf("Blue Bird in %s, want on the stack: the payment stranded", got)
			}
		})
	}
}
