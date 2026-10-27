package effects

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

const definedSelectorBaseline = "testdata/defined-selector-unknown.txt"

// TestDefinedSelectorCensus ratchets, both ways, the Defined$ and
// DefinedPlayer$ spellings in the corpus that knownDefinedTargets does not
// model. DefinedRef resolves such a spelling to the source (or the chosen
// targets), so an unknown spelling is a silent "the source": that is how
// CardOwner, Spawner>TriggeredTarget and TriggeredSpellAbilityTargets went
// unnoticed. A row is a lead, not proof of a bug: an API with its own table
// (ChangeZone's DirectRemembered, RepeatEach) consumes some. A new unknown
// spelling fails with its carriers; a row the build now models fails as stale.
// DEFINED_CENSUS_REGEN=1 rewrites the baseline. DefinedCards$ is out: RepeatEach
// resolves it through repeatedCardsCodes.
func TestDefinedSelectorCensus(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	h, c := fixtureHost(t)
	type key struct{ param, spec string }
	carriers := map[key]map[string]bool{}
	scanned := 0
	visit := func(card string, sa *cards.SA) {
		for seen := 0; sa != nil && seen < 64; sa, seen = sa.Sub, seen+1 {
			for _, p := range []struct {
				name string
				k    cards.ParamKey
			}{{"Defined", cards.PKDefined}, {"DefinedPlayer", cards.PKDefinedPlayer}} {
				spec := strings.TrimSpace(sa.ParamStr(p.k))
				if spec == "" {
					continue
				}
				scanned++
				if _, ok := knownDefinedTargets(h, c, spec); ok {
					continue
				}
				k := key{p.name, spec}
				if carriers[k] == nil {
					carriers[k] = map[string]bool{}
				}
				carriers[k][card] = true
			}
		}
	}
	for _, card := range reg.AllCards() {
		for _, f := range card.Faces {
			for _, sa := range f.Abilities {
				visit(f.Name, sa)
			}
			names := make([]string, 0, len(f.SVars))
			for name := range f.SVars {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				visit(f.Name, cards.ResolveSVar(f.SVars, name))
			}
		}
	}
	if scanned < 1000 {
		t.Fatalf("census read %d Defined$ values: the scan is not reading the corpus", scanned)
	}

	rows := make([]string, 0, len(carriers))
	byRow := map[string]key{}
	for k, cs := range carriers {
		row := fmt.Sprintf("%s$ %s\t%d", k.param, k.spec, len(cs))
		rows = append(rows, row)
		byRow[row] = k
	}
	sort.Strings(rows)
	got := strings.Join(rows, "\n") + "\n"
	if os.Getenv("DEFINED_CENSUS_REGEN") != "" {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(definedSelectorBaseline, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	raw, err := os.ReadFile(definedSelectorBaseline)
	if err != nil {
		t.Fatalf("baseline: %v (regenerate with DEFINED_CENSUS_REGEN=1)", err)
	}
	want := map[string]bool{} // "<Key>$ <spelling>" -> present
	for line := range strings.SplitSeq(strings.TrimSpace(string(raw)), "\n") {
		if line == "" {
			continue
		}
		id, _, _ := strings.Cut(line, "\t")
		want[id] = true
	}
	have := map[string]bool{}
	for k := range carriers {
		id := k.param + "$ " + k.spec
		have[id] = true
		if !want[id] {
			names := make([]string, 0, len(carriers[k]))
			for n := range carriers[k] {
				names = append(names, n)
			}
			sort.Strings(names)
			if len(names) > 8 {
				names = append(names[:8], "...")
			}
			t.Errorf("new unmodelled selector %s (carriers: %s): it resolves to the source; model it, or triage it and regenerate the baseline", id, strings.Join(names, "; "))
		}
	}
	for id := range want {
		if !have[id] {
			t.Errorf("baseline row %q is stale: the build now models it; delete the row (DEFINED_CENSUS_REGEN=1)", id)
		}
	}
	if got != string(raw) && !t.Failed() {
		t.Errorf("baseline carrier counts moved; regenerate with DEFINED_CENSUS_REGEN=1")
	}
}
