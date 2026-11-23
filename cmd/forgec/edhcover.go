package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/deck"
	"github.com/adams-shaun/gorge/engine/effects"
)

// The edhcover command measures gorge's playable coverage over Commander
// decklists: every official precon MTGJSON lists and Archidekt's most-viewed
// community decks, overall and per Commander bracket. scripts/edh_decks.py
// fetches the lists and writes two files under the gitignored .edh-decks/: the
// manifest, edh-manifest.tsv (ids, names, bracket, rank, source URL, fetch
// date), and edh-decks.jsonl, the stored decks themselves, one per line (source,
// id, kind, commanders and the 99 card names). The decklists are third-party
// content and are never committed to this public repo; they are kept in a private
// repo. edhcover reads only those two files, so a
// measurement needs no network, and when either is absent it skips cleanly
// (writing nothing and removing any stale edh.md and edh.json).
//
// A card is playable by exactly the predicate `make report` and `make
// coverage` use: Registry.Unsupported against effects.Supported() with
// etbCopyShapeGate. A deck is fully supported when every card resolves to a
// corpus card and every one is playable; an unresolved name blocks its deck.
//
// dirac's column reads dirac/support as committed: a card is dirac-supported
// when an evidence list (dirac/support/evidence/*.txt) names it and
// dirac/support/refused does not. That is the support.md predicate minus the
// "still lowered" check, which kgen's TestSupportDocIsCurrent holds for every
// evidenced card, so it agrees with support.md's ALL row. It needs no kernel
// tables.
//
// The output is deterministic for a fixed corpus, tree and set of deck files.

// edhRow is one manifest line.
type edhRow struct {
	Source, ID, Kind, Name   string
	Bracket                  int
	OverallRank, BracketRank int
	path                     string
}

func readEDHManifest(r io.Reader) ([]edhRow, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	var cols map[string]int
	var out []edhRow
	for line := 1; sc.Scan(); line++ {
		f := strings.Split(sc.Text(), "\t")
		if cols == nil {
			cols = map[string]int{}
			for i, c := range f {
				cols[c] = i
			}
			for _, need := range []string{"source", "id", "kind", "name", "bracket", "overall_rank", "bracket_rank"} {
				if _, ok := cols[need]; !ok {
					return nil, fmt.Errorf("manifest: no %q column", need)
				}
			}
			continue
		}
		get := func(c string) string {
			if i := cols[c]; i < len(f) {
				return f[i]
			}
			return ""
		}
		num := func(c string) (int, error) {
			s := get(c)
			if s == "" {
				return 0, nil
			}
			n, err := strconv.Atoi(s)
			if err != nil {
				return 0, fmt.Errorf("manifest line %d: %s %q: %w", line, c, s, err)
			}
			return n, nil
		}
		row := edhRow{Source: get("source"), ID: get("id"), Kind: get("kind"), Name: get("name")}
		var err error
		if row.Bracket, err = num("bracket"); err != nil {
			return nil, err
		}
		if row.OverallRank, err = num("overall_rank"); err != nil {
			return nil, err
		}
		if row.BracketRank, err = num("bracket_rank"); err != nil {
			return nil, err
		}
		switch row.Source {
		case "mtgjson":
			row.path = filepath.Join("precon", row.ID+".json")
		case "archidekt":
			row.path = filepath.Join("popular", "archidekt-"+row.ID+".json")
		default:
			return nil, fmt.Errorf("manifest line %d: unknown source %q", line, row.Source)
		}
		out = append(out, row)
	}
	return out, sc.Err()
}

// edhList is one measured list: a name and the predicate choosing its decks.
type edhList struct {
	Name string
	in   func(edhRow) bool
}

