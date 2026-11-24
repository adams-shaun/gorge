package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/oracle"
	"github.com/adams-shaun/gorge/engine/cards"
)

// -scenario <dir> plays scripted starts (docs/design/scripted-scenario-starts.md,
// sections 5 and 6): every <name>.json under dir, or the one file, is a
// scenario (oracle.Scenario) played -pairs games in lockstep, one run named
// sc-<name> each. A game's seed is the first seed at or past the run's whose
// toss names the scenario's active seat (Scenario.GameSeed), the focus seat
// plays oracle.FocusPolicy over the -policy bot, the other seats play the
// scenario's opponent.
//
// Outputs. -out is the directory a run's files go to: <out>/sc-<name>.verdict
// (the run's printed report, its verdict line first among them) and
// <out>/sc-<name>/ for the repros of its bad games. -coverage, -branches and
// -evidence each name a directory here, not a file: sc-<name>.coverage,
// .branches and .evidence land in it.
//
// Evidence (design section 5, D10): a scripted clean game adds key uses and
// never casts or land plays. The setup itself credits nothing, since
// CoverageOf credits casts from PutOnStack and land plays from LandPlayed and
// the start emits neither.
// scenarioUsage is the help of -scenario (declared in run(), with the other flags).
const scenarioUsage = "play scripted starts: this scenario file, or every .json file under this directory, -pairs games each (one run sc-<file> each), from -seed; the focus seat plays -policy through oracle.FocusPolicy, the other seats the scenario's opponent. -out is the directory of the sc-<file>.verdict files and of the bad games' repros; -coverage, -branches and -evidence name directories, one sc-<file>.<kind> file each. The pool comes from -synth (with -synth-with and -focus) or -set; -max-turns is the scenario's own"

// -branches is registered by init() on the process flag set, so the run reads its value from branchesPath.

// scFlag is the parsed value of the flag name, read from the flag set run() parsed.
func scFlag[T any](name string) T { return flag.Lookup(name).Value.(flag.Getter).Get().(T) }

// scenarioConflicts are the flags a scenario run has no use for: they pick
// another kind of run, or a seat count the scenario sets itself.
var scenarioConflicts = []string{"repro", "replay", "v2", "pairs-file", "seats", "pods", "cmd-mini", "reach",
	"search-seats", "play-refused", "play-interaction-refused", "kernel", "features", "stats", "only", "synth-decks", "focus-per-deck"}

// scenarioTables is the matchup table of every scenario run played so far, by
// run name: a bad-game line names its game's cards from it (badGameTable).
var scenarioTables sync.Map

// badGameTable is the card names of the decks of pairing p: the table of the
// scenario run it belongs to, or the pool decks' matchup.
func badGameTable(c *oracle.Corpus, p oracle.Pairing) ([]string, error) {
	if t, ok := scenarioTables.Load(p.A); ok {
		return t.([]string), nil
	}
	m, err := c.MatchupN(p.Decks(), 0)
	if err != nil {
		return nil, err
	}
	return m.Table, nil
}

// scenarioRunner is what every scenario of one -scenario run shares.
type scenarioRunner struct {
	c       *oracle.Corpus
	set     cardset.Set
	refused map[string]bool
	rc      oracle.RunConfig
	pol     func(uint64) oracle.Policy
	opts    sweepOpts

	seed          uint64
	first, pairs  int
	mulligans     int
	lead          string
	outDir        string
	covDir, brDir string
	evDir         string
}

