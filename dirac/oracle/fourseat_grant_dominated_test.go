package oracle

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// rhonasGrantRepro is a four-seat cmd5 pod (kfuzz -kernel cmd5 -seats 4
// -first-pair 739 -seed 2): seat 3 activates Rhonas the Indomitable's {2}{G}
// pump with Ghalta and Whiptongue Hydra tapped for the {G} Rishkar, Peema
// Renegade grants every creature with a +1/+1 counter. Paradise Druid holds the
// same granted "{T}: Add {G}" beside its printed any-colour {T}: a source's
// free {T} abilities are alternatives of one tap (CR 605.3b), so the grant adds
// nothing the planner's census lacks, and gorge's census does not count it a
// gap (engine/rules/potential_plan.go manaTapGapDominated). gorge's prefix
// search stops at the two real gaps and the Druid pays the last {1} after the
// target is chosen; the kernel tapped the Druid in the prefix, before the target
// ask, one mana up in the pool and the Druid tapped and no longer hexproof.
const rhonasGrantRepro = "cmd5_rhonas_pump_granted_mana_covered_by_printed_four_seat_pod"

// rhonasGrantAt is the decision the recording diverged at.
const rhonasGrantAt = 1491

// fourSeatCmd5Replay is fourSeatEdhReplay on the cmd5 tables.
func fourSeatCmd5Replay(t *testing.T, name string) Result {
	t.Helper()
	reg := synthReg(t)
	r, err := ReadRepro(filepath.Join("testdata", "repro", name+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	m, err := r.Matchup(reg)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.ByName("cmd5")
	if errors.Is(err, cardset.ErrNoTables) || errors.Is(err, cardset.ErrNoCorpus) {
		t.Skipf("no cmd5 table (make dirac-tables): %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	set = cardset.AllowUnverified(set)
	es := make([][]cardset.Entry, 0, 4)
	for _, names := range [][]string{r.Cards[0], r.Cards[1], r.MoreCards[0], r.MoreCards[1]} {
		es = append(es, ReproEntries(reg, set, names))
	}
	tbl, _, err := set.MatchupN(es)
	if err != nil {
		t.Fatal(err)
	}
	rc := r.RunConfig()
	rc.MaxDecisions = len(r.Actions) + 200
	k, err := SetSide(set, tbl, rc.Config)(m, r.Seed)
	if err != nil {
		t.Fatal(err)
	}
	defer CloseSide(k)
	return Lockstep(m, r.Seed, k, &ScriptPolicy{Actions: r.Actions}, rc)
}

func TestFourSeatGrantCoveredByPrintedAbilityIsNoGap(t *testing.T) {
	if isolate(t) {
		return
	}
	res := fourSeatCmd5Replay(t, rhonasGrantRepro)
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if strings.HasPrefix(res.Stall, KernelStallPrefix) || res.Decisions <= rhonasGrantAt {
		t.Fatalf("stall %q after %d decisions, want a clean game past %d", res.Stall, res.Decisions, rhonasGrantAt)
	}
}

// TestFourSeatGrantCoveredByPrintedAbilityMutation restores the old census
// (every holder of a grant is a gap): the recording diverges again.
func TestFourSeatGrantCoveredByPrintedAbilityMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugGrantDominatedGaps = true
	defer func() { dirac.DebugGrantDominatedGaps = false }()
	res := fourSeatCmd5Replay(t, rhonasGrantRepro)
	if res.Div == nil {
		t.Fatal("the old census changed no game: the replay does not reach the Druid")
	}
}
