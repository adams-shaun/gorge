package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// mustTargetPodEntries is the Flagbearer fixture's four decks: the
// MustTarget carriers (and the Aura that grants Flagbearer) beside targeted
// removal and pump spells of each colour, so a seat that opposes the
// Flagbearer controller casts a single-target and a multi-target ask at a
// board where a Flagbearer is a legal target.
func mustTargetPodEntries() [4][]cardset.Entry {
	return [4][]cardset.Entry{
		{
			{Name: "Standard Bearer", Count: 4}, {Name: "Coalition Honor Guard", Count: 3},
			{Name: "Coalition Flag", Count: 2}, {Name: "Grizzly Bears", Count: 4},
			{Name: "Savannah Lions", Count: 4}, {Name: "Giant Growth", Count: 2},
			{Name: "Pacifism", Count: 2}, {Name: "Plains", Count: 10}, {Name: "Forest", Count: 9},
		},
		{
			{Name: "Invigorated Rampage", Count: 3}, {Name: "When We Were Young", Count: 3},
			{Name: "Run Away Together", Count: 2}, {Name: "Lightning Bolt", Count: 4}, {Name: "Grizzly Bears", Count: 4},
			{Name: "Savannah Lions", Count: 4}, {Name: "Plains", Count: 10}, {Name: "Mountain", Count: 10},
		},
		{
			{Name: "Unsummon", Count: 3}, {Name: "Doom Blade", Count: 3}, {Name: "Run Away Together", Count: 4},
			{Name: "Grizzly Bears", Count: 8}, {Name: "Savannah Lions", Count: 4},
			{Name: "Island", Count: 10}, {Name: "Swamp", Count: 10},
		},
		{
			{Name: "Giant Growth", Count: 4}, {Name: "Pacifism", Count: 4},
			{Name: "Grizzly Bears", Count: 8}, {Name: "Savannah Lions", Count: 4},
			{Name: "Plains", Count: 10}, {Name: "Forest", Count: 10},
		},
	}
}

type mustTargetPod struct {
	reg   *cards.Registry
	set   cardset.Set
	tbl   *kop.Table
	decks [4][]cardset.Entry
	four  bool
}

// newMustTargetPodOf builds the fixture's table for a four-seat pod or for
// its two-seat first half (the same Repro decks, no MoreDecks).
func newMustTargetPodOf(t *testing.T, decks [4][]cardset.Entry, four bool) mustTargetPod {
	t.Helper()
	reg := synthReg(t)
	set := cardsettest.Set(t, "musttarget")
	n := 2
	if four {
		n = 4
	}
	lists := make([][]cardset.Entry, n)
	copy(lists, decks[:n])
	tbl, _, err := set.MatchupN(lists)
	if err != nil {
		t.Fatal(err)
	}
	return mustTargetPod{reg: reg, set: set, tbl: tbl, decks: decks, four: four}
}

func newMustTargetPod(t *testing.T, four bool) mustTargetPod {
	t.Helper()
	return newMustTargetPodOf(t, mustTargetPodEntries(), four)
}

// play plays one seed and returns the lockstep result and the MustTarget asks
// the kernel narrowed (the gorge side leads, so a divergence is gorge's
// option list against the kernel's).
func (p mustTargetPod) play(t *testing.T, seed uint64, maxTurns int32) (Result, dirac.MustTargetStats) {
	t.Helper()
	expand := func(d []cardset.Entry) []string {
		var out []string
		for _, e := range d {
			for j := 0; j < e.Count; j++ {
				out = append(out, e.Name)
			}
		}
		return out
	}
	r := &Repro{Format: reproFormat, Decks: [2]string{"x0", "x1"},
		Cards: [2][]string{expand(p.decks[0]), expand(p.decks[1])}, Seed: seed, PassElision: true, Surface: "full"}
	if p.four {
		r.MoreDecks = []string{"x2", "x3"}
		r.MoreCards = [][]string{expand(p.decks[2]), expand(p.decks[3])}
	}
	m, err := r.Matchup(p.reg)
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.MaxDecisions, rc.MaxTurns = 8000, maxTurns
	st := &dirac.MustTargetStats{}
	dirac.DebugMustTargetStats = st
	defer func() { dirac.DebugMustTargetStats = nil }()
	k, err := SetSide(p.set, p.tbl, rc.Config)(m, seed)
	if err != nil {
		t.Fatalf("seed %d: %v", seed, err)
	}
	defer CloseSide(k)
	lead, err := NewGorgeSide(m, seed, rc.Config, nil)
	if err != nil {
		t.Fatal(err)
	}
	res := LockstepLed(m, seed, lead, k, castFirstPolicy(seed), rc)
	return res, *st
}

// mustTargetSeeds are the seeds both seat-count locksteps play.
var mustTargetSeeds = []uint64{1, 2, 3, 4, 5, 6}

// TestMustTargetFourSeatLockstep plays the Flagbearer fixture at four seats:
// every opponent of a Flagbearer's controller is bound, the controller is not,
// and a single-target ask keeps only the Flagbearers offered while a
// multi-target ask still requires one match.
func TestMustTargetFourSeatLockstep(t *testing.T) {
	p := newMustTargetPod(t, true)
	var sum dirac.MustTargetStats
	for _, seed := range mustTargetSeeds {
		res, st := p.play(t, seed, 40)
		if res.Div != nil {
			ventureFail(t, seed, res)
		}
		if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
		sum.Single += st.Single
		sum.Multi += st.Multi
		sum.Partnered += st.Partnered
		t.Logf("seed %d: %d decisions, over %v %s; narrowed asks single=%d multi=%d partnered=%d", seed, res.Decisions, res.Over, res.Stall, st.Single, st.Multi, st.Partnered)
	}
	if sum.Single == 0 || sum.Multi == 0 {
		t.Fatalf("narrowed asks single=%d multi=%d: want both above zero", sum.Single, sum.Multi)
	}
	if sum.Partnered == 0 {
		t.Fatalf("no set-constrained multi-target pick was dropped for want of a matching partner: the fixture never reached it")
	}
}

// TestMustTargetTwoSeatLockstep is the same fixture at two seats: the one
// opponent is bound and the controller is not.
func TestMustTargetTwoSeatLockstep(t *testing.T) {
	p := newMustTargetPod(t, false)
	var sum dirac.MustTargetStats
	for _, seed := range mustTargetSeeds {
		res, st := p.play(t, seed, 40)
		if res.Div != nil {
			ventureFail(t, seed, res)
		}
		if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
		sum.Single += st.Single
		sum.Multi += st.Multi
		t.Logf("seed %d: %d decisions, over %v %s; narrowed asks single=%d multi=%d", seed, res.Decisions, res.Over, res.Stall, st.Single, st.Multi)
	}
	if sum.Single == 0 || sum.Multi == 0 {
		t.Fatalf("narrowed asks single=%d multi=%d: want both above zero", sum.Single, sum.Multi)
	}
}

// TestMustTargetMutation is the mutation test: skipping the narrowing in the
// kernel must diverge on an ask the fixture reaches.
func TestMustTargetMutation(t *testing.T) {
	p := newMustTargetPod(t, true)
	dirac.DebugMustTargetOff = true
	defer func() { dirac.DebugMustTargetOff = false }()
	for _, seed := range mustTargetSeeds {
		res, _ := p.play(t, seed, 40)
		if res.Div != nil {
			t.Logf("mutation diverged at seed %d decision %d: %s %s", seed, res.Div.At, res.Div.What, res.Div.Field)
			return
		}
	}
	t.Fatal("skipping the MustTarget narrowing diverged on no decision")
}
