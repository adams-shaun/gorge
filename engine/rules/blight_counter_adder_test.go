package rules

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// blightAdderCount reads Count$CountersAddedThisTurn Any <actor> Creature as
// the given seat sees it.
func blightAdderCount(e *Engine, seat state.PlayerID, actor string) int32 {
	c := &effects.Ctx{Controller: seat}
	return e.CountersAddedThisTurn("Any", actor, "Creature", c.SpecContext(seat))
}

// TestBlightCountersAreAddedByTheBlightingPlayer pins CR 701.60: the player
// who blights puts the counters, so an opponent forced to blight by High
// Perfect Morcant (cross-engine farm ECL-s119 xe:RAND:3:13:w12 step 25) is the
// adder that Lasting Tarfire's "if you put a counter on a creature this turn"
// (Count$CountersAddedThisTurn Any You Creature) reads, not Morcant's
// controller. Both the silent single-creature leg and the asked leg.
func TestBlightCountersAreAddedByTheBlightingPlayer(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		bear int
	}{{"single creature (silent)", 1}, {"two creatures (asked)", 2}} {
		t.Run(tc.name, func(t *testing.T) {
			reg := searchTestRegistry(t)
			e, _ := blightEngine(t, reg, 3, "High Perfect Morcant")
			morcant := blightMove(t, e, 0, "High Perfect Morcant", state.ZBattlefield)
			for i := 0; i < tc.bear; i++ {
				blightMove(t, e, 1, "Grizzly Bears", state.ZBattlefield)
			}
			resolveUnderProbe(e, blightCtx(t, reg, "High Perfect Morcant", morcant),
				blightSVar(t, reg, "High Perfect Morcant", "TrigBlight"))
			if tc.bear > 1 {
				blightAnswerBlight(t, e)
			}
			if got := blightAdderCount(e, 1, "You"); got != 1 {
				t.Errorf("seat 1 (the blighter) counters added = %d, want 1", got)
			}
			if got := blightAdderCount(e, 0, "You"); got != 0 {
				t.Errorf("seat 0 (Morcant's controller) counters added = %d, want 0", got)
			}
		})
	}
}

var blightEffectShape = regexp.MustCompile(`\$ Blight\b`)
var blightWho = regexp.MustCompile(`(Defined|ValidTgts)\$ ([A-Za-z.]+)`)

// TestBlightEffectShapesAttributeCountersToTheBlighter is the class ratchet:
// every corpus Blight EFFECT script (not the Blight<N> cost, which
// pay.EmitBlightCounters attributes to the paying creature's controller) is
// one of the who-shapes pinned here, and each shape puts the counters under
// the blighting player's name. A new shape fails the census and must be
// added with its own adder pin.
func TestBlightEffectShapesAttributeCountersToTheBlighter(t *testing.T) {
	root := cards.CorpusDir(testutil.CorpusDirOrSkip(t))
	shapes := map[string]int{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".txt") {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for _, line := range strings.Split(string(b), "\n") {
			if !blightEffectShape.MatchString(line) || strings.HasPrefix(line, "Oracle") || strings.Contains(line, "Cost$ Blight") && !strings.Contains(line, "DB$ Blight") && !strings.Contains(line, "AB$ Blight") {
				continue
			}
			if m := blightWho.FindStringSubmatch(line); m != nil {
				shapes[m[1]+"$ "+m[2]]++
			} else {
				shapes["(none)"]++
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	known := map[string]string{
		"Defined$ You":        "Defined$ You",      // the controller blights: adder seat 0
		"Defined$ Opponent":   "Defined$ Opponent", // each opponent blights: adder is that opponent
		"ValidTgts$ Opponent": "ValidTgts$ Opponent",
	}
	var seen []string
	for k, n := range shapes {
		seen = append(seen, k)
		if _, ok := known[k]; !ok {
			t.Errorf("corpus Blight shape %q (%d scripts) has no adder pin here", k, n)
		}
	}
	sort.Strings(seen)
	t.Logf("corpus Blight effect shapes: %v %v", seen, shapes)

	reg := searchTestRegistry(t)
	// Defined$ You: the controller is the adder.
	e, _ := blightEngine(t, reg, 3, "High Perfect Morcant")
	src := blightMove(t, e, 0, "High Perfect Morcant", state.ZBattlefield)
	resolveUnderProbe(e, blightCtx(t, reg, "High Perfect Morcant", src),
		blightSVar(t, reg, "Shadow Urchin", "DBBlight"))
	if blightAdderCount(e, 0, "You") != 1 || blightAdderCount(e, 1, "You") != 0 {
		t.Errorf("Defined$ You: adder counts seat0=%d seat1=%d, want 1/0",
			blightAdderCount(e, 0, "You"), blightAdderCount(e, 1, "You"))
	}
}
