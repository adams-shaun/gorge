package rules

// Produced$ Special EachColorAmong_Valid <spec> must read each matching
// permanent's LIVE layer-5 colours (CR 613.1e), not its printed face: a
// permanent whose colour set a continuous effect rewrote (Puca's Eye's ETB
// chain DBChooseColor -> DBAnimate Colors$ ChosenColor OverwriteColors$ True)
// contributes its derived colour to "for each color among permanents you
// control". Bloom Tender is the reported carrier; the executor here is driven
// through an inline fixture carrying the same Produced$ token (never the
// corpus .txt, per the licensing rule), with the real corpus Puca's Eye
// supplying the layer-5 colour grant.
//
// The executor previously unioned effects.ColorMaskOf -- the printed read --
// so the animated permanent's colour was invisible and the tap added nothing.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestEachColorAmongValidReadsDerivedLayer5Colors is the defect pin: Puca's
// Eye enters, its real ETB chain poses the ChooseColor ask, seat 0 answers
// BLUE (deliberately not the first WUBRG option), the chained Animate
// overwrites the artifact's colours to blue -- and a bloom-tender-shaped
// {T}: for each color among permanents you control on the same board must
// then add exactly one U (nothing else on the board is coloured). The
// printed-only union added nothing; the derived union adds the U.
func TestEachColorAmongValidReadsDerivedLayer5Colors(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	eye := lookup(t, reg, "Puca's Eye")
	sifter := card(t, colorlessFixture("Special EachColorAmong_Valid Permanent.YouCtrl"))

	e, cfg := eachColorGame(t, 496, []*cards.Card{eye, sifter})
	// Puca's Eye first: its ETB trigger is the chain below. The Sifter joins
	// afterwards, so tapForManaAfterTurnClears can own its CR 302.6 dance.
	id := moveByName(t, e, 0, "Puca's Eye", state.ZBattlefield)

	// PRECONDITION: the artifact is on the battlefield, colourless, and
	// carries no chosen colour until its own ask is answered.
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Puca's Eye not on the battlefield: %+v", o)
	}
	if got := e.Colors(id); got != "" {
		t.Fatalf("precondition: Puca's Eye is already coloured %q before any choice", got)
	}

	answered := false
	for i := 0; i < 40 && !answered; i++ {
		if e.putTriggersOnStack() {
			continue
		}
		d := e.Pending()
		if d == nil {
			e.Advance()
			continue
		}
		switch {
		case d.Kind == decision.KChoose && d.ResumeKind == "choosecolor":
			blue := optionByLabel(d.Options, "Blue")
			if blue < 0 {
				t.Fatalf("choosecolor ask offers no Blue option: %+v", d.Options)
			}
			submitChoices(t, e, blue)
			answered = true
		case d.Kind == decision.KPriority:
			idx := -1
			for _, o := range d.Options {
				if o.Kind == "pass" {
					idx = o.Index
				}
			}
			if idx < 0 {
				t.Fatalf("priority decision with no pass option: %+v", d)
			}
			submitChoices(t, e, idx)
		default:
			t.Fatalf("unexpected %s decision (resume %q) in Puca's Eye's chain: %+v", d.Kind, d.ResumeKind, d)
		}
	}
	if !answered {
		t.Fatal("the real ChooseColor ask was never posed and answered")
	}
	passUntilStackEmpty(t, e, 20)

	// The ETB chain applied: the artifact's derived colour is the answered
	// BLUE. Without this the assertion below would pass for the wrong reason
	// (an animate that never landed).
	if got := e.G.Obj(id).ChosenColor; got != "U" {
		t.Fatalf("ChosenColor = %q, want \"U\"", got)
	}
	if got := e.Colors(id); got != "U" {
		t.Fatalf("Puca's Eye colours = %q, want \"U\" after choosing Blue", got)
	}

	// CR 302.6: the Sifter enters this turn, so tapForManaAfterTurnClears
	// gives it a logged TurnChange before the EachColorAmong batch.
	tapForManaAfterTurnClears(t, e, "Colorless Sifter")
	// The board is the colourless Sifter, the now-blue Puca's Eye and
	// mountains: exactly one colour is among the matching permanents, so the
	// batch is exactly one U.
	poolIs(t, e, [state.MC + 1]int32{state.MU: 1})
	replayCheck(t, e, cfg)
}
