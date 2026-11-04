//go:build spellbench

package neutral_test

import (
	"context"
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/spellbench/catalog"
	"github.com/adams-shaun/gorge/server/spellbench/gamecfg"
	"github.com/adams-shaun/gorge/server/spellbench/identity"
	"github.com/adams-shaun/gorge/server/spellbench/internal/testcorpus"
	"github.com/adams-shaun/gorge/server/spellbench/internal/testgame"
	"github.com/adams-shaun/gorge/server/spellbench/neutral"
	"github.com/adams-shaun/gorge/server/spellbench/observe"
)

const viewCensusBaseline = "testdata/viewcensus.txt"

// censusJSON is the wire shape of a view: json:"-" and unexported fields are
// deliberately outside the census, because no observation could carry them.
func censusJSON(t *testing.T, v view.View) any {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

// censusWalk records the path of every unequal leaf, and the path plus "[]" of
// every unequal list. Object and map indices are folded to "[]" and the key's
// own name, so one field has one census name across games.
func censusWalk(path string, a, b any, out map[string]bool) {
	switch av := a.(type) {
	case map[string]any:
		bv, ok := b.(map[string]any)
		if !ok {
			out[path] = true
			return
		}
		keys := make([]string, 0, len(av)+len(bv))
		for k := range av {
			keys = append(keys, k)
		}
		for k := range bv {
			if _, ok := av[k]; !ok {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		for _, k := range keys {
			p := k
			if path != "" {
				p = path + "." + k
			}
			censusWalk(p, av[k], bv[k], out)
		}
	case []any:
		bv, ok := b.([]any)
		if !ok {
			out[path+"[]"] = true
			return
		}
		if len(av) != len(bv) {
			out[path+"[]"] = true
		}
		for i := 0; i < min(len(av), len(bv)); i++ {
			censusWalk(path+"[]", av[i], bv[i], out)
		}
	case nil:
		if b != nil {
			out[path] = true
		}
	default:
		if !reflect.DeepEqual(a, b) {
			out[path] = true
		}
	}
}

// censusBaseline reads the table of paths known to differ: one per line, with
// "rare" after a tab on a row that may go unreached.
func censusBaseline(t *testing.T) map[string]observationDegradationRow {
	t.Helper()
	b, err := os.ReadFile(viewCensusBaseline)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]observationDegradationRow{}
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		path, mark, marked := strings.Cut(line, "\t")
		if path == "" || (marked && mark != "rare") {
			t.Fatalf("%s: malformed line %q, want path or path<TAB>rare", viewCensusBaseline, line)
		}
		out[path] = observationDegradationRow{rare: marked}
	}
	return out
}

// seedCensusState makes one seeded game carry the fields under census:
// a permanent with two P1P1 counters, a title-case keyword counter, two status
// designations, player poison and experience counters, the monarch and
// initiative designations, and CR 731's day. Without it a pauper catalog game
// need never reach any of them, and the census could not notice a rebuild that
// dropped them.
func seedCensusState(t *testing.T, g *gamecfg.Game) {
	t.Helper()
	var id state.ObjID
	for _, z := range []state.Zone{state.ZHand, state.ZLibrary} {
		if ids := g.E.G.Zone(z, 0); len(ids) > 0 {
			id = ids[0]
			break
		}
	}
	if id == 0 {
		t.Fatal("seeded game has no card to move")
	}
	o := g.E.G.Obj(id)
	events.Emit(g.E.G, g.E.L, events.Event{Kind: events.MoveZone, Obj: id, From: o.Zone, To: state.ZBattlefield})
	events.Emit(g.E.G, g.E.L, events.Event{Kind: events.CounterChange, Obj: id, Counter: "P1P1", Amount: 2})
	events.Emit(g.E.G, g.E.L, events.Event{Kind: events.CounterChange, Obj: id, Counter: "Flying", Amount: 1})
	for _, text := range []string{"Suspected", "Monstrous", "Renowned"} {
		events.Emit(g.E.G, g.E.L, events.Event{Kind: events.AlterAttribute, Obj: id, Text: text, Amount: 1})
	}
	events.Emit(g.E.G, g.E.L, events.Event{Kind: events.PlayerCounterChange, Player: 0, Counter: "POISON", Amount: 3})
	events.Emit(g.E.G, g.E.L, events.Event{Kind: events.PlayerCounterChange, Player: 0, Counter: "Experience", Amount: 2})
	events.Emit(g.E.G, g.E.L, events.Event{Kind: events.MonarchChange, Player: 0})
	events.Emit(g.E.G, g.E.L, events.Event{Kind: events.InitiativeChange, Player: 1})
	events.Emit(g.E.G, g.E.L, events.Event{Kind: events.DayNightChange, Amount: int32(state.DayNightDay)})
	if o := g.E.G.Obj(id); o.Zone != state.ZBattlefield || o.Counter("P1P1") != 2 ||
		!o.Suspected || !o.Monstrous || !o.Renowned {
		t.Fatalf("precondition: seeded permanent %+v", o)
	}
	if !g.E.G.IsMonarch(0) || !g.E.G.IsInitiative(1) || g.E.G.Players[0].Counter("POISON") != 3 ||
		g.E.G.DayNight != state.DayNightDay {
		t.Fatalf("precondition: seeded designations/day-night/player counters missing")
	}
}

// TestNeutralViewCensus structurally compares the native view beside every bot
// decision with the view neutral rebuilds from the observation.
// testdata/viewcensus.txt is the set of fields known to differ, and the test holds
// the set as TestObservationDegradationCensus holds its own: a measured field with
// no row fails, and so does a row the games never reach, unless it is marked rare.
// A few of the 50 games produce a rare row, so a change in their course can remove
// it without fixing anything; delete one by hand when the field is fixed.
func TestNeutralViewCensus(t *testing.T) {
	t.Parallel()
	reg := testcorpus.Registry(t)
	counts := map[string]int{}
	checked := 0
	for i := 0; i < 50; i++ {
		d := catalog.Decks()[i%len(catalog.Decks())]
		g := testgame.New(t, reg, d.CatalogID, d.CatalogID, byte(i), "none")
		tr := identity.New(g.R, g.Secret)
		builders := [2]*neutral.Builder{neutral.NewBuilder(reg), neutral.NewBuilder(reg)}
		bots := testgame.Bots(uint64(i))
		seeded, forcePass := i != 0, false
		for n := 0; n < 1200; n++ {
			pending := g.E.Pending()
			if g.E.G.Over || pending == nil {
				break
			}
			if !seeded && g.E.G.Turn > 0 {
				if pending.Kind != decision.KPriority {
					t.Fatalf("game %d: first turn decision is %v, want priority", i, pending.Kind)
				}
				seedCensusState(t, g)
				seeded, forcePass = true, true
			}
			if err := tr.Sync(g.R); err != nil {
				t.Fatalf("game %d step %d: %v", i, n, err)
			}
			priority := g.E.G.Priority
			obs, err := (&observe.Projector{R: g.R, IDs: tr}).Observation(pending.Player, observe.State{PriorityHolder: &priority})
			if err != nil {
				t.Fatalf("game %d step %d: observation: %v", i, n, err)
			}
			got, err := builders[pending.Player].View(&obs)
			if err != nil {
				t.Fatalf("game %d step %d: neutral view: %v", i, n, err)
			}
			want := view.Project(g.E.G, g.E, pending.Player, nil)
			diff := map[string]bool{}
			censusWalk("", censusJSON(t, want), censusJSON(t, got), diff)
			for p := range diff {
				counts[p]++
			}
			checked++
			in := decision.Intent{Seq: pending.Seq, Player: pending.Player}
			if forcePass {
				pass := -1
				for k, opt := range pending.Options {
					if opt.Kind == "pass" {
						pass = k
						break
					}
				}
				if pass < 0 {
					t.Fatalf("game %d: injected state left no pass option", i)
				}
				in.Choices = []int{pass}
			} else {
				in, err = bots[pending.Player].Decide(context.Background(), want, *pending)
				if err != nil {
					t.Fatalf("game %d step %d: bot: %v", i, n, err)
				}
			}
			forcePass = false
			if err := g.Submit(in); err != nil {
				t.Fatalf("game %d step %d: submit: %v", i, n, err)
			}
		}
	}
	baseline := censusBaseline(t)
	added, unreached := observationDegradationDrift(baseline, counts)
	if len(added) > 0 || len(unreached) > 0 {
		t.Fatalf("view census differs from %s:\nnew unequal fields (fix the projection, or add a row by hand if the loss is accepted): %v\nfields that became equal (delete their rows): %v",
			viewCensusBaseline, added, unreached)
	}
	t.Logf("view census: %d decisions, %d unequal fields", checked, len(counts))
}
