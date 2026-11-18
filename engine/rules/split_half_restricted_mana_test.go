package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// CR 709.3b: a split half cast alone is a spell with only that half's
// characteristics, so mana restricted to a multicolored spell (Pillar of the
// Paruns) can't pay for Away ({2}{B}) though Far // Away in hand shows both
// halves' colours. The restriction is read with the cast half's face index,
// as the potential planner's split_alt descriptor carries it.
func TestSplitHalfRestrictedManaReadsTheCastHalf(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, _ := corpusEngineCfg(t, reg, []*cards.Card{corpusCard(t, "Far")}, nil)
	toMain1(t, e)
	id := moveByName(t, e, 0, "Far", state.ZHand)
	if o := e.G.Obj(id); o.Zone != state.ZHand || len(o.Card.Faces) != 2 {
		t.Fatalf("precondition: Far zone=%s faces=%d, want a split card in hand", o.Zone, len(o.Card.Faces))
	}
	match := func(faceIdx uint8) bool {
		d := pay.DescriptorFor(id, false, Cost{Generic: 2})
		d.FaceIdx = faceIdx
		return pay.RestrictValidMatches(asPayer(e), 0, d, "Spell.MultiColor", 0)
	}
	e.pricingFused = true
	if !match(0) {
		t.Fatal("precondition: the split card in hand, read as the fused spell, is multicolored")
	}
	e.pricingFused = false
	if match(2) {
		t.Fatal("Away cast alone is mono-black: multicolor-only mana can't pay for it")
	}
}
