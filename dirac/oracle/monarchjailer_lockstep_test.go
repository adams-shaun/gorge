package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Palace Jailer's exile until an opponent becomes the monarch (CR 610.3,
// 610.3b), in lockstep against gorge: the exile and its one-shot promise
// (ExtMonarchJail, dirac/act_monarchjail.go), the creature's return when an
// opponent of the Jailer's controller takes the crown, and the skip when one
// did so after the exile ability was put on the stack. The decks hold Jailers
// beside crown makers (Throne of the High City is an instant-speed crown) and
// fliers so the crown changes hands while a creature is exiled.

// monarchJailerDecks are four 40-card decks of the monarchreads pool; seats 0
// and 1 are the two-seat match.
func monarchJailerDecks() [4][]cardset.Entry {
	return [4][]cardset.Entry{
		{ // W/U: Jailers and fliers
			{Name: "Palace Jailer", Count: 4}, {Name: "Wind Drake", Count: 4}, {Name: "Suntail Hawk", Count: 4},
			{Name: "Savannah Lions", Count: 3}, {Name: "Throne of the High City", Count: 3}, {Name: "Plains", Count: 10}, {Name: "Island", Count: 12},
		},
		{ // W/U: Jailers and the crown makers
			{Name: "Palace Jailer", Count: 3}, {Name: "Court of Grace", Count: 2}, {Name: "Palace Sentinels", Count: 3},
			{Name: "Wind Drake", Count: 4}, {Name: "Suntail Hawk", Count: 3}, {Name: "Throne of the High City", Count: 4},
			{Name: "Plains", Count: 10}, {Name: "Island", Count: 11},
		},
		{ // R/B: crown makers, creatures to jail and burn
			{Name: "Queen Marchesa", Count: 3}, {Name: "Court of Embereth", Count: 2}, {Name: "Crimson Fleet Commodore", Count: 3},
			{Name: "Goblin Piker", Count: 4}, {Name: "Hill Giant", Count: 4}, {Name: "Throne of the High City", Count: 3},
			{Name: "Shock", Count: 2}, {Name: "Mountain", Count: 10}, {Name: "Swamp", Count: 9},
		},
		{ // W/U: Jailers beside the readers
			{Name: "Palace Jailer", Count: 3}, {Name: "Isamaru, Hound of Konda", Count: 3}, {Name: "Wind Drake", Count: 4},
			{Name: "Suntail Hawk", Count: 4}, {Name: "Palace Sentinels", Count: 3}, {Name: "Throne of the High City", Count: 3},
			{Name: "Plains", Count: 10}, {Name: "Island", Count: 10},
		},
	}
}

func newMonarchJailerPod(t *testing.T) monarchReadsPod {
	t.Helper()
	p := newMonarchReadsPod(t)
	decks := monarchJailerDecks()
	lists := make([][]cardset.Entry, len(decks))
	for i, d := range decks {
		n := 0
		for _, e := range d {
			n += e.Count
		}
		if n != 40 {
			t.Fatalf("deck %d: %d cards", i, n)
		}
		lists[i] = d
	}
	tbl, _, err := p.set.MatchupN(lists)
	if err != nil {
		t.Fatal(err)
	}
	p.tbl, p.decks = tbl, decks
	return p
}

const monarchJailerSeeds = 40

// TestMonarchReadsJailerLockstep plays four-seat pods with Palace Jailer:
// every decision matches gorge and the kernel never stalls, a creature is
// exiled and comes back when an opponent takes the crown, and an opponent's
// crowning while the exile ability is on the stack leaves the creature alone
// (CR 610.3b).
func TestMonarchReadsJailerLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	p := newMonarchJailerPod(t)
	st := &dirac.MonarchJailStats{}
	dirac.DebugMonarchJailStats = st
	defer func() { dirac.DebugMonarchJailStats = nil }()
	for seed := uint64(1); seed <= monarchJailerSeeds; seed++ {
		res, _ := p.play(t, seed, nil)
		if res.Div != nil {
			lo := max(res.Div.At-12, 0)
			t.Fatalf("seed %d: divergence at %d: %s %s %v stall=%q over=%v\nref %q\nker %q\n%s last %s\nlast actions %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Stall, res.Over,
				res.Div.RefOptions, res.Div.KerOptions, res.Div.Detail, res.Div.LastSourced, actionStrings(res.Actions[lo:res.Div.At]))
		}
		if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
	}
	t.Logf("kernel: %+v", *st)
	if st.Exiles == 0 || st.Returns == 0 {
		t.Fatalf("no Jailer exile came back: %+v", *st)
	}
	if st.Skips == 0 {
		t.Fatalf("no CR 610.3b skip happened (an opponent crowned while the exile ability was on the stack): %+v", *st)
	}
}

// TestMonarchReadsJailerTwoSeatLockstep is the two-seat form.
func TestMonarchReadsJailerTwoSeatLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	st := &dirac.MonarchJailStats{}
	dirac.DebugMonarchJailStats = st
	defer func() { dirac.DebugMonarchJailStats = nil }()
	d := monarchJailerDecks()
	if div := lockstepRunPolicy(t, "monarchreads", [2][]cardset.Entry{d[0], d[1]}, monarchJailerSeeds, true, nil, nil, castFirstPolicy); div != 0 {
		t.Errorf("%d of %d seeds diverged", div, monarchJailerSeeds)
	}
	t.Logf("kernel: %+v", *st)
	if st.Exiles == 0 || st.Returns == 0 {
		t.Fatalf("no Jailer exile came back: %+v", *st)
	}
}

// TestMonarchJailerMutation makes the Jailer's own controller's crowning
// return the card too (dirac.DebugMonarchJailAnyCrown): the promise then
// names every player, so some seed diverges from gorge.
func TestMonarchJailerMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugMonarchJailAnyCrown = true
	defer func() { dirac.DebugMonarchJailAnyCrown = false }()
	d := monarchJailerDecks()
	if !lockstepDiverges(t, "monarchreads", [2][]cardset.Entry{d[0], d[1]}, monarchJailerSeeds) {
		t.Fatal("no seed diverged with the Jailer's own crowning returning the card")
	}
}
