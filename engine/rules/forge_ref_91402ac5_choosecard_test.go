package rules

// The corpus pin 91402ac51 adds Mandatory$ True to the ChooseCard lines of
// cards whose Oracle says "choose" (Card-Forge/forge#12481). Fireglass
// Mentor's real second-main trigger must therefore pose its pick with Min 1:
// no empty or decline answer is legal.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// mandatoryChooseCardRef is the corpus commit that added Mandatory$ True.
const mandatoryChooseCardRef = "91402ac511046c0b7ed0bb691a8a446ca7dfc873"

// skipUnlessCorpusAtMandatoryRef skips while the resolved shared corpus is
// still at an older pin: the main checkout's .cards is refetched only after
// this bump merges, and until then the script has no Mandatory$ to read.
func skipUnlessCorpusAtMandatoryRef(t *testing.T) {
	t.Helper()
	dir, ok := testutil.SharedCorpusDir(".")
	if !ok {
		return
	}
	if l, err := cards.ReadLock(dir); err == nil && l.Commit != mandatoryChooseCardRef {
		t.Skipf("corpus at %s, need %s: refetch .cards", l.Commit, mandatoryChooseCardRef)
	}
}

func TestFireglassMentorChoiceIsMandatory(t *testing.T) {
	t.Parallel()
	skipUnlessCorpusAtMandatoryRef(t)
	reg := searchTestRegistry(t)
	e, _ := corpusEngineCfg(t, reg, []*cards.Card{lookup(t, reg, "Fireglass Mentor")}, nil)
	mentor := moveByName(t, e, 0, "Fireglass Mentor", state.ZBattlefield)
	if o := e.G.Obj(mentor); o == nil || o.Zone != state.ZBattlefield {
		t.Fatal("precondition: Fireglass Mentor not on the battlefield")
	}
	e.emit(events.Event{Kind: events.LifeChange, Player: 1, Amount: -1})
	if e.G.Players[1].LifeLostThisTurn != 1 {
		t.Fatal("precondition: the opponent has not lost life this turn")
	}

	d := passUntilAskKind(t, e, decision.KChoose, 200)
	if d.Player != 0 || len(d.Options) != 2 {
		t.Fatalf("ask = %+v, want seat 0 choosing between the two exiled cards", d)
	}
	if d.Min != 1 || d.Max != 1 {
		t.Fatalf("ask bounds = [%d,%d], want [1,1] (Mandatory$ True)", d.Min, d.Max)
	}
	for _, o := range d.Options {
		if got := e.G.Obj(o.Obj); got == nil || got.Zone != state.ZExile {
			t.Fatalf("option %+v is not an exiled card", o)
		}
	}
	if err := d.Validate(decision.Intent{Seq: d.Seq, Player: d.Player}); err == nil {
		t.Fatal("the empty answer validated; the choice must not be declinable")
	}
	if err := d.Validate(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{d.Options[0].Index}}); err != nil {
		t.Fatalf("picking one exiled card rejected: %v", err)
	}
}
