package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/adams-shaun/gorge/bots"
	_ "github.com/adams-shaun/gorge/bots/all" // link every built-in hosted policy
	"github.com/adams-shaun/gorge/draft/event"
	"github.com/adams-shaun/gorge/draft/limited/pickbot"
	"github.com/adams-shaun/gorge/draft/limited/setdata"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// runner carries everything one draftsim run reads more than once: the card
// corpus, the drafted set's spec, the pod's personas and the output sink.
type runner struct {
	reg     *cards.Registry
	spec    setdata.Spec
	drafter pickbot.Drafter
	builder pickbot.Builder
	players []string
	verbose bool
	// shape is the run's resolved pod shape (options.shape). The tests
	// build runners as literals without one; podShape fills the
	// defaults in (4 seats, draft, round robin, 3 packs).
	shape podShape
	// noGames stops an event after the build phase (the draft-only mode
	// the pool-shape tests use): the series stay pending, no EventEnded is
	// applied and runOneEvent returns the built event and decks. run()
	// never sets it.
	noGames bool
	out     io.Writer
}

// result is what one run produced, in the shape the tests assert on.
type result struct {
	// events is one record per event, in run order.
	events []eventResult
}

// eventResult is one event's outcome: its marshaled log, its final table
// and the per-game records in play order.
type eventResult struct {
	// log is the event's complete log, marshaled as JSON lines.
	log []byte
	// standings is the final table (event COMPLETE only).
	standings []event.Standing
	// games is one record per played game, in play order.
	games []gameRecord
	// decks is each seat's built deck, resolved to engine card pointers —
	// the gauntlet replays them against the human decks after the event.
	decks [][]*cards.Card
}

