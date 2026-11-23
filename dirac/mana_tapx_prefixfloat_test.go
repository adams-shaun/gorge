package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestTapxPrefixFloatSkipsUnfundablePip: payNeededColours (mana_resolutionpay.go)
// pre-floats a tapX-funded activation's needed-colour share before the target
// ask, one unit of a pip colour per iteration. A pip colour the board can fund
// no more of must not be demanded: the pre-fix loop asked for one NEW unit of
// every pip colour unconditionally, and tapFor panicked ("kernel: autopay
// found no plan") on a legal, offered, fully payable play. Board (the
// dr-mbc-baba pool's probe pair): Nephalia Smuggler's {3}{U},{T} exile
// activation is offered with NTargets=1; the tapX prefix floats Jaspera
// Sentinel's unit as {U} — the board's only {U} — and the four Swamps fund the
// {3} generic at the post-push payment. payNeededColours then asked for a
// second {U}, found no plan, and crashed at the activation's apply.
func TestTapxPrefixFloatSkipsUnfundablePip(t *testing.T) {
	names := []string{"Nephalia Smuggler", "Jaspera Sentinel", "Nirkana Cutthroat", "Swamp", "Plains"}
	deck := []cardset.Entry{}
	for _, n := range names {
		deck = append(deck, cardset.Entry{Name: n, Count: 1})
	}
	deck[3].Count = 4 // the four Swamps fund the {3} generic
	sc, id := newSetScenTweak(t, "mbc", deck, nil)
	s := sc.s
	sm := sc.perm(id["Nephalia Smuggler"], 0)
	sc.perm(id["Jaspera Sentinel"], 0)
	sc.perm(id["Nirkana Cutthroat"], 0)
	for i := 0; i < 4; i++ {
		sc.perm(id["Swamp"], 0)
	}
	if s.Stalled() {
		t.Fatalf("precondition: the board alone latched %s", StallName(s.StallReason()))
	}
	// the offered, payable activation: applyPriority pre-floats the pip share
	// before the target ask
	sc.do(t, AkActivate, TCode(sm), scAny)
	// the activation's target ask appears (the exile's target), answered below
	d := s.Step()
	if d.Kind != DkTarget {
		t.Fatalf("after the activation the decision is kind %d, want the exile's target ask", d.Kind)
	}
	if len(d.Options) < 2 || d.N < 1 {
		t.Fatalf("the target ask offers %d options, want the board's other creatures", d.N)
	}
	// answer the ask (any offered creature but the Smuggler itself)
	if s.objs[sm].zone != kop.ZoneBattlefield {
		t.Fatal("precondition: the Smuggler left the battlefield at its own activation")
	}
	var answer Action
	for i := uint16(0); i < d.N; i++ {
		a := d.Options[i]
		if a.Kind() != AkTarget {
			continue
		}
		o := Obj(a.Target())
		if o != sm && s.objs[o].zone == kop.ZoneBattlefield {
			answer = a
			break
		}
	}
	if answer == 0 {
		t.Fatal("the target ask offers no other creature")
	}
	s.Apply(answer)
	// the play resolves through its payment: the {3} generic leaves the
	// Swamps, the floating {U} the pip, and nothing floats on
	sawPriority := false
	for i := 0; i < 30; i++ {
		d := s.Step()
		if d.Kind == DkPriority {
			sawPriority = true
			break
		}
		if d.N == 0 {
			t.Fatalf("decision kind %d sub %d offers no answer", d.Kind, d.Sub)
		}
		s.Apply(d.Options[0])
	}
	if !sawPriority {
		t.Fatal("the activation never resolved back to priority")
	}
	if n := s.prefixPoolMana(0); n != 0 {
		t.Fatalf("pool holds %d mana after the activation's payment, want 0", n)
	}
	if !sc.tapped(sm) {
		t.Fatal("the Smuggler's {T} was not paid")
	}
}
