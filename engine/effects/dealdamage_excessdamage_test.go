package effects

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

const (
	excessTrampler = "Name:Excess Trampler\nTypes:Creature Beast\nPT:5/5\nK:Trample\nOracle:x\n"
	excessPlain    = "Name:Excess Plain\nTypes:Creature Beast\nPT:5/5\nOracle:x\n"
	excessVictim   = "Name:Excess Victim\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"
)

// ramThroughDamage resolves Ram Through's real DealDamage link with `mine` as
// the parent's chosen creature (the DamageSource$ ParentTarget referent) and
// the 2/2 as its own target, returning the victim, its controller's life and
// the log.
func ramThroughDamage(t *testing.T, mineSrc string) (victim *state.Object, life int32, h *fakeHost) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	card, ok := reg.Lookup("Ram Through")
	if !ok {
		t.Fatal("corpus has no Ram Through")
	}
	face := card.Faces[0]
	line := face.SVars["SoulsDamage"]
	for _, want := range []string{"ExcessDamage$ ThisTargetedController", "ExcessDamageCondition$ Card.withTrample"} {
		if !strings.Contains(line, want) {
			t.Fatalf("corpus moved: SoulsDamage no longer carries %q: %q", want, line)
		}
	}
	h = newHost(t, 2)
	mine := h.g.AddObject(mkCard(t, mineSrc), 0)
	mine.Zone = state.ZBattlefield
	victim = h.g.AddObject(mkCard(t, excessVictim), 1)
	victim.Zone = state.ZBattlefield
	if p, tough := h.Power(mine.ID), h.Toughness(victim.ID); p != 5 || tough != 2 {
		t.Fatalf("precondition: power %d toughness %d, want 5 and 2", p, tough)
	}
	c := &Ctx{Source: mine.ID, Controller: 0, SVars: face.SVars,
		Targets:       []state.Target{{Obj: mine.ID}},
		PickedTargets: []state.Target{{Obj: victim.ID}}}
	Resolve(h, c, svarSA(t, face, "SoulsDamage"))
	return victim, h.g.Players[1].Life, h
}

// TestDealDamageExcessDamage: Ram Through's "if the creature you control has
// trample, excess damage is dealt to that creature's controller instead". The
// 5-power trampler gives the 2-toughness creature exactly lethal (2) and its
// controller the other 3; without trample the creature takes all 5 and the
// controller nothing.
func TestDealDamageExcessDamage(t *testing.T) {
	t.Run("trample", func(t *testing.T) {
		victim, life, h := ramThroughDamage(t, excessTrampler)
		if victim.Damage != 2 {
			t.Fatalf("the creature took %d damage, want exactly lethal 2", victim.Damage)
		}
		if life != 17 {
			t.Fatalf("controller life = %d, want 17 (lost the 3 excess)", life)
		}
		if h.g.Players[0].Life != 20 {
			t.Fatalf("the damage source's controller life = %d, want 20", h.g.Players[0].Life)
		}
	})
	t.Run("no trample", func(t *testing.T) {
		victim, life, h := ramThroughDamage(t, excessPlain)
		if victim.Damage != 5 {
			t.Fatalf("the creature took %d damage, want all 5", victim.Damage)
		}
		if life != 20 {
			t.Fatalf("controller life = %d, want 20 (no excess without trample)", life)
		}
		for _, e := range h.log {
			if e.Kind == events.Note && strings.Contains(e.Text, "ExcessDamage") {
				t.Fatalf("unexpected note: %s", e.Text)
			}
		}
	})
}