// runScenarios is -scenario's run: it plays every scenario file and returns
// the worst exit code (2 for a usage error, 1 for a failed run).
func runScenarios(path string) int {
	explicit := map[string]bool{}
	flag.Visit(func(f *flag.Flag) { explicit[f.Name] = true })
	for _, name := range scenarioConflicts {
		if explicit[name] {
			fmt.Fprintf(os.Stderr, "kfuzz: -scenario cannot be used with -%s\n", name)
			return 2
		}
	}
	synth, with, focus := scFlag[string]("synth"), scFlag[string]("synth-with"), scFlag[string]("focus")
	if synth != "" {
		if err := checkSynthFlags(explicit, with, focus); err != nil {
			fmt.Fprintln(os.Stderr, "kfuzz:", err)
			return 2
		}
	} else if explicit["synth-with"] || explicit["focus"] {
		fmt.Fprintln(os.Stderr, "kfuzz: -synth-with and -focus need -synth")
		return 2
	}
	surfaceName, err := resolveSurface(scFlag[string]("surface"), scFlag[bool]("autopay-surface"), explicit["autopay-surface"])
	if err != nil {
		fmt.Fprintln(os.Stderr, "kfuzz:", err)
		return 2
	}
	lead := scFlag[string]("lead")
	if lead != "gorge" && lead != "kernel" {
		fmt.Fprintf(os.Stderr, "kfuzz: -lead %q: want gorge or kernel\n", lead)
		return 2
	}
	pol, err := policyFor(scFlag[string]("policy"), lead)
	if err != nil {
		fmt.Fprintln(os.Stderr, "kfuzz:", err)
		return 2
	}
	files, err := scenarioFiles(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "kfuzz:", err)
		return 1
	}
	cardsDir := scFlag[string]("cards")
	if cardsDir != "" {
		os.Setenv("GORGE_CARDS", cardsDir) // the tables come from the corpus this run plays
	} else if cardsDir, err = cardset.CorpusDir(); err != nil {
		fmt.Fprintln(os.Stderr, "kfuzz:", err)
		return 2
	}
	set, err := scenarioSet(synth, with, focus, scFlag[string]("set"), cardsDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "kfuzz:", err)
		return 2
	}
	decks := scFlag[string]("decks")
	if decks == "" {
		decks = oracle.PoolDecks("fdn")
	}
	c, err := oracle.OpenCorpus(cardsDir, decks)
	if err != nil {
		fmt.Fprintln(os.Stderr, "kfuzz:", err)
		return 1
	}
	sr := &scenarioRunner{c: c, set: set, pol: pol, lead: lead,
		rc: oracle.RunConfig{Config: oracle.Config{PassElision: scFlag[bool]("pass-elision"), GorgeAutopaySurface: surfaceName == "autopay"},
			MaxDecisions: scFlag[int]("max-decisions")},
		seed: scFlag[uint64]("seed"), first: scFlag[int]("first-pair"), pairs: scFlag[int]("pairs"), mulligans: scFlag[int]("mulligans"),
		outDir: scFlag[string]("out"), covDir: scFlag[string]("coverage"), brDir: branchesPath, evDir: scFlag[string]("evidence"),
		opts: sweepOpts{lead: lead, surface: surfaceName, workers: scFlag[int]("workers"), timeout: scFlag[time.Duration]("game-timeout"),
			absolute: scFlag[bool]("absolute"), mulligans: scFlag[int]("mulligans"), corpus: c}}
	for _, d := range []string{sr.outDir, sr.covDir, sr.brDir, sr.evDir} {
		if d == "" {
			continue
		}
		if err := os.MkdirAll(d, 0o755); err != nil {
			fmt.Fprintln(os.Stderr, "kfuzz:", err)
			return 1
		}
	}
	code := 0
	for _, f := range files {
		code = max(code, sr.run(f))
	}
	return code
}

// scenarioFiles is the scenario files path names: the file itself, or the
// .json files of the directory, sorted.
func scenarioFiles(path string) ([]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return []string{path}, nil
	}
	ents, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range ents {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			files = append(files, filepath.Join(path, e.Name()))
		}
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("-scenario %s: no .json scenario file", path)
	}
	slices.Sort(files)
	return files, nil
}

