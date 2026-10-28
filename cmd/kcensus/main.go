// Command kcensus is a mechanic census: it
// loads a deck pool, resolves every distinct card through the compiled
// corpus, tabulates the mechanics each card reaches (API codes along every
// Sub/trigger/replacement/SVar chain, trigger modes, static modes,
// replacement events, keyword heads, mana-cost class and non-mana cost
// tokens) and prints the smallest mechanic set that fully covers 50/80/95/100%
// of the decks, plus the share of decks that need the general layer,
// replacement, copy or control machinery.
//
//	go run ./cmd/kcensus -decks <dir> -cards .cards [-out <dir>]
//
// The mechanic labels reuse Face.Primitives' spelling (api:/trig:/stat:/
// repl:/kw:) so they match the coverage ratchet's; cost: and mana: labels and
// the sa:<Kind> ability-kind labels are this program's own.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

type deckFile struct {
	Name  string `json:"name"`
	Cards []struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	} `json:"cards"`
}

type cardInfo struct {
	name   string
	basic  bool
	decks  int
	copies int
	mechs  []string // sorted, distinct
	class  []string // machinery classes
}

func main() {
	decksDir := flag.String("decks", "", "deck pool directory (*.json)")
	cardsDir := flag.String("cards", ".cards", "card corpus directory")
	outDir := flag.String("out", "", "directory for the report files (optional)")
	flag.Parse()
	if *decksDir == "" {
		fmt.Fprintln(os.Stderr, "kcensus: -decks is required")
		os.Exit(2)
	}
	if err := run(*decksDir, *cardsDir, *outDir); err != nil {
		fmt.Fprintln(os.Stderr, "kcensus:", err)
		os.Exit(1)
	}
}

