package gorgebe

import (
	"sort"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// respellRow is one closed default: a parameter the engine reads the same
// written with Default as absent, which Forge's corpus writes either way.
// Reader and Marker cite the engine code that reads it (Marker is text on
// that line); TestRespellDefaults checks the citation.
type respellRow struct {
	API, Param, Default string
	Reader, Marker      string
	// NoTargets: Default names the source, which an ability acts on only when
	// it declares no ValidTgts$; the row never touches a targeting node.
	NoTargets bool
}

// respellDefaults is the closed list. Adding a row needs a reader citation
// showing that absent and Default read the same.
var respellDefaults = []respellRow{
	{"Draw", "NumCards", "1", "engine/effects/draw.go:22", "numText(h, c, dp.NumCards, 1)", false},
	{"ChangeZone", "ChangeNum", "1", "engine/effects/zone_change.go:984", "numText(h, c, cz.ChangeNum, 1)", false},
	{"Discard", "NumCards", "1", "engine/effects/cardflow.go:407", `Num(h, c, sa, "NumCards", 1)`, false},
	{"Draw", "Defined", "You", "engine/effects/cardflow.go:301", "func actingPlayers", false},
	{"Discard", "Defined", "You", "engine/effects/cardflow.go:301", "func actingPlayers", false},
	{"GainLife", "Defined", "You", "engine/effects/cardflow.go:301", "func actingPlayers", false},
	{"LoseLife", "Defined", "You", "engine/effects/cardflow.go:301", "func actingPlayers", false},
	{"Mill", "Defined", "You", "engine/effects/cardflow.go:301", "func actingPlayers", false},
	{"RearrangeTopOfLibrary", "Defined", "You", "engine/effects/cardflow.go:1783", "actingPlayers(h, c, sa)", false},
	{"PutCounter", "CounterNum", "1", "engine/effects/counters_put.go:25", "n := int32(1)", false},
	{"PutCounter", "Defined", "Self", "engine/effects/context.go:79", "any other on its source", true},
	{"Pump", "Defined", "Self", "engine/effects/context.go:79", "any other on its source", true},
	{"Animate", "Defined", "Self", "engine/effects/context.go:79", "any other on its source", true},
}

func respellRowFor(api, param string) *respellRow {
	for i := range respellDefaults {
		if respellDefaults[i].API == api && respellDefaults[i].Param == param {
			return &respellDefaults[i]
		}
	}
	return nil
}

// followForge makes text agree with forge on the listed defaults only: for a
// printed ability node paired with a Forge node of the same API, a listed
// param that one writes with its default and the other omits is added or
// removed. No other text changes.
func followForge(text []byte, forge *cards.Card) []byte {
	pc, diags := ParseScript("cardc/respell.txt", text)
	if pc == nil || len(diags) > 0 || len(pc.Faces) != len(forge.Faces) {
		return text
	}
	lines := strings.Split(string(text), "\n")
	claimed := map[int]bool{}
	var order []*cards.SA
	fns := map[*cards.SA][]func(string) string{}
	edit := func(sa *cards.SA, fn func(string) string) {
		if _, ok := fns[sa]; !ok {
			order = append(order, sa)
		}
		fns[sa] = append(fns[sa], fn)
	}
	for fi := range pc.Faces {
		gc, gf := respellItems(pc.Faces[fi]), respellItems(forge.Faces[fi])
		keys := make([]string, 0, len(gc))
		for k := range gc {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			for i := 0; i < len(gc[k]) && i < len(gf[k]); i++ {
				a, b := gc[k][i], gf[k][i]
				for n := 0; n < len(a) && n < len(b); n++ {
					if a[n].API != b[n].API {
						continue
					}
					for _, r := range respellDefaults {
						if r.API != a[n].API || r.NoTargets && (a[n].Params["ValidTgts"] != "" || b[n].Params["ValidTgts"] != "") {
							continue
						}
						cv, cok := a[n].Params[r.Param]
						fv, fok := b[n].Params[r.Param]
						switch {
						case cok && !fok && strings.TrimSpace(cv) == r.Default:
							edit(a[n], func(l string) string { return dropParam(l, r.Param) })
						case !cok && fok && strings.TrimSpace(fv) == r.Default:
							edit(a[n], func(l string) string { return addParam(l, r.Param, r.Default, b[n].Line) })
						}
					}
				}
			}
		}
	}
	for _, sa := range order {
		for i, l := range lines {
			if claimed[i] || !strings.HasSuffix(l, sa.Line) || strings.HasPrefix(l, "Oracle:") {
				continue
			}
			claimed[i] = true
			edited := sa.Line
			for _, fn := range fns[sa] {
				edited = fn(edited)
			}
			lines[i] = l[:len(l)-len(sa.Line)] + edited
			break
		}
	}
	return []byte(strings.Join(lines, "\n"))
}

// respellItems is a face's ability chains by item key, in diff's keying.
func respellItems(f *cards.Face) map[string][][]*cards.SA {
	m := map[string][][]*cards.SA{}
	add := func(key string, sa *cards.SA) {
		var chain []*cards.SA
		var apis []string
		for ; sa != nil; sa = sa.Sub {
			if sa.API != "Cleanup" {
				chain = append(chain, sa)
				apis = append(apis, sa.API)
			}
		}
		m[key+strings.Join(apis, ">")] = append(m[key+strings.Join(apis, ">")], chain)
	}
	for _, a := range f.Abilities {
		add("A:"+a.Kind+":", a)
	}
	for _, t := range f.Triggers {
		add("T:"+t.Mode+":", t.Effect)
	}
	for _, r := range f.Repls {
		add("R:"+r.Event+":", r.With)
	}
	return m
}

func segKey(seg string) string {
	k, _, _ := strings.Cut(strings.TrimSpace(seg), "$")
	return k
}

// dropParam removes param from a script line.
func dropParam(line, param string) string {
	segs := strings.Split(line, " | ")
	out := segs[:0:0]
	for _, s := range segs {
		if segKey(s) != param {
			out = append(out, s)
		}
	}
	return strings.Join(out, " | ")
}

// addParam inserts "param$ val" after the param that precedes it in Forge's
// line, or right after the head when none of those is present.
func addParam(line, param, val, forgeLine string) string {
	segs := strings.Split(line, " | ")
	pos := 1
	have := map[string]int{}
	for i, s := range segs {
		have[segKey(s)] = i
	}
	for _, s := range strings.Split(forgeLine, " | ") {
		k := segKey(s)
		if k == param {
			break
		}
		if i, ok := have[k]; ok && i >= pos {
			pos = i + 1
		}
	}
	out := append([]string{}, segs[:pos]...)
	out = append(out, param+"$ "+val)
	return strings.Join(append(out, segs[pos:]...), " | ")
}
