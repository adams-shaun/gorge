package main

import (
	"bytes"
	"fmt"
	"math/rand/v2"
	"strconv"
)

// saltPairs is the pairs stream's salt: pod i draws its pairs and their
// games' seeds from PCG(seed, saltPairs<<32|i), one uint64 per game in pair
// order. The pod seeds themselves stay seed+i, so the whole output is a
// pure function of the flags.
const saltPairs = 0x9A11

// pairLine is one pairs.tsv line: the two decks' names and the game's seed.
type pairLine struct {
	a, b string
	seed uint64
}

// poolRow is one pool.tsv row: a deck's name, its colours (the two
// heaviest, then the splash) and its card count.
type poolRow struct {
	Deck    string
	Colours string
	Splash  string
	Cards   int
}

// validate refuses a bad pairing flag, after run filled the zero values
// with the flags' defaults.
func (o options) validate() error {
	switch o.pairing {
	case "roundrobin", "random", "cross":
	default:
		return fmt.Errorf("--pairing %q: want roundrobin, random or cross", o.pairing)
	}
	if o.pairing == "random" && o.pairsN < 1 {
		return fmt.Errorf("--pairs-per-pod must be >= 1 with --pairing random, got %d", o.pairsN)
	}
	if o.gamesN < 1 {
		return fmt.Errorf("--games-per-pair must be >= 1, got %d", o.gamesN)
	}
	return nil
}

// pairsFor is a run's pairs.tsv lines, pods in pod order: every pod's
// --pairing pairs, each --games-per-pair times with its own seed. Pairing
// is within a pod, except cross, which pairs seat s of pod i against seat s
// of pod i+1.
func pairsFor(o options, pods [][]string) ([]pairLine, error) {
	var out []pairLine
	for i, decks := range pods {
		switch o.pairing {
		case "roundrobin", "random":
			lines, err := podPairs(o, i, decks)
			if err != nil {
				return nil, err
			}
			out = append(out, lines...)
		case "cross":
			if i+1 == len(pods) {
				continue
			}
			next := pods[i+1]
			rng := rand.New(rand.NewPCG(o.seed, saltPairs<<32|uint64(uint32(i))))
			for s := range decks {
				for g := 0; g < o.gamesN; g++ {
					out = append(out, pairLine{a: decks[s], b: next[s], seed: rng.Uint64()})
				}
			}
		default:
			return nil, fmt.Errorf("--pairing %q: want roundrobin, random or cross", o.pairing)
		}
	}
	return out, nil
}

// podPairs is pod i's pairs.tsv lines: the seat pairs --pairing names, each
// --games-per-pair times with its own seed, from PCG(seed, saltPairs<<32|i).
// A pair's orientation alternates with its seats' sum, so neither seat
// index is always deckA.
func podPairs(o options, i int, decks []string) ([]pairLine, error) {
	rng := rand.New(rand.NewPCG(o.seed, saltPairs<<32|uint64(uint32(i))))
	var all [][2]int
	for a := 0; a < len(decks); a++ {
		for b := a + 1; b < len(decks); b++ {
			if (a+b)%2 == 0 {
				all = append(all, [2]int{b, a})
			} else {
				all = append(all, [2]int{a, b})
			}
		}
	}
	var chosen [][2]int
	switch o.pairing {
	case "roundrobin":
		chosen = all
	case "random":
		for len(chosen) < o.pairsN {
			for _, k := range rng.Perm(len(all)) {
				if len(chosen) == o.pairsN {
					break
				}
				chosen = append(chosen, all[k])
			}
		}
	default:
		return nil, fmt.Errorf("--pairing %q: want roundrobin or random", o.pairing)
	}
	out := make([]pairLine, 0, len(chosen)*o.gamesN)
	for _, p := range chosen {
		for g := 0; g < o.gamesN; g++ {
			out = append(out, pairLine{a: decks[p[0]], b: decks[p[1]], seed: rng.Uint64()})
		}
	}
	return out, nil
}

// poolTSV is a pool.tsv (deck, split, colors, main_colors, cards), the
// columns dirac's oracle.ReadPool reads: one row per deck written, split
// the --split value on every row. colors is every colour the deck's spells
// source and main_colors its two heaviest.
func poolTSV(rows []poolRow, split string) []byte {
	var buf bytes.Buffer
	buf.WriteString("deck\tsplit\tcolors\tmain_colors\tcards\n")
	for _, r := range rows {
		colors := r.Colours + r.Splash
		if colors == "" {
			colors = "C" // a pool.tsv row needs three non-empty columns
		}
		fmt.Fprintf(&buf, "%s\t%s\t%s\t%s\t%d\n", r.Deck, split, colors, r.Colours, r.Cards)
	}
	return buf.Bytes()
}

// pairsTSV is a pairs.tsv: a comment naming the run, then one
// deckA<TAB>deckB<TAB>seed line per game, in the shape dirac's
// oracle.ExplicitPairs reads.
func pairsTSV(pairs []pairLine, o options) []byte {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "# deckpool %d pairs: %s, %d games a pair, seed %d\n", len(pairs), o.pairing, o.gamesN, o.seed)
	for _, p := range pairs {
		buf.WriteString(p.a + "\t" + p.b + "\t" + strconv.FormatUint(p.seed, 10) + "\n")
	}
	return buf.Bytes()
}
