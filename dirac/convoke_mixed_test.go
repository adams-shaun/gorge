package dirac

import "testing"

// TestConvokeMixedExpend: a Convoke (granted) plus Improvise (printed) cast
// counts its convoke picks in the expend tally but not its improvise picks;
// CastTotalManaSpent moves by the mana alone.
func TestConvokeMixedExpend(t *testing.T) {
	sc, m, objs := stackKwBoardScen(t, "Kappa Cannoneer")
	sc.k.gen.anyHist = true
	sc.k.gen.anySpent = true
	sc.do(t, AkCast, TCode(m), scAny)
	d := sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChGenConvoke {
		t.Fatalf("first decision %v/%d, want the convoke ask", d.Kind, d.Sub)
	}
	pickMode(t, sc, objs["Chief Engineer"], 1) // convoke_U
	pickMode(t, sc, objs["Mind Stone"], 6)     // improvise_generic
	sc.do(t, AkDone, scAny, scAny)
	if d = sc.s.Step(); d.Kind != DkPriority {
		t.Fatalf("decision after payment %v/%d, want priority", d.Kind, d.Sub)
	}
	// {5}{U}: MV 6; the improvise pick is no mana for the expend tally.
	if got := sc.s.g.spent[0]; got != 5 {
		t.Fatalf("expend tally %d, want 5", got)
	}
	found := false
	for i := 0; i < int(sc.s.sp); i++ {
		if e := &sc.s.stack[i]; e.kind == seSpell && e.obj == m {
			if e.trigAmt != 4 {
				t.Fatalf("CastTotalManaSpent %d, want 4", e.trigAmt)
			}
			found = true
		}
	}
	if !found {
		t.Fatalf("Kappa Cannoneer not on the stack")
	}
}

// pickMode answers the pending ask with o's option of the given mode.
func pickMode(t *testing.T, sc *scen, o Obj, mode uint8) {
	t.Helper()
	d := sc.s.Step()
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkChoose && Obj(a.Source()) == o && a.Mode() == mode {
			sc.s.Apply(a)
			return
		}
	}
	t.Fatalf("pick %d/%d not offered: %v", o, mode, d.Options[:d.N])
}
