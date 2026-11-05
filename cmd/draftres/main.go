// Command draftres is a draft-research prototype: it drafts 8-seat pods
// with a mix of drafter variants (seat s of pod i plays variant (s+i) mod K),
// builds every pool with one or more builders and writes decks, pool.tsv and
// a pairs.tsv of cross-label games (both seat orders) for dirac's dplay.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/draft/draftlab"
	"github.com/adams-shaun/gorge/draft/limited"
	"github.com/adams-shaun/gorge/draft/limited/pickbot"
	"github.com/adams-shaun/gorge/draft/limited/setdata"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/deck"
)

func main() {
	set := flag.String("set", "FDN", "set")
	pods := flag.Int("pods", 10, "pods")
	seed := flag.Uint64("seed", 1, "first pod seed")
	variants := flag.String("variants", "committed", "comma list of drafter variants: random, rating, rc, committed, signals[:w], each optionally +r=path#field")
	builders := flag.String("builders", "balanced", "comma list of builders (each pool is built by each; labels become variant/builder)")
	corpus := flag.String("corpus", ".cards", "corpus")
	out := flag.String("out", "", "out dir")
	games := flag.Int("games", 2, "games per cross-label pair (alternating seat order)")
	sameVariantOnly := flag.Bool("builder-ab", false, "pair only decks from the same seat's pool built by different builders")
	flag.Parse()

	lab, err := draftlab.Open(*corpus, draftlab.Options{})
	must(err)
	lookup := func(name string) *cards.Card { c, _ := lab.Card(name); return c }
	vs := strings.Split(*variants, ",")
	if 8%len(vs) != 0 {
		must(fmt.Errorf("variant count must divide 8"))
	}
	var nbs []draftlab.NamedBuilder
	bnames := strings.Split(*builders, ",")
	for _, b := range bnames {
		name, rpath, _ := strings.Cut(b, "+r=")
		name, knobs, _ := strings.Cut(name, ":")
		ref, err := draftlab.ParseAgent(name)
		must(err)
		nb, err := lab.Builder(ref)
		must(err)
		if rpath != "" {
			ov := loadOverride(rpath, *set)
			switch x := nb.Builder.(type) {
			case *pickbot.Balanced:
				x.Ratings = ov
			case *pickbot.Best23:
				x.Ratings = ov
			}
		}
		if x, ok := nb.Builder.(*pickbot.Balanced); ok && knobs != "" {
			s := pickbot.DefaultDeckShape
			for _, kv := range strings.Split(knobs, ";") {
				k, v, _ := strings.Cut(kv, "=")
				n, _ := strconv.Atoi(v)
				switch k {
				case "main":
					s.MainSize = n
				case "cmin":
					s.CreatureMin = n
				case "cmax":
					s.CreatureMax = n
				case "top":
					s.CurveMax[5] = n
				default:
					must(fmt.Errorf("builder knob %q", k))
				}
			}
			x.Shape = s
			t := pickbot.DefaultTuning
			t.MainSize = s.MainSize
			x.Tuning = t
		}
		nbs = append(nbs, nb)
	}
	must(os.MkdirAll(filepath.Join(*out, "decks"), 0o755))
	var pool, pairs, index strings.Builder
	pool.WriteString("deck\tsplit\tcolors\tmain_colors\tcards\n")
	rng := rand.New(rand.NewPCG(*seed, 0xd4af7))
	for i := 0; i < *pods; i++ {
		podSeed := *seed + uint64(i)
		packs, err := lab.OpenPod(*set, setdata.DefaultCollation, podSeed, 8, 3)
		must(err)
		var nds []draftlab.NamedDrafter
		labels := make([]string, 8)
		for s := 0; s < 8; s++ {
			v := vs[(s+i)%len(vs)]
			labels[s] = v
			nds = append(nds, draftlab.NamedDrafter{Ref: draftlab.AgentRef{Name: "x"}, Drafter: newDrafter(v, lookup, *set)})
		}
		res, err := lab.Draft(packs, nds, podSeed)
		must(err)
		type dk struct{ id, label, seat string }
		var ds []dk
		for s := 0; s < 8; s++ {
			for bi, nb := range nbs {
				d, err := lab.Build(res.Pools[s], nb)
				must(err)
				lab2 := labels[s]
				if len(nbs) > 1 {
					lab2 += "/" + bnames[bi]
				}
				id := fmt.Sprintf("%s-%d-s%d-b%d", *set, podSeed, s, bi)
				m := describe(lab, res.Pools[s], d)
				f := d.File
				f.Name, f.Format = id, "limited"
				n := 0
				for k, e := range f.Cards {
					n += e.Count
					c, ok := lab.Card(e.Name)
					if ok && len(c.Faces) > 0 && c.Faces[0] != nil && !slices.Contains(c.Faces[0].Types, "Basic") {
						f.Cards[k].Name = c.Faces[0].Name
					}
				}
				writeJSON(filepath.Join(*out, "decks", id+".json"), f)
				fmt.Fprintf(&pool, "%s\tall\t%s\t%s\t%d\n", id, m.Colours, m.Colours, n)
				line, _ := json.Marshal(map[string]any{"id": id, "pod": i, "seat": s, "label": lab2, "variant": labels[s], "builder": bnames[bi], "colours": m.Colours, "splash": m.Splash, "pool": refsNames(res.Pools[s]), "cards": f.Cards})
				index.Write(append(line, '\n'))
				ds = append(ds, dk{id, lab2, strconv.Itoa(s)})
			}
		}
		for a := 0; a < len(ds); a++ {
			for b := a + 1; b < len(ds); b++ {
				if ds[a].label == ds[b].label {
					continue
				}
				if *sameVariantOnly != (ds[a].seat == ds[b].seat) {
					continue
				}
				for g := 0; g < *games; g++ {
					x, y := ds[a].id, ds[b].id
					if g%2 == 1 {
						x, y = y, x
					}
					fmt.Fprintf(&pairs, "%s\t%s\t%d\n", x, y, rng.Uint64())
				}
			}
		}
	}
	must(os.WriteFile(filepath.Join(*out, "pool.tsv"), []byte(pool.String()), 0o644))
	must(os.WriteFile(filepath.Join(*out, "pairs.tsv"), []byte(pairs.String()), 0o644))
	must(os.WriteFile(filepath.Join(*out, "decks.jsonl"), []byte(index.String()), 0o644))
	fmt.Println("draftres: wrote", *out)
}