// run is main's testable body. It validates the flags, opens the corpus,
// plays every event and — with -verify — replays each event from its own
// log, returning a non-nil error (and therefore a non-zero exit code) on
// the first refusal, failed game or replay mismatch.
func run(o options, out io.Writer) (*result, error) {
	players, err := o.playerList()
	if err != nil {
		return nil, err
	}
	// The Env refusal comes before anything else: a policy that builds its
	// search over every seat's REAL decklist must never seat in a draft,
	// where the whole point is that the decks are not known until the build
	// phase (by design, the rule host's TableConfig.AllowDecklistBots gate
	// enforces for tables).
	for i, p := range players {
		if _, err := bots.Normalize(p); err != nil {
			return nil, fmt.Errorf("seat %d: %w", i, err)
		}
		if bots.ReadsDecklists(p) {
			return nil, fmt.Errorf("seat %d policy %q reads every seat's real decklist (bots.Info.Env); by design, draftsim refuses Env policies", i, p)
		}
	}
	if err := personaNamesOK(o.drafter, o.builder); err != nil {
		return nil, err
	}
	// The pod shape is validated next, still before anything expensive:
	// a bad -seats/-mode/-structure/-rounds/-draftlog pairing is refused
	// without opening the corpus.
	sh, err := o.shape()
	if err != nil {
		return nil, err
	}
	if o.events < 1 {
		return nil, fmt.Errorf("-events must be at least 1, got %d", o.events)
	}
	// The gauntlet is validated before anything expensive runs: its
	// refusals (unknown catalog value, a non-FDN set, an Env policy on the
	// measurement seats) must not depend on the corpus.
	g, err := gauntletFromOptions(o)
	if err != nil {
		return nil, err
	}
	// An unset collation (options built in tests, not via the flag set)
	// means the default; a name the set does not carry is the error below.
	if o.collation == "" {
		o.collation = string(setdata.DefaultCollation)
	}
	spec, err := setdata.LoadCollation(o.set, setdata.Collation(o.collation))
	if err != nil {
		if colls := specCollations(o.set); len(colls) > 0 {
			return nil, fmt.Errorf("%w (collations: %s)", err, strings.Join(colls, ", "))
		}
		// An unknown set carries no collations to list; say so distinctly
		// instead of an empty list.
		return nil, fmt.Errorf("unknown set %q (known: %s)", o.set, strings.Join(setdata.Codes(), ", "))
	}

	reg, err := testutil.OpenCorpusRegistry(o.dir)
	if err != nil {
		return nil, fmt.Errorf("opening corpus at %s: %w (run `make fetch-cards compile-cards` first)", o.dir, err)
	}
	lookup := func(name string) *cards.Card { c, _ := reg.Lookup(name); return c }

	r := &runner{
		reg:     reg,
		spec:    spec,
		players: players,
		verbose: o.verbose,
		shape:   sh,
		out:     out,
	}
	r.drafter, r.builder, err = personaPair(o.drafter, o.builder, lookup)
	if err != nil {
		return nil, err
	}
	if err := applyRatingsOverride(o, r, out); err != nil {
		return nil, err
	}

	var csvOut *os.File
	csvHeader := true
	if o.draftlog != "" {
		if csvOut, err = os.OpenFile(o.draftlog, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644); err != nil {
			return nil, fmt.Errorf("opening -draftlog %s: %w", o.draftlog, err)
		}
		defer csvOut.Close()
		// The header line belongs to a fresh file only: appended runs add
		// rows, and a second header mid-file is not a 17Lands CSV.
		if st, serr := csvOut.Stat(); serr == nil && st.Size() > 0 {
			csvHeader = false
		}
	}

	var acc *gauntletAccum
	if g != nil {
		acc = newGauntletAccum(sh.seats)
	}
	res := &result{}
	for n := 0; n < o.events; n++ {
		seed := o.seed + uint64(n)
		// The header line is the stable contract the tests parse. The
		// default flags keep it exactly as it has always been; the
		// non-default shape values are appended as `, key value` pairs.
		fmt.Fprintf(out, "event %d: seed %d, set %s (%s), drafter %s, builder %s", n+1, seed, spec.Code, spec.Collation, o.drafter, o.builder)
		if sh.mode == event.ModeSealed {
			fmt.Fprint(out, ", mode sealed")
		}
		if sh.seats != 4 {
			fmt.Fprintf(out, ", seats %d", sh.seats)
		}
		if sh.structure != event.StructureRoundRobin {
			fmt.Fprintf(out, ", structure %s", sh.structure)
		}
		fmt.Fprintln(out)
		if o.verbose {
			for i, p := range players {
				fmt.Fprintf(out, "  seat %d: %s (persona %s/%s/%s)\n", i, p, o.drafter, o.builder, p)
			}
		}
		ev, er, err := runOneEvent(r, seed, o.verify)
		if err != nil {
			return nil, fmt.Errorf("event %d (seed %d): %w", n+1, seed, err)
		}
		logBytes, err := event.MarshalLog(ev.Log())
		if err != nil {
			return nil, fmt.Errorf("event %d: %w", n+1, err)
		}
		er.log = logBytes
		if o.verify {
			if err := verifyEvent(r, seed, logBytes); err != nil {
				return nil, fmt.Errorf("event %d (seed %d): %v", n+1, seed, err)
			}
			fmt.Fprintf(out, "event %d: replay OK (%d log entries, %d games)\n", n+1, len(ev.Log()), len(er.games))
		}
		if !ev.Done() {
			return nil, fmt.Errorf("event %d ended in phase %s, want COMPLETE", n+1, ev.Phase())
		}
		if csvOut != nil {
			write := event.DraftDataCSV
			if !csvHeader {
				write = event.DraftDataCSVRows
			}
			if err := write(ev, csvOut, sheetNames(spec)); err != nil {
				return nil, fmt.Errorf("event %d: draft CSV: %w", n+1, err)
			}
			csvHeader = false
		}
		printStandings(out, er)
		if g != nil {
			if err := gauntletEvent(r, g, ev, er.decks, seed, o.verify, acc, out); err != nil {
				return nil, fmt.Errorf("event %d (seed %d): gauntlet: %w", n+1, seed, err)
			}
		}
		res.events = append(res.events, *er)
	}
	if g != nil {
		// The human-vs-human sanity pairings do not depend on the draft,
		// so they run once per run, seeded from -seed itself.
		if err := gauntletSanity(r, g, o.seed, o.verify, acc, out); err != nil {
			return nil, fmt.Errorf("gauntlet sanity: %w", err)
		}
		acc.print(out, g.baseline)
	}
	return res, nil
}

// personaNamesOK validates the -drafter and -builder names against the
// pickbot registry before anything expensive runs.
func personaNamesOK(drafter, builder string) error {
	if _, _, ok := pickbot.Lookup(drafter); !ok {
		return fmt.Errorf("unknown -drafter %q (known: %s)", drafter, pickbot.Names())
	}
	if _, _, ok := pickbot.Lookup(builder); !ok {
		return fmt.Errorf("unknown -builder %q (known: %s)", builder, pickbot.Names())
	}
	return nil
}

