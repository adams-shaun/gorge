package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestFlagbearerRuleSeesAChangeling: a Changeling is every creature type
// (CR 702.73a), Flagbearer included, so an opponent of Coalition Honor Guard's
// controller casting a spell that targets "a creature you control" must choose
// the Changeling when it is the only Flagbearer among their creatures.
func TestFlagbearerRuleSeesAChangeling(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	guard := lookup(t, reg, "Coalition Honor Guard")
	avian := lookup(t, reg, "Avian Changeling")
	bears := lookup(t, reg, "Grizzly Bears")
	fangs := lookup(t, reg, "Fangs of Kalonia")

	e, _ := corpusEngineCfg(t, reg, []*cards.Card{fangs, avian, bears}, []*cards.Card{guard})
	moveByName(t, e, 1, "Coalition Honor Guard", state.ZBattlefield)
	av := moveByName(t, e, 0, "Avian Changeling", state.ZBattlefield)
	moveByName(t, e, 0, "Grizzly Bears", state.ZBattlefield)
	moveByName(t, e, 0, "Fangs of Kalonia", state.ZHand)
	addMana(t, e, 0, "GG")
	e.askPriority(0)
	castFirst(t, e, "cast")
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("expected Fangs of Kalonia's target decision, got %+v", d)
	}
	if len(d.Options) != 1 || d.Options[0].Obj != av {
		t.Fatalf("only the Changeling is a Flagbearer among the offered creatures: %+v", d.Options)
	}
}
