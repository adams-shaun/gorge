package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// CR 603.3c/d: Ayula, Queen Among Bears' fight mode is a Pump body whose
// SubAbility$ Fight declares the second target ("target creature you don't
// control"). Both targets are chosen as the trigger is put on the stack: after
// the mode's own Bear target the chain link's ask follows at once (ResumeKind
// "trig_sub"), not at resolution.
func TestCharmModeBodyLinkTargetAskedAtPlacement(t *testing.T) {
	t.Parallel()
	reg := searchTestRegistry(t)
	forest := searchCorpusCard(t, reg, "Forest")
	bear := searchCorpusCard(t, reg, "Grizzly Bears")
	ayula := searchCorpusCard(t, reg, "Ayula, Queen Among Bears")
	deck := make([]*cards.Card, 40)
	opp := make([]*cards.Card, 40)
	for i := range deck {
		deck[i], opp[i] = forest, bear
	}
	deck[0], deck[1] = ayula, bear
	e := New(seatZeroStart(Config{Seed: 7713, Names: []string{"protagonist", "opponent"},
		Decks: [][]*cards.Card{deck, opp}, Tokens: reg.Tokens}))
	e.Advance()
	toMain1(t, e)
	searchMoveByNameSeat(t, e, 0, "Ayula, Queen Among Bears", state.ZBattlefield)
	searchMoveByNameSeat(t, e, 1, "Grizzly Bears", state.ZBattlefield)
	searchMoveByNameSeat(t, e, 0, "Grizzly Bears", state.ZBattlefield)

	d := passUntilAsk(t, e)
	if d == nil || d.Kind != decision.KModes {
		t.Fatalf("first ask = %+v, want Ayula's mode choice", d)
	}
	fight := -1
	for _, o := range d.Options {
		if len(o.Label) > 0 && o.Label[0] == 'T' { // "Target Bear you control fights ..."
			fight = o.Index
		}
	}
	if fight < 0 {
		t.Fatalf("no fight mode among %+v", d.Options)
	}
	submitChoices(t, e, fight)

	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget || d.ResumeKind == "trig_sub" {
		t.Fatalf("second ask = %+v, want the mode body's own Bear target", d)
	}
	submitChoices(t, e, d.Options[0].Index)

	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget || d.ResumeKind != "trig_sub" {
		t.Fatalf("third ask = %+v, want the Fight link's placement target (trig_sub)", d)
	}
}
