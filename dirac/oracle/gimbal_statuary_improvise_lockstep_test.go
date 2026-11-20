package oracle

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// A commander cast from the command zone is the plain cast (CR 903.8), so an
// Improvise grant credits it as it does a hand cast (CR 702.126a): Inspiring
// Statuary ("Nonartifact spells you cast have improvise") under Gimbal,
// Gremlin Prodigy. gorge's cast planner prices the ZCommand origin on the
// hand's body (payment_plan.go: improviseCost before the tax, planImprovise)
// and offers it as a planned cast; the kernel's improviseLifted took only a
// hand origin, so it read the commander as a "shape:contribution" cast that
// only the floating pool pays and dropped cast(#30) where gorge listed it
// (Tinker Time in kfuzz -kernel edh -seats 4 -pods 64 -cmd-mini 100
// -allow-unverified -seed 1, pairs 19 and 37: decisions 301 and 1060). With the
// offer fixed, gorge counts the commander cast (CR 903.8's tax counter) at the
// push, ahead of the Improvise helper ask, and the kernel only counted it at
// the commit: the projection read CmdCasts 0 against 1 at that ask (the third
// recording, decision 352).
//
// The edh sweep plays the committed slice's pairs (pairs.tsv), so each
// recording is a two-seat game of the full 100-card precons, replayed as in
// shares_creature_type_drops_third_mask_word_repro_test.go.
var gimbalStatuaryRepros = []struct {
	name    string
	firstAt int // the decision the recording diverged at before the fixes
}{
	{"gimbal_statuary_improvise_command_zone_cast", 301},
	{"gimbal_statuary_improvise_command_zone_cast_pair37", 1060},
	{"gimbal_command_zone_cast_counts_at_push", 352},
}

func gimbalStatuaryReplay(t *testing.T, reg *cards.Registry, name string) Result {
	t.Helper()
	r, err := ReadRepro(filepath.Join("testdata", "repro", name+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	m, err := r.Matchup(reg)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.ByName("edh")
	if errors.Is(err, cardset.ErrNoTables) || errors.Is(err, cardset.ErrNoCorpus) {
		t.Skipf("no edh table (make dirac-tables): %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	set = cardset.AllowUnverified(set)
	es := make([][]cardset.Entry, 0, 2+len(r.MoreCards))
	for _, names := range append([][]string{r.Cards[0], r.Cards[1]}, r.MoreCards...) {
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

// TestGimbalStatuaryImproviseLockstep: the recordings replay clean, past the
// decision each once diverged at.
func TestGimbalStatuaryImproviseLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	reg := synthReg(t)
	for _, c := range gimbalStatuaryRepros {
		res := gimbalStatuaryReplay(t, reg, c.name)
		if res.Div != nil {
			t.Errorf("%s: divergence at %d: %s %s %v", c.name, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
		}
		if res.Decisions <= c.firstAt {
			t.Errorf("%s: %d decisions, the game must go past decision %d", c.name, res.Decisions, c.firstAt)
		}
	}
}