func edhLists() []edhList {
	l := []edhList{
		{"precon (all Commander Deck products)", func(r edhRow) bool { return r.Kind == "precon" }},
		{"precon + Arsenal/Collection pools", func(r edhRow) bool { return r.Source == "mtgjson" }},
	}
	for _, n := range []int{100, 500, 1000} {
		n := n
		l = append(l, edhList{fmt.Sprintf("Archidekt top %d", n), func(r edhRow) bool {
			return r.Source == "archidekt" && r.OverallRank > 0 && r.OverallRank <= n
		}})
	}
	for b := 1; b <= 5; b++ {
		b := b
		l = append(l, edhList{fmt.Sprintf("Archidekt bracket %d top 100", b), func(r edhRow) bool {
			return r.Source == "archidekt" && r.Bracket == b && r.BracketRank > 0 && r.BracketRank <= 100
		}})
	}
	return l
}

// resolveEDHName finds name in the corpus. Lookup already folds case,
// punctuation, diacritics and a "Front // Back" name to its front; on a miss
// this also tries an Alchemy "A-" rebalance's paper name and each " // " face
// on its own (a deck site that lists the back face first).
func resolveEDHName(r *cards.Registry, name string) (*cards.Card, bool) {
	if c, ok := r.Lookup(name); ok {
		return c, true
	}
	if s, ok := strings.CutPrefix(name, "A-"); ok {
		if c, ok := r.Lookup(s); ok {
			return c, true
		}
	}
	for _, face := range strings.Split(name, "//") {
		if c, ok := r.Lookup(strings.TrimSpace(face)); ok {
			return c, true
		}
	}
	return nil, false
}

// edhPaper reports whether a blocker ranks with the paper, tournament-legal
// cards: not an Alchemy "A-" card and not a Scheme, Plane, Phenomenon,
// Attraction, Contraption, Conspiracy or Dungeon.
func edhPaper(name string, c *cards.Card) bool {
	if strings.HasPrefix(name, "A-") {
		return false
	}
	if c == nil || len(c.Faces) == 0 {
		return true
	}
	for _, t := range c.Faces[0].Types {
		switch t {
		case "Scheme", "Plane", "Phenomenon", "Attraction", "Contraption", "Conspiracy", "Dungeon":
			return false
		}
	}
	return true
}

type edhListResult struct {
	Name           string  `json:"name"`
	Decks          int     `json:"decks"`
	DecksSupported int     `json:"decks_supported"`
	DeckPct        float64 `json:"decks_supported_pct"`
	Cards          int     `json:"cards"`
	CardsSupported int     `json:"cards_supported"`
	CardPct        float64 `json:"cards_supported_pct"`
	Unresolved     int     `json:"unresolved_names"`
	DiracDecks     int     `json:"dirac_decks_supported,omitempty"`
	DiracCards     int     `json:"dirac_cards_supported,omitempty"`
}

type edhBlocker struct {
	Card       string   `json:"card"`
	Decks      int      `json:"decks"`
	SoleDecks  int      `json:"sole_blocker_decks"`
	Paper      bool     `json:"paper"`
	Unresolved bool     `json:"unresolved,omitempty"`
	Missing    []string `json:"missing,omitempty"`
}

type edhReport struct {
	Corpus     string          `json:"corpus,omitempty"`
	Lists      []edhListResult `json:"lists"`
	Blockers   []edhBlocker    `json:"blockers"`
	Unresolved []string        `json:"unresolved"`
	Skipped    []string        `json:"skipped,omitempty"`
	Dirac      bool            `json:"dirac"`
}

// edhDeck is one deck reduced to its distinct card keys and which block it.
type edhDeck struct {
	row      edhRow
	keys     []string // distinct corpus names (or "?name" when unresolved)
	blockers []string
	diracOK  bool
}

