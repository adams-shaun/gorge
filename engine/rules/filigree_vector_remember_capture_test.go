package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// Filigree Vector's ETB resolves two target-only links --
// `ValidTgts$ Creature | RememberTargets$ True` then
// `ValidTgts$ Artifact | ImprintCards$ ThisTargetedCard` -- before its
// PutCounterAll body reads Creature.IsRemembered / Artifact.IsImprinted.
// Both links may name the Vector itself, which is also the ChangesZone
// trigger's fire-time capture. handleRemembering must record the Vector as a
// real remembered card (Forge's host list holds every RememberTargets$ target,
// the host included), so the remembered half sees it as well as the
// imprint half. Before the fix the seeded capture was the only occurrence and
// the selection rule dropped it, so the Vector got the charge counter but not
// the +1/+1 counter (and neither did Wall of Stone, whose remembered entry
// was written but ordered behind the capture).
func TestFiligreeVectorTargetsItselfGetsBothCounters(t *testing.T) {
	t.Parallel()
	e, _ := putCounterTable(t, 20261124,
		[]*cards.Card{corpusAlternativeCard(t, "Filigree Vector"), corpusAlternativeCard(t, "Wall of Stone")},
		[]*cards.Card{corpusAlternativeCard(t, "Ornithopter")})
	wall := findAndMoveToBattlefield(t, e, 0, "Wall of Stone")
	orni := findAndMoveToBattlefield(t, e, 1, "Ornithopter")
	for _, id := range []state.ObjID{wall, orni} {
		if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
			t.Fatalf("precondition: object %d is not on the battlefield", id)
		}
	}
	fv := searchMoveByName(t, e, "Filigree Vector", state.ZHand)
	addMana(t, e, 0, "WWWW")
	d := castSpellObj(t, e, fv)
	// Two target asks: any number of creatures, then any number of artifacts.
	// Name every offered target on each.
	for ask := 0; ask < 4; ask++ {
		if d == nil || d.Kind != decision.KTarget {
			t.Fatalf("target ask %d: %+v", ask, d)
		}
		if d.Min != 0 {
			t.Fatalf("target ask %d minimum = %d, want 0 (any number)", ask, d.Min)
		}
		idx := make([]int, 0, len(d.Options))
		for _, o := range d.Options {
			idx = append(idx, o.Index)
		}
		d = pickChoices(t, e, idx...)
		if d == nil {
			break
		}
	}
	if d != nil {
		t.Fatalf("unexpected decision after the target asks: %+v", d)
	}
	if o := e.G.Obj(fv); o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Filigree Vector is in %s, want battlefield", o.Zone)
	}
	if got := counterCount(e.G.Obj(fv), "P1P1"); got != 1 {
		t.Errorf("Filigree Vector P1P1 counters = %d, want 1 (it was a RememberTargets$ target)", got)
	}
	if got := counterCount(e.G.Obj(fv), "CHARGE"); got != 1 {
		t.Errorf("Filigree Vector CHARGE counters = %d, want 1 (it was an Artifact target)", got)
	}
	if got := counterCount(e.G.Obj(orni), "P1P1"); got != 1 {
		t.Errorf("Ornithopter P1P1 counters = %d, want 1", got)
	}
	if got := counterCount(e.G.Obj(orni), "CHARGE"); got != 1 {
		t.Errorf("Ornithopter CHARGE counters = %d, want 1", got)
	}
	if got := counterCount(e.G.Obj(wall), "P1P1"); got != 1 {
		t.Errorf("Wall of Stone P1P1 counters = %d, want 1", got)
	}
}