func run(decksDir, cardsDir, outDir string) error {
	paths, err := filepath.Glob(filepath.Join(decksDir, "*.json"))
	if err != nil {
		return err
	}
	sort.Strings(paths)
	// deckCards[i] = distinct card names of deck i.
	deckCards := make([][]string, 0, len(paths))
	deckNames := make([]string, 0, len(paths))
	byName := map[string]*cardInfo{}
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		var d deckFile
		if err := json.Unmarshal(b, &d); err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		seen := map[string]bool{}
		var names []string
		for _, c := range d.Cards {
			ci := byName[c.Name]
			if ci == nil {
				ci = &cardInfo{name: c.Name}
				byName[c.Name] = ci
			}
			ci.copies += c.Count
			if !seen[c.Name] {
				seen[c.Name] = true
				ci.decks++
				names = append(names, c.Name)
			}
		}
		sort.Strings(names)
		deckCards = append(deckCards, names)
		deckNames = append(deckNames, d.Name)
	}
	names := make([]string, 0, len(byName))
	for n := range byName {
		names = append(names, n)
	}
	sort.Strings(names)

	reg, err := cards.OpenCorpusSubset(cardsDir, names)
	if err != nil {
		return err
	}
	var unresolved []string
	for _, n := range names {
		c, ok := reg.Lookup(n)
		if !ok {
			unresolved = append(unresolved, n)
			continue
		}
		ci := byName[n]
		ci.basic = len(c.Faces) > 0 && c.Faces[0].IsBasic()
		ci.mechs, ci.class = mechanics(c)
	}

	var out io.Writer = os.Stdout
	var files map[string]*os.File
	if outDir != "" {
		if err := os.MkdirAll(outDir, 0o755); err != nil {
			return err
		}
		files = map[string]*os.File{}
	}
	open := func(name string) io.Writer {
		if files == nil {
			return out
		}
		f, err := os.Create(filepath.Join(outDir, name))
		if err != nil {
			panic(err)
		}
		files[name] = f
		return io.MultiWriter(out, f)
	}
	defer func() {
		for _, f := range files {
			f.Close()
		}
	}()

	nDecks := len(deckCards)
	nonBasic := 0
	for _, ci := range byName {
		if !ci.basic {
			nonBasic++
		}
	}
	w := open("summary.txt")
	fmt.Fprintf(w, "decks: %d  distinct cards: %d  non-basic: %d  unresolved: %d\n", nDecks, len(names), nonBasic, len(unresolved))
	for _, n := range unresolved {
		fmt.Fprintf(w, "UNRESOLVED: %s\n", n)
	}

	// Mechanic index.
	mechIdx := map[string]int{}
	var mechs []string
	for _, n := range names {
		for _, m := range byName[n].mechs {
			if _, ok := mechIdx[m]; !ok {
				mechIdx[m] = len(mechs)
				mechs = append(mechs, m)
			}
		}
	}
	words := (len(mechs) + 63) / 64
	cardMask := map[string][]uint64{}
	for _, n := range names {
		m := make([]uint64, words)
		for _, l := range byName[n].mechs {
			i := mechIdx[l]
			m[i/64] |= 1 << (i % 64)
		}
		cardMask[n] = m
	}
	deckMask := make([][]uint64, nDecks)
	for i, dc := range deckCards {
		m := make([]uint64, words)
		for _, n := range dc {
			for k, v := range cardMask[n] {
				m[k] |= v
			}
		}
		deckMask[i] = m
	}
	mechCards := make([]int, len(mechs))
	mechDecks := make([]int, len(mechs))
	for _, n := range names {
		for _, l := range byName[n].mechs {
			mechCards[mechIdx[l]]++
		}
	}
	for _, m := range deckMask {
		for i := range mechs {
			if m[i/64]&(1<<(i%64)) != 0 {
				mechDecks[i]++
			}
		}
	}

	// (a) mechanic table.
	w = open("mechanics.tsv")
	order := make([]int, len(mechs))
	for i := range order {
		order[i] = i
	}
	sort.Slice(order, func(a, b int) bool {
		x, y := order[a], order[b]
		if mechDecks[x] != mechDecks[y] {
			return mechDecks[x] > mechDecks[y]
		}
		if mechCards[x] != mechCards[y] {
			return mechCards[x] > mechCards[y]
		}
		return mechs[x] < mechs[y]
	})
	fmt.Fprintf(w, "mechanic\tclass\tcards\tdecks\tdeck_share\n")
	for _, i := range order {
		cls, _, _ := strings.Cut(mechs[i], ":")
		fmt.Fprintf(w, "%s\t%s\t%d\t%d\t%.1f%%\n", mechs[i], cls, mechCards[i], mechDecks[i], 100*float64(mechDecks[i])/float64(nDecks))
	}
	fmt.Fprintf(w, "# %d distinct mechanics\n", len(mechs))

	// (b) coverage curve, mechanic unit.
	w = open("coverage_mechanics.txt")
	totalCards := len(names)
	cardCovered := func(sel []uint64) int {
		n := 0
		for _, nm := range names {
			if subset(cardMask[nm], sel) {
				n++
			}
		}
		return n
	}
	greedy := func(w io.Writer, unit string, nItems int, label func(int) string, itemCards func(int) int, itemDecks func(int) int, itemMask func(int) []uint64) {
		// Both units are reduced to the mechanic bitset: a card "item" is
		// its mechanic mask, so a deck is covered when its mask is a subset
		// of the union of the chosen items' masks.
		sel := make([]uint64, words)
		covered := make([]bool, nDecks)
		chosen := []int{}
		have := make([]bool, nItems)
		thresholds := []float64{50, 80, 95, 100}
		ti := 0
		lastT := 0
		nCov := 0
		fmt.Fprintf(w, "unit=%s  size\tdecks_covered\tdeck%%\tcards_covered\tcard%%\tadded\n", unit)
		for ti < len(thresholds) && len(chosen) < nItems {
			bestI, bestGain, bestDecks, bestCards := -1, -1, -1, -1
			for i := 0; i < nItems; i++ {
				if have[i] {
					continue
				}
				m := itemMask(i)
				if subset(m, sel) {
					// Adds nothing to the mechanic union (a card whose
					// mechanics are already chosen): take it for free.
					have[i] = true
					chosen = append(chosen, i)
					continue
				}
				trial := make([]uint64, words)
				for k := range trial {
					trial[k] = sel[k] | m[k]
				}
				gain := 0
				for d := range deckMask {
					if !covered[d] && subset(deckMask[d], trial) {
						gain++
					}
				}
				if gain > bestGain || (gain == bestGain && itemDecks(i) > bestDecks) || (gain == bestGain && itemDecks(i) == bestDecks && itemCards(i) > bestCards) {
					bestI, bestGain, bestDecks, bestCards = i, gain, itemDecks(i), itemCards(i)
				}
			}
			if bestI < 0 {
				break
			}
			have[bestI] = true
			chosen = append(chosen, bestI)
			for k, v := range itemMask(bestI) {
				sel[k] |= v
			}
			for d := range deckMask {
				if !covered[d] && subset(deckMask[d], sel) {
					covered[d] = true
					nCov++
				}
			}
			pct := 100 * float64(nCov) / float64(nDecks)
			cc := cardCovered(sel)
			fmt.Fprintf(w, "%d\t%d\t%.1f%%\t%d\t%.1f%%\t%s\n", len(chosen), nCov, pct, cc, 100*float64(cc)/float64(totalCards), label(bestI))
			for ti < len(thresholds) && pct >= thresholds[ti] {
				fmt.Fprintf(w, "== %.0f%% of decks fully covered by %d %s(s) (+%d since previous threshold):\n", thresholds[ti], len(chosen), unit, len(chosen)-lastT)
				for _, c := range chosen[lastT:] {
					fmt.Fprintf(w, "   %s\n", label(c))
				}
				lastT = len(chosen)
				ti++
			}
		}
	}
	greedy(w, "mechanic", len(mechs), func(i int) string { return mechs[i] },
		func(i int) int { return mechCards[i] }, func(i int) int { return mechDecks[i] },
		func(i int) []uint64 { m := make([]uint64, words); m[i/64] = 1 << (i % 64); return m })

	w = open("coverage_cards.txt")
	nb := []string{}
	for _, n := range names {
		nb = append(nb, n)
	}
	greedy(w, "card", len(nb), func(i int) string { return fmt.Sprintf("%s (%d decks)", nb[i], byName[nb[i]].decks) },
		func(i int) int { return 1 }, func(i int) int { return byName[nb[i]].decks },
		func(i int) []uint64 { return cardMask[nb[i]] })

	// (c) stop-condition check.
	w = open("machinery.txt")
	classes := []string{"layers-hard", "layers-light", "grant", "repl-hard", "repl-etb", "copy", "control"}
	for _, cls := range classes {
		var named []string
		hit := make([]bool, nDecks)
		for _, n := range names {
			ci := byName[n]
			if !contains(ci.class, cls) {
				continue
			}
			named = append(named, fmt.Sprintf("%s (%d decks)", n, ci.decks))
		}
		for d, dc := range deckCards {
			for _, n := range dc {
				if contains(byName[n].class, cls) {
					hit[d] = true
					break
				}
			}
		}
		nh := 0
		for _, h := range hit {
			if h {
				nh++
			}
		}
		fmt.Fprintf(w, "%s: %d cards, %d decks (%.1f%%)\n", cls, len(named), nh, 100*float64(nh)/float64(nDecks))
		for _, s := range named {
			fmt.Fprintf(w, "   %s\n", s)
		}
	}
	hard := map[string]bool{"layers-hard": true, "repl-hard": true, "copy": true, "control": true}
	anyHard, anyAll := 0, 0
	for _, dc := range deckCards {
		h, a := false, false
		for _, n := range dc {
			for _, c := range byName[n].class {
				a = true
				if hard[c] {
					h = true
				}
			}
		}
		if h {
			anyHard++
		}
		if a {
			anyAll++
		}
	}
	fmt.Fprintf(w, "decks with any HARD machinery card (layers-hard, repl-hard, copy, control): %d (%.1f%%)\n", anyHard, 100*float64(anyHard)/float64(nDecks))
	fmt.Fprintf(w, "decks with any classed card at all: %d (%.1f%%)\n", anyAll, 100*float64(anyAll)/float64(nDecks))

	// (d) per-card listing.
	w = open("cards.tsv")
	fmt.Fprintf(w, "card\tdecks\tcopies\tclasses\tmechanics\n")
	byDecks := append([]string(nil), names...)
	sort.Slice(byDecks, func(a, b int) bool {
		x, y := byName[byDecks[a]], byName[byDecks[b]]
		if x.decks != y.decks {
			return x.decks > y.decks
		}
		return x.name < y.name
	})
	for _, n := range byDecks {
		ci := byName[n]
		fmt.Fprintf(w, "%s\t%d\t%d\t%s\t%s\n", n, ci.decks, ci.copies, strings.Join(ci.class, ","), strings.Join(ci.mechs, " "))
	}
	_ = deckNames
	return nil
}

