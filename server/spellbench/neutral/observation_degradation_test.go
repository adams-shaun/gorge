package neutral_test

// TestObservationDegradationCensus measures what a v2 client loses by rendering
// from the plain observation instead of the native view carried in
// x_gorge_view_v1. It is the sbp-2 view census (viewcensus_test.go) over a
// larger, production-shaped corpus with a frequency baseline of its own.
//
// For every decision of 100 seeded two-seat games, the production heuristic
// bot plays both seats from the legacy deck list, the observation is rebuilt
// into a neutral view, and that view is compared field by field with the view
// xview puts in x_gorge_view_v1 (the plane the web client renders from). Each
// unequal census path adds one decision to its tally; testdata/
// observation_degradation.tsv is the checked-in tally. It is a ratchet: it may
// only shrink, so a path that gains decisions or a row whose count dropped (or
// disappeared) fails until the table is regenerated. OBSDEG_REGEN=1 rewrites
// it, refusing to bless growth.

import (
	"context"
	"fmt"
	"os"
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
// storage, not a fact a seat can lose (the brief's class (c)), so both planes
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

func parseObservationDegradation(t *testing.T, b []byte) map[string]int {
	t.Helper()
	out := map[string]int{}
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if line = strings.TrimSpace(line); line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		path, n, ok := strings.Cut(line, "\t")
		if !ok || path == "" {
			t.Fatalf("%s: malformed line %q, want path<TAB>count", observationDegradationBaseline, line)
		}
		v, err := strconv.Atoi(n)
		if err != nil || v < 0 {
			t.Fatalf("%s: bad count in %q: %v", observationDegradationBaseline, line, err)
		}
		out[path] = v
	}
	return out
}

// renderObservationDegradation writes the stable TSV, most frequent first.
func renderObservationDegradation(counts map[string]int) string {
	paths := make([]string, 0, len(counts))
	for p := range counts {
		paths = append(paths, p)
	}
	sort.Slice(paths, func(i, j int) bool {
		if counts[paths[i]] != counts[paths[j]] {
			return counts[paths[i]] > counts[paths[j]]
		}
		return paths[i] < paths[j]
	})
	var b strings.Builder
	for _, p := range paths {
		fmt.Fprintf(&b, "%s\t%d\n", p, counts[p])
	}
	return b.String()
}

// TestObservationDegradationCensus compares, at every decision, the neutral
// view rebuilt from the plain v2 observation with the native view carried in
// x_gorge_view_v1, and ratchets the differing paths by frequency.
func TestObservationDegradationCensus(t *testing.T) {
	reg := testcorpus.Registry(t)
	names := testutil.LegacyDeckNames()
	counts := map[string]int{}
	checked := 0
	for i := 0; i < observationDegradationGames; i++ {
		name := names[i%len(names)]
		g := observationDegradationGame(t, reg, testutil.RepoDeck(t, reg, name), byte(i))
		tr := identity.New(g.E, g.Secret)
		builders := [2]*neutral.Builder{neutral.NewBuilder(reg), neutral.NewBuilder(reg)}
		bots := testgame.Bots(uint64(i))
		for n := 0; n < 1200; n++ {
			pending := g.E.Pending()
			if g.E.G.Over || pending == nil {
				break
			}
			if err := tr.Sync(g.E); err != nil {
				t.Fatalf("game %d (%s) step %d: %v", i, name, n, err)
			}
			priority := g.E.G.Priority
			obs, err := (&observe.Projector{E: g.E, IDs: tr}).Observation(pending.Player, observe.State{PriorityHolder: &priority})
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
	got := []byte(renderObservationDegradation(counts))
	if os.Getenv("OBSDEG_REGEN") != "" {
		// A path that gained decisions is a regression the ratchet exists to
		// catch; only a shrink (or a path that disappeared) may be written
		// without review. A missing baseline is the bootstrap write.
		if prior, err := os.ReadFile(observationDegradationBaseline); err == nil {
			before := parseObservationDegradation(t, prior)
			for p, n := range counts {
				if n > before[p] {
					t.Fatalf("refusing to regenerate %s: %s grew from %d to %d; the baseline may only shrink",
						observationDegradationBaseline, p, before[p], n)
				}
			}
		} else if !os.IsNotExist(err) {
			t.Fatalf("reading %s before regen: %v", observationDegradationBaseline, err)
		}
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(observationDegradationBaseline, got, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("regenerated %s: %d unequal fields over %d decisions", observationDegradationBaseline, len(counts), checked)
		return
	}
	want, err := os.ReadFile(observationDegradationBaseline)
	if err != nil {
		t.Fatalf("reading %s: %v (generate with OBSDEG_REGEN=1 go test -run TestObservationDegradationCensus ./server/spellbench/neutral/)",
			observationDegradationBaseline, err)
	}
	if string(got) != string(want) {
		t.Fatalf("observation degradation census moved: %s is a ratchet, so it may only shrink and a stale row must be removed\n--- measured ---\n%s--- baseline ---\n%s",
			observationDegradationBaseline, got, want)
	}
	t.Logf("observation degradation census: %d decisions, %d unequal fields", checked, len(counts))
	for _, line := range strings.Split(strings.TrimSpace(string(got)), "\n") {
		t.Logf("%s", line)
	}
}
