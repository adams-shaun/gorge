package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// bearScen: p0 has two Abzan Devotees ("{1}: Add {W}, {B}, or {G}. Activate
// only once each turn") and two Plains, and Grizzly Bears ({1}{G}) in hand;
// the Bears' cast is a scripted potential play over the Devotees' filter
// (gorge's census misses it, TestDevoteeFilterPays's shape).
func bearScen(t *testing.T, lands ...string) (sc *scen, bears, d1, d2 Obj) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Abzan Devotee", Count: 2}, {Name: "Grizzly Bears", Count: 1},
		{Name: "Plains", Count: 8}, {Name: "Swamp", Count: 8}}
	set := cardsettest.Set(t, "manaabmanacost")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc = newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
	for _, n := range lands {
		sc.perm(id[n], 0)
	}
	d1 = sc.perm(id["Abzan Devotee"], 0)
	d2 = sc.perm(id["Abzan Devotee"], 0)
	bears = sc.hand(id["Grizzly Bears"], 0)
	return sc, bears, d1, d2
}

// filtSearchVisit drives the filter search the way filtScript sets it up and
// returns whether it found a script and the path it found (reorders only:
// the answer set is the ask's either way).
func filtSearchVisit(s *State, k *prep, a Action, need [kop.NumManaKinds]bool) (bool, []filtStep) {
	var fs filtSearch
	fs.a, fs.p, fs.gaps, fs.own, fs.sp0 = a, 0, s.scriptFilt(k, 0), NoSlot, s.sp
	fs.need = need
	// the cast's filter search (filtScript): a cast may count the grants (noGrant false)
	fs.other = s.otherGaps(k, 0, a)
	if s.filtRelaxable(k, fs.gaps) && !(k.tapMana && s.tapXTwoLive(k, 0)) {
		for b := fs.gaps; b.any(); b = b.rest() {
			fs.fees += s.filtFee(k, b.first())
		}
	}
	t := s.fork()
	t.g.noGrant = true
	ok := fs.visit(k, t, 0, false)
	s.unfork()
	return ok, append([]filtStep{}, fs.path[:fs.n]...)
}

// TestFiltNeedFirstColours: the play's announced colours (need) order the
// filter search's colour ask (filtColours's needFirst, gorge's
// scriptOrderAnswers on the same ask): a budget-capped search answers the
// first ask with a colour the cast announces before the colours it does
// not, so the Bears' {1}{G} script answers the first Devotee's ask with G
// and completes there. With no need (an alternative cost leaves the set
// empty) the search keeps the engine order, W U B R G: G is tried last, so
// the first completing answer is B -- W's node is seen-pruned after the
// ask's {C} answer visited its key, and B completes through the second
// Devotee's G. The answer set is the same either way: both runs find a
// script that pays the cast, G among the answers.
func TestFiltNeedFirstColours(t *testing.T) {
	sc, bears, d1, d2 := bearScen(t, "Plains", "Plains")
	s, k := sc.s, sc.k
	// preconditions: both Devotees are census filter gaps (no otherGap), the
	// cast is offered over them, and {1}{G} announces only G.
	gaps := s.scriptFilt(k, 0)
	if !gaps.has(sc.slot(d1)) || !gaps.has(sc.slot(d2)) {
		t.Fatalf("a Devotee is no census filter gap: %v", gaps)
	}
	if s.otherGaps(k, 0, MakeAction(AkPass, 0, 0, 0)).any() {
		t.Fatal("the Devotees are not pure filter gaps (otherGaps non-empty)")
	}
	var cast Action
	found := false
	d := s.Step()
	for _, a := range d.Options[:d.N] {
		if a.Kind() == AkCast && Obj(a.Source()) == bears {
			cast, found = a, true
		}
	}
	if !found {
		t.Fatal("Grizzly Bears's {1}{G} cast not offered over the Devotees")
	}
	need := s.playNeed(k, cast)
	if !need[kop.G] || need[kop.W] || need[kop.B] || need[kop.C] {
		t.Fatalf("need %v, want only G ({1}{G} announces green)", need)
	}
	first := func(path []filtStep) (Slot, uint8) {
		for _, st := range path {
			if st.gap && st.ans > 0 {
				return st.sl, st.ans - 1
			}
		}
		return NoSlot, 255
	}
	madeG := func(path []filtStep) bool {
		for _, st := range path {
			if st.gap && st.ans > 0 && st.ans-1 == kop.G {
				return true
			}
		}
		return false
	}
	ok, pn := filtSearchVisit(s, k, cast, need)
	if !ok {
		t.Fatal("no script with the play's need set")
	}
	sN, aN := first(pn)
	ok, pe := filtSearchVisit(s, k, cast, [kop.NumManaKinds]bool{})
	if !ok {
		t.Fatal("no script with an empty need set")
	}
	sE, aE := first(pe)
	if sN != sc.slot(d1) || sE != sN {
		t.Fatalf("first ask from slots %d (need) and %d (empty), want the same first Devotee's (%d)",
			sN, sE, sc.slot(d1))
	}
	if aN != kop.G {
		t.Fatalf("with the need set the first ask was answered colour %d, want G", aN)
	}
	if aE != kop.B {
		t.Fatalf("with an empty need set the first ask was answered colour %d, want B (the engine order's first "+
			"completing answer: W's node is seen-pruned after the ask's {C} answer, B's completes through the "+
			"second Devotee's G)", aE)
	}
	if !madeG(pe) {
		t.Fatal("the empty-need run never answered G: the order changed the ask's answer set")
	}
}
