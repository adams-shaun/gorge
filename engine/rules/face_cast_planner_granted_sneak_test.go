package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestGrantedSneakOfferedFromUntappedLands pins the grant-instance half of the
// planner's Sneak pricing: Ninja Teen's level 3 grants "Creature cards in your
// graveyard have sneak {3}{B}" to Oroku Saki, which also prints Sneak {1}{B}.
// The graveyard offer therefore carries TWO Sneak modes, and the granted one
// ("sneaked_grant_2") must be witnessed for its OWN {3}{B} -- beginCast charges
// the instance named by selectedMode -- not re-priced as the printed {1}{B}.
// Four untapped Swamps pay the granted cost; the printed cost would tap two.
func TestGrantedSneakOfferedFromUntappedLands(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	deck := []*cards.Card{
		searchCorpusCard(t, reg, "Ninja Teen"),
		searchCorpusCard(t, reg, "Oroku Saki, Shredder Rising"),
		card(t, ninjutsuBearSrc),
	}
	for len(deck) < 40 {
		deck = append(deck, searchCorpusCard(t, reg, "Forest"))
	}
	cfg := seatZeroStart(Config{Seed: 97203, Names: []string{"sneak", "defender"},
		Decks: [][]*cards.Card{deck, mountainDeck(t, 40)}, Tokens: reg.Tokens})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)

	class := classMove(t, e, "Ninja Teen", state.ZBattlefield)
	if got := e.G.Obj(class).ClassLevel(); got != 1 {
		t.Fatalf("precondition: Ninja Teen level=%d, want 1", got)
	}
	addMana(t, e, 0, "B")
	classLevelUp(t, e, class, 0)
	addMana(t, e, 0, "B")
	classLevelUp(t, e, class, 1)
	if got := e.G.Obj(class).ClassLevel(); got != 3 {
		t.Fatalf("precondition: Ninja Teen level=%d, want 3", got)
	}

	sakiID := classMove(t, e, "Oroku Saki, Shredder Rising", state.ZGraveyard)
	if o := e.G.Obj(sakiID); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("precondition: Oroku Saki not in the graveyard: %+v", o)
	}
	attackWithBear(t, e) // unblocked attacker + declare-blockers window
	if pool := e.G.Players[0].Pool; pool.Total() != 0 {
		t.Fatalf("precondition: pool %v not empty", pool)
	}
	lands := make([]state.ObjID, 0, 4)
	for range 4 {
		lands = append(lands, onBoardCard(t, e, 0, corpusCard(t, "Swamp")))
	}
	ppPriority(t, e)

	// The printed instance prices {1}{B}; the grant prices {3}{B}. Assert the
	// two modes really differ before relying on the granted one.
	printed := faceCastPlan(t, e.PotentialPaymentPlans(0), sakiID, "sneak")
	if printed.Plan == nil || printed.Reason != "" {
		t.Fatalf("sneak verdict %+v, want a witness", printed)
	}
	granted := faceCastPlan(t, e.PotentialPaymentPlans(0), sakiID, "sneaked_grant_2")
	if granted.Plan == nil || granted.Reason != "" {
		t.Fatalf("sneaked_grant_2 verdict %+v, want a witness from the four untapped Swamps", granted)
	}
	if len(granted.Plan.Activations) != len(lands) {
		t.Fatalf("granted witness %s, want all four Swamps", lrPlanString(granted.Plan.Activations))
	}
	for _, a := range granted.Plan.Activations {
		found := false
		for _, id := range lands {
			found = found || a.Source == id
		}
		if !found {
			t.Fatalf("granted witness tapped %d, want only the four Swamps", a.Source)
		}
	}
}
