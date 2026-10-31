package conformance

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/game"
	"github.com/adams-shaun/gorge/engine/game/gorgegame"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

const foundPath = "testdata/found.tsv"

// foundRow is a game a nightly run found a diff in.
type foundRow struct {
	wire, set string
	seed      uint64
	pair      int
}

func (r foundRow) String() string {
	return fmt.Sprintf("%s\t%s\t%d\t%d", r.wire, r.set, r.seed, r.pair)
}

// play runs the row's game on a and b, with the picker a chunk gives it.
func (r foundRow) play(t *testing.T, reg *cards.Registry, a, b game.Engine) *Diff {
	t.Helper()
	w, ok := chunkWires[r.wire]
	if !ok {
		t.Fatalf("found row %s: no wire %q", r, r.wire)
	}
	c := pairConfig(t, reg, r.set, r.seed, r.pair)
	return Run(t, w, a, b, c, Random(c.Seed))
}

// check is play's result as an error naming the row.
func (r foundRow) check(t *testing.T, reg *cards.Registry, a, b game.Engine) error {
	t.Helper()
	if d := r.play(t, reg, a, b); d != nil {
		return fmt.Errorf("found row %s: %s", r, d)
	}
	return nil
}

// parseFound reads tab-separated wire, set, seed and pair rows. A line starting
// with # is a comment.
func parseFound(text string) ([]foundRow, error) {
	var rows []foundRow
	sc := bufio.NewScanner(strings.NewReader(text))
	for n := 1; sc.Scan(); n++ {
		line := sc.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) != 4 {
			return nil, fmt.Errorf("line %d: %d fields, want wire, set, seed and pair", n, len(f))
		}
		seed, err := strconv.ParseUint(f[2], 10, 32)
		if err != nil {
			return nil, fmt.Errorf("line %d: seed: %w", n, err)
		}
		pair, err := strconv.Atoi(f[3])
		if err != nil {
			return nil, fmt.Errorf("line %d: pair: %w", n, err)
		}
		rows = append(rows, foundRow{wire: f[0], set: f[1], seed: seed, pair: pair})
	}
	return rows, sc.Err()
}

// TestFoundPairs replays every game a nightly run has found a diff in, on gorge
// against itself. A row stays in the file for good once its cause is fixed.
func TestFoundPairs(t *testing.T) {
	text, err := os.ReadFile(foundPath)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := parseFound(string(text))
	if err != nil {
		t.Fatalf("%s: %v", foundPath, err)
	}
	if len(rows) == 0 {
		return
	}
	reg := testutil.CorpusRegistry(t)
	for _, r := range rows {
		t.Run(strings.ReplaceAll(r.String(), "\t", "/"), func(t *testing.T) {
			t.Parallel()
			if err := r.check(t, reg, gorgegame.Engine{}, gorgegame.Engine{}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// A row whose game diffs on a stub engine fails naming the row and the path.
func TestFoundRowWithADiffIsNamed(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	rows, err := parseFound("# a comment\n\nmanabrew\tfdn\t5\t3\n")
	if err != nil {
		t.Fatal(err)
	}
	stub := stubEngine{wrap: func(g gorgegame.Game) game.Game { return wrongLife{g} }}
	err = rows[0].check(t, reg, gorgegame.Engine{}, stub)
	if err == nil {
		t.Fatal("a moved life total produced no diff")
	}
	for _, want := range []string{"manabrew\tfdn\t5\t3", "$.state.seat0.players[1].life"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("%q does not name %q", err, want)
		}
	}
}

func TestParseFoundRefusesAMalformedRow(t *testing.T) {
	for _, text := range []string{"manabrew\tfdn\t5", "manabrew\tfdn\tx\t3", "manabrew\tfdn\t5\tx"} {
		if _, err := parseFound(text); err == nil {
			t.Errorf("%q parsed", text)
		}
	}
}