func subset(m, of []uint64) bool {
	for k := range m {
		if m[k]&^of[k] != 0 {
			return false
		}
	}
	return true
}

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

// mechanics is the card's mechanic label set and its machinery classes.
func mechanics(c *cards.Card) (labels, classes []string) {
	set := map[string]bool{}
	cls := map[string]bool{}
	for _, f := range c.Faces {
		if f == nil {
			continue
		}
		for _, p := range f.Primitives() {
			set[p] = true
		}
		set["mana:"+manaClass(f.ManaCost)] = true
		for _, a := range f.Abilities {
			set["sa:"+a.Kind] = true
			if a.Params["Planeswalker"] == "True" {
				set["sa:Loyalty"] = true
			}
		}
		var walk func(sa *cards.SA, depth int)
		walk = func(sa *cards.SA, depth int) {
			if sa == nil || depth > 16 {
				return
			}
			for _, tok := range costTokens(sa.Params["Cost"]) {
				if h, ok := costHead(tok); ok {
					set["cost:"+h] = true
				}
			}
			switch sa.API {
			case "CopyPermanent", "CopySpellAbility", "Clone":
				cls["copy"] = true
			case "GainControl", "ControlSpell", "ExchangeControl":
				cls["control"] = true
			case "Animate", "AnimateAll":
				// A type or P/T change is the layer system; a grant of
				// keywords or triggers alone is an until-EOT delta.
				if sa.Params["Types"] != "" || sa.Params["Power"] != "" || sa.Params["Toughness"] != "" || sa.Params["AddAllCreatureTypes"] != "" {
					cls[animateClass(sa.Params)] = true
				} else {
					cls["grant"] = true
				}
			}
			walk(sa.Sub, depth+1)
		}
		for _, a := range f.Abilities {
			walk(a, 0)
		}
		for _, t := range f.Triggers {
			walk(t.Effect, 0)
		}
		for _, r := range f.Repls {
			walk(r.With, 0)
		}
		f.EachSVarAbility(func(sa *cards.SA) { walk(sa, 0) })
		for _, r := range f.Repls {
			cls[replClass(r)] = true
		}
		for _, s := range f.Statics {
			if c := staticClass(s); c != "" {
				cls[c] = true
			}
			if s.Params["GainControl"] != "" {
				cls["control"] = true
			}
		}
	}
	for l := range set {
		labels = append(labels, l)
	}
	sort.Strings(labels)
	for k := range cls {
		classes = append(classes, k)
	}
	sort.Strings(classes)
	return labels, classes
}

