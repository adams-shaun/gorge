package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestChangelingSharesACreatureTypeInEveryZone: Changeling is a
// characteristic-defining ability, so a Changeling card shares a creature type
// with an Elf in a graveyard (CR 702.73a, 604.3) and Unbury's two-card mode is
// open.
func TestChangelingSharesACreatureTypeInEveryZone(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := corpusEngine(t, reg, []*cards.Card{lookup(t, reg, "Unbury"),
		lookup(t, reg, "Changeling Outcast"), lookup(t, reg, "Llanowar Elves")}, nil)
	shifter := moveByName(t, e, 0, "Changeling Outcast", state.ZGraveyard)
	elf := moveByName(t, e, 0, "Llanowar Elves", state.ZGraveyard)
	if !slices.Contains(e.setPropTokens("creaturetype", shifter), "elf") {
		t.Fatalf("Changeling in the graveyard reads creature types %v, want elf among them",
			e.setPropTokens("creaturetype", shifter))
	}
	id := moveByName(t, e, 0, "Unbury", state.ZHand)
	addMana(t, e, 0, "BB")
	submitChoices(t, e, findOption(e.Pending(), "cast", id))
	var offered []state.ObjID
	d := e.Pending()
	if d == nil || d.Kind != decision.KModes || len(d.Options) != 2 {
		t.Fatalf("pending = %+v, want Unbury's two modes", d)
	}
	submitChoices(t, e, d.Options[1].Index)
	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget || d.Min != 2 || d.Max != 2 {
		t.Fatalf("pending = %+v, want the two-card target ask", d)
	}
	for _, o := range d.Options {
		offered = append(offered, o.Obj)
	}
	if !slices.Contains(offered, shifter) || !slices.Contains(offered, elf) {
		t.Fatalf("two-card mode offered %v, want %d and %d", offered, shifter, elf)
	}
}