// scenarioSet is the set a scenario run plays: the -synth pool (widened as a
// -synth-with run is) or -set, with the unverified cards playing, since a
// scenario exists to verify them.
func scenarioSet(synth, with, focus, setName, cardsDir string) (cardset.Set, error) {
	var set cardset.Set
	var err error
	if synth != "" {
		set, err = synthSetOf(synth, with, focus, cardsDir)
	} else {
		set, err = cardset.ByName(setName)
	}
	if err != nil {
		return nil, err
	}
	return cardset.AllowUnverified(set), nil
}

// refusedNames is the cards set refuses by name: a run's evidence leaves them out.
func refusedNames(set cardset.Set) map[string]bool {
	refused := map[string]bool{}
	for i, n := range set.Names()[:set.NumCards()] {
		if set.Status(i) == cardset.StatusRefused {
			refused[n] = true
		}
	}
	return refused
}

// run plays one scenario file, -pairs games, and reports it as the run
// sc-<file>. It returns the run's exit code.
func (sr *scenarioRunner) run(file string) int {
	name := "sc-" + strings.TrimSuffix(filepath.Base(file), ".json")
	var sc *oracle.Scenario
	raw, err := os.ReadFile(file)
	if err == nil {
		sc, err = oracle.ParseScenario(raw)
	}
	ps := scenarioPairings(name, sc, sr.seed, sr.first, sr.pairs)
	seats := 2
	if sc != nil {
		seats = sc.Players
	}
	// A pod run has no coverage (-seats above 2 refuses -coverage and -branches).
	cov, br, ev := sr.covDir, sr.brDir, sr.evDir
	if seats > 2 {
		cov, br = "", ""
	}
	path := func(dir, kind string) string {
		if dir == "" {
			return ""
		}
		return filepath.Join(dir, name+"."+kind)
	}
	oldCov, oldEv, oldBr := coveragePath, evidencePath, branchesPath
	defer func() { coveragePath, evidencePath, branchesPath = oldCov, oldEv, oldBr }()
	coveragePath, evidencePath, branchesPath = path(cov, "coverage"), path(ev, "evidence"), path(br, "branches")
	collect := coveragePath != "" || evidencePath != "" || branchesPath != ""
	rc := sr.rc
	if sc != nil {
		rc.MaxTurns = int32(sc.MaxTurns)
	}
	out := sr.playAll(ps, func(p oracle.Pairing) played { return sr.play(sc, err, p, rc, collect) })
	o := sr.opts
	o.seats = seats
	if sr.outDir != "" {
		o.out = filepath.Join(sr.outDir, name)
	}
	if evidencePath != "" {
		o.refused = refusedNames(sr.set)
	}
	code, text := captureStdout(func() int { return report(name, ps, out, rc, o, nil) })
	if sr.outDir != "" {
		if werr := os.WriteFile(filepath.Join(sr.outDir, name+".verdict"), []byte(text), 0o644); werr != nil {
			fmt.Fprintln(os.Stderr, "kfuzz:", werr)
			code = max(code, 1)
		}
	}
	return code
}

// playAll plays every pairing under the per-game watchdog. A hung game stops
// the feed (playAll); the rest play on in this process, which keeps the hung
// goroutine's core until the run ends: the continuation exec a pool run uses
// would rerun every scenario of the directory.
func (sr *scenarioRunner) playAll(ps []oracle.Pairing, play func(oracle.Pairing) played) []played {
	out := make([]played, 0, len(ps))
	for start := 0; start < len(ps); {
		res, n := playAll(len(ps)-start, max(1, sr.opts.workers), sr.opts.timeout, true, func(i int) played { return play(ps[start+i]) })
		if n == 0 {
			break
		}
		out = append(out, res[:n]...)
		start += n
	}
	return out
}

// captureStdout runs f with stdout redirected to a file, then prints what it
// wrote to the real stdout and returns it with f's result.
func captureStdout(f func() int) (int, string) {
	tmp, err := os.CreateTemp("", "kfuzz-scenario")
	if err != nil {
		return f(), ""
	}
	defer os.Remove(tmp.Name())
	old := os.Stdout
	os.Stdout = tmp
	code := f()
	os.Stdout = old
	tmp.Close()
	b, _ := os.ReadFile(tmp.Name())
	os.Stdout.Write(b)
	return code, string(b)
}

