package oracle

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// A Commander pod's shares-a-creature-type-with-your-commander filters read
// the controller's commanders (CR 702.73a): Path of Ancestry's "When that mana
// is spent to cast a creature spell that shares a creature type with your
// commander, scry 1". The kernel's sharesCr compared only the first two
// kop.SubMask words, so a subtype whose index sits past the second word (an
// edh table's Wizard is 186 of 192) never shared: the kernel dropped Path's
// scry trigger where gorge queued it, on a cast of Arcanis the Omnipotent paid
// with the land's mana (pod 29 of the edh sweep, seed 5148651490203427580,
// decision 1793). sharesCr now reads every SubMask word.
//
// The two-seat ancestryscry games (TestPathOfAncestryScryLockstep) cannot pin
// this: their table holds few subtypes, all within the first two words. The
// recording is a four-seat pod (kfuzz -kernel edh -seats 4): its table is the
// edh kernel tables over its own four decks, which no glob helper builds
// (replayDecodedScript builds one from the first two decks), so it is replayed
// here under its own name and carries the package's one-off pool label.
const sharesCrThirdWordRepro = "shares_creature_type_drops_third_mask_word"

// sharesCrThirdWordFirstAt is the decision the recording diverged at before
// the fix (sharesCr's third SubMask word).
const sharesCrThirdWordFirstAt = 1793

// sharesCrThirdWordReplay replays the pod recording in lockstep.
func sharesCrThirdWordReplay(t *testing.T, reg *cards.Registry) Result {
	t.Helper()
	r, err := ReadRepro(filepath.Join("testdata", "repro", sharesCrThirdWordRepro+".repro.json"))
	if err != nil {
		t.Fatal(err)
	}
	if r.Pool != "one-off" {
		t.Fatalf("pool %q, want one-off", r.Pool)
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

// TestSharesCrThirdMaskWordReproLockstep: the pod replays clean, past the
// decision where the kernel's sharesCr once dropped Path of Ancestry's scry.
func TestSharesCrThirdMaskWordReproLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	res := sharesCrThirdWordReplay(t, synthReg(t))
	if res.Div != nil {
		t.Errorf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if res.Decisions <= sharesCrThirdWordFirstAt {
		t.Errorf("%d decisions, the game must go past decision %d where the scry is queued", res.Decisions, sharesCrThirdWordFirstAt)
	}
}

// TestSharesCrThirdMaskWordReproMutation: with every sharesCr answer flipped,
// the kernel queues the scry where gorge does not and drops it on the
// recorded cast, so the replay diverges at or before the recorded decision.
func TestSharesCrThirdMaskWordReproMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugSharesCrCmdrFlip = true
	defer func() { dirac.DebugSharesCrCmdrFlip = false }()
	res := sharesCrThirdWordReplay(t, synthReg(t))
	if res.Div == nil {
		t.Error("no divergence with every shares-a-creature-type-with-your-commander answer flipped")
	} else if res.Div.At > sharesCrThirdWordFirstAt {
		t.Errorf("diverged at %d, after the recorded decision %d", res.Div.At, sharesCrThirdWordFirstAt)
	}
}
