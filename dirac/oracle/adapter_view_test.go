package oracle

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/adapter"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

// viewIDKeys are the JSON fields of a view that hold object ids, compared by the
// instance they name: the engines number tokens and abilities differently.
var viewIDKeys = map[string]bool{"id": true, "attached_to": true, "blocked_by": true, "source": true, "obj": true,
	"imprinted": true, "paired": true, "exiled_cards": true, "attacking_battle": true}

// noReader are fields no client, bot or wire reads, which a served game leaves empty.
var noReader = []string{"available", "ability_costs", "spells_cast_this_turn", "life_gained_this_turn", "life_lost_this_turn", "tokens_created_this_turn", "label"}

// renameIDs rewrites x's object ids through ref and drops what is not compared: the
// fields without a reader, a counter kind at zero, a blocker that has left the game.
// Keywords and blockers compare as sets.
func renameIDs(x any, key string, ref func(state.ObjID) Ref) any {
	switch v := x.(type) {
	case map[string]any:
		for _, k := range noReader {
			delete(v, k)
		}
		if key == "counters" {
			for k, n := range v {
				if n == float64(0) {
					delete(v, k)
				}
			}
			if len(v) == 0 {
				return nil
			}
		}
		for k, e := range v {
			if r := renameIDs(e, k, ref); r != nil {
				v[k] = r
			} else {
				delete(v, k)
			}
		}
	case string:
		if key == "token" && strings.HasPrefix(v, "#") {
			var id int
			fmt.Sscanf(v, "#%d", &id)
			return fmt.Sprintf("#%d", ref(state.ObjID(id)))
		}
	case []any:
		for i, e := range v {
			v[i] = renameIDs(e, key, ref)
		}
		if key == "blocked_by" {
			v = slices.DeleteFunc(v, func(e any) bool { return e == float64(RefNone) })
			if len(v) == 0 {
				return nil
			}
		}
		if key == "blocked_by" || key == "keywords" {
			sort.Slice(v, func(i, j int) bool { return fmt.Sprint(v[i]) < fmt.Sprint(v[j]) })
		}
		return v
	case float64:
		if viewIDKeys[key] {
			return float64(ref(state.ObjID(v)))
		}
	}
	return x
}

func toJSONAny(v any) any {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	var x any
	if err := json.Unmarshal(b, &x); err != nil {
		panic(err)
	}
	return x
}

// viewDiffs counts differences by JSON path.
type viewDiffs map[string]int

func (d viewDiffs) walk(path string, a, b any) {
	if ma, ok := a.(map[string]any); ok {
		mb, ok := b.(map[string]any)
		if !ok {
			d[path]++
			return
		}
		for k, va := range ma {
			if vb, has := mb[k]; has {
				d.walk(path+"."+k, va, vb)
			} else {
				d[path+"."+k+" (gorge only)"]++
			}
		}
		for k := range mb {
			if _, has := ma[k]; !has {
				d[path+"."+k+" (dirac only)"]++
			}
		}
		return
	}
	if la, ok := a.([]any); ok {
		lb, ok := b.([]any)
		if !ok {
			d[path+" (type)"]++
			return
		}
		if len(la) != len(lb) {
			d[path+"[len]"]++
			return
		}
		for i := range la {
			d.walk(path+"[]", la[i], lb[i])
		}
		return
	}
	if fmt.Sprint(a) != fmt.Sprint(b) {
		d[path]++
	}
}

// comparableView is one projection in the form both engines' are compared in. A
// seat's potential actions are compared where it holds priority, and only there: a
// seat without it has nothing it could do on dirac's surface.
func comparableView(g *state.Game, ch view.Chars, seat state.PlayerID, vis view.Visibility, ref func(state.ObjID) Ref) any {
	x := renameIDs(toJSONAny(view.ProjectFor(g, ch, seat, vis, nil)), "", ref).(map[string]any)
	if ps, ok := x["players"].([]any); ok {
		for _, p := range ps {
			delete(p.(map[string]any), "potential_actions")
		}
	}
	return x
}

// viewCheck compares both engines' projections of one position for every viewer.
func viewCheck(d viewDiffs, positions *int) func(ref *GorgeSide, a *adapter.Game) {
	return func(ref *GorgeSide, a *adapter.Game) {
		*positions++
		for _, vis := range []view.Visibility{view.Seat, view.Public} {
			for seat := state.PlayerID(0); seat < 2; seat++ {
				if vis == view.Public && seat == 1 {
					continue
				}
				d.walk("view", comparableView(ref.E.G, ref.E, seat, vis, ref.InstRef), comparableView(a.Board(), a.Chars(), seat, vis, a.RefOf))
			}
		}
	}
}

// servedViewGaps are the paths a served view does not carry yet: each needs a kernel
// read that does not exist. It only shrinks.
var servedViewGaps = map[string]bool{
	"exiled_cards (gorge only)": true, "effective_mana_cost (gorge only)": true,
	"optional": true, "decider (gorge only)": true, "pending[len]": true,
	"name": true, "card_name (gorge only)": true, "targets[len]": true, "keywords (gorge only)": true,
	"archetype.top_score": true, "archetype.scores.aggro": true, "archetype.scores.burn": true, "archetype.scores.control": true,
	"archetype.scores.go-wide": true, "archetype.scores.midrange": true, "archetype.scores.ramp": true, "archetype.scores.tempo": true,
}

func TestNativeViewMatchesGorge(t *testing.T) {
	pairs := envPairs(100)
	d := viewDiffs{}
	n, seen := 0, 0
	check := viewCheck(d, &n)
	clean, total := adapterRun(t, "fdn", 0, pairs, 1, Config{GorgeAutopaySurface: true}, func(ref *GorgeSide, a *adapter.Game) {
		seen++
		if seen%viewSampling == 0 {
			check(ref, a)
		}
	})
	var b strings.Builder
	fmt.Fprintf(&b, "games %d clean %d positions compared %d of %d\n", total, clean, n, seen)
	var keys []string
	for k := range d {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(&b, "%8d %s\n", d[k], k)
		if !servedViewGaps[gapKey(k)] {
			t.Errorf("view differs at %s (%d)", k, d[k])
		}
	}
	if p := os.Getenv("ADAPTER_OUT"); p != "" {
		os.WriteFile(p, []byte(b.String()), 0o644)
	}
	t.Log(b.String())
}

// viewSampling compares every n-th decision: a projection and its JSON cost far more
// than the game.
const viewSampling = 4

// gapKey is a path's last field: the archetype row keeps its parents.
func gapKey(path string) string {
	if i := strings.Index(path, "archetype."); i >= 0 {
		return path[i:]
	}
	return path[strings.LastIndex(path, ".")+1:]
}