func refsNames(rs []limited.CardRef) []string {
	var n []string
	for _, r := range rs {
		n = append(n, r.Name)
	}
	return n
}

func loadOverride(spec, set string) *pickbot.RatingsOverride {
	path, field, _ := strings.Cut(spec, "#")
	if field == "" {
		field = "gih"
	}
	b, err := os.ReadFile(path)
	must(err)
	ov, err := pickbot.ParseRatingsOverride(b, path, field, set)
	must(err)
	return ov
}

func newDrafter(v string, lookup pickbot.CardLookup, set string) pickbot.Drafter {
	v, rpath, _ := strings.Cut(v, "+r=")
	var ov *pickbot.RatingsOverride
	if rpath != "" {
		ov = loadOverride(rpath, set)
	}
	name, arg, _ := strings.Cut(v, ":")
	w, _ := strconv.ParseFloat(arg, 64)
	switch name {
	case "random":
		return pickbot.RandomPick{}
	case "rating":
		t := pickbot.DefaultTuning
		t.ColourFitMax = 1e-9
		return &pickbot.RatingColour{Lookup: lookup, Tuning: t, Ratings: ov}
	case "rc":
		t := pickbot.DefaultTuning
		if w > 0 {
			t.ColourFitMax = w
		}
		return &pickbot.RatingColour{Lookup: lookup, Tuning: t, Ratings: ov}
	case "committed":
		return &pickbot.Committed{Lookup: lookup, Ratings: ov}
	case "knob":
		k := &pickbot.Knobbed{Signals: pickbot.Signals{Committed: pickbot.Committed{Lookup: lookup, Ratings: ov}, Weight: -1}}
		t := pickbot.CommittedTuning
		for _, kv := range strings.Split(arg, ";") {
			key, val, _ := strings.Cut(kv, "=")
			f, _ := strconv.ParseFloat(val, 64)
			switch key {
			case "sig":
				k.Weight = f
			case "deny":
				k.Deny = f
			case "cfm":
				t.ColourFitMax = f
			case "need":
				k.NeedWeight = f
			case "bias":
				for _, cb := range strings.Split(val, "/") {
					x, _ := strconv.ParseFloat(cb[1:], 64)
					k.Bias[strings.IndexByte("WUBRG", cb[0])] = x
				}
			case "":
			default:
				must(fmt.Errorf("drafter knob %q", key))
			}
		}
		k.Tuning = t
		return k
	case "signals":
		return &pickbot.Signals{Committed: pickbot.Committed{Lookup: lookup, Ratings: ov}, Weight: w}
	}
	must(fmt.Errorf("unknown variant %q", v))
	return nil
}

func writeJSON(p string, v any) {
	b, err := json.MarshalIndent(v, "", "  ")
	must(err)
	must(os.WriteFile(p, append(b, '\n'), 0o644))
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "draftres:", err)
		os.Exit(1)
	}
}

var _ = deck.File{}
