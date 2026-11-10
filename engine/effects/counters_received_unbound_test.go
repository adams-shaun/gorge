package effects

import "testing"

// TestCountersReceivedThisTurnUnbound holds the fail-closed contract outside
// the count path: a SpecContext that never had the counter ledger bound
// (a static Filter$, a ValidTgts$, a trigger ValidCard$) answers neither the
// positive nor the '!' spelling, so the negation cannot match every object.
// A bound ledger, even an empty one, does answer both.
func TestCountersReceivedThisTurnUnbound(t *testing.T) {
	h, g, src := playersBoard(t, 2, mkCard(t, "Name:Forge\nTypes:Creature Human\nPT:2/4\nOracle:x\n"))
	mine := addPermanent(t, h, 0, "Creature")
	o := g.Obj(mine)
	if o == nil {
		t.Fatal("test precondition: the permanent is not on the battlefield")
	}
	const word = "countersReceivedThisTurn_GE1_Any_You"

	unbound := NewSpecContext(0, src)
	if unbound.CounterAdds != nil {
		t.Fatal("test precondition: NewSpecContext binds a counter ledger")
	}
	for _, p := range []string{word, "!" + word} {
		if r, ok := matchPredicate(g, p, o, unbound); ok {
			t.Errorf("unbound %s = (%v, ok=true), want ok=false", p, r)
		}
	}

	bound := NewSpecContext(0, src)
	bound.CounterAdds = counterAddsFor(h, word)
	if bound.CounterAdds == nil {
		t.Fatal("counterAddsFor on a host without the role returned nil (unbound), want an empty bound ledger")
	}
	if r, ok := matchPredicate(g, word, o, bound); !ok || r {
		t.Errorf("bound empty %s = (%v, %v), want (false, true)", word, r, ok)
	}
	if r, ok := matchPredicate(g, "!"+word, o, bound); !ok || !r {
		t.Errorf("bound empty !%s = (%v, %v), want (true, true)", word, r, ok)
	}
	bound.CounterAdds = []CounterAdd{{Adder: 0, Obj: mine, Kind: "P1P1", Amount: 1}}
	if r, ok := matchPredicate(g, word, o, bound); !ok || !r {
		t.Errorf("bound marked %s = (%v, %v), want (true, true)", word, r, ok)
	}
}
