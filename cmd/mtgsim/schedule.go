package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/deck"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/server/matchrecord"
	sbenv "github.com/adams-shaun/gorge/server/spellbench/env"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// parseFormat maps -format to the rules format and whether it is commander.
func parseFormat(name string, seats int) (format rules.Format, commander bool, err error) {
	switch name {
	case "constructed":
		return format, false, nil
	case "commander":
		if seats < 2 {
			return format, true, fmt.Errorf("-format commander needs at least 2 seats")
		}
		return rules.FormatCommander, true, nil
	}
	return format, false, fmt.Errorf("-format %q: want constructed or commander", name)
}

// scheduleLine is one parsed -schedule line.
type scheduleLine struct {
	seedText string
	seed     uint64
	format   string
	seats    int
	decks    string
	policies []string
	mull     int
}

// parseScheduleLine reads "seed<TAB>format<TAB>seats<TAB>decks<TAB>policies<TAB>mulligans".
// The seed text is returned even for a line that does not parse, so the
// SCHEDULE-ERROR line can name it.
func parseScheduleLine(text string) (scheduleLine, error) {
	f := strings.Split(text, "\t")
	l := scheduleLine{seedText: strings.TrimSpace(f[0])}
	if len(f) != 6 {
		return l, fmt.Errorf("want 6 tab-separated fields (seed format seats decks policies mulligans), got %d", len(f))
	}
	var err error
	if l.seed, err = strconv.ParseUint(l.seedText, 10, 64); err != nil {
		return l, fmt.Errorf("seed %q: %w", f[0], err)
	}
	l.format = strings.TrimSpace(f[1])
	if l.seats, err = strconv.Atoi(strings.TrimSpace(f[2])); err != nil {
		return l, fmt.Errorf("seats %q: %w", f[2], err)
	}
	l.decks = strings.TrimSpace(f[3])
	if p := strings.TrimSpace(f[4]); p != "" {
		l.policies = strings.Split(p, ",")
	}
	if l.mull, err = strconv.Atoi(strings.TrimSpace(f[5])); err != nil {
		return l, fmt.Errorf("mulligans %q: %w", f[5], err)
	}
	return l, nil
}

// options is the single-game run this line maps to: `mtgsim -format F -seats
// N -decks D -policies P -mulligans M -games 1 -seed S` plus the process's
// own -dir, -deckdir and -verify.
func (l scheduleLine) options(base simOptions) simOptions {
	return simOptions{
		Decks: l.decks, Seats: l.seats, Seed: l.seed, Games: 1, Policies: l.policies,
		Format: l.format, Mulligans: l.mull,
		Verify: base.Verify, Dir: base.Dir, DeckDir: base.DeckDir,
	}
}

// deckCache loads each deck once per process.
type deckCache struct {
	o         simOptions
	reg       *cards.Registry
	files     map[string]deck.File
	cards     map[string][]*cards.Card
	validated map[string]error
}

func (c *deckCache) file(name string) (deck.File, error) {
	if f, ok := c.files[name]; ok {
		return f, nil
	}
	f, err := c.o.deckFile(name)
	if err != nil {
		return f, err
	}
	c.files[name] = f
	return f, nil
}

func (c *deckCache) deck(name string) ([]*cards.Card, error) {
	if d, ok := c.cards[name]; ok {
		return d, nil
	}
	d, err := c.o.deck(c.reg, name)
	if err != nil {
		return nil, err
	}
	c.cards[name] = d
	return d, nil
}

// commanderDeck checks name against the Commander deck rules (once) and
// returns its commander indices.
func (c *deckCache) commanderDeck(name string) ([]int, error) {
	f, err := c.file(name)
	if err != nil {
		return nil, err
	}
	if len(f.CommanderNames()) == 0 {
		return nil, fmt.Errorf("deck %q names no commander (-format commander)", name)
	}
	verr, ok := c.validated[name]
	if !ok {
		if verr = f.ValidateCommander(c.reg); verr != nil {
			verr = fmt.Errorf("deck %q: %w", name, verr)
		}
		c.validated[name] = verr
	}
	return f.CommanderIndices(), verr
}

// scheduledGame is one line's resolved game.
type scheduledGame struct {
	names    []string
	decks    [][]*cards.Card
	cmds     [][]int
	policies []string
	format   rules.Format
}

