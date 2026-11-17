package main

import (
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"sort"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/server/matchrecord"
	"github.com/adams-shaun/gorge/testkit/cardcov"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// covColumns is the -coverage file's column line.
const covColumns = "seed\tcard\tkey\tused\toffered\tclean"

// covRun is one run's -coverage writer: the append-only file and the first
// write error (playOne reports only ok/not ok, so runSim reads err after each
// game).
type covRun struct {
	f   *os.File
	reg *cards.Registry
	err error
}

// openCoverage opens path for append and, when it is empty, writes the
// header and the column line.
func openCoverage(path, ctx, format string, seats int, dir string, reg *cards.Registry) (*covRun, error) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("opening -coverage file %s: %w", path, err)
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("-coverage file %s: %w", path, err)
	}
	if st.Size() == 0 {
		_, err := fmt.Fprintf(f, "#cov-1\tcommit=%s\tctx=%s\tformat=%s\tseats=%d\tcorpus=%s\n%s\n",
			buildCommit(), ctx, format, seats, matchrecord.ForgeRef(dir), covColumns)
		if err != nil {
			f.Close()
			return nil, fmt.Errorf("writing -coverage header to %s: %w", path, err)
		}
	}
	return &covRun{f: f, reg: reg}, nil
}

// buildCommit is the binary's vcs.revision (first 12 chars), else "unknown".
func buildCommit() string {
	if bi, ok := debug.ReadBuildInfo(); ok {
		for _, s := range bi.Settings {
			if s.Key == "vcs.revision" && s.Value != "" {
				return s.Value[:min(12, len(s.Value))]
			}
		}
	}
	return "unknown"
}

// castKey is the key a card's cast (or land play) is credited under.
func castKey(c *cards.Card) string {
	if c != nil && len(c.Faces) > 0 && c.Faces[0].IsLand() {
		return "land"
	}
	return "cast"
}

func cardName(c *cards.Card) string { return c.Faces[0].Name }

func b01(b bool) int {
	if b {
		return 1
	}
	return 0
}

// covKey is one (card, key) cell of a game.
type covKey struct{ used, offered bool }

// covRows is a finished game's rows, one per (card, key) in sorted order:
// "deck" for every distinct dealt card; per dealt card its cast/land key and
// every Keys / ModeKeys key (so an unused key is a 0 row, the denominator);
// and a cast row for any card cast that was not dealt. cast and land have no
// separate offer probe, so offered = used there; a deck row is offered.
// offered is the priority-offer probe only, so a mana or triggered key can be
// used without ever being offered.
func covRows(g cardcov.Game, decks [][]*cards.Card, lookup func(string) *cards.Card) map[string]map[string]covKey {
	rows := map[string]map[string]covKey{}
	set := func(card, key string, k covKey) {
		if rows[card] == nil {
			rows[card] = map[string]covKey{}
		}
		rows[card][key] = k
	}
	dealt := map[string]*cards.Card{}
	for _, d := range decks {
		for _, c := range d {
			dealt[cardName(c)] = c
		}
	}
	for name, c := range dealt {
		set(name, "deck", covKey{true, true})
		set(name, castKey(c), covKey{g.Cast[name], g.Cast[name]})
		for _, k := range cardcov.Keys(c) {
			set(name, k, covKey{g.Used[name][k], g.Offered[name][k]})
		}
		for _, k := range cardcov.ModeKeys(c) {
			set(name, k, covKey{g.Used[name][k], g.Offered[name][k]})
		}
	}
	for name := range g.Cast {
		if _, ok := dealt[name]; !ok {
			set(name, castKey(lookup(name)), covKey{true, true})
		}
	}
	// Credited keys the inventory does not list (none today) are kept.
	for name, ks := range g.Used {
		for k := range ks {
			if _, ok := rows[name][k]; !ok {
				set(name, k, covKey{true, true})
			}
		}
	}
	return rows
}

// writeGame appends one game's rows. clean is playOne's ok. The first error
// is kept in c.err.
func (c *covRun) writeGame(seed uint64, e *rules.Engine, decks [][]*cards.Card, probe *cardcov.Probe, clean bool) {
	g := cardcov.Played(e, decks, probe)
	rows := covRows(g, decks, func(n string) *cards.Card {
		if c.reg == nil {
			return nil
		}
		card, _ := c.reg.Lookup(n)
		return card
	})
	names := make([]string, 0, len(rows))
	for n := range rows {
		names = append(names, n)
	}
	sort.Strings(names)
	var sb strings.Builder
	for _, n := range names {
		keys := make([]string, 0, len(rows[n]))
		for k := range rows[n] {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			v := rows[n][k]
			fmt.Fprintf(&sb, "%d\t%s\t%s\t%d\t%d\t%d\n", seed, n, k, b01(v.used), b01(v.offered), b01(clean))
		}
	}
	if _, err := io.WriteString(c.f, sb.String()); err != nil && c.err == nil {
		c.err = err
	}
}

// listKeys prints card<TAB>key for every card in the corpus at dir, in name
// order: cast (or land), then each Keys and ModeKeys key.
func listKeys(dir string, out io.Writer) error {
	reg, err := testutil.OpenCorpusRegistry(dir)
	if err != nil {
		return fmt.Errorf("opening corpus at %s: %w (run `make fetch-cards compile-cards` first)", dir, err)
	}
	all := append([]*cards.Card(nil), reg.AllCards()...)
	sort.SliceStable(all, func(i, j int) bool { return cardName(all[i]) < cardName(all[j]) })
	var sb strings.Builder
	for _, c := range all {
		n := cardName(c)
		fmt.Fprintf(&sb, "%s\t%s\n", n, castKey(c))
		for _, k := range cardcov.Keys(c) {
			fmt.Fprintf(&sb, "%s\t%s\n", n, k)
		}
		for _, k := range cardcov.ModeKeys(c) {
			fmt.Fprintf(&sb, "%s\t%s\n", n, k)
		}
	}
	_, err = io.WriteString(out, sb.String())
	return err
}