// animateClass is "layers-hard" for an Animate that sets a card type, P/T
// or every creature type (the layer system proper) and "layers-light" for
// one that only swaps creature subtypes.
func animateClass(p map[string]string) string {
	if p["Power"] != "" || p["Toughness"] != "" || p["AddAllCreatureTypes"] != "" {
		return "layers-hard"
	}
	for _, t := range strings.Split(p["Types"], ",") {
		switch strings.TrimSpace(t) {
		case "Creature", "Artifact", "Enchantment", "Land", "Planeswalker", "Battle":
			return "layers-hard"
		}
	}
	return "layers-light"
}

// staticClass is "layers-hard" for a Continuous static that sets P/T, a card
// type, colour or strips abilities (real layer ordering), "layers-light" for
// one that only adds a subtype, a CDA P/T or a hand-size rule, "" otherwise.
func staticClass(s cards.Static) string {
	if s.Mode != "Continuous" {
		return ""
	}
	p := s.Params
	if p["CharacteristicDefining"] == "True" {
		return "layers-light"
	}
	for _, k := range []string{"SetPower", "SetToughness", "SetColor", "AddColor", "RemoveCardTypes", "RemoveAllAbilities", "AddAllCreatureTypes", "RemoveType", "SetName"} {
		if p[k] != "" {
			return "layers-hard"
		}
	}
	if t := p["AddType"]; t != "" {
		for _, x := range strings.FieldsFunc(t, func(r rune) bool { return r == '&' || r == ',' || r == ' ' }) {
			switch x {
			case "Creature", "Artifact", "Enchantment", "Land", "Planeswalker", "Battle":
				return "layers-hard"
			}
		}
		return "layers-light"
	}
	if p["SetMaxHandSize"] != "" {
		return "layers-light"
	}
	return ""
}