// load resolves one line the way runSim resolves a single-game run.
func (c *deckCache) load(o simOptions) (g scheduledGame, err error) {
	var commander bool
	if g.format, commander, err = parseFormat(o.Format, o.Seats); err != nil {
		return g, err
	}
	if o.Seats < 1 {
		return g, fmt.Errorf("-seats must be at least 1, got %d", o.Seats)
	}
	if g.policies, err = seatPolicies(o.Policies, "", "", o.Seats); err != nil {
		return g, err
	}
	if g.names, err = deckNames(o.Decks, o.Seats, commander); err != nil {
		return g, err
	}
	if commander {
		g.cmds = make([][]int, len(g.names))
	}
	g.decks = make([][]*cards.Card, len(g.names))
	for i, name := range g.names {
		if commander {
			if g.cmds[i], err = c.commanderDeck(name); err != nil {
				return g, err
			}
		}
		if g.decks[i], err = c.deck(name); err != nil {
			return g, err
		}
	}
	return g, nil
}

// scheduleConflict names the per-game options a schedule replaces.
func scheduleConflict(o simOptions) error {
	var bad []string
	if o.Decks != "" {
		bad = append(bad, "-decks")
	}
	if o.Seed != 0 {
		bad = append(bad, "-seed")
	}
	if o.Games > 1 {
		bad = append(bad, "-games")
	}
	if o.PolicyA != "" {
		bad = append(bad, "-a")
	}
	if o.PolicyB != "" {
		bad = append(bad, "-b")
	}
	if o.Policies != nil {
		bad = append(bad, "-policies")
	}
	if o.Shape != "" {
		bad = append(bad, "-shape")
	}
	if len(bad) > 0 {
		return usageError{"-schedule excludes " + strings.Join(bad, ", ")}
	}
	return nil
}

// runSchedule plays o.Schedule's games in this process: the corpus registry
// opens once and each deck loads once, and every line plays exactly as the
// single-game run its line maps to (scheduleLine.options). A bad line prints
// "seed S: SCHEDULE-ERROR err" and the run goes on; the run fails at the end
// if any line or game failed.
func runSchedule(o simOptions, out io.Writer) error {
	if err := scheduleConflict(o); err != nil {
		return err
	}
	if err := registerSBAgents(o.SBAgents); err != nil {
		return err
	}
	sf, err := os.Open(o.Schedule)
	if err != nil {
		return fmt.Errorf("opening -schedule file: %w", err)
	}
	defer sf.Close()
	reg, err := testutil.OpenCorpusRegistry(o.Dir)
	if err != nil {
		return fmt.Errorf("opening corpus at %s: %w (run `make fetch-cards compile-cards` first)", o.Dir, err)
	}
	sbenv.SetCorpusRef(matchrecord.ForgeRef(o.Dir))
	var cov *covRun
	if o.Coverage != "" {
		// One header for a run of mixed formats and seat counts.
		c, err := openCoverage(o.Coverage, o.CoverageCtx, "schedule", 0, o.Dir, reg)
		if err != nil {
			return err
		}
		defer c.f.Close()
		cov = c
	}
	rec, err := openRecorder(o.Record, o.Dir)
	if err != nil {
		return err
	}
	cache := &deckCache{o: o, reg: reg, files: map[string]deck.File{}, cards: map[string][]*cards.Card{}, validated: map[string]error{}}

	failed, total := 0, 0
	sc := bufio.NewScanner(sf)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for sc.Scan() {
		text := strings.TrimRight(sc.Text(), "\r")
		if strings.TrimSpace(text) == "" || strings.HasPrefix(strings.TrimSpace(text), "#") {
			continue
		}
		total++
		line, err := parseScheduleLine(text)
		var g scheduledGame
		lo := line.options(o)
		if err == nil {
			g, err = cache.load(lo)
		}
		if err != nil {
			fmt.Fprintf(out, "seed %s: SCHEDULE-ERROR %v\n", line.seedText, err)
			failed++
			continue
		}
		if !playOne(out, line.seed, g.names, g.decks, reg, o.Verify, g.policies, g.format, g.cmds, line.mull, nil, cov, rec) {
			failed++
		}
		if cov != nil && cov.err != nil {
			return fmt.Errorf("writing -coverage file %s: %w", o.Coverage, cov.err)
		}
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("reading -schedule file: %w", err)
	}
	if failed > 0 {
		return fmt.Errorf("%d of %d scheduled games failed to parse, terminate or verify", failed, total)
	}
	return nil
}