// readDiracSupported is the dirac-supported card set (normalised names) under
// supportDir, nil when the directory is absent.
func readDiracSupported(supportDir string) (map[string]bool, error) {
	ev, err := filepath.Glob(filepath.Join(supportDir, "evidence", "*.txt"))
	if err != nil || len(ev) == 0 {
		return nil, err
	}
	out := map[string]bool{}
	for _, p := range ev {
		raw, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		for _, l := range strings.Split(string(raw), "\n") {
			// An evidence row's trailing tab-separated columns (a pod row's
			// seats field) are not part of the card name.
			if i := strings.IndexByte(l, '\t'); i >= 0 {
				l = l[:i]
			}
			if l = strings.TrimSpace(l); l != "" && !strings.HasPrefix(l, "#") {
				out[cards.NormalizeName(l)] = true
			}
		}
	}
	ref, err := filepath.Glob(filepath.Join(supportDir, "refused", "*.json"))
	if err != nil {
		return nil, err
	}
	for _, p := range ref {
		raw, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		var e struct {
			Card string `json:"card"`
		}
		if err := json.Unmarshal(raw, &e); err != nil {
			return nil, fmt.Errorf("%s: %w", p, err)
		}
		delete(out, cards.NormalizeName(e.Card))
	}
	return out, nil
}

// edhDecklist is one line of edh-decks.jsonl.
type edhDecklist struct {
	Source     string   `json:"source"`
	ID         string   `json:"id"`
	Kind       string   `json:"kind"`
	Commanders []string `json:"commanders"`
	Cards      []string `json:"cards"`
}

// readEDHDecklists indexes the stored decks by the path their manifest row
// names, as repo-deck JSON (commanders first, names counted).
func readEDHDecklists(r io.Reader) (map[string][]byte, error) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1<<20), 1<<20)
	out := map[string][]byte{}
	for line := 1; sc.Scan(); line++ {
		if strings.TrimSpace(sc.Text()) == "" {
			continue
		}
		var l edhDecklist
		if err := json.Unmarshal(sc.Bytes(), &l); err != nil {
			return nil, fmt.Errorf("decklists line %d: %w", line, err)
		}
		var p string
		switch l.Source {
		case "mtgjson":
			p = filepath.Join("precon", l.ID+".json")
		case "archidekt":
			p = filepath.Join("popular", "archidekt-"+l.ID+".json")
		default:
			return nil, fmt.Errorf("decklists line %d: unknown source %q", line, l.Source)
		}
		var entries []deck.Entry
		idx := map[string]int{}
		for _, n := range append(append([]string(nil), l.Commanders...), l.Cards...) {
			if i, ok := idx[n]; ok {
				entries[i].Count++
				continue
			}
			idx[n] = len(entries)
			entries = append(entries, deck.Entry{Name: n, Count: 1})
		}
		raw, err := json.Marshal(deck.File{Name: l.ID, Format: "commander", Cards: entries})
		if err != nil {
			return nil, err
		}
		out[p] = raw
	}
	return out, sc.Err()
}

