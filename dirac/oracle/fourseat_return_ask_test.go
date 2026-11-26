package oracle

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Two four-seat cmd5 pods (kfuzz -kernel cmd5 -seats 4 -seed 2, pairs 629 and
// 941) where a Grasp of Fate's until-it-leaves exile returns cards whose
// as-enters choice gorge asks with the cards still in exile:
//
//   - pair 629: seat 2 loses at priority (a Bontu's Monument drain) holding
//     the Grasp, which exiled Patchwork Banner and Count on Luck. gorge asks
//     the Banner's creature type with the Banner in exile (no other decision is
//     outstanding); the kernel parked that ask only at a step's start, so it
//     entered the Banner unasked (CR 800.4a, 610.3, 614.12).
//   - pair 941: All Is Dust sacrifices the Grasp, which exiled Outpost Siege
//     and Tamiyo's Logbook. gorge poses the Siege's Khans-or-Dragons ask as it
//     enters and the Logbook waits in exile for the answer; the kernel returned
//     both and then posed the queued ask (CR 610.3d).
var returnAskRepros = map[string]int{
	"cmd5_four_seat_leave_return_banner_choice_at_priority": 4269,
	"cmd5_four_seat_return_batch_waits_behind_siege_ask":    1713,
}

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

func TestFourSeatReturnAsksWithCardInExile(t *testing.T) {
	if isolate(t) {
		return
	}
	for name, at := range returnAskRepros {
		res := fourSeatCmd5Replay(t, name)
		if res.Div != nil {
			t.Fatalf("%s: divergence at %d: %s %s %v", name, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
		}
		if strings.HasPrefix(res.Stall, KernelStallPrefix) || res.Decisions <= at {
			t.Fatalf("%s: stall %q after %d decisions, want a clean game past %d", name, res.Stall, res.Decisions, at)
		}
	}
}

// TestFourSeatReturnAsksWithCardInExileMutation proves each replay reaches its
// return: the old parking (priority) and the old batch order (queued ask)
// each diverge their own pair.
func TestFourSeatReturnAsksWithCardInExileMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	defer func() { dirac.DebugReturnPriorityNoAsk, dirac.DebugReturnQueuedAskNoHold = false, false }()
	for name, flip := range map[string]*bool{
		"cmd5_four_seat_leave_return_banner_choice_at_priority": &dirac.DebugReturnPriorityNoAsk,
		"cmd5_four_seat_return_batch_waits_behind_siege_ask":    &dirac.DebugReturnQueuedAskNoHold,
	} {
		*flip = true
		res := fourSeatCmd5Replay(t, name)
		*flip = false
		if res.Div == nil {
			t.Fatalf("%s: the old behaviour changed no game: the replay does not reach the return", name)
		}
	}
}
