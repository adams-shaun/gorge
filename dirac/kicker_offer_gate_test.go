package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// kickedOffered runs the priority walk and reports whether the hand card o is
// offered as the kicked cast (mode 1).
func kickedOffered(sc *scen, o Obj) bool {
	var d Decision
	sc.s.priorityOptions(sc.k, &d)
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkCast && a.Source() == TCode(o) && a.Mode() == 1 {
			return true
		}
	}
	return false
}

// TestKickedOfferConvokeGatesOnCreditedPool (CR 601.2b, 601.2f): gorge's menu
// lists a Convoke card's kicked cast only when its pool gate prices it with the
// creature credit, whatever the sources could pay. Five Islands pay the {6}
// kicked total; the floating pool decides the offer: {4} does not pay the
// {5}-plus-{1}-less-one total, {5} does.
func TestKickedOfferConvokeGatesOnCreditedPool(t *testing.T) {
	carrier := kop.CardDef{
		Name: "Test ConvokeKicker", Types: kop.TCreature,
		Keywords: kop.KwKicker, Kicker: kop.ManaCost{Generic: 1},
		Cost: kop.ManaCost{Generic: 5}, Convoke: true,
	}
	sc, o := mpocScen(t, "tapall", carrier, Config{NoElide: true, GorgeAutopaySurface: true}, false,
		"Coral Merfolk", "Island", "Island", "Island", "Island", "Island", "Island")
	s := sc.s
	if sc.k.gen == nil || !sc.k.gen.anyConvoke {
		t.Fatal("precondition: the generated extension carries no convoke card")
	}
	if s.g.availX&availWalk != 0 {
		t.Fatal("precondition: the walk bit is set, so the pool gate is not the one under test")
	}
	s.pl[0].pool[kop.U] = 4
	if s.FeatPoolPays(0, o, 1) {
		t.Fatal("precondition: the {4} pool prices the credited kicked cast")
	}
	if kickedOffered(sc, o) {
		t.Errorf("the kicked cast was offered with a {4} pool that does not price it (sources alone do)")
	}
	s.pl[0].pool[kop.U] = 5
	if !s.FeatPoolPays(0, o, 1) {
		t.Fatal("precondition: the {5} pool does not price the credited kicked cast")
	}
	if !kickedOffered(sc, o) {
		t.Errorf("the kicked cast was withheld with a {5} pool that prices the credited total")
	}
}

// TestKickedOfferWalkBitKeepsAffordability: under the potential walk's bit a
// plain Kicker card's kicked cast is still gated on what the census pays; only
// the Convoke/Improvise/Delve credit cards leave that gate when the walk bit is
// unset.
func TestKickedOfferWalkBitKeepsAffordability(t *testing.T) {
	carrier := kop.CardDef{
		Name: "Test PlainKicker", Types: kop.TCreature,
		Keywords: kop.KwKicker, Kicker: kop.ManaCost{Generic: 3},
		Cost: kop.ManaCost{Generic: 2},
	}
	sc, o := mpocScen(t, "tapall", carrier, Config{NoElide: true, GorgeAutopaySurface: true}, false,
		"Island", "Island", "Island", "Island")
	s := sc.s
	s.g.availX |= availWalk
	if kickedOffered(sc, o) {
		t.Errorf("the {5} kicked cast was offered in the walk with four Islands")
	}
	sc2, o2 := mpocScen(t, "tapall", carrier, Config{NoElide: true, GorgeAutopaySurface: true}, false,
		"Island", "Island", "Island", "Island", "Island")
	sc2.s.g.availX |= availWalk
	if !kickedOffered(sc2, o2) {
		t.Errorf("the {5} kicked cast was withheld in the walk with five Islands")
	}
}
