package rules

// An Effect with RememberObjects$ remembers on the Effect itself (Forge's
// created effect card), and its Triggers$ body reads that memory as
// Defined$ Remembered, not the firing event's object. Mountain Titan
// ("{1}{R}{R}: Until end of turn, whenever you cast a black spell, put a
// +1/+1 counter on Mountain Titan") remembers Self; the SpellCast body read
// the cast spell as Remembered and put its counter on the spell on the stack.
// The lockstep twin is dirac/oracle TestMBCEffectSelfRememberLowers.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func TestEffectRememberSelfSpellCastCountersSource(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	titanCard := lookup(t, reg, "Mountain Titan")
	_, cfg := corpusEngineCfg(t, reg, []*cards.Card{titanCard, lookup(t, reg, "Walking Corpse")}, nil)
	cfg = seatZeroStart(cfg)
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	titan := putOnBattlefield(t, e, 0, titanCard)
	face := e.G.Obj(titan).Face()
	var ab *cards.SA
	for _, sa := range face.Abilities {
		if sa.APIKind() == cards.APIEffect {
			ab = sa
		}
	}
	if ab == nil {
		t.Fatal("corpus precondition: Mountain Titan has no AB$ Effect")
	}
	effects.Resolve(e, effects.NewCtxPtr(titan, 0, effects.CtxInit{SVars: face.SVars}), ab)

	castNoAsk(t, e, "Walking Corpse", "BB")
	if got := e.G.Obj(titan).Counter("P1P1"); got != 1 {
		t.Fatalf("Mountain Titan +1/+1 counters = %d, want 1 (the Effect's remembered source)", got)
	}
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Walking Corpse" && o.Counter("P1P1") != 0 {
			t.Fatalf("Walking Corpse took %d +1/+1 counters, want 0", o.Counter("P1P1"))
		}
	}
}