// replClass is "repl-etb" for an enters-the-battlefield modifier (tapped,
// counters, an as-enters choice) or a CantHappen flag (can't be countered,
// doesn't untap, can't lose), and "repl-hard" for a replacement that rewrites
// an event in flight (damage, tokens, counters, a zone change).
func replClass(r cards.Repl) string {
	p := r.Params
	if p["Layer"] == "CantHappen" {
		return "repl-etb"
	}
	if r.Event == "Moved" && p["Destination"] == "Battlefield" {
		return "repl-etb"
	}
	return "repl-hard"
}

// costTokens splits a Cost$ value on spaces outside <...>.
func costTokens(cost string) []string {
	var out []string
	depth := 0
	start := -1
	for i, r := range cost {
		switch {
		case r == '<':
			depth++
		case r == '>':
			depth--
		}
		if r == ' ' && depth == 0 {
			if start >= 0 {
				out = append(out, cost[start:i])
				start = -1
			}
			continue
		}
		if start < 0 {
			start = i
		}
	}
	if start >= 0 {
		out = append(out, cost[start:])
	}
	return out
}

// manaClass is the printed mana cost's shape.
func manaClass(cost string) string {
	cost = strings.TrimSpace(cost)
	if cost == "" || cost == "no cost" {
		return "none"
	}
	hybrid, x, phy := false, false, false
	for _, t := range strings.Fields(cost) {
		switch {
		case strings.Contains(t, "P/") || strings.HasPrefix(t, "P"):
			phy = true
		case strings.Contains(t, "/"):
			hybrid = true
		case t == "X" || t == "Y" || t == "Z":
			x = true
		}
	}
	switch {
	case phy:
		return "phyrexian"
	case hybrid && x:
		return "hybrid+X"
	case hybrid:
		return "hybrid"
	case x:
		return "X"
	}
	return "plain"
}

// costHead is the non-mana head of one Cost$ token ("" , false for a mana
// symbol).
func costHead(tok string) (string, bool) {
	if tok == "" {
		return "", false
	}
	if isManaToken(tok) {
		return "", false
	}
	if i := strings.IndexByte(tok, '<'); i > 0 {
		return tok[:i], true
	}
	return tok, true
}

func isManaToken(t string) bool {
	if t == "X" || t == "Y" || t == "Z" {
		return true
	}
	for _, part := range strings.Split(t, "/") {
		switch part {
		case "W", "U", "B", "R", "G", "C", "S", "P",
			"WU", "WB", "UB", "UR", "BR", "BG", "RG", "RW", "GW", "GU":
			continue
		}
		for _, r := range part {
			if r < '0' || r > '9' {
				return false
			}
		}
	}
	return true
}