// scenarioPairings is the run's games: game i (from first, pairs of them)
// plays the first seed at or past the one before it whose toss names the
// scenario's active seat. A scenario that did not parse, or names a seat or
// player count its validation refuses, plays seeds in a row: each game is
// refused with the reason.
func scenarioPairings(name string, sc *oracle.Scenario, seed uint64, first, n int) []oracle.Pairing {
	var ps []oracle.Pairing
	next := seed
	for i := 0; i < first+n; i++ {
		s := next
		if sc != nil && (sc.Players == 2 || sc.Players == 4) && sc.Active >= 0 && sc.Active < sc.Players {
			s = sc.GameSeed(next)
		}
		next = s + 1
		if i >= first {
			ps = append(ps, oracle.Pairing{Index: i, A: name, B: name, Seed: s})
		}
	}
	return ps
}

// play is one game of scenario sc, or the reason it cannot be played: sc is
// nil when the file did not parse (perr says why).
func (sr *scenarioRunner) play(sc *oracle.Scenario, perr error, p oracle.Pairing, rc oracle.RunConfig, collect bool) played {
	if perr != nil {
		return played{skip: perr}
	}
	m, err := sr.c.ScenarioMatchup(sc, sr.mulligans)
	if err != nil {
		return played{skip: err}
	}
	tbl, err := scenarioTable(sr.set, sr.c.Reg, m)
	if err != nil {
		return played{skip: err}
	}
	scenarioTables.LoadOrStore(p.A, m.Table)
	spec, err := scenarioFocus(sr.c.Reg, sc, m)
	if err != nil {
		return played{skip: err}
	}
	var cov *oracle.Coverage
	if collect {
		rc = oracle.WithCoverage(rc, m, &cov)
	}
	gp := scenarioPolicy(sc, spec, sr.pol(p.Seed), p.Seed)
	g, err := oracle.NewGorgeSide(m, p.Seed, rc.Config, nil)
	if err != nil {
		return played{skip: err}
	}
	if got := int(g.E.G.StartingPlayer); got != sc.Active {
		return played{skip: fmt.Errorf("scenario %s: seed %d: starting player %d, the scenario wants %d", sc.Focus, p.Seed, got, sc.Active)}
	}
	k, err := oracle.SetSide(sr.set, tbl, rc.Config)(m, p.Seed)
	if err != nil {
		return played{skip: err}
	}
	var res oracle.Result
	if sr.lead == "kernel" {
		res = oracle.LockstepLed(m, p.Seed, k, g, gp, rc)
	} else {
		res = oracle.LockstepLed(m, p.Seed, g, k, gp, rc)
	}
	oracle.CloseSide(k)
	if cov != nil {
		scriptedCoverage(cov)
		cov.Games, cov.Decisions, cov.Fallbacks = 1, res.Decisions, oracle.PolicyFallbacks(gp)
	}
	profDecisions.Add(int64(res.Decisions))
	if !verboseFDN && (sr.outDir == "" || res.Div == nil && !strings.HasPrefix(res.Stall, oracle.KernelStallPrefix)) && (!implicatesAll || res.Clean()) {
		res.Actions = nil // only the reports of a bad game read them
	}
	return played{m: m, res: res, cov: cov}
}

// scriptedCoverage makes cov a scripted game's: it keeps the key uses and the
// branches and drops the casts and land plays (design section 5: a scripted
// game never credits plays; they come from organic games).
func scriptedCoverage(cov *oracle.Coverage) {
	for _, cc := range cov.Cards {
		cc.Cast, cc.Land = 0, 0
	}
}