// measureEDH measures rows' decks, as read returns them (os.ErrNotExist for a
// deck it lacks), against
// effects.Supported(); r.ShapeGate is the caller's. dirac is
// readDiracSupported's set, nil to leave dirac's columns out.
func measureEDH(r *cards.Registry, read func(path string) ([]byte, error), rows []edhRow, dirac map[string]bool) (edhReport, error) {
	sup := effects.Supported()
	rep := edhReport{Dirac: dirac != nil}
	playable := map[string]bool{}
	missing := map[string][]string{}
	card := map[string]*cards.Card{}
	unresolved := map[string]bool{}
	var decks []edhDeck
	for _, row := range rows {
		raw, err := read(row.path)
		if os.IsNotExist(err) {
			rep.Skipped = append(rep.Skipped, row.path)
			continue
		}
		if err != nil {
			return rep, err
		}
		f, err := deck.Parse(raw)
		if err != nil {
			return rep, fmt.Errorf("%s: %w", row.path, err)
		}
		d := edhDeck{row: row, diracOK: dirac != nil}
		seen := map[string]bool{}
		for _, e := range f.Cards {
			c, ok := resolveEDHName(r, e.Name)
			key := "?" + e.Name
			if ok {
				key = c.Faces[0].Name
			}
			if seen[key] {
				continue
			}
			seen[key] = true
			d.keys = append(d.keys, key)
			if dirac != nil && (!ok || !dirac[cards.NormalizeName(key)]) {
				d.diracOK = false
			}
			if !ok {
				unresolved[e.Name] = true
				d.blockers = append(d.blockers, key)
				continue
			}
			if _, done := playable[key]; !done {
				m := r.Unsupported(c, sup)
				playable[key] = len(m) == 0
				missing[key] = m
				card[key] = c
			}
			if !playable[key] {
				d.blockers = append(d.blockers, key)
			}
		}
		decks = append(decks, d)
	}
	for _, l := range edhLists() {
		res := edhListResult{Name: l.Name}
		cs := map[string]bool{}
		for _, d := range decks {
			if !l.in(d.row) {
				continue
			}
			res.Decks++
			if len(d.blockers) == 0 {
				res.DecksSupported++
			}
			if d.diracOK {
				res.DiracDecks++
			}
			for _, k := range d.keys {
				cs[k] = true
			}
		}
		if res.Decks == 0 {
			continue // a list none of whose deck files are present (a precon-only CI fetch)
		}
		for k := range cs {
			switch {
			case strings.HasPrefix(k, "?"):
				res.Unresolved++
			case playable[k]:
				res.Cards++
				res.CardsSupported++
			default:
				res.Cards++
			}
			if dirac[cards.NormalizeName(k)] && !strings.HasPrefix(k, "?") {
				res.DiracCards++
			}
		}
		res.Cards += res.Unresolved
		res.DeckPct = pct(res.DecksSupported, res.Decks)
		res.CardPct = pct(res.CardsSupported, res.Cards)
		rep.Lists = append(rep.Lists, res)
	}
	// Blockers are counted over the distinct decks of every list.
	by := map[string]*edhBlocker{}
	for _, d := range decks {
		for _, k := range d.blockers {
			b := by[k]
			if b == nil {
				name := strings.TrimPrefix(k, "?")
				b = &edhBlocker{Card: name, Unresolved: strings.HasPrefix(k, "?"),
					Paper: edhPaper(name, card[k]), Missing: missing[k]}
				by[k] = b
			}
			b.Decks++
			if len(d.blockers) == 1 {
				b.SoleDecks++
			}
		}
	}
	for _, b := range by {
		rep.Blockers = append(rep.Blockers, *b)
	}
	sort.Slice(rep.Blockers, func(i, j int) bool {
		a, b := rep.Blockers[i], rep.Blockers[j]
		if a.Paper != b.Paper {
			return a.Paper
		}
		if a.Decks != b.Decks {
			return a.Decks > b.Decks
		}
		return a.Card < b.Card
	})
	for n := range unresolved {
		rep.Unresolved = append(rep.Unresolved, n)
	}
	sort.Strings(rep.Unresolved)
	return rep, nil
}

func pct(n, d int) float64 {
	if d == 0 {
		return 0
	}
	return float64(int(1000*float64(n)/float64(d)+0.5)) / 10
}

