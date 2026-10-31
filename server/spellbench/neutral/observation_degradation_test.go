package neutral_test

// TestObservationDegradationCensus measures what a v2 client loses by rendering
// from the plain observation instead of the native view carried in
// x_gorge_view_v1. It is the view census (viewcensus_test.go) over a larger,
// production-shaped corpus.
//
// For every decision of 100 seeded two-seat games, the production heuristic
// bot plays both seats from the legacy deck list, the observation is rebuilt
// into a neutral view, and that view is compared field by field with the view
// xview puts in x_gorge_view_v1 (the plane the web client renders from).
//
// testdata/observation_degradation.tsv is the set of census paths known to
// differ, one per line. The test holds the set, not the frequencies, because
// any rules change moves the bots' games and with them every count:
//
//   - A measured path with no row fails: the observation lost a field, or one
//     that was fixed came back. Fix the observation, or add a row by hand if
//     the loss is accepted.
//   - A row the games never reached fails, so a fixed path cannot leave a row
//     behind to hide its return. Delete the row.
//   - A row marked "rare" may go unreached: a few of the 100 games produce it,
//     so a change in their course can remove it without fixing anything. Delete
//     one by hand when the path is fixed.
//
// The count column is how many decisions differed when the table was last
// regenerated. It is information only and nothing compares it;
// OBSDEG_REGEN=1 refreshes it and removes unreached rows, refusing a new path.

import (
	"context"
	"fmt"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/spellbench/gamecfg"
	"github.com/adams-shaun/gorge/server/spellbench/identity"
	"github.com/adams-shaun/gorge/server/spellbench/internal/testcorpus"
	"github.com/adams-shaun/gorge/server/spellbench/internal/testgame"
	"github.com/adams-shaun/gorge/server/spellbench/neutral"
	"github.com/adams-shaun/gorge/server/spellbench/observe"
	"github.com/adams-shaun/gorge/server/spellbench/secrets"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

const (
	observationDegradationBaseline = "testdata/observation_degradation.tsv"
	observationDegradationGames    = 100
)

// observationDegradationGame seats one legacy deck at both seats of a game
// whose every draw comes from the game secret, so a seed names one game.
func observationDegradationGame(t *testing.T, reg *cards.Registry, deck []*cards.Card, seed byte) *gamecfg.Game {
	t.Helper()
	s := make([]byte, 32)
	s[0] = seed
	g, err := gamecfg.New(reg, secrets.NewGame(s), [2][]*cards.Card{deck, deck},
		gamecfg.Rules{Mulligan: "none", StartingSeat: state.PlayerID(0)})
	if err != nil {
		t.Fatal(err)
	}
	return g
}

// normalizeOffBattlefield drops the engine transients that only mean something
// on the battlefield. native's cardView copies SummonSick and
// ActivatedThisTurn straight from state.Object in every zone, so a creature
// that died still reads summon-sick in the graveyard; the observation carries
// summoning_sick only inside a battlefield permanent and no activated count at
// all. Comparing those fields off the battlefield measures gorge's transient
// storage, not a fact a seat can lose, so both planes
// are normalised to the battlefield's own answer.
func normalizeOffBattlefield(v *view.View) {
	for i := range v.Players {
		p := &v.Players[i]
		for _, z := range [][]view.CardView{p.Hand, p.Graveyard, p.Exile, p.Command} {
			for k := range z {
				z[k].SummonSick, z[k].ActivatedThisTurn = false, 0
			}
		}
	}
	for i := range v.Stack {
		if c := v.Stack[i].Card; c != nil {
			c.SummonSick, c.ActivatedThisTurn = false, 0
		}
	}
}

// observationDegradationNative is the view xview carries in x_gorge_view_v1
// for the pending decision: the production projection the web client renders
// from. xview re-keys object ids to the seat's small integers and drops
// pending triggers whose source is hidden; the ids are transport-local and
// differ from the neutral rebuild's own numbering either way, so this projects
// the same fields in the same omit mode and drops the same sourceless pending
// triggers. The payload's decision, ops and payment data are decision-plane
// facts, not the view this census measures.
func observationDegradationNative(g *gamecfg.Game, seat state.PlayerID) view.View {
	v := view.View{}
	view.ProjectLeanInto(&v, g.E.G, g.E, seat, nil, view.OmitLibrary|view.OmitOwnDeck|view.OmitArchetype)
	v.Round = view.RoundOf(g.E.G, g.E.L.Events)
	kept := v.Pending[:0]
	for _, pv := range v.Pending {
		o := g.E.G.Obj(pv.Source)
		if o != nil && (o.Zone == state.ZCeased || observe.Visible(seat, o)) {
			kept = append(kept, pv)
		}
	}
	v.Pending = kept
	normalizeOffBattlefield(&v)
	return v
}

// observationDegradationRow is one path of the table.
type observationDegradationRow struct {
	count int
	rare  bool
}

func parseObservationDegradation(t *testing.T, b []byte) map[string]observationDegradationRow {
	t.Helper()
	out := map[string]observationDegradationRow{}
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		f := strings.Split(line, "\t")
		if len(f) < 2 || len(f) > 3 || f[0] == "" || (len(f) == 3 && f[2] != "rare") {
			t.Fatalf("%s: malformed line %q, want path<TAB>count or path<TAB>count<TAB>rare", observationDegradationBaseline, line)
		}
		n, err := strconv.Atoi(f[1])
		if err != nil || n < 0 {
			t.Fatalf("%s: bad count in %q: %v", observationDegradationBaseline, line, err)
		}
		out[f[0]] = observationDegradationRow{count: n, rare: len(f) == 3}
	}
	return out
}

