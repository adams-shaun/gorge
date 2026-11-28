package dirac

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestPermTargetsSurviveMaskedController: a census may clear a seat's
// controller bits to hide its mana sources (offersOnPath) while the
// permanents stay on the battlefield. They are still legal targets (CR
// 601.2c), listed seat by seat in the same order.
func TestPermTargetsSurviveMaskedController(t *testing.T) {
	sc := newScen(t, Config{NoElide: true})
	s, k := sc.s, sc.k
	// seat 1's creature enters first, so slot order is not seat order
	theirs := sc.perm(v.GrizzlyBears, 1)
	first := sc.perm(v.GrizzlyBears, 0)
	second := sc.perm(v.GrizzlyBears, 0)
	if !sc.onBF(theirs) || !sc.onBF(first) || !sc.onBF(second) || sc.slot(theirs) >= sc.slot(first) {
		t.Fatalf("slots %d %d %d: want all on the battlefield, seat 1's lowest", sc.slot(theirs), sc.slot(first), sc.slot(second))
	}
	bolt := k.t.Cards[v.LightningBolt].Abilities[0].Targets[0]
	if bolt.Kind != kop.TgtAny {
		t.Fatalf("Lightning Bolt's slot is kind %d, want TgtAny", bolt.Kind)
	}
	perms := []TCode{TCode(first), TCode(second), TCode(theirs)}
	players := []TCode{PlayerCode(0), PlayerCode(1)}
	for _, tc := range []struct {
		name string
		kind kop.TargetKind
		want []TCode
	}{
		{"object", kop.TgtObject, perms},
		{"any", kop.TgtAny, append(slices.Clone(players), perms...)},
		{"permanent or spell", kop.TgtPermOrSpell, perms},
		{"permanent or graveyard card", kop.TgtPermOrGraveCard, perms},
	} {
		t.Run(tc.name, func(t *testing.T) {
			spec := bolt
			spec.Kind = tc.kind
			cands := func() []TCode {
				var buf [MaxOptions]TCode
				n := s.targetCands(k, &spec, 0, NoSlot, NoObj, &buf)
				return slices.Clone(buf[:n])
			}
			if got := cands(); !slices.Equal(got, tc.want) {
				t.Fatalf("unmasked candidates %v, want %v", got, tc.want)
			}
			held := s.ctrlM[0]
			if !held.any() {
				t.Fatal("seat 0 holds no controller bits to mask")
			}
			s.ctrlM[0] = slotMask{}
			defer func() { s.ctrlM[0] = held }()
			if got := cands(); !slices.Equal(got, tc.want) {
				t.Errorf("masked candidates %v, want %v", got, tc.want)
			}
			if !s.hasTarget(k, &spec, 0, NoSlot, NoObj) {
				t.Error("masked: hasTarget = false")
			}
		})
	}
}
