package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// selfDamageTable appends a 3/3 creature carrying ReplPreventDamageCounters
// and a can't-put-counters enchantment (any +1/+1 placement, any permanent).
func selfDamageTable() (tbl *kop.Table, host, ward, dbl, plus CardID) {
	t := *v.Table
	t.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	t.Filters = append([]kop.Filter(nil), v.Table.Filters...)
	t.Exprs = []kop.Expr{{}} // prepareGen only runs a table with generated side tables
	fid := kop.FilterID(len(t.Filters))
	t.Filters = append(t.Filters, kop.Filter{})
	host = CardID(len(t.Cards))
	t.Cards = append(t.Cards, kop.CardDef{Name: "Test Self Prevent", Types: kop.TCreature, Power: 3, Toughness: 3,
		Abilities: []kop.Ability{{Kind: kop.AbReplacement, Repl: kop.Repl{Kind: kop.ReplPreventDamageCounters}}}})
	ward = CardID(len(t.Cards))
	t.Cards = append(t.Cards, kop.CardDef{Name: "Test No Counters", Types: kop.TEnchantment,
		Abilities: []kop.Ability{{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatCantPutCounter, Filter: fid, A: int16(kop.CtrP1P1)}}}})
	dbl = CardID(len(t.Cards))
	t.Cards = append(t.Cards, kop.CardDef{Name: "Test Doubler", Types: kop.TEnchantment,
		Abilities: []kop.Ability{{Kind: kop.AbReplacement, Repl: kop.Repl{Kind: kop.ReplDoubleCountersOn, Filter: fid}}}})
	plus = CardID(len(t.Cards))
	t.Cards = append(t.Cards, kop.CardDef{Name: "Test Plus One", Types: kop.TEnchantment,
		Abilities: []kop.Ability{{Kind: kop.AbReplacement, Repl: kop.Repl{Kind: kop.ReplAddCountersOn, A: 1, Filter: fid}}}})
	return &t, host, ward, dbl, plus
}

// TestSelfDamageCountersLedger: the +1/+1 counters Anti-Venom's replacement
// places during a combat damage pass (no stack object resolving, so
// counterAdder() is "none") are recorded in the counters-added-this-turn
// ledger under the putter the placement used, the replacement's controller
// (gorge replacementBodyCounterAdder); and a prohibited placement asks no
// counter-replacement order even when two mixed replacements would match.
func TestSelfDamageCountersLedger(t *testing.T) {
	tbl, hostCard, _, _, _ := selfDamageTable()
	sc := newScenTable(t, tbl, Config{NoElide: true})
	h := sc.perm(hostCard, 1)
	hs := sc.slot(h)
	sc.k.anyCtrAdded = true
	if sc.s.counterAdder() != 255 {
		t.Fatalf("counterAdder %d outside a resolution, want 255", sc.s.counterAdder())
	}
	if got := sc.s.selfDamage(sc.k, hs, true, 3); got != 0 {
		t.Fatalf("combat damage dealt %d, want 0 (prevented)", got)
	}
	if n := sc.s.perms[hs].ctrLow(int(kop.CtrP1P1)); n != 3 {
		t.Fatalf("host holds %d +1/+1 counters, want 3", n)
	}
	if sc.s.g.ctrAdd.pl&(1<<1) == 0 {
		t.Fatalf("ledger actor mask %b, want seat 1 (the host's controller) recorded", sc.s.g.ctrAdd.pl)
	}
	if sc.s.g.ctrAdd.pl&(1<<0) != 0 {
		t.Fatalf("ledger actor mask %b records seat 0", sc.s.g.ctrAdd.pl)
	}
}

// TestSelfDamageCountersBlocked: a doubler and a plus-one beside the host
// pose the counter-replacement order, which stalls a self-damage placement;
// a can't-put-counters static on the host places nothing and asks nothing.
func TestSelfDamageCountersBlocked(t *testing.T) {
	for _, blocked := range []bool{false, true} {
		tbl, hostCard, wardCard, dblCard, plusCard := selfDamageTable()
		sc := newScenTable(t, tbl, Config{NoElide: true})
		hs := sc.slot(sc.perm(hostCard, 1))
		sc.perm(dblCard, 1)
		sc.perm(plusCard, 1)
		if blocked {
			sc.perm(wardCard, 0)
		}
		if c := sc.s.ctrGather(sc.k, hs, int(kop.CtrP1P1)); !c.ok || c.n != 2 || !c.mixed() {
			t.Fatalf("blocked=%v: gather ok=%v n=%d, want two mixed candidates", blocked, c.ok, c.n)
		}
		if got := sc.s.putCounterBlocked(sc.k, hs, int(kop.CtrP1P1)); got != blocked {
			t.Fatalf("blocked=%v: putCounterBlocked %v", blocked, got)
		}
		sc.s.selfDamage(sc.k, hs, false, 2)
		n := sc.s.perms[hs].ctrLow(int(kop.CtrP1P1))
		if blocked {
			if sc.s.Stalled() || n != 0 {
				t.Errorf("prohibited placement: stalled %q, %d counters; want neither", StallName(sc.s.StallReason()), n)
			}
		} else if got := sc.s.StallReason(); got != StallReplCtr || n != 0 {
			t.Errorf("open placement: stall %q, %d counters; want replctr and none", StallName(got), n)
		}
	}
}