// renderObservationDegradation writes the stable TSV, most frequent first.
func renderObservationDegradation(table map[string]observationDegradationRow) string {
	paths := make([]string, 0, len(table))
	for p := range table {
		paths = append(paths, p)
	}
	sort.Slice(paths, func(i, j int) bool {
		if table[paths[i]].count != table[paths[j]].count {
			return table[paths[i]].count > table[paths[j]].count
		}
		return paths[i] < paths[j]
	})
	var b strings.Builder
	for _, p := range paths {
		fmt.Fprintf(&b, "%s\t%d", p, table[p].count)
		if table[p].rare {
			b.WriteString("\trare")
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// observationDegradationDrift compares the measured paths with the table:
// added are measured paths with no row, unreached are rows the games did not
// reach and that are not marked rare. Counts play no part.
func observationDegradationDrift(table map[string]observationDegradationRow, measured map[string]int) (added, unreached []string) {
	for p := range measured {
		if _, ok := table[p]; !ok {
			added = append(added, p)
		}
	}
	for p, row := range table {
		if _, ok := measured[p]; !ok && !row.rare {
			unreached = append(unreached, p)
		}
	}
	sort.Strings(added)
	sort.Strings(unreached)
	return added, unreached
}

func TestObservationDegradationDrift(t *testing.T) {
	t.Parallel()
	table := map[string]observationDegradationRow{
		"common": {count: 100},
		"sparse": {count: 3, rare: true},
	}
	for _, tc := range []struct {
		name             string
		measured         map[string]int
		added, unreached []string
	}{
		{name: "counts move freely", measured: map[string]int{"common": 900, "sparse": 1}},
		{name: "a rare row goes unreached", measured: map[string]int{"common": 1}},
		{name: "a new path", measured: map[string]int{"common": 1, "lost": 1}, added: []string{"lost"}},
		{name: "a fixed path keeps its row", measured: map[string]int{"sparse": 1}, unreached: []string{"common"}},
	} {
		added, unreached := observationDegradationDrift(table, tc.measured)
		if !slices.Equal(added, tc.added) || !slices.Equal(unreached, tc.unreached) {
			t.Errorf("%s: added %v, unreached %v; want %v, %v", tc.name, added, unreached, tc.added, tc.unreached)
		}
	}
}

// TestObservationDegradationCensus compares, at every decision, the neutral
// view rebuilt from the plain v2 observation with the native view carried in
// x_gorge_view_v1, and holds the set of differing paths to the table.
func TestObservationDegradationCensus(t *testing.T) {
	t.Parallel()
	reg := testcorpus.Registry(t)
	names := testutil.LegacyDeckNames()
	counts := map[string]int{}
	checked := 0
	for i := 0; i < observationDegradationGames; i++ {
		name := names[i%len(names)]
		g := observationDegradationGame(t, reg, testutil.RepoDeck(t, reg, name), byte(i))
		tr := identity.New(g.R, g.Secret)
		builders := [2]*neutral.Builder{neutral.NewBuilder(reg), neutral.NewBuilder(reg)}
		bots := testgame.Bots(uint64(i))
		for n := 0; n < 1200; n++ {
			pending := g.E.Pending()
			if g.E.G.Over || pending == nil {
				break
			}
			if err := tr.Sync(g.R); err != nil {
				t.Fatalf("game %d (%s) step %d: %v", i, name, n, err)
			}
			priority := g.E.G.Priority
			obs, err := (&observe.Projector{R: g.R, IDs: tr}).Observation(pending.Player, observe.State{PriorityHolder: &priority})
			if err != nil {
				t.Fatalf("game %d (%s) step %d: observation: %v", i, name, n, err)
			}
			got, err := builders[pending.Player].View(&obs)
			if err != nil {
				t.Fatalf("game %d (%s) step %d: neutral view: %v", i, name, n, err)
			}
			want := observationDegradationNative(g, pending.Player)
			diff := map[string]bool{}
			censusWalk("", censusJSON(t, want), censusJSON(t, got), diff)
			for p := range diff {
				counts[p]++
			}
			checked++
			in, err := bots[pending.Player].Decide(context.Background(), want, *pending)
			if err != nil {
				t.Fatalf("game %d (%s) step %d: bot: %v", i, name, n, err)
			}
			if err := g.Submit(in); err != nil {
				t.Fatalf("game %d (%s) step %d: submit: %v", i, name, n, err)
			}
		}
	}
	if checked == 0 {
		t.Fatal("precondition: the corpus compared no decisions")
	}
	if len(counts) == 0 {
		t.Fatal("precondition: no field differed, so the census proves nothing")
	}
	prior, err := os.ReadFile(observationDegradationBaseline)
	if err != nil {
		t.Fatal(err)
	}
	table := parseObservationDegradation(t, prior)
	added, unreached := observationDegradationDrift(table, counts)
	if len(added) > 0 {
		t.Fatalf("the plain v2 observation differs from the native view at %d path(s) with no row in %s: %s\n"+
			"fix the observation, or add a row by hand if the loss is accepted",
			len(added), observationDegradationBaseline, strings.Join(added, ", "))
	}
	for p, row := range table {
		row.count = counts[p]
		table[p] = row
	}
	if os.Getenv("OBSDEG_REGEN") != "" {
		for _, p := range unreached {
			delete(table, p)
		}
		if err := os.WriteFile(observationDegradationBaseline, []byte(renderObservationDegradation(table)), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	if len(unreached) > 0 {
		t.Fatalf("%d path(s) in %s no longer differ: %s\ndelete their rows, so a return is caught (OBSDEG_REGEN=1 does it)",
			len(unreached), observationDegradationBaseline, strings.Join(unreached, ", "))
	}
	t.Logf("observation degradation census: %d decisions, %d unequal fields", checked, len(counts))
	for _, line := range strings.Split(strings.TrimSpace(renderObservationDegradation(table)), "\n") {
		t.Logf("%s", line)
	}
}
