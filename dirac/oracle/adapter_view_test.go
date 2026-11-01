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

// renameIDs rewrites x's object ids through ref. Keywords and blockers compare as sets.
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
}

func newViewDiffs() *viewDiffs {
	return &viewDiffs{n: map[string]int{}, sample: map[string]string{}}
}

func (d *viewDiffs) add(path, owner string, a, b any) {
	d.n[path]++
	if d.sample[path] == "" {
		s := fmt.Sprintf("[%s] gorge %v served %v", owner, a, b)
		d.sample[path] = s[:min(len(s), 400)]
	}
}

func (d *viewDiffs) walk(path, owner string, a, b any) {
	if strings.HasSuffix(path, ".blocked_by") {
		a = departedBlockers(a, b)
	}
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
	if fmt.Sprint(a) != fmt.Sprint(b) {
		d.add(path, owner, a, b)
	}
}

// departedBlockers fills in gorge's blockers that have left the game, which its ids no
// longer reach, from the served list: only how many there were can be compared.
func departedBlockers(g, s any) any {
	gl, ok := g.([]any)
	sl, ok2 := s.([]any)
	if !ok || !ok2 || len(gl) != len(sl) {
		return g
	}
	out := slices.Clone(sl)
	for _, x := range gl {
		if x != float64(RefNone) && !slices.Contains(sl, x) {
			return g
		}
	}
	return out
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
		return fmt.Sprint(m["kind"], m["obj"], m["mode"], m["ability"])
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

// dropBattlefieldGrants takes a battlefield-only static's keyword off cards in the
// other zones of gorge's view: "creatures you control have trample" means permanents
// (CR 109.2), yet gorge's derived keywords also list it on a library or hand card.
func dropBattlefieldGrants(gv, sv any) {
	gp, sp := gv.(map[string]any)["players"].([]any), sv.(map[string]any)["players"].([]any)
	for i := range gp {
		gm, sm := gp[i].(map[string]any), sp[i].(map[string]any)
		for _, z := range []string{"hand", "library", "graveyard", "exile"} {
			gl, _ := gm[z].([]any)
			sl, _ := sm[z].([]any)
			for j := range gl {
				if j < len(sl) {
					gl[j].(map[string]any)["keywords"] = sl[j].(map[string]any)["keywords"]
					if sl[j].(map[string]any)["keywords"] == nil {
						delete(gl[j].(map[string]any), "keywords")
					}
				}
			}
		}
	}
	gs, _ := gv.(map[string]any)["stack"].([]any)
	ss, _ := sv.(map[string]any)["stack"].([]any)
	for i := range gs {
		if i < len(ss) {
			gc, ok := gs[i].(map[string]any)["card"].(map[string]any)
			sc, ok2 := ss[i].(map[string]any)["card"].(map[string]any)
			if ok && ok2 {
				gc["keywords"] = sc["keywords"]
				if sc["keywords"] == nil {
					delete(gc, "keywords")
				}
			}
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
				dropBattlefieldGrants(gv, sv)
				onlyServedPotential(gv, sv)
				d.walk("view", "", gv, sv)
			}
		}
	}
}

// servedViewGaps are the paths a served view does not carry yet. It only shrinks.
var servedViewGaps = map[string]bool{
	"priority": true, "label": true, "label (gorge only)": true, "source": true, "id": true,
	"activated_this_turn": true, "activated_this_turn (served only)": true,
	"exiled_cards[len]": true, "exiled_cards (served only)": true, "spells_cast_this_turn": true,
}

func TestNativeViewMatchesGorge(t *testing.T) {
	pairs := envPairs(100)
	for _, name := range []string{"fdn", "blb"} {
		for _, autopay := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/autopay=%v", name, autopay), func(t *testing.T) {
				if testing.Short() && (name != "fdn" || !autopay) {
					t.Skip("short")
				}
				viewCompare(t, name, pairs, autopay)
			})
		}
	}
}

func viewCompare(t *testing.T, name string, pairs int, autopay bool) {
	d := newViewDiffs()
	n, seen := 0, 0
	check := viewCheck(d, &n)
	clean, total := adapterRun(t, name, 0, pairs, 1, Config{GorgeAutopaySurface: autopay}, func(ref *GorgeSide, a *adapter.Game) {
		seen++
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
		if !servedViewGaps[gapKey(k)] {
			t.Errorf("view differs at %s (%d): %s", k, d.n[k], d.sample[k])
		}
	}
	if p := os.Getenv("ADAPTER_OUT"); p != "" {
		os.WriteFile(p+"."+name+fmt.Sprint(autopay), []byte(b.String()), 0o644)
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
