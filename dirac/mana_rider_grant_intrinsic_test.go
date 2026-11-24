package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestRiderGrantIntrinsicActivation: the Taps-deferred gap route still lists
// a bounded rider source under a granted basic land type (scriptTapsDefer's
// planDeferred branch), and its activation reaches manaTapped with the
// intrinsic view manaPCGen returns there (blgView). The tap is the rider's
// own activation, not the intrinsic unit's: the rider resolves with the mana
// (CR 605.3b). Twitching Doll's "{T}: Add one mana of any color. Put a nest
// counter on this" leaves a nest counter on itself beside its mana, and the
// gap's answers are the rider's colours, not the intrinsic view's.
func TestRiderGrantIntrinsicActivation(t *testing.T) {
	sc, _, objs := blgRiderBoard(t, Config{}, "Twitching Doll")
	o := objs["Twitching Doll"]
	sl := sc.slot(o)
	kind, n, ok := cardManaRider(&sc.k.t.Cards[sc.s.cardAt(sl)])
	if !ok || kind == 0 || n != 1 {
		t.Fatalf("precondition: Twitching Doll's first mana ability carries no counter-on-self rider (kind %d n %d ok %v)", kind, n, ok)
	}
	if pc := sc.s.manaPCGen(sc.k, sl); !pc.manaIntrinsic {
		t.Fatal("precondition: manaPCGen does not return the intrinsic view: no granted basic land type on the doll")
	}
	if !sc.s.scriptTapsDefer(sc.k, 0).has(sl) {
		t.Fatal("precondition: the doll is not a Taps-deferred gap under the grant")
	}
	if got := sc.s.deferGapAnswers(sc.k, sl); got != 5 {
		t.Fatalf("precondition: the gap's answers %d, want 5: the rider's production is any colour (ColAny)", got)
	}
	life := sc.s.pl[0].life
	var spill [kop.NumManaKinds]uint8
	sc.s.tapGapChoice(sc.k, 0, 0, sl, slotMask{}, slotMask{}, slotMask{}.with(sl), 0, &spill) // the first answer: {W}
	if got := sc.s.perms[sl].ctrN(sc.s, kind); got != 1 {
		t.Fatalf("nest counters %d, want 1: the rider resolves with the activation's mana (CR 605.3b)", got)
	}
	if got := sc.s.pl[0].pool[kop.W]; got != 1 {
		t.Fatalf("{W} in pool %d, want 1: the gap floats the rider's answer, not the intrinsic unit's", got)
	}
	if sc.s.pl[0].life != life {
		t.Fatalf("life %d, want %d: the activation costs no life", sc.s.pl[0].life, life)
	}
	if !sc.tapped(o) {
		t.Fatal("the activation left the doll untapped")
	}
}

// TestRiderGrantIntrinsicRiderGap: the ManaRider-flagged depletion class
// keeps its explicit activation under a granted basic land type: riderGap
// removes a mining counter, floats the rider's answer and leaves the source
// alive (its rider's own routes, mana_window.go windowGapArm, tapGapChoice,
// mana_tapanother.go).
func TestRiderGrantIntrinsicRiderGap(t *testing.T) {
	sc, _, objs := blgRiderBoardSubs(t, Config{}, []string{"Island"}, "Gemstone Mine")
	o := objs["Gemstone Mine"]
	sl := sc.slot(o)
	a, ok := sc.s.riderAbilityOf(sc.k, sl)
	if !ok {
		t.Fatal("precondition: Gemstone Mine is not the bounded-rider shape")
	}
	blgRiderReady(sc, sl, int(a.ManaRiderCtr), 3)
	if pc := sc.s.manaPCGen(sc.k, sl); !pc.manaIntrinsic {
		t.Fatal("precondition: manaPCGen does not return the intrinsic view")
	}
	if !sc.s.riderGap(sc.k, 0, sl, 2) { // the third answer: {B}
		t.Fatal("activation refused")
	}
	if got := sc.s.perms[sl].ctrN(sc.s, int(a.ManaRiderCtr)); got != 2 {
		t.Fatalf("mining counters %d, want 2: the activation pays its own cost", got)
	}
	if got := sc.s.pl[0].pool[kop.B]; got != 1 {
		t.Fatalf("{B} in pool %d, want 1: the rider's answer floats", got)
	}
	if !sc.onBF(o) {
		t.Fatal("Gemstone Mine left the battlefield with counters left")
	}
}

// TestRiderGrantIntrinsicCensusTap: the intrinsic unit's census activation
// (availForMode's ordinary unit) runs no rider: the tap is the granted basic
// land type's "{T}: Add [c}", not the card's own rider (blgView,
// trig_taps.go). The doll stays at no nest counters and only the granted
// colour floats.
func TestRiderGrantIntrinsicCensusTap(t *testing.T) {
	sc, _, objs := blgRiderBoard(t, Config{}, "Twitching Doll")
	o := objs["Twitching Doll"]
	sl := sc.slot(o)
	kind, _, ok := cardManaRider(&sc.k.t.Cards[sc.s.cardAt(sl)])
	if !ok {
		t.Fatal("precondition: Twitching Doll has no counter-on-self rider")
	}
	var av avail
	sc.s.availFor(sc.k, 0, &av)
	c := cost(0, kop.W)
	if !av.affords(&c, pipMaskOf(&c), 0) {
		t.Fatal("the intrinsic unit's granted {W} is not available")
	}
	b := cost(0, kop.W)
	sc.s.pay(sc.k, 0, &b, 0, NoObj)
	if got := sc.s.perms[sl].ctrN(sc.s, kind); got != 0 {
		t.Fatalf("nest counters %d, want 0: the census unit's tap runs no rider", got)
	}
}
