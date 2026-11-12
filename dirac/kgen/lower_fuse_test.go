package kgen

import (
	"reflect"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestFuseLowering (CR 702.102): a split card printed with fuse lowers its
// halves as ever plus one fused spell entry, the halves combined; a card
// whose fused spell cannot lower is refused by name with the true reason,
// never as a multi-face card.
func TestFuseLowering(t *testing.T) {
	lw := lowerNamedPool(t, []string{"Alive", "Wear", "Double Jump", "Gallifrey Falls"})
	cards, ents := map[string]*lCard{}, map[string]*lCard{}
	for _, c := range lw.cards {
		cards[c.Name] = c
	}
	for _, c := range lw.tokens {
		ents[c.Name] = c
	}
	for _, name := range []string{"Alive", "Wear"} {
		c, f := cards[name], ents[fuseKey(name)]
		if c == nil || f == nil {
			t.Fatalf("%s: card %v, fused entry %v", name, c, f)
		}
		if len(c.Refusals) > 0 || len(f.Refusals) > 0 {
			t.Errorf("%s refused: %v %v", name, c.Refusals, f.Refusals)
		}
		if c.FaceMode != kop.FaceSplit || c.FuseKey != fuseKey(name) || c.FusedSpell || !f.FusedSpell || f.Face != 0 || len(f.Faces) != 2 {
			t.Errorf("%s: layout %d fuse key %q, entry fused %v face %d faces %v", name, c.FaceMode, c.FuseKey, f.FusedSpell, f.Face, f.Faces)
		}
		if want := addManaCost(c.Cost, ents[faceKey(name, 1)].Cost); !reflect.DeepEqual(f.Cost, want) {
			t.Errorf("%s: fused cost %+v, want the halves' sum %+v", name, f.Cost, want)
		}
	}
	for name, want := range map[string]string{"Double Jump": "SubAbility", "Gallifrey Falls": "unread params"} {
		f := ents[fuseKey(name)]
		if f == nil {
			t.Fatalf("%s: no fused entry", name)
		}
		var got []string
		for _, r := range f.Refusals {
			got = append(got, r.Reason)
		}
		if all := strings.Join(got, " | "); !strings.Contains(all, want) {
			t.Errorf("%s: fused refusals %q, want one containing %q", name, all, want)
		}
		for _, r := range cards[name].Refusals {
			if strings.Contains(r.Reason, "multi-face") {
				t.Errorf("%s: stale shape refusal %q", name, r.Reason)
			}
		}
		if len(cards[name].Refusals) == 0 {
			t.Errorf("%s: lowers although its fused spell is refused", name)
		}
	}
}
