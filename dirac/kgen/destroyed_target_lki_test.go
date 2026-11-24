package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// A later node's ConditionDefined$ Targeted after the chain's own Destroy of
// that target reads its last known controller (CR 608.2h): the Destroy takes
// the capture mark (C bit 4, the kernel's markRemLKI). A gate on the Destroy
// node itself reads the target before it moves and takes no mark.
const destroyThenTargetedSrc = `Name:Probe Demolition
ManaCost:R
Types:Sorcery
A:SP$ Destroy | ValidTgts$ Artifact | SubAbility$ DBToken | SpellDescription$ destroy, then tokens if you controlled it.
SVar:DBToken:DB$ Token | TokenAmount$ 3 | TokenScript$ r_1_1_phyrexian_goblin | TokenOwner$ You | ConditionDefined$ Targeted | ConditionPresent$ Card.YouCtrl
Oracle:probe
`

const destroyGatedSelfSrc = `Name:Probe Push
ManaCost:B
Types:Instant
A:SP$ Destroy | ValidTgts$ Creature | ConditionDefined$ Targeted | ConditionPresent$ Creature.cmcLE2 | ConditionCompare$ EQ1 | SpellDescription$ destroy a small creature.
Oracle:probe
`

func destroyLKIMarks(t *testing.T, src string) (marked, destroys int) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	card, diags := cards.ParseBytes("probe.txt", []byte(src))
	if card == nil {
		t.Fatalf("script did not parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	p := &pool{byName: map[string]*cards.Card{card.Faces[0].Name: card}, nDecks: map[string]int{}, reg: reg}
	lc := lowerCard(p, card.Faces[0].Name, card, false)
	for _, rf := range lc.Refusals {
		t.Errorf("refused: %s", rf.Reason)
	}
	for _, a := range lc.Abilities {
		for _, in := range a.Prog {
			if in.Op != "Destroy" {
				continue
			}
			destroys++
			if in.C&4 != 0 {
				marked++
			}
		}
	}
	return marked, destroys
}

func TestDestroyThenTargetedConditionReadsLKI(t *testing.T) {
	if m, d := destroyLKIMarks(t, destroyThenTargetedSrc); d != 1 || m != 1 {
		t.Errorf("destroy then Targeted gate: %d of %d Destroys marked, want 1 of 1", m, d)
	}
	if m, d := destroyLKIMarks(t, destroyGatedSelfSrc); d != 1 || m != 0 {
		t.Errorf("gate on the Destroy node itself: %d of %d Destroys marked, want 0 of 1", m, d)
	}
}
