package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
)

// TestKindredDominanceDestroysOnlyTheNonChosenType pins the IsNotChosenType
// filter end to end on the real corpus card. Before the predicate existed the
// spec failed closed and DestroyAll destroyed NOTHING, so the whole spell was
// a no-op on the board; the test seats creatures of two types so a critic
// that "destroys everything" is caught as well as one that destroys nothing.
func TestKindredDominanceDestroysOnlyTheNonChosenType(t *testing.T) {
	t.Parallel()
	e := handEngine(t, corpusAlternativeCard(t, "Kindred Dominance"))
	id := e.G.Zone(state.ZHand, 0)[0]
	// Creature (chosen) and a non-Goblin creature (destroyed).
	goblin := battlefieldCreature(t, e, "Name:Goblin Piker\nManaCost:R\nTypes:Creature Goblin Warrior\nPT:2/1\nOracle:x\n")
	bear := battlefieldCreature(t, e, "Name:Grizzly Bears\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	driveToTurn3Main(t, e)
	// Kindred Dominance costs {5}{B}{B}.
	e.G.Players[0].Pool[state.MC], e.G.Players[0].Pool[state.MB] = 5, 2
	submitOption(t, e, "", "Cast Kindred Dominance")
	// Pass priority once: the spell resolves and suspends on the
	// mid-resolution creature-type ask (ct1). Answer it "Goblin".
	passUntilAsk(t, e)
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "choosetype" ||
		d.Player != 0 || d.Min != 1 || d.Max != 1 || d.Prompt != "Choose a creature type" {
		t.Fatalf("expected the mid-resolution creature-type ask, got %+v", d)
	}
	idx := optionByLabel(d.Options, "Goblin")
	if idx < 0 {
		t.Fatalf("no Goblin option in %+v", d.Options)
	}
	submitChoices(t, e, idx)
	finishCast(t, e, id)
	if o := e.G.Obj(goblin); o.Zone != state.ZBattlefield {
		t.Fatalf("the CHOSEN type's Goblin moved to %s, want battlefield (it must survive)", o.Zone)
	}
	if o := e.G.Obj(bear); o.Zone != state.ZGraveyard {
		t.Fatalf("the non-chosen-type Bear stayed in %s, want graveyard (IsNotChosenType must match it)", o.Zone)
	}
	if o := e.G.Obj(id); o.Zone != state.ZGraveyard {
		t.Fatalf("resolved Kindred Dominance in %s, want graveyard", o.Zone)
	}
}