func renderEDH(rep edhReport, top int) string {
	var b strings.Builder
	b.WriteString("# Commander deck coverage\n\n")
	if rep.Corpus != "" {
		fmt.Fprintf(&b, "Forge corpus `%s`. ", rep.Corpus)
	}
	b.WriteString("A card is playable by `make report`'s predicate; a deck is supported when every card resolves and is playable.\n\n")
	b.WriteString("| list | decks | decks supported | % | unique cards | cards playable | % | unresolved |")
	if rep.Dirac {
		b.WriteString(" dirac decks | dirac cards | dirac cards % |")
	}
	b.WriteString("\n|---|---:|---:|---:|---:|---:|---:|---:|")
	if rep.Dirac {
		b.WriteString("---:|---:|---:|")
	}
	b.WriteString("\n")
	for _, l := range rep.Lists {
		fmt.Fprintf(&b, "| %s | %d | %d | %.1f | %d | %d | %.1f | %d |",
			l.Name, l.Decks, l.DecksSupported, l.DeckPct, l.Cards, l.CardsSupported, l.CardPct, l.Unresolved)
		if rep.Dirac {
			fmt.Fprintf(&b, " %d | %d | %.1f |", l.DiracDecks, l.DiracCards, pct(l.DiracCards, l.Cards))
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "\n## Top %d blocking cards\n\nRanked by decks blocked across every list, paper cards first; *sole* counts the decks it is the only blocker of.\n\n", top)
	b.WriteString("| # | card | decks | sole | paper | missing |\n|---:|---|---:|---:|---|---|\n")
	for i, bl := range rep.Blockers {
		if i == top {
			break
		}
		m := strings.Join(bl.Missing, ", ")
		if bl.Unresolved {
			m = "(not in corpus)"
		}
		paper := "yes"
		if !bl.Paper {
			paper = "no"
		}
		fmt.Fprintf(&b, "| %d | %s | %d | %d | %s | %s |\n", i+1, bl.Card, bl.Decks, bl.SoleDecks, paper, m)
	}
	fmt.Fprintf(&b, "\n## Unresolved names (%d)\n\n", len(rep.Unresolved))
	for _, n := range rep.Unresolved {
		fmt.Fprintf(&b, "- %s\n", n)
	}
	if len(rep.Skipped) > 0 {
		fmt.Fprintf(&b, "\n%d manifest decks have no deck file (re-run scripts/edh_decks.py normalise).\n", len(rep.Skipped))
	}
	return b.String()
}

// edhInputsPresent reports whether both input files exist.
func edhInputsPresent(paths ...string) bool {
	for _, p := range paths {
		if st, err := os.Stat(p); err != nil || st.IsDir() {
			return false
		}
	}
	return true
}

// edhCoverCmd measures the decks manifest names, read from the decklists file,
// and writes edh.md (and edh.json) into outDir.
func edhCoverCmd(dir, decklists, manifest, diracSupport, outDir string, top int) error {
	if !edhInputsPresent(decklists, manifest) {
		fmt.Fprintf(os.Stderr, "forgec: edhcover: no decklists (%s, %s); skipping Commander deck coverage\n", decklists, manifest)
		for _, f := range []string{"edh.md", "edh.json"} {
			if err := os.Remove(filepath.Join(outDir, f)); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
		return nil
	}
	mf, err := os.Open(manifest)
	if err != nil {
		return err
	}
	rows, err := readEDHManifest(mf)
	mf.Close()
	if err != nil {
		return err
	}
	r, err := loadReportRegistry(dir)
	if err != nil {
		return err
	}
	r.ShapeGate = etbCopyShapeGate
	dirac, err := readDiracSupported(diracSupport)
	if err != nil {
		return err
	}
	df, err := os.Open(decklists)
	if err != nil {
		return err
	}
	stored, err := readEDHDecklists(df)
	df.Close()
	if err != nil {
		return err
	}
	rep, err := measureEDH(r, func(p string) ([]byte, error) {
		if raw, ok := stored[p]; ok {
			return raw, nil
		}
		return nil, os.ErrNotExist
	}, rows, dirac)
	if err != nil {
		return err
	}
	if l, err := cards.ReadLock(dir); err == nil {
		rep.Corpus = l.Commit
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	js, err := json.MarshalIndent(rep, "", " ")
	if err != nil {
		return err
	}
	md := renderEDH(rep, top)
	if err := os.WriteFile(filepath.Join(outDir, "edh.json"), append(js, '\n'), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, "edh.md"), []byte(md), 0o644); err != nil {
		return err
	}
	fmt.Print(md)
	return nil
}
