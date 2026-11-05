// Command draftres is the draft-research harness: it drafts 8-seat pods with
// a mix of drafter variants (seat s of pod i plays variant (s+i) mod K),
// builds every pool with one or more builders, writes decks, pool.tsv and a
// pairs.tsv of cross-label games (both seat orders), plays them in dirac's
// dplay (hbot vs hbot, lockstep-autopay, refused cards skipped) and prints
// each label's win rate with a 95% pod-bootstrap interval.
// docs/site/research/drafting.md documents the method and the flags.
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
)

func main() {
	set := flag.String("set", "FDN", "set")
	pods := flag.Int("pods", 10, "pods")
	seed := flag.Uint64("seed", 1, "first pod seed; pod i drafts with seed+i, and the summary's bootstrap seeds from it")
	variants := flag.String("variants", "committed", "comma list of drafter variants: random, rating, rc[:w], committed, signals[:w], knob:k=v;..., each optionally +r=path#field")
	builders := flag.String("builders", "balanced", "comma list of builders (each pool is built by each; labels become variant/builder)")
	corpus := flag.String("corpus", ".cards", "corpus")
	out := flag.String("out", "", "out dir: one chunk-NNNN directory per chunk")
	games := flag.Int("games", 2, "games per cross-label pair (alternating seat order)")
	sameVariantOnly := flag.Bool("builder-ab", false, "pair only decks from the same seat's pool built by different builders")
	chunk := flag.Int("chunk", 0, "pods per chunk (0: one chunk); a rerun with the same -out skips finished chunks")
	dplay := flag.String("dplay", defaultDplay, "command that plays a chunk's pairs (dirac dplay); \"none\" only drafts")
	workers := flag.Int("workers", 1, "dplay -workers")
	flag.Parse()
	if *out == "" {
		must(fmt.Errorf("-out is required"))
	}

	lab, err := draftlab.Open(*corpus, draftlab.Options{})
	must(err)
	lookup := func(name string) *cards.Card { c, _ := lab.Card(name); return c }
	bnames := strings.Split(*builders, ",")
	h := &harness{
		lab: lab, set: *set, seed: *seed, games: *games, builderAB: *sameVariantOnly,
		variants: strings.Split(*variants, ","), bnames: bnames, lookup: lookup,
	}
	for _, b := range bnames {
		h.builders = append(h.builders, parseBuilder(lab, b, *set))
	}
	var play func(dir string) error
	if *dplay != "none" {
		cmd := strings.Fields(*dplay)
		play = func(dir string) error { return playChunk(cmd, dir, *set, *corpus, *workers) }
	}
	ran, err := runChunks(*out, planChunks(*pods, *chunk), func(c chunkPlan, dir string) error {
		if err := h.draftChunk(c, dir); err != nil {
			return err
		}
		if play == nil {
			return nil
		}
		return play(dir)
	})
	must(err)
	fmt.Printf("draftres: %s: ran chunks %v\n", *out, ran)
	if play == nil {
		return
	}
	games2, err := readResults(*out)
	must(err)
	printSummary(os.Stdout, games2, bootstrap(games2, *seed, bootstrapResamples))
}

// harness is one run's fixed inputs: what drafts, what builds, how pairs are dealt.
type harness struct {
	lab       *draftlab.Lab
	lookup    pickbot.CardLookup
	set       string
	seed      uint64
	games     int
	builderAB bool
	variants  []string
	bnames    []string
	builders  []draftlab.NamedBuilder
}

// seatVariant is the variant seat s of pod i drafts with: (s+i) mod K, so every
// variant sits in every seat once per K pods.
func seatVariant(vs []string, pod, seat int) string { return vs[(seat+pod)%len(vs)] }

