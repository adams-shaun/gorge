package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// filtDeadEndBoard: the mbc board of TestFiltScriptStallPropagates without the
// filter: Thermopod's "Sacrifice a creature: Add {R}" is an other gap of the
// filter search, funded by the Plains' {1} for Abrade ({1}{R}). samurai is how
// many Samurai of the Pale Curtain compete for the sacrificed creature.
func filtDeadEndBoard(t *testing.T, samurai int) (*scen, Obj, Slot, Slot) {
	t.Helper()
	set := cardsettest.Set(t, "mbc")
	names := []string{"Apprentice Wizard", "Thermopod", "Samurai of the Pale Curtain", "Abrade", "Island", "Havoc Jester"}
	deck := []cardset.Entry{}
	for _, n := range names {
		i, ok := set.Lookup(n)
		if !ok {
			t.Fatalf("%s not in the mbc pool", n)
		}
		if !set.Supported()[i] {
			t.Fatalf("%s not supported in the mbc table", n)
		}
		c := 1
		if n == "Samurai of the Pale Curtain" {
			c = samurai
		}
		deck = append(deck, cardset.Entry{Name: n, Count: c})
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Island"], Config{NoElide: true})
	sc.perm(id["Island"], 0)
	sc.perm(id["Apprentice Wizard"], 0)
	sc.perm(id["Havoc Jester"], 0)
	th := sc.perm(id["Thermopod"], 0)
	sl := sc.perm(id["Samurai of the Pale Curtain"], 0)
	if samurai > 1 {
		sc.perm(id["Samurai of the Pale Curtain"], 0)
	}
	abrade := sc.hand(id["Abrade"], 0)
	return sc, abrade, sc.slot(th), sc.slot(sl)
}

// TestFiltDeadEndStallPropagates: filtDeadEnd replays a path the filter search
// found and declares the stall its replay latches. The path is Thermopod's
// sacrifice step (the search's own other gap): with one Samurai of the Pale
// Curtain the search finds and keeps the play, proving the step is the
// search's; with two the sacrifice dies under two competing exile-instead
// replacements, so the replay's dieZone latches StallReplLeave. filtDeadEnd
// declares it on the real state instead of dropping the found play as a dead
// end (TestTapXScriptStallPropagates's mechanism).
func TestFiltDeadEndStallPropagates(t *testing.T) {
	// Control: one replacement, the real search finds and keeps the play.
	{
		sc, abrade, _, _ := filtDeadEndBoard(t, 1)
		s, k := sc.s, sc.k
		a := MakeAction(AkCast, TCode(abrade), 0, 0)
		if s.Stalled() {
			t.Fatalf("precondition: the board alone latched %s", StallName(s.StallReason()))
		}
		if !s.filtScript(k, a, 0, false) {
			t.Fatal("control: the search did not find the Thermopod sacrifice path")
		}
		if s.Stalled() {
			t.Fatalf("control: the search latched %s, want a priced path", StallName(s.StallReason()))
		}
	}
	// Latch: the same path replayed under two replacements.
	sc, abrade, thermopod, samurai := filtDeadEndBoard(t, 2)
	s, k := sc.s, sc.k
	a := MakeAction(AkCast, TCode(abrade), 0, 0)
	if s.Stalled() {
		t.Fatalf("precondition: the board alone latched %s", StallName(s.StallReason()))
	}
	if !s.otherGaps(k, 0, a).has(thermopod) {
		t.Fatal("precondition: Thermopod is no other gap of the filter search")
	}
	var fs filtSearch
	fs.a, fs.p, fs.gaps, fs.own, fs.sp0 = a, 0, s.scriptFilt(k, 0), NoSlot, s.sp
	fs.need = s.playNeed(k, a)
	fs.other = s.otherGaps(k, 0, a)
	fs.fees = s.filtFunding(k, 0, fs.gaps)
	fs.path[0] = filtStep{sl: thermopod, gap: true, other: otherStep{kind: otherTapX, cand: samurai}}
	fs.n = 1
	if !s.filtDeadEnd(k, 0, a, &fs) {
		t.Fatal("the dead end kept a play whose replay's sacrifice stalls")
	}
	if !s.Stalled() || s.StallReason() != StallReplLeave {
		t.Fatalf("the dead end dropped the replay's stall silently: stalled %v reason %s, want stalled %s",
			s.Stalled(), StallName(s.StallReason()), StallName(StallReplLeave))
	}
}
