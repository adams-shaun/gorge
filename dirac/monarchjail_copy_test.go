package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestMonarchJailCopyKeepsTheMark (CR 610.3b, 707.10; gorge keeps an ability
// copy's PushMonarchSeq): Palace Jailer's exile ability is on the stack when
// the opponent becomes the monarch, so it exiles nothing; Strionic Resonator's
// copy of it carries the same mark and exiles nothing either.
func TestMonarchJailCopyKeepsTheMark(t *testing.T) {
	set := cardsettest.Set(t, "monarchjailcopy")
	deck := []cardset.Entry{{Name: "Grizzly Bears", Count: 1}, {Name: "Palace Jailer", Count: 1},
		{Name: "Strionic Resonator", Count: 1}, {Name: "Plains", Count: 1}, {Name: "Island", Count: 1}}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
	s := sc.s
	st := &MonarchJailStats{}
	DebugMonarchJailStats = st
	defer func() { DebugMonarchJailStats = nil }()
	res := sc.perm(id["Strionic Resonator"], 0)
	bears := sc.perm(id["Grizzly Bears"], 1)
	s.pl[0].pool[kop.W] = 8
	jailer := sc.hand(id["Palace Jailer"], 0)
	sc.castCard(t, jailer)
	// resolve the Jailer; stop at the first priority with abilities on the stack
	for i := 0; i < 40 && !(s.sp > 0 && s.stack[s.sp-1].kind == seTrigger && s.ctl == ctlPriority && s.npend == 0); i++ {
		d := s.Step()
		if d.Kind == DkPriority {
			if s.sp > 0 && s.stack[s.sp-1].kind == seTrigger {
				break
			}
			s.Apply(MakeAction(AkPass, 0, 0, 0))
			continue
		}
		s.Apply(d.Options[0])
	}
	if s.sp == 0 || s.stack[s.sp-1].kind != seTrigger {
		t.Fatalf("no Jailer trigger on the stack (sp %d)", s.sp)
	}
	// the opponent becomes the monarch: every ability on the stack is marked
	s.becomeMonarch(sc.k, 1)
	marked := 0
	for i := 0; i < int(s.sp); i++ {
		if s.stack[i].flags&sfOppCrowned != 0 {
			marked++
		}
	}
	if marked == 0 {
		t.Fatalf("no stack ability marked after the opponent's crowning")
	}
	// copy the exile trigger (the stack entry holding ExtMonarchJail) with Resonator
	exile := -1
	for i := 0; i < int(s.sp); i++ {
		for _, in := range tbl.Cards[s.stack[i].card].Abilities[s.stack[i].ab].Prog {
			if in.Op == kop.OpExt && in.Ext() == kop.ExtMonarchJail {
				exile = i
			}
		}
	}
	if exile < 0 {
		t.Fatalf("no exile ability on the stack")
	}
	d := s.Step()
	activated := false
	for i := 0; i < int(d.N) && !activated; i++ {
		if a := d.Options[i]; a.Kind() == AkActivate && a.Source() == TCode(uint8(res)) {
			s.Apply(a)
			activated = true
		}
	}
	if !activated {
		t.Fatalf("Resonator's copy not offered: %v", d.Options[:d.N])
	}
	sc.do(t, AkTarget, scAny, StackTarget0+TCode(exile))
	sc.resolveAll(t)
	if !sc.onBF(bears) {
		t.Fatalf("the creature was exiled by an ability marked by the crowning (stats %+v)", *st)
	}
	if st.Exiles != 0 || st.Skips < 2 {
		t.Fatalf("stats %+v, want no exile and the original and the copy both skipped", *st)
	}
}