// scenarioTable is the kernel table over m's decks, as the lockstep tests
// build it: one entry a card name, in the set's spelling.
func scenarioTable(set cardset.Set, reg *cards.Registry, m *oracle.Matchup) (*kop.Table, error) {
	es := make([][]cardset.Entry, m.NSeat())
	for s := range es {
		var names []string
		for _, c := range m.SeatDeck(s) {
			names = append(names, c.Faces[0].Name)
		}
		for _, e := range oracle.ReproEntries(reg, set, names) {
			i := slices.IndexFunc(es[s], func(x cardset.Entry) bool { return x.Name == e.Name })
			if i < 0 {
				es[s] = append(es[s], cardset.Entry{Name: e.Name})
				i = len(es[s]) - 1
			}
			es[s][i].Count += e.Count
		}
	}
	if len(es) == 2 {
		tbl, _, _, err := set.Matchup(es[0], es[1])
		return tbl, err
	}
	tbl, _, err := set.MatchupN(es)
	return tbl, err
}

// scenarioFocus is the FocusSpec of sc on m: the first seat whose setup names
// the focus card plays for it, and every copy of the card in that seat's deck
// is a focus instance (Matchup.InstanceCard's layout: seat by seat, card by
// card).
func scenarioFocus(reg *cards.Registry, sc *oracle.Scenario, m *oracle.Matchup) (oracle.FocusSpec, error) {
	focus, ok := reg.Lookup(sc.Focus)
	if !ok {
		return oracle.FocusSpec{}, fmt.Errorf("scenario %s: no card by that name", sc.Focus)
	}
	seat := 0
	for p := 0; p < sc.Players; p++ {
		if scenarioNames(sc.Seats[fmt.Sprintf("p%d", p)], sc.Focus) {
			seat = p
			break
		}
	}
	off := 0
	for s := 0; s < seat; s++ {
		off += len(m.SeatDeck(s))
	}
	spec := oracle.FocusSpec{Seat: uint8(seat), Keys: sc.Keys, Turn: sc.Turn, Phase: sc.Phase}
	for i, c := range m.SeatDeck(seat) {
		if c != focus {
			continue
		}
		if off+i >= oracle.MaxInst {
			return spec, fmt.Errorf("scenario %s: instance %d is past the %d the projection holds", sc.Focus, off+i, oracle.MaxInst)
		}
		spec.Refs = append(spec.Refs, oracle.Ref(off+i))
	}
	return spec, nil
}

// scenarioNames reports whether seat's setup names card in any of its lists.
func scenarioNames(seat oracle.ScenarioSeat, card string) bool {
	for _, list := range [...][]string{seat.Hand, seat.Battlefield, seat.Graveyard, seat.Exile, seat.Library, seat.LibraryTop, seat.Commanders} {
		if slices.ContainsFunc(list, func(n string) bool { return strings.EqualFold(n, card) }) {
			return true
		}
	}
	return false
}

// scenarioPolicy is the policy of a scenario game: oracle.FocusPolicy for the
// focus seat over base, which answers the focus seat's other decisions with
// the -policy bot and every other seat's with the scenario's opponent.
func scenarioPolicy(sc *oracle.Scenario, spec oracle.FocusSpec, bot oracle.Policy, seed uint64) oracle.Policy {
	var opp oracle.Policy
	switch sc.Opponent {
	case "heuristic":
		opp = oracle.HeuristicPolicy(seed)
	case "passive":
		opp = passivePolicy()
	default:
		opp = oracle.ExplorePolicy(seed)
	}
	return oracle.FocusPolicy(oracle.PolicyFunc(func(c *oracle.PickCtx) oracle.Canon {
		if c.D != nil && c.D.Player == spec.Seat {
			return bot.Pick(c)
		}
		return opp.Pick(c)
	}), spec)
}

// passivePolicy passes whenever passing is legal, else closes the decision
// with Done, else takes the first option.
func passivePolicy() oracle.Policy {
	return oracle.PolicyFunc(func(c *oracle.PickCtx) oracle.Canon {
		for _, kind := range [...]oracle.CanonKind{oracle.ActPass, oracle.ActDone} {
			for _, a := range c.D.Options {
				if a.Kind() == kind {
					return a
				}
			}
		}
		return c.D.Options[0]
	})
}
