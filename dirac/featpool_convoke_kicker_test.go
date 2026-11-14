package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestFeatPoolPaysConvokeKickedCastCredit (CR 601.2b, 601.2f): a Convoke
// card's kicked cast is one of gorge's pool-gate offers -- the potential
// planner declines a credit-carrying mode altCastFor lacks
// (potentialModeCastPlan's cost:credit) -- so FeatPoolPays' mode-1 bound
// prices the convoke credit beside the kicker raise. Coral Merfolk's tap pays
// {1}, so a {5} pool pays the credited {6}-less-one total but not the raw
// {6}, and a {4} pool pays neither.
func TestFeatPoolPaysConvokeKickedCastCredit(t *testing.T) {
	carrier := kop.CardDef{
		Name: "Test ConvokeKicker", Types: kop.TCreature,
		Keywords: kop.KwKicker, Kicker: kop.ManaCost{Generic: 1},
		Cost: kop.ManaCost{Generic: 5}, Convoke: true,
	}
	sc, o := mpocScen(t, "tapall", carrier, Config{}, false, "Coral Merfolk", "Island")
	s, k := sc.s, sc.k
	if k.gen == nil || !k.gen.anyConvoke {
		t.Fatal("precondition: the generated extension carries no convoke card")
	}
	s.pl[0].pool[kop.U] = 5
	if got := s.FeatPoolPays(0, o, 1); !got {
		t.Errorf("the {5} pool did not price the kicked cast with its creature credit beside the {1} raise")
	}
	s.pl[0].pool[kop.U] = 4
	if got := s.FeatPoolPays(0, o, 1); got {
		t.Errorf("the {4} pool priced the credited kicked cast ({5} less the credit plus the raise)")
	}
}
