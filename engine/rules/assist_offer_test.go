package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// assist_offer_test.go covers the review round's two MAJOR findings: the cast
// must be OFFERED when the caster can pay only the coloured part and the other
// seats' mana is what makes the generic part payable, and a contributing seat
// must get its CR 702.132a mana-ability window instead of being skipped because
// no mana is already floating.
//
// The helper window is a mid-cast KChoose: one "assist_activate" option per
// untapped mana source, then "assist_done". The option kinds are asserted here
// as wire vocabulary, exactly as a client would see them.

// assistWindow is the pending mana-ability window asked of seat p while it
// decides how much to contribute. It fails when the pending decision is not
// that window or names another seat.
func assistWindow(t *testing.T, e *Engine, p state.PlayerID) *decision.Decision {
	t.Helper()
	d := e.Pending()
	if d == nil || d.Player != p || len(d.Options) == 0 ||
		(d.Options[0].Kind != "assist_activate" && d.Options[0].Kind != "assist_done") {
		t.Fatalf("expected seat %d's assist mana window, got %+v", p, d)
	}
	return d
}

// assistWindowActivate answers the pending assist mana window by activating
// one offered source.
func assistWindowActivate(t *testing.T, e *Engine, p state.PlayerID) {
	t.Helper()
	d := assistWindow(t, e, p)
	for _, o := range d.Options {
		if o.Kind == "assist_activate" {
			submitChoices(t, e, o.Index)
			return
		}
	}
	t.Fatalf("assist mana window offered no source to activate: %+v", d)
}

// TestAssistOffersCastWithOnlyColouredMana is the MAJOR finding's case: the
// caster holds only the {W} of Bring Down's {3}{W}, and seat 2 controls three
// untapped Mountains. The spell MUST be offered (the potential assist makes the
// generic part payable), and once cast, seat 2 gets the mana window, taps the
// three Mountains, pays {3}, and the caster pays only the {W}.
func TestAssistOffersCastWithOnlyColouredMana(t *testing.T) {
	t.Parallel()
	e, cfg, reg := assistEngine(t, "Bring Down")
	giant := assistSeedBattlefield(t, e, reg, 1, "Bonebreaker Giant")
	for range 3 {
		assistSeedBattlefield(t, e, reg, 2, "Mountain")
	}
	hero := searchMoveByName(t, e, "Bring Down", state.ZHand)
	addMana(t, e, 0, "W") // only the coloured part: the generic needs the assist

	submitChoices(t, e, castOptMode(t, e.Pending().Options, hero, "").Index)
	answerAssistTarget(t, e, giant)

	for range 3 {
		assistWindowActivate(t, e, 2)
	}
	if got := assistAmounts(t, e, 2); len(got) != 4 || got[0] != 0 || got[3] != 3 {
		t.Fatalf("assist offers %v, want [0 1 2 3] after the mana window", got)
	}
	assistAnswer(t, e, 3)
	passUntilStackEmpty(t, e, 40)

	if o := e.G.Obj(giant); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("Bring Down did not destroy its target: %+v", o)
	}
	if got := assistSpent(e, 2); got != 3 {
		t.Fatalf("the assisting seat spent %d, want 3", got)
	}
	if got := assistSpent(e, 0); got != 1 {
		t.Fatalf("the caster spent %d, want 1 (the {W} left after the assist)", got)
	}
	replayCheck(t, e, cfg)
}

// TestAssistPayerGetsManaWindow is the second MAJOR finding: a seat with an
// untapped land and an EMPTY pool must be asked after activating that land,
// even when the caster could pay the whole cost alone. The old shape skipped it
// (AssistCap read only the floating pool), making Assist inert in normal play.
func TestAssistPayerGetsManaWindow(t *testing.T) {
	t.Parallel()
	e, cfg, reg := assistEngine(t, "Bring Down")
	giant := assistSeedBattlefield(t, e, reg, 1, "Bonebreaker Giant")
	assistSeedBattlefield(t, e, reg, 2, "Mountain")
	hero := searchMoveByName(t, e, "Bring Down", state.ZHand)
	addMana(t, e, 0, "WWWW")

	submitChoices(t, e, castOptMode(t, e.Pending().Options, hero, "").Index)
	answerAssistTarget(t, e, giant)

	assistWindowActivate(t, e, 2)
	if got := assistAmounts(t, e, 2); len(got) != 2 || got[0] != 0 || got[1] != 1 {
		t.Fatalf("assist offers %v, want [0 1] after the untapped land", got)
	}
	assistAnswer(t, e, 1)
	passUntilStackEmpty(t, e, 40)

	if got := assistSpent(e, 2); got != 1 {
		t.Fatalf("the assisting seat spent %d, want 1", got)
	}
	if got := assistSpent(e, 0); got != 3 {
		t.Fatalf("the caster spent %d, want 3 (the {2}{W} left)", got)
	}
	replayCheck(t, e, cfg)
}

// TestAssistAbortedCastKeepsPayerMana is the reversal finding: a seat's
// contribution is drained only when the caster's payment commits, so a cast
// that aborts after the election (here the caster cannot pay the remaining
// {2}{W}) leaves the contributing seat's mana in its pool.
func TestAssistAbortedCastKeepsPayerMana(t *testing.T) {
	t.Parallel()
	e, cfg, reg := assistEngine(t, "Bring Down")
	giant := assistSeedBattlefield(t, e, reg, 1, "Bonebreaker Giant")
	for range 3 {
		assistSeedBattlefield(t, e, reg, 2, "Mountain")
	}
	hero := searchMoveByName(t, e, "Bring Down", state.ZHand)
	addMana(t, e, 0, "W")

	submitChoices(t, e, castOptMode(t, e.Pending().Options, hero, "").Index)
	answerAssistTarget(t, e, giant)
	for range 3 {
		assistWindowActivate(t, e, 2)
	}
	assistAnswer(t, e, 1) // promises {1}; the caster cannot pay the other {2}{W}
	passUntilStackEmpty(t, e, 40)

	if got := assistSpent(e, 2); got != 0 {
		t.Fatalf("an aborted cast drained the assisting seat of %d mana, want 0", got)
	}
	if got := assistPool(e, 2); got != 3 {
		t.Fatalf("the assisting seat's pool holds %d, want the 3 it floated", got)
	}
	replayCheck(t, e, cfg)
}

// TestAssistStaleAnswerIsIgnored pins the guard on a duplicate/late assist
// answer: once the APNAP round is over, an answer naming a payer must not move
// any mana. The old answerAssist guarded only assistPaid < assistCap, so a
// stale answer after every seat declined could still drain the last payer.
func TestAssistStaleAnswerIsIgnored(t *testing.T) {
	t.Parallel()
	e, cfg, _ := assistEngine(t, "Bring Down")
	addMana(t, e, 2, "CCC")

	pc := &pendingCast{assistDone: true, assistPayer: 2, assistCap: 3}
	d := &decision.Decision{Player: 2, Kind: decision.KChoose, Min: 1, Max: 1,
		Options: []decision.Option{{Index: 0, Kind: assistAnswerKind, Amount: 3}}}
	e.answerAssist(pc, d, d.Options)

	if got := assistPool(e, 2); got != 3 {
		t.Fatalf("a stale assist answer drained the payer to %d, want 3", got)
	}
	if got := pc.assistPaid; got != 0 {
		t.Fatalf("a stale assist answer credited %d, want 0", got)
	}
	replayCheck(t, e, cfg)
}