// personaPair builds the drafter and builder the -drafter and -builder
// flags name: fresh instances from the pickbot registry with the corpus
// lookup injected, so the flags choose the persona instead of the tool
// hard-coding the types. personaNamesOK has already validated the names
// before the corpus opens; the errors here keep the function total for
// direct callers.
func personaPair(drafter, builder string, lookup pickbot.CardLookup) (pickbot.Drafter, pickbot.Builder, error) {
	d, _, ok := pickbot.New(drafter, lookup)
	if !ok {
		return nil, nil, fmt.Errorf("unknown -drafter %q (known: %s)", drafter, pickbot.Names())
	}
	_, b, ok := pickbot.New(builder, lookup)
	if !ok {
		return nil, nil, fmt.Errorf("unknown -builder %q (known: %s)", builder, pickbot.Names())
	}
	return d, b, nil
}

// specCollations names the set's collations for the -collation error
// message, so a typo lists what exists.
func specCollations(set string) []string {
	names := setdata.Collations(set)
	out := make([]string, len(names))
	for i, c := range names {
		out[i] = string(c)
	}
	return out
}

// sheetNames is the set's sheet-card universe, sorted: the pack_card_ and
// pool_ columns the -draftlog export writes. No map iteration reaches the
// output: the names are collected per sheet in slice order and the sort
// fixes the final order.
func sheetNames(spec setdata.Spec) []string {
	seen := map[string]bool{}
	var names []string
	for _, sheet := range spec.Config.Sheets {
		for _, sc := range sheet.Cards {
			if !seen[sc.Name] {
				seen[sc.Name] = true
				names = append(names, sc.Name)
			}
		}
	}
	sort.Strings(names)
	return names
}

// verifyEvent re-runs one event from its own log (the -verify contract):
// the log is replayed through the event state machine, the replayed log is
// byte-compared against the recorded one, and every game is replayed from
// the rebuilt event's saved decks and compared head for head. The first
// difference is the error.
func verifyEvent(r *runner, seed uint64, logBytes []byte) error {
	ev, err := event.ReplayLog(logBytes)
	if err != nil {
		return fmt.Errorf("event log replay: %w", err)
	}
	again, err := event.MarshalLog(ev.Log())
	if err != nil {
		return fmt.Errorf("event log re-marshal: %w", err)
	}
	if !bytes.Equal(again, logBytes) {
		return fmt.Errorf("event log replay diverges at %s", firstDiffLine(logBytes, again))
	}
	if ev.Seed() != seed {
		return fmt.Errorf("event log replay carries seed %d, want %d", ev.Seed(), seed)
	}
	for _, en := range ev.Log() {
		if en.Kind != event.KindGameEnded {
			continue
		}
		head, err := replayGameHead(r, ev, en.Round, en.A, en.B, en.Game)
		if err != nil {
			return fmt.Errorf("game round %d seats %d–%d: %w", en.Round, en.A, en.B, err)
		}
		if head != en.Head {
			return fmt.Errorf("game round %d seats %d–%d: replayed chain head %s, log has %s", en.Round, en.A, en.B, head, en.Head)
		}
	}
	return nil
}

// firstDiffLine names the first JSON log line where want and got part
// ways, for the -verify failure message.
func firstDiffLine(want, got []byte) string {
	wl := bytes.Split(bytes.TrimRight(want, "\n"), []byte("\n"))
	gl := bytes.Split(bytes.TrimRight(got, "\n"), []byte("\n"))
	for i := 0; i < len(wl) && i < len(gl); i++ {
		if !bytes.Equal(wl[i], gl[i]) {
			return fmt.Sprintf("log line %d", i+1)
		}
	}
	if len(wl) != len(gl) {
		return fmt.Sprintf("log length %d vs %d", len(gl), len(wl))
	}
	return "an unknown position"
}

// printStandings writes the standings table under the event's per-game
// lines. The percentages are fractions with MTR Appendix C's 33% floor,
// printed to three places. The persona column is read back out of the
// event's own EventCreated log entry, one per seat whatever the pod size.
func printStandings(out io.Writer, er *eventResult) {
	var personas []string
	entries, err := event.ParseLog(er.log)
	if err == nil {
		for _, en := range entries {
			if en.Kind == event.KindEventCreated {
				personas = make([]string, len(en.Seats))
				for i, s := range en.Seats {
					personas[i] = s.Persona
				}
			}
		}
	}
	fmt.Fprintln(out, "  standings: seat persona                pts    MW    OMW    GW    OGW")
	for _, s := range er.standings {
		persona := "?"
		if s.Seat >= 0 && s.Seat < len(personas) {
			persona = personas[s.Seat]
		}
		fmt.Fprintf(out, "    %d %-24s %3d  %.3f  %.3f  %.3f  %.3f\n",
			s.Seat, persona, s.Points, s.MW, s.OMW, s.GW, s.OGW)
	}
}
