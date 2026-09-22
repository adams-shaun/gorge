package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

func taleSharesFixture(t *testing.T) (*fakeHost, state.ObjID, state.ObjID, state.ObjID) {
	t.Helper()
	h, same1, same2, unlike, _ := sharesAllFixture(t)
	ids := []state.ObjID{same1, same2, unlike}
	for _, id := range ids {
		o := h.g.Obj(id)
		if o == nil {
			t.Fatalf("fixture object %d is missing", id)
		}
		o.Zone = state.ZGraveyard
	}
	h.g.SetZone(state.ZGraveyard, 0, ids)
	return h, same1, same2, unlike
}

func TestSharesCardTypeWithOtherClassified(t *testing.T) {
	h, same1, same2, unlike := taleSharesFixture(t)
	g := h.g
	spec := "Card.sharesCardTypeWithOther Remembered"
	if got := UnknownPredicates(spec); len(got) != 0 {
		t.Fatalf("UnknownPredicates(%q) = %v, want []", spec, got)
	}
	if got := UnknownPredicates("Card.sharesCardTypeWithOther Sacrificed"); len(got) == 0 {
		t.Fatal("unsupported referent was accepted")
	}

	sc := SpecContext{You: 0, Remembered: []state.Target{{Obj: same1}, {Obj: same2}}}
	if !MatchesObjectCtx(g, spec, g.Obj(same1), sc) || !MatchesObjectCtx(g, spec, g.Obj(same2), sc) {
		t.Fatal("distinct same-type objects must match")
	}
	sc = SpecContext{You: 0, Remembered: []state.Target{{Obj: same1}, {Obj: unlike}}}
	if MatchesObjectCtx(g, spec, g.Obj(same1), sc) {
		t.Fatal("unlike objects must not match")
	}
	sc = SpecContext{You: 0, Remembered: []state.Target{{Obj: same1}}}
	if MatchesObjectCtx(g, spec, g.Obj(same1), sc) {
		t.Fatal("a remembered set containing only the candidate must not match")
	}
}

func TestRememberedValidSharesCardTypeWithOtherCount(t *testing.T) {
	h, same1, same2, unlike := taleSharesFixture(t)
	body := "Remembered$Valid Card.sharesCardTypeWithOther Remembered"

	c := &Ctx{Controller: 0, Remembered: []state.Target{{Obj: same1}, {Obj: same2}}, SVars: map[string]string{
		"MilledSharesType": body,
	}}
	if n, ok := EvalCountOK(h, c, body); !ok || n != 2 {
		t.Fatalf("EvalCountOK(two same-type cards) = %d,%v, want 2,true", n, ok)
	}
	holds, evaluated := repeatGateHolds(h, c, "MilledSharesType", "GE2")
	if !holds || !evaluated {
		t.Fatalf("repeatGateHolds(two same-type cards) = %v,%v, want true,true", holds, evaluated)
	}

	c.Remembered = []state.Target{{Obj: same1}, {Obj: unlike}}
	if n, ok := EvalCountOK(h, c, body); !ok || n != 0 {
		t.Fatalf("EvalCountOK(unlike cards) = %d,%v, want 0,true", n, ok)
	}
	holds, evaluated = repeatGateHolds(h, c, "MilledSharesType", "GE2")
	if holds || !evaluated {
		t.Fatalf("repeatGateHolds(unlike cards) = %v,%v, want false,true", holds, evaluated)
	}

	c.Remembered = []state.Target{{Obj: same1}}
	if n, ok := EvalCountOK(h, c, body); !ok || n != 0 {
		t.Fatalf("EvalCountOK(self only) = %d,%v, want 0,true", n, ok)
	}
}
