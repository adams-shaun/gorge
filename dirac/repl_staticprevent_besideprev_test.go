package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestStaticPreventBesidePrevAskBound: the combat walk's spv candidate bits
// stay inside ro.cand's four slots (bit 7 is spOptionalAsk, a lone optional
// line's skip, never a candidate): a lone static prevention appended beside
// ro.prev's full preventions lands at bit n, and the truncation check
// (n+c > len(ro.cand), repl_staticprevent.go) stalls any append past slot 3
// loudly instead of setting a bit the ask's options would misread.
func TestStaticPreventBesidePrevAskBound(t *testing.T) {
	tb := newSPTable()
	// a prevent-N shield: not bare (spBare) and orderable (spOrderable), the
	// lone static prevention the beside-prevs arm appends
	host := tb.host("Test Sphere", kop.Repl{Kind: kop.ReplPreventDamageN, C: kop.ReplSPYou, A: 2})
	sc := tb.scen(t)
	sc.perm(host, 0)
	giant := sc.perm(v.HillGiant, 1)
	if sc.k.gen == nil || !sc.k.gen.uses.has(useStaticPrev) {
		t.Fatal("precondition: the table does not read the static prevention")
	}
	s := sc.s
	fe := fzEv{src: sc.slot(giant), dst: NoSlot, card: s.objs[giant].card, ctrl: 1, apl: 0, n: 5}
	ro := &s.g.ro
	// the last appendable candidate: three prevs leave the shield bit 3
	ro.prev, ro.shld, ro.plus, ro.pnx, ro.pcol, ro.spv = 0x7, 0, 0, 0, 0, 0
	if n := s.spCombatCands(sc.k, &fe, 1, 0, 3); n != 4 || ro.spv != 1<<3 || s.g.stalled != 0 {
		t.Fatalf("beside three prevs: n %d spv %#x stalled %s; want the shield appended at bit 3, no stall", n, ro.spv, StallName(s.g.stalled))
	}
	// the bound: the walk caps its own candidates at len(ro.cand) before
	// spCombatCands runs, but a forged n past slot 3 must still never set a
	// candidate bit: the truncation check stalls StallReplCands, spv untouched
	for _, n := range []int{4, 6, 7} {
		s.g.stalled = 0
		ro.spv = 0
		ro.prev = uint8(1<<n) - 1
		if got := s.spCombatCands(sc.k, &fe, 1, 0, n); got != -1 || s.g.stalled != StallReplCands || ro.spv&spOptionalAsk != 0 {
			t.Fatalf("beside %d prevs: n %d spv %#x stalled %s; want the loud StallReplCands, never a candidate bit at or past slot 4", n, got, ro.spv, StallName(s.g.stalled))
		}
	}
}