func parseBuilder(lab *draftlab.Lab, b, set string) draftlab.NamedBuilder {
	name, rpath, _ := strings.Cut(b, "+r=")
	name, knobs, _ := strings.Cut(name, ":")
	ref, err := draftlab.ParseAgent(name)
	must(err)
	nb, err := lab.Builder(ref)
	must(err)
	if rpath != "" {
		ov := loadOverride(rpath, set)
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
	return nb
}

// draftChunk drafts pods [c.First, c.Last) and writes their decks, pool.tsv,
// pairs.tsv and decks.jsonl under dir. Pair seeds come from a stream seeded by
// the chunk's first pod, so a chunk's files do not depend on the chunk size of
// the run that wrote the chunks before it.
func (h *harness) draftChunk(c chunkPlan, dir string) error {
	lab := h.lab
	if err := os.MkdirAll(filepath.Join(dir, "decks"), 0o755); err != nil {
		return err
	}
	var pool, pairs, index strings.Builder
	pool.WriteString("deck\tsplit\tcolors\tmain_colors\tcards\n")
	rng := rand.New(rand.NewPCG(h.seed+uint64(c.First), 0xd4af7))
	for i := c.First; i < c.Last; i++ {
		podSeed := h.seed + uint64(i)
		packs, err := lab.OpenPod(h.set, setdata.DefaultCollation, podSeed, 8, 3)
		if err != nil {
			return err
		}
		var nds []draftlab.NamedDrafter
		labels := make([]string, 8)
		for s := 0; s < 8; s++ {
			v := seatVariant(h.variants, i, s)
			labels[s] = v
			nds = append(nds, draftlab.NamedDrafter{Ref: draftlab.AgentRef{Name: "x"}, Drafter: newDrafter(v, h.lookup, h.set)})
		}
		res, err := lab.Draft(packs, nds, podSeed)
		if err != nil {
			return err
		}
		type dk struct{ id, label, seat string }
		var ds []dk
		for s := 0; s < 8; s++ {
			for bi, nb := range h.builders {
				d, err := lab.Build(res.Pools[s], nb)
				if err != nil {
					return err
				}
				lab2 := labels[s]
				if len(h.builders) > 1 {
					lab2 += "/" + h.bnames[bi]
				}
				id := fmt.Sprintf("%s-%d-s%d-b%d", h.set, podSeed, s, bi)
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
				writeJSON(filepath.Join(dir, "decks", id+".json"), f)
				fmt.Fprintf(&pool, "%s\tall\t%s\t%s\t%d\n", id, m.Colours, m.Colours, n)
				line, _ := json.Marshal(map[string]any{"id": id, "pod": i, "seat": s, "label": lab2, "variant": labels[s], "builder": h.bnames[bi], "colours": m.Colours, "splash": m.Splash, "pool": refsNames(res.Pools[s]), "cards": f.Cards})
				index.Write(append(line, '\n'))
				ds = append(ds, dk{id, lab2, strconv.Itoa(s)})
			}
		}
		for a := 0; a < len(ds); a++ {
			for b := a + 1; b < len(ds); b++ {
				if ds[a].label == ds[b].label {
					continue
				}
				if h.builderAB != (ds[a].seat == ds[b].seat) {
					continue
				}
				for g := 0; g < h.games; g++ {
					x, y := ds[a].id, ds[b].id
					if g%2 == 1 {
						x, y = y, x
					}
					fmt.Fprintf(&pairs, "%s\t%s\t%d\n", x, y, rng.Uint64())
				}
			}
		}
	}
	for _, f := range [...]struct{ name, body string }{{"pool.tsv", pool.String()}, {"pairs.tsv", pairs.String()}, {"decks.jsonl", index.String()}} {
		if err := os.WriteFile(filepath.Join(dir, f.name), []byte(f.body), 0o644); err != nil {
			return err
		}
	}
	return nil
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
		t, err := pickbot.ParseResearchKnobs(arg)
		must(err)
		return &pickbot.Knobbed{Lookup: lookup, Tuning: t, Ratings: ov}
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
