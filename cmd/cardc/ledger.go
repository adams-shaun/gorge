package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/engine/cardc/diff"
	"github.com/adams-shaun/gorge/engine/cardc/gorgebe"
	"github.com/adams-shaun/gorge/engine/cardc/lockstep"
	"github.com/adams-shaun/gorge/engine/cardc/mtgish"
	"github.com/adams-shaun/gorge/engine/cardc/slice"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

const ledgerWorkers = 4

// ledgerCmd holds the divergence ledger against the live static diff and lockstep sweep.
// Without -write it prints the missing and stale rows and exits 1 on any; with -write it
// rewrites the ledger, keeping the triage of every row that survives.
func ledgerCmd(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("ledger", flag.ContinueOnError)
	fs.SetOutput(stderr)
	write := fs.Bool("write", false, "rewrite the ledger and its counts")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	fail := func(format string, a ...any) int {
		fmt.Fprintf(stderr, "cardc: "+format+"\n", a...)
		return 1
	}
	root, err := mtgish.ModuleRoot()
	if err != nil {
		return fail("%v", err)
	}
	td := filepath.Join(root, "engine", "cardc", "testdata")
	ledgerDir, unPath, countsPath := filepath.Join(td, "ledger"), filepath.Join(td, "ledger-unattributed.tsv"), filepath.Join(td, "ledger-counts.tsv")
	forge, err := cards.OpenCorpus(filepath.Join(root, ".cards"))
	if err != nil {
		return fail("no Forge corpus (run make fetch-cards): %v", err)
	}
	reg, err := slice.Build(root, forge, "fdn", "legacy")
	if errors.Is(err, slice.ErrNoData) {
		return fail("no mtgish data (run make mtgish-fetch)")
	}
	if err != nil {
		return fail("%v", err)
	}
	var static []lockstep.Row
	for _, e := range reg.Entries() {
		if e.Origin != gorgebe.OriginCardc {
			continue
		}
		name := e.Forge.Faces[0].Name
		for _, r := range diff.Card(name, e.Card, e.Forge) {
			static = append(static, lockstep.Row{Card: name, Source: lockstep.SourceStatic, Key: r.Key(), Sig: r.Sig(),
				Class: "cardc-gap", Note: "untriaged"})
		}
	}
	newBot := func(seed uint64) rules.Answerer {
		b := botpolicy.NewGameBot(seed)
		return func(e *rules.Engine, d *decision.Decision) decision.Intent { return b.Answer(e.G, e, d) }
	}
	dirs := []lockstep.Direction{lockstep.Forward, lockstep.Reverse}
	games, err := lockstep.FDN(root, forge, 0, 1500)
	if err != nil {
		return fail("%v", err)
	}
	for _, seats := range []int{2, 4, 6, 8} {
		names, decks, err := testutil.AcceptanceDecks(forge, seats)
		if err != nil {
			return fail("%v", err)
		}
		games = append(games, lockstep.Legacy(forge, names, decks))
	}
	found := lockstep.Sweep(games, dirs, reg.Deck, newBot, ledgerWorkers)

	old, err := lockstep.ReadLedger(ledgerDir)
	if err != nil {
		return fail("%v", err)
	}
	oldUn, err := lockstep.ReadUnattributed(unPath)
	if err != nil {
		return fail("%v", err)
	}
	if !*write {
		missing, stale := lockstep.Hold(static, staticOf(old))
		mL, sL, mU, sU := lockstep.HoldFound(found, old, oldUn, true)
		missing, stale = append(missing, mL...), append(stale, sL...)
		for _, r := range missing {
			fmt.Fprintf(stdout, "missing\t%s\t%s\t%s\t%s\n", r.Card, r.Source, r.Key, r.Sig)
		}
		for _, r := range stale {
			fmt.Fprintf(stdout, "stale\t%s\t%s\t%s\t%s\n", r.Card, r.Source, r.Key, r.Sig)
		}
		for _, u := range mU {
			fmt.Fprintf(stdout, "missing\t-\tunattributed\t%s\t-\n", u.Witness)
		}
		for _, u := range sU {
			fmt.Fprintf(stdout, "stale\t-\tunattributed\t%s\t-\n", u.Witness)
		}
		n := len(missing) + len(stale) + len(mU) + len(sU)
		fmt.Fprintf(stdout, "total\tmissing=%d\tstale=%d\n", len(missing)+len(mU), len(stale)+len(sU))
		if n > 0 {
			return 1
		}
		return 0
	}

	type id struct{ card, source, key string }
	kept := map[id]lockstep.Row{}
	for _, r := range old {
		kept[id{r.Card, r.Source, r.Key}] = r
	}
	var rows []lockstep.Row
	add := func(r lockstep.Row) {
		if o, ok := kept[id{r.Card, r.Source, r.Key}]; ok {
			r.Class, r.Issue, r.Note = o.Class, o.Issue, o.Note
		}
		rows = append(rows, r)
	}
	for _, r := range static {
		add(r)
	}
	per := map[string]int{}
	var un []lockstep.Unattributed
	keptUn := map[string]lockstep.Unattributed{}
	for _, u := range oldUn {
		keptUn[u.Witness] = u
	}
	for _, f := range found {
		if f.Card == "" {
			u := lockstep.Unattributed{Witness: f.Witness, Class: "cardc-gap", Note: "untriaged"}
			if o, ok := keptUn[f.Witness]; ok {
				u = o
			}
			un = append(un, u)
			continue
		}
		if per[f.Card] < 2 {
			per[f.Card]++
			add(lockstep.Row{Card: f.Card, Source: lockstep.SourceLockstep, Key: f.Witness, Sig: "-", Class: "cardc-gap", Note: "untriaged"})
		}
	}
	if err := lockstep.WriteLedger(ledgerDir, rows); err != nil {
		return fail("%v", err)
	}
	if err := lockstep.WriteUnattributed(unPath, un); err != nil {
		return fail("%v", err)
	}
	counts := lockstep.Counts(rows, un)
	if err := os.WriteFile(countsPath, []byte(lockstep.FormatCounts(counts)), 0o644); err != nil {
		return fail("%v", err)
	}
	blamed := make([]string, 0, len(per))
	for c := range per {
		blamed = append(blamed, c)
	}
	sort.Strings(blamed)
	fmt.Fprintf(stdout, "total\trows=%d\tcards-blamed=%d\tunattributed=%d\n", len(rows), len(blamed), len(un))
	return 0
}

func staticOf(rows []lockstep.Row) []lockstep.Row {
	var out []lockstep.Row
	for _, r := range rows {
		if r.Source == lockstep.SourceStatic {
			out = append(out, r)
		}
	}
	return out
}
