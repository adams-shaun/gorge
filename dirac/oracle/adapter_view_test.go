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
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

// viewIDKeys are the JSON fields of a view that hold object ids, compared by the
// instance they name: the engines number tokens and abilities differently.
var viewIDKeys = map[string]bool{"id": true, "attached_to": true, "blocked_by": true, "source": true, "obj": true,
	"imprinted": true, "paired": true, "exiled_cards": true, "attacking_battle": true}

// renameIDs rewrites x's object ids through ref. Keywords compare as a set.
func renameIDs(x any, key string, ref func(state.ObjID) Ref) any {
	switch v := x.(type) {
	case map[string]any:
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

// viewDiffs counts differences by JSON path, with the first of each.
type viewDiffs struct {
	n      map[string]int
	sample map[string]string
	// shown counts, by field name, the values gorge's view set to something.
	shown map[string]int
}

func newViewDiffs() *viewDiffs {
	return &viewDiffs{n: map[string]int{}, sample: map[string]string{}, shown: map[string]int{}}
}

func (d *viewDiffs) add(path, owner string, a, b any) {
	d.n[path]++
	if d.sample[path] == "" {
		s := fmt.Sprintf("[%s] gorge %v served %v", owner, a, b)
		d.sample[path] = s[:min(len(s), 400)]
	}
}

func (d *viewDiffs) walk(path, owner string, a, b any) {
	if ma, ok := a.(map[string]any); ok {
		mb, ok := b.(map[string]any)
		if !ok {
			d.add(path, owner, a, b)
			return
		}
		if n, ok := ma["name"].(string); ok {
			owner = n
		}
		for k, va := range ma {
			if vb, has := mb[k]; has {
				d.walk(path+"."+k, owner, va, vb)
			} else {
				d.add(path+"."+k+" (gorge only)", owner, va, nil)
			}
		}
		for k, vb := range mb {
			if _, has := ma[k]; !has {
				d.add(path+"."+k+" (served only)", owner, nil, vb)
			}
		}
		return
	}
	if la, ok := a.([]any); ok {
		lb, ok := b.([]any)
		if !ok {
			d.add(path+" (type)", owner, a, b)
			return
		}
		if len(la) != len(lb) {
			d.add(path+"[len]", owner, a, b)
			return
		}
		for i := range la {
			d.walk(path+"[]", owner, la[i], lb[i])
		}
		return
	}
	if a != nil && a != false && a != float64(0) && a != "" {
		d.shown[fieldOf(path)]++
	}
	if fmt.Sprint(a) != fmt.Sprint(b) {
		d.add(path, owner, a, b)
	}
}

// fieldOf is a path's last field, without its list marks and side notes.
func fieldOf(path string) string {
	if strings.Contains(path, ".counters.") {
		return "counters"
	}
	f := path[strings.LastIndex(path, ".")+1:]
	f, _, _ = strings.Cut(f, " ")
	f, _, _ = strings.Cut(f, "[")
	return f
}

// comparableView is one projection in the form both engines' are compared in.
// available: no reader, and the kernel prices sources by payment, not by this census.
func comparableView(g *state.Game, ch view.Chars, seat state.PlayerID, vis view.Visibility, ref func(state.ObjID) Ref) any {
	x := renameIDs(toJSONAny(view.ProjectFor(g, ch, seat, vis, nil)), "", ref).(map[string]any)
	for _, p := range x["players"].([]any) {
		delete(p.(map[string]any), "available")
	}
	return x
}

// keepPotentialFor leaves a seat's potential actions only where it holds a priority
// decision, as a set: a kernel offers plays to the seat with priority and to no other.
func keepPotentialFor(x any, holder state.PlayerID) {
	if x == nil {
		return
	}
	for i, p := range x.(map[string]any)["players"].([]any) {
		m := p.(map[string]any)
		if state.PlayerID(i) != holder {
			delete(m, "potential_actions")
			continue
		}
		if l, ok := m["potential_actions"].([]any); ok {
			sort.Slice(l, func(a, b int) bool { return fmt.Sprint(l[a]) < fmt.Sprint(l[b]) })
		}
	}
}

// onlyServedPotential narrows gorge's potential actions to what the served list names:
// gorge also lists a cast its planner cannot pay for, which it leaves to manual mana a
// served game does not have. A play only the served list has stays a difference.
func onlyServedPotential(gv, sv any) {
	gp, sp := gv.(map[string]any)["players"].([]any), sv.(map[string]any)["players"].([]any)
	key := func(x any) string {
		m := x.(map[string]any)
		return fmt.Sprint(m["kind"], m["obj"], m["mode"], m["ability"], m["label"])
	}
	for i := range gp {
		gl, ok := gp[i].(map[string]any)["potential_actions"].([]any)
		if !ok {
			continue
		}
		sl, _ := sp[i].(map[string]any)["potential_actions"].([]any)
		names := map[string]bool{}
		for _, x := range sl {
			names[key(x)] = true
		}
		kept := slices.DeleteFunc(gl, func(x any) bool { return !names[key(x)] })
		if len(kept) == 0 && len(sl) == 0 {
			delete(gp[i].(map[string]any), "potential_actions")
		} else {
			gp[i].(map[string]any)["potential_actions"] = kept
		}
	}
}

// viewCheck compares both engines' projections of one position for every viewer.
func viewCheck(d *viewDiffs, positions *int) func(ref *GorgeSide, a *adapter.Game) {
	return func(ref *GorgeSide, a *adapter.Game) {
		*positions++
		for _, vis := range []view.Visibility{view.Seat, view.Public} {
			for seat := state.PlayerID(0); seat < 2; seat++ {
				if vis == view.Public && seat == 1 {
					continue
				}
				gv, sv := comparableView(ref.E.G, ref.E, seat, vis, ref.InstRef), comparableView(a.Board(), a.Chars(), seat, vis, a.RefOf)
				holder := state.PlayerID(9)
				if p := a.Pending(); p != nil && p.Kind == decision.KPriority {
					holder = p.Player
				}
				keepPotentialFor(gv, holder)
				keepPotentialFor(sv, holder)
				onlyServedPotential(gv, sv)
				d.walk("view", "", gv, sv)
			}
		}
	}
}

// The view comparison is one test per set and surface, each inside the time budget:
// its pairs are cut into viewChunks ranges that run side by side.
func TestNativeViewMatchesGorgeFDN(t *testing.T)           { viewCompareChunked(t, "fdn", true) }
func TestNativeViewMatchesGorgeFDNManualMana(t *testing.T) { viewCompareChunked(t, "fdn", false) }
func TestNativeViewMatchesGorgeBLB(t *testing.T)           { viewCompareChunked(t, "blb", true) }
func TestNativeViewMatchesGorgeBLBManualMana(t *testing.T) { viewCompareChunked(t, "blb", false) }

// viewChunks is how many pair ranges one view test plays side by side.
const viewChunks = 4

func viewCompareChunked(t *testing.T, name string, autopay bool) {
	if testing.Short() && (name != "fdn" || !autopay) {
		t.Skip("short")
	}
	t.Parallel()
	pairChunks(t, envPairs(100), viewChunks, func(t *testing.T, first, n int) {
		viewCompare(t, name, first, n, autopay)
	})
}

// pairChunks runs pairs [0, total) as k contiguous ranges, each a parallel subtest;
// the ranges are disjoint and cover every pair exactly once.
func pairChunks(t *testing.T, total, k int, run func(t *testing.T, first, n int)) {
	k = max(1, min(k, total))
	for i := 0; i < k; i++ {
		first, end := i*total/k, (i+1)*total/k
		t.Run(fmt.Sprintf("pairs%d-%d", first, end), func(t *testing.T) {
			t.Parallel()
			run(t, first, end-first)
		})
	}
}

func viewCompare(t *testing.T, name string, first, pairs int, autopay bool) {
	d := newViewDiffs()
	n, seen := 0, 0
	check := viewCheck(d, &n)
	clean, total := adapterRun(t, name, first, pairs, 1, Config{GorgeAutopaySurface: autopay}, func(ref *GorgeSide, a *adapter.Game) {
		seen++
		if gp, sp := ref.E.G.Priority, a.Board().Priority; gp != sp {
			d.add("priority at every position", "", gp, sp)
		}
		if seen%viewSampling == 0 {
			check(ref, a)
		}
	})
	var b strings.Builder
	fmt.Fprintf(&b, "games %d clean %d positions compared %d of %d\n", total, clean, n, seen)
	var keys []string
	for k := range d.n {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(&b, "%8d %s\n         %s\n", d.n[k], k, d.sample[k])
		t.Errorf("view differs at %s (%d): %s", k, d.n[k], d.sample[k])
	}
	if p := os.Getenv("ADAPTER_OUT"); p != "" {
		os.WriteFile(p+"."+name+fmt.Sprint(autopay)+"."+fmt.Sprint(first), []byte(b.String()), 0o644)
	}
	t.Log(b.String())
}

// viewSampling compares every n-th decision: a projection and its JSON cost far more
// than the game.
const viewSampling = 4

// servedFields are the view's fields a kernel read fills, each compared on its own, and
// shown to have been read at all: a stubbed read fails its field and no other.
var servedFields = []struct {
	field string
	// absent: nothing in these pools sets the field, so only silence is checked.
	absent bool
}{
	{field: "land_drop_spent"},
	{field: "life_gained_this_turn"},
	{field: "life_lost_this_turn"},
	{field: "effective_mana_cost"},
	{field: "activated_this_turn"},
	{field: "summon_sick"},
	{field: "counters"},
	{field: "blocked_by"},
	{field: "keywords"},
	{field: "power"},
	{field: "toughness"},
	{field: "types"},
	{field: "optional"},
	{field: "spells_cast_this_turn"},
	{field: "tokens_created_this_turn"},
	{field: "label"},
	{field: "source"},
	{field: "decider"},
	{field: "library_top", absent: true},
	{field: "pool_restrictions", absent: true},
}

// TestServedFieldsMatchGorge reads the fields over one shared run of FDN and BLB games,
// so the whole table costs a game pool once.
func TestServedFieldsMatchGorge(t *testing.T) {
	d := newViewDiffs()
	positions, seen := 0, 0
	check := viewCheck(d, &positions)
	for _, name := range []string{"fdn", "blb"} {
		adapterRun(t, name, 0, envPairs(25), 1, Config{GorgeAutopaySurface: true}, func(ref *GorgeSide, a *adapter.Game) {
			seen++
			if seen%viewSampling == 0 {
				check(ref, a)
			}
		})
	}
	differs := map[string]int{}
	sample := map[string]string{}
	for path, n := range d.n {
		differs[fieldOf(path)] += n
		sample[fieldOf(path)] = d.sample[path]
	}
	for _, f := range servedFields {
		t.Run(f.field, func(t *testing.T) {
			if n := differs[f.field]; n > 0 {
				t.Errorf("%d differences: %s", n, sample[f.field])
			}
			if shown := d.shown[f.field]; f.absent != (shown == 0) {
				t.Errorf("gorge's view set it %d times over %d positions", shown, positions)
			}
		})
	}
}
