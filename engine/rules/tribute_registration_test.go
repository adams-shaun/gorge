package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// kw:Tribute is expanded by cards/kw_tribute.go and its behaviour is pinned
// by tribute_test.go; this file is the SUPPORT half: the corpus reader must
// see the keyword as registered, or the Tribute cards never enter a synth
// pool (CR 702.104, Forge's CountersPutEffect.java:493 pair).
func TestTributeCardsAreSupported(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	for _, name := range []string{"Fanatic of Xenagos", "Nessian Demolok", "Oracle of Bones"} {
		c, ok := reg.Lookup(name)
		if !ok {
			t.Fatalf("%s: not in the corpus", name)
		}
		if miss := reg.Unsupported(c, effects.Supported()); len(miss) != 0 {
			t.Errorf("%s unsupported: %v", name, miss)
		}
	}
}

// CR 702.104b on a carrier with a destructive unpaid trigger, driven through
// a real cast so the two asks are posed by the resolution run (the raw-emit
// path answers them with the no-run deterministic default): the chosen
// opponent declines Nessian Demolok (Tribute 3), so it enters plain and its
// "when it enters, if tribute wasn't paid" trigger destroys the target
// noncreature permanent.
func TestTributeDeclinedFiresTrigger(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := proliferateEngine(t, reg, "Nessian Demolok", "Mountain")
	mountain := putNamedOnBattlefield(t, e, "Mountain")
	demolok := findAndMoveToHand(t, e, 0, "Nessian Demolok")
	addMana(t, e, 0, "GGGGG")
	// The entry is the cast resolution's last step, so the payer-pick ask is
	// the first decision the resolution poses.
	d := castFixture(t, e, demolok, -1)
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "tribute_payer" {
		t.Fatalf("decision = %+v, want the tribute payer-pick ask", d)
	}
	submitChoices(t, e, d.Options[0].Index)
	d = passUntilNonPriority(t, e, 60)
	if d == nil || d.Kind != decision.KChoose || d.ResumeKind != "tribute" {
		t.Fatalf("decision = %+v, want the tribute pay-or-decline ask", d)
	}
	submitChoices(t, e, d.Options[1].Index) // no — decline the tribute
	o := e.G.Obj(demolok)
	if o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("Nessian Demolok = %+v, want it on the battlefield", o)
	}
	if o.Tributed {
		t.Fatalf("declined tribute left the Tributed designation set")
	}
	if got := o.Counter("P1P1"); got != 0 {
		t.Fatalf("Nessian Demolok P1P1 = %d, want 0 (tribute declined)", got)
	}
	// The unpaid trigger is mandatory and on the stack; pass priority into
	// its resolution and pick the Mountain (the only noncreature permanent
	// on the battlefield) as Destroy's target.
	d = passUntilNonPriority(t, e, 60)
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("decision = %+v, want the unpaid trigger's destroy target ask", d)
	}
	idx := -1
	for _, opt := range d.Options {
		if opt.Obj == mountain {
			idx = opt.Index
		}
	}
	if idx < 0 {
		t.Fatalf("destroy target options %+v do not offer the Mountain %d", d.Options, mountain)
	}
	submitChoices(t, e, idx)
	passUntilStackEmpty(t, e, 60)
	if m := e.G.Obj(mountain); m == nil || m.Zone != state.ZGraveyard {
		t.Fatalf("Mountain = %+v, want it destroyed into the graveyard", m)
	}
	if o := e.G.Obj(demolok); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("Nessian Demolok = %+v, want it still on the battlefield", o)
	}
	replayCheck(t, e, cfg)
}
