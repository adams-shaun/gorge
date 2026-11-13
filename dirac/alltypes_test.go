package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// TestAllCreatureTypesHasOutsideType: a creature type no card of the
// matchup has (its TypeVocab entry 255) still belongs to a permanent with
// all creature types: Banner of Kinship naming "Vulcan" counts an animated
// Soulstone Sanctuary (pairs 11143 47181 59287 75189 78264 84109). Before,
// the choice was stored as 255 ("none chosen") and matched nothing.
func TestAllCreatureTypesHasOutsideType(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Soulstone Sanctuary", Count: 1}, {Name: "Plains", Count: 8}})
	ss := sc.perm(id["Soulstone Sanctuary"], 0)
	for i := 0; i < 4; i++ {
		sc.perm(id["Plains"], 0)
	}
	out := -1
	for v, si := range sc.k.t.TypeVocab {
		if si == 255 {
			out = v
			break
		}
	}
	if out < 0 {
		t.Skip("every creature type is in the matchup")
	}
	ch := typeVocabSub(sc.k, out)
	if sc.s.hasSubGen(sc.k, sc.slot(ss), ch) {
		t.Fatalf("the unanimated Sanctuary has creature type %d", out)
	}
	activated, pushed := false, false
	sc.stepTo(t, 12, func() bool { return pushed && sc.s.sp == 0 }, func(d *Decision) (Action, bool) {
		pushed = pushed || sc.s.sp > 0
		for _, a := range d.Options[:d.N] {
			if !activated && a.Kind() == AkActivate && a.Source() == TCode(uint8(ss)) {
				activated = true
				return a, true
			}
		}
		for _, a := range d.Options[:d.N] {
			if a.Kind() == AkPass {
				return a, true
			}
		}
		return 0, false
	})
	if !activated || !sc.s.isCreature(sc.k, sc.slot(ss)) {
		t.Fatalf("the Sanctuary was not animated (activated %v)", activated)
	}
	if !sc.s.hasSubGen(sc.k, sc.slot(ss), ch) {
		t.Fatalf("the animated Sanctuary (all creature types) lacks creature type %d, outside the matchup", out)
	}
}
