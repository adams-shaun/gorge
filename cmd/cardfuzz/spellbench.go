package main

// -spellbench measures a gorge bot's play through the SpellBench v2 API
// against the same bot's native play, on fuzzed decks (the generator and the
// per-game seeds are -manabrew's and the bot lane's).
//
//	-spellbench twin     every seat is a twin: the policy picks on gorge's own
//	  view (x_gorge_view_v1) and, behind the v2 neutral world, on the v2
//	  choose request for the same decision. The native pick is played, so the
//	  game stays on the native trajectory; each disagreement is counted by
//	  decision kind and written to -failures as one JSON line carrying the v2
//	  request ("spellbench" field; -repro replays to it). Exit status 1 on any
//	  disagreement or harness error, or with -min-agree on a decision kind
//	  with at least 50 compared decisions whose agreement is below the
//	  threshold (the default 0 keeps the any-disagreement rule).
//	-spellbench outcome  each deck pair is played twice on one seed, once with
//	  native seats and once with seat 0 (or both seats, -sb-v2-seats both) a v2
//	  neutral agent behind server/spellbench/seat, the path mtgsim -sb-agent
//	  takes, run in process. Prints win, turn and length per pair and the
//	  win-rate parity.
//
// -autopay off|all applies to both: all is the real v2 configuration (the v2
// environment offers engine_autopay; the native bot seats auto-pay). The
// coverage state is not read or written, so a seed names the same games on
// every run.

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"os"
	"sort"
	"strings"
	"sync"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/server/spellbench/agent"
	"github.com/adams-shaun/gorge/server/spellbench/catalog"
	"github.com/adams-shaun/gorge/server/spellbench/env"
	"github.com/adams-shaun/gorge/server/spellbench/minihost"
	"github.com/adams-shaun/gorge/server/spellbench/neutral/twin"
	"github.com/adams-shaun/gorge/server/spellbench/observe"
	sbseat "github.com/adams-shaun/gorge/server/spellbench/seat"
	"github.com/adams-shaun/gorge/server/spellbench/validate"
	"github.com/adams-shaun/gorge/server/spellbench/wire"
	gbench "github.com/adams-shaun/gorge/testkit/bench"
)

// sbOptions is one -spellbench run's configuration.
type sbOptions struct {
	mode    string // "twin" or "outcome"
	policy  string
	autopay bool
	v2Both  bool // outcome: both seats are v2 agents, not just seat 0
	// maxSteps bounds a twin game's v2 steps (a truncated game is not a failure).
	maxSteps                         uint64
	maxTurns, maxIntents, maxObjects int
	games                            int
	seed                             uint64
	workers                          int
	// minAgree fails the twin run when a decision kind with at least
	// sbMinAgreeSamples compared decisions agrees below this percentage
	// (0 keeps the any-disagreement rule).
	minAgree float64
	// maxOutcomeGap fails the outcome run when the 95% CI of (v2 - native)
	// seat-0 win rate excludes 0 and |gap| exceeds this many percentage
	// points (0 disables the check).
	maxOutcomeGap float64
}

// sbMinAgreeSamples is the smallest compared-decision count -min-agree gates:
// a kind with fewer samples is too noisy for a percentage threshold.
const sbMinAgreeSamples = 50

// sbRecord is the "spellbench" field of a twin failure record.
type sbRecord struct {
	Mode      string          `json:"mode"`
	Policy    string          `json:"policy"`
	Index     int             `json:"index"` // decision index in the game (twin.Disagreement.Index)
	Kind      string          `json:"kind"`
	Seat      string          `json:"seat"`
	Native    int             `json:"native_pick"`
	V2        int             `json:"v2_pick"`
	NativeSem json.RawMessage `json:"native_semantic"`
	V2Sem     json.RawMessage `json:"v2_semantic"`
	V2Error   string          `json:"v2_error,omitempty"`
	Request   json.RawMessage `json:"request"`
}

// fullCardName is the Oracle name the v2 catalog resolves: faces joined.
func fullCardName(c *cards.Card) string {
	names := make([]string, len(c.Faces))
	for i, f := range c.Faces {
		names[i] = f.Name
	}
	return strings.Join(names, " // ")
}

// sbDeck is a generated deck as a catalog deck: rows in first-appearance order.
func sbDeck(reg *cards.Registry, d genDeck, id string) (catalog.Deck, error) {
	cs, err := resolveDeck(reg, d)
	if err != nil {
		return catalog.Deck{}, err
	}
	var rows []wire.DeckRow
	at := map[string]int{}
	for _, c := range cs {
		n := fullCardName(c)
		if i, ok := at[n]; ok {
			rows[i].Count++
			continue
		}
		at[n] = len(rows)
		rows = append(rows, wire.DeckRow{Name: n, Count: 1})
	}
	return catalog.Deck{CatalogID: id, Name: id, Rows: rows}, nil
}

// sbDomain is the distinct card names of both decks in code point order.
func sbDomain(decks [2]catalog.Deck) []string {
	set := map[string]bool{}
	for _, d := range decks {
		for _, r := range d.Rows {
			set[r.Name] = true
		}
	}
	out := make([]string, 0, len(set))
	for n := range set {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// sbTwinGame is one twin game's result.
type sbTwinGame struct {
	tw          *twin.Twin
	fails       []*failure
	steps       int
	truncated   bool
	unsupported string // why v2 cannot express the game ("" when it can)
	err         string // a harness error ("" when none)
}

// playSBTwin plays one twin game and returns its tally and one failure record
// per disagreement.
func playSBTwin(reg *cards.Registry, decks []genDeck, seed uint64, o sbOptions) (res sbTwinGame) {
	res.tw = twin.New()
	// The ChosenColour skip is armed only when the Twin can look cards up.
	res.tw.SetRegistry(reg)
	defer func() {
		if r := recover(); r != nil {
			res.err = fmt.Sprintf("panic: %v", r)
		}
	}()
	var cd [2]catalog.Deck
	for i := range cd {
		d, err := sbDeck(reg, decks[i], fmt.Sprintf("fuzz-%d", i))
		if err != nil {
			res.err = err.Error()
			return
		}
		cd[i] = d
	}
	srv := env.New(reg, nil)
	if o.autopay {
		srv.EnableAutoPay()
	}
	srv.AddDeck(cd[0])
	srv.AddDeck(cd[1])
	kinds := map[string]bool{}
	for _, k := range env.DecisionKinds {
		kinds[k] = true
	}
	h := &minihost.Host{RunSecret: make([]byte, 32), Engine: &minihost.EngineLink{S: srv},
		Profile:  validate.Profile{Kinds: kinds, Flags: observe.Flags, Extensions: map[string]bool{"x_gorge_view_v1": true}},
		MaxSteps: o.maxSteps, MaxDecisions: 9999}
	var links [2]minihost.Link
	for s := range links {
		va, err := agent.New(o.policy)
		if err != nil {
			res.err = err.Error()
			return
		}
		na, _ := agent.New(o.policy)
		na.SetRegistry(reg)
		if err := na.EnableNeutral(); err != nil {
			res.err = err.Error()
			return
		}
		if !o.autopay {
			// A manual host offers taps, not plans; ask both worlds the same
			// question so the twin compares the policy, not the mana surface.
			va.SetManualMana()
			na.SetManualMana()
		}
		links[s] = res.tw.Seat(va, na)
	}
	res.tw.OnDisagree = func(d twin.Disagreement) {
		rec := &sbRecord{Mode: "twin", Policy: o.policy, Index: d.Index, Kind: d.Kind, Seat: d.Seat, Native: d.Native, V2: d.V2,
			NativeSem: d.NativeSem, V2Sem: d.V2Sem, V2Error: d.V2Error, Request: d.Request}
		diag := fmt.Sprintf("%s decision %d (%s): gorge %s, neutral %s", d.Kind, d.Index, d.Seat, d.NativeSem, d.V2Sem)
		if d.V2Error != "" {
			diag += " (" + d.V2Error + ")"
		}
		f := &failure{Kind: "sbtwin", Seed: seed, Decks: decks, SB: rec, Diag: diag, Sig: sbClass(d)}
		if o.autopay {
			f.AutoPay = "all"
		}
		res.fails = append(res.fails, f)
	}
	r, err := h.PlayDecks(seed, cd, sbDomain(cd), "london", []string{"x_gorge_view_v1"}, links)
	res.steps = r.Steps
	switch {
	case err != nil && strings.Contains(err.Error(), "unsupported_deck"):
		res.unsupported = "reset:" + trunc(stripDigits(err.Error()), 120)
	case err != nil:
		res.err = trunc(err.Error(), 200)
	case r.Terminal.Classification == "truncated":
		res.truncated = true
	case r.Terminal.Classification == "halted" && strings.HasPrefix(r.Terminal.Reason, "engine_contract_failure:"):
		res.unsupported = strings.TrimPrefix(r.Terminal.Reason, "engine_contract_failure:")
	}
	return res
}

// sbSemKind is the "kind" of a candidate semantic's JSON.
func sbSemKind(sem json.RawMessage) string {
	var k struct{ Kind string }
	if json.Unmarshal(sem, &k) != nil || k.Kind == "" {
		return "?"
	}
	return k.Kind
}

// sbClass names a disagreement's divergence class: the decision kind, what
// gorge picked and what the neutral world picked.
func sbClass(d twin.Disagreement) string {
	v2 := sbSemKind(d.V2Sem)
	if d.V2 < 0 {
		v2 = "error"
	}
	return fmt.Sprintf("%s: %s -> %s", d.Kind, sbSemKind(d.NativeSem), v2)
}

// sbTwinSummary folds a run's games in game order.
type sbTwinSummary struct {
	Games, Truncated, Errors, Disagreements int
	Unsupported                             map[string]int
	ErrorText                               map[string]int
	Agree, Total                            map[string]int
	Classes                                 map[string]*sbClassStat
}

type sbClassStat struct {
	N       int
	Example string
}

func newSBTwinSummary() *sbTwinSummary {
	return &sbTwinSummary{Unsupported: map[string]int{}, ErrorText: map[string]int{}, Agree: map[string]int{}, Total: map[string]int{}, Classes: map[string]*sbClassStat{}}
}

func (s *sbTwinSummary) add(seed uint64, g sbTwinGame) {
	s.Games++
	if g.truncated {
		s.Truncated++
	}
	if g.unsupported != "" {
		s.Unsupported[g.unsupported]++
	}
	if g.err != "" {
		s.Errors++
		s.ErrorText[g.err]++
	}
	for k, n := range g.tw.Total {
		s.Total[k] += n
	}
	for k, n := range g.tw.Agree {
		s.Agree[k] += n
	}
	for _, f := range g.fails {
		s.Disagreements++
		c := s.Classes[f.Sig]
		if c == nil {
			c = &sbClassStat{Example: fmt.Sprintf("seed %d: %s", seed, f.Diag)}
			s.Classes[f.Sig] = c
		}
		c.N++
	}
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// failingKinds returns the decision kinds, in code point order, whose
// agreement is below minAgree percent over at least sbMinAgreeSamples compared
// decisions. minAgree <= 0 disables the check.
func (s *sbTwinSummary) failingKinds(minAgree float64) []string {
	if minAgree <= 0 {
		return nil
	}
	var out []string
	for _, k := range sortedKeys(s.Total) {
		if k == twin.Floated || k == twin.Opaque || k == twin.ChosenColour {
			continue
		}
		n := s.Total[k]
		if n < sbMinAgreeSamples {
			continue
		}
		if 100*float64(s.Agree[k])/float64(n) < minAgree {
			out = append(out, k)
		}
	}
	return out
}

func (s *sbTwinSummary) print(w io.Writer, o sbOptions) {
	fmt.Fprintf(w, "== spellbench twin: policy=%s autopay=%s seed=%d games=%d ==\n", o.policy, map[bool]string{true: "all", false: "off"}[o.autopay], o.seed, o.games)
	unsupported := 0
	for _, n := range s.Unsupported {
		unsupported += n
	}
	fmt.Fprintf(w, "games: %d played, %d unsupported_kind, %d truncated, %d errors\n", s.Games, unsupported, s.Truncated, s.Errors)
	for _, k := range sortedKeys(s.Unsupported) {
		fmt.Fprintf(w, "  unsupported_kind %-40s %d\n", k, s.Unsupported[k])
	}
	for _, k := range sortedKeys(s.ErrorText) {
		fmt.Fprintf(w, "  error %s: %d\n", k, s.ErrorText[k])
	}
	agree, total := 0, 0
	for _, k := range sortedKeys(s.Total) {
		if k == twin.Floated || k == twin.Opaque || k == twin.ChosenColour {
			continue
		}
		agree += s.Agree[k]
		total += s.Total[k]
	}
	fmt.Fprintf(w, "agreement %d/%d (%s); %d decisions skipped where gorge floated mana, %d where the neutral world could not rebuild the pick, %d where a chosen-colour source was in play\n", agree, total, sbPct(agree, total), s.Total[twin.Floated], s.Total[twin.Opaque], s.Total[twin.ChosenColour])
	failing := map[string]bool{}
	for _, k := range s.failingKinds(o.minAgree) {
		failing[k] = true
	}
	fmt.Fprintln(w, "agreement by decision kind:")
	for _, k := range sortedKeys(s.Total) {
		if k != twin.Floated && k != twin.Opaque && k != twin.ChosenColour {
			mark := ""
			if failing[k] {
				mark = "  FAIL"
			}
			fmt.Fprintf(w, "  %-32s %6d/%6d%s\n", k, s.Agree[k], s.Total[k], mark)
		}
	}
	classes := sortedKeys(s.Classes)
	sort.SliceStable(classes, func(a, b int) bool { return s.Classes[classes[a]].N > s.Classes[classes[b]].N })
	fmt.Fprintf(w, "divergence classes: %d (top %d)\n", len(classes), min(10, len(classes)))
	for _, c := range classes[:min(10, len(classes))] {
		fmt.Fprintf(w, "%6d  %s\n        e.g. %s\n", s.Classes[c].N, c, trunc(s.Classes[c].Example, 400))
	}
}

func sbPct(a, b int) string {
	if b == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%.1f%%", 100*float64(a)/float64(b))
}

// sbJobs generates the run's deck pairs exactly as the bot lane does: game i
// plays at mix(seed, i) from a fresh coverage state.
type sbJob struct {
	idx   int
	seed  uint64
	decks []genDeck
}

func sbJobs(p *pool, seed uint64, games int) []sbJob {
	c, _ := loadCov("") // no file: the generator's uncovered weights
	jobs := make([]sbJob, games)
	for i := range jobs {
		gs := mix(seed, uint64(i))
		r := rand.New(rand.NewPCG(gs, gs^0xdeadbeefcafef00d))
		jobs[i] = sbJob{idx: i, seed: gs, decks: []genDeck{generate(r, p, c), generate(r, p, c)}}
	}
	return jobs
}

// sbRun plays play(job) on workers and folds the results in game order.
func sbRun[T any](jobs []sbJob, workers int, play func(sbJob) T, fold func(sbJob, T)) {
	type out struct {
		idx int
		res T
	}
	ch := make(chan sbJob)
	res := make(chan out, workers)
	var wg sync.WaitGroup
	for w := 0; w < max(workers, 1); w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range ch {
				res <- out{j.idx, play(j)}
			}
		}()
	}
	go func() {
		for _, j := range jobs {
			ch <- j
		}
		close(ch)
		wg.Wait()
		close(res)
	}()
	pending := map[int]T{}
	next := 0
	for got := range res {
		pending[got.idx] = got.res
		for {
			r, ok := pending[next]
			if !ok {
				break
			}
			delete(pending, next)
			fold(jobs[next], r)
			next++
		}
	}
}

// runSBTwin plays the twin run, appends each disagreement to failPath and
// prints the summary to out.
func runSBTwin(reg *cards.Registry, p *pool, o sbOptions, failPath string, out io.Writer) (*sbTwinSummary, error) {
	sum := newSBTwinSummary()
	var fw *bufio.Writer
	var ff *os.File
	defer func() {
		if fw != nil {
			fw.Flush()
			ff.Close()
		}
	}()
	var werr error
	sbRun(sbJobs(p, o.seed, o.games), o.workers,
		func(j sbJob) sbTwinGame { return playSBTwin(reg, j.decks, j.seed, o) },
		func(j sbJob, g sbTwinGame) {
			sum.add(j.seed, g)
			for _, f := range g.fails {
				if fw == nil {
					var err error
					if ff, err = os.OpenFile(failPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err != nil {
						werr = err
						return
					}
					fw = bufio.NewWriter(ff)
				}
				b, _ := json.Marshal(f)
				fw.Write(b)
				fw.WriteByte('\n')
			}
			fmt.Fprintf(os.Stderr, "cardfuzz: spellbench twin %d/%d games, %d disagreements\n", sum.Games, o.games, sum.Disagreements)
		})
	sum.print(out, o)
	return sum, werr
}

// runSBRepro replays a twin record's game and checks the disagreement at its
// decision index recurs. It returns 2 when it does, 0 when it does not.
func runSBRepro(reg *cards.Registry, rec failure, o sbOptions, out io.Writer) int {
	o.policy, o.autopay = rec.SB.Policy, rec.AutoPay == "all"
	g := playSBTwin(reg, rec.Decks, rec.Seed, o)
	if g.err != "" {
		fmt.Fprintf(out, "REPRO sbtwin error: %s\n", g.err)
		return 1
	}
	for _, f := range g.fails {
		if f.SB.Index == rec.SB.Index {
			fmt.Fprintf(out, "REPRO sbtwin seed=%d decision %d\n%s\n", rec.Seed, f.SB.Index, f.Diag)
			return 2
		}
	}
	fmt.Fprintf(out, "REPRO: no disagreement at decision %d (not reproduced)\n", rec.SB.Index)
	return 0
}

// sbSide is one lane's result for a deck pair.
type sbSide struct {
	winner  int // seat, -1 for a draw or an unfinished game
	done    bool
	turns   int32
	intents int
	stall   string
	forfeit [2]string
	err     string
}

func (s sbSide) String() string {
	r := fmt.Sprintf("winner=%d turns=%d intents=%d", s.winner, s.turns, s.intents)
	switch {
	case s.err != "":
		r = "error: " + trunc(s.err, 80)
	case s.stall != "":
		r = fmt.Sprintf("stall=%s turns=%d intents=%d", s.stall, s.turns, s.intents)
	case !s.done:
		r = "unfinished"
	case s.winner < 0:
		r = fmt.Sprintf("draw turns=%d intents=%d", s.turns, s.intents)
	}
	for i, f := range s.forfeit {
		if f != "" {
			r += fmt.Sprintf(" forfeit(seat %d)=%s", i, f)
		}
	}
	return r
}

// playSBLane plays one game at seed with v2[i] marking the seats that are v2
// neutral agents behind the seat path (native bot seats elsewhere).
func playSBLane(reg *cards.Registry, decks []genDeck, seed uint64, o sbOptions, v2 []bool) (res sbSide) {
	res.winner = -1
	defer func() {
		if r := recover(); r != nil {
			res.err = fmt.Sprintf("panic: %v", r)
		}
	}()
	var dk [][]*cards.Card
	names := make([]string, len(decks))
	for i, d := range decks {
		cs, err := resolveDeck(reg, d)
		if err != nil {
			res.err = err.Error()
			return
		}
		dk = append(dk, cs)
		names[i] = fmt.Sprintf("%s-%d", d.Colour, i)
	}
	seats := make([]seat.Seat, len(decks))
	agents := make([]*sbseat.Agent, len(decks))
	for i := range seats {
		opt := bots.Options{Seed: seed ^ (0x9e3779b97f4a7c15 * uint64(i+1)), AutoPayMana: o.autopay}
		if !v2[i] {
			s, err := bots.New(o.policy, opt)
			if err != nil {
				res.err = err.Error()
				return
			}
			seats[i] = s
			continue
		}
		a, err := sbseat.NewInProcess("sb-"+o.policy, func(r io.Reader, w io.Writer) error {
			s, err := agent.New(o.policy)
			if err != nil {
				return err
			}
			s.SetRegistry(reg)
			if err := s.EnableNeutral(); err != nil {
				return err
			}
			return agent.Serve(r, w, s)
		}, opt)
		if err != nil {
			res.err = err.Error()
			return
		}
		agents[i], seats[i] = a, a
	}
	cfg := rules.Config{Names: names, Decks: dk, Tokens: reg.Tokens, Seed: seed, NameUniverse: reg.Universe(), LoopGuard: objectCapGuard(o.maxObjects)}
	oc, _, err := gbench.PlayGame(cfg, seats, o.maxTurns, o.maxIntents, gbench.Hooks{Guard: boardGuard(o.maxObjects)})
	for i, a := range agents {
		if a != nil {
			res.forfeit[i] = a.Forfeit()
		}
	}
	res.turns, res.intents, res.stall = oc.Turns, oc.Intents, oc.StallOn
	switch {
	case err != nil:
		res.err = trunc(err.Error(), 200)
	case oc.StallOn == "":
		res.done = true
		if !oc.Draw {
			res.winner = oc.WinnerSeat
		}
	}
	return res
}

type sbPair struct{ native, v2 sbSide }

// wilson is the 95% Wilson score interval of k successes in n.
func wilson(k, n int) (lo, hi float64) {
	if n == 0 {
		return 0, 1
	}
	const z = 1.959964
	p, nn := float64(k)/float64(n), float64(n)
	den := 1 + z*z/nn
	c := (p + z*z/(2*nn)) / den
	h := z * math.Sqrt(p*(1-p)/nn+z*z/(4*nn*nn)) / den
	return c - h, c + h
}

// sbOutcomeParity is the (v2 - native) seat-0 win-rate gap and its 95% normal
// interval, in percentage points. ok is false when a lane finished no game.
type sbOutcomeParity struct {
	nativeWin, nativeDone, v2Win, v2Done int
	gapPP, loPP, hiPP                    float64
	ok                                   bool
}

// sbOutcomeParityOf computes the outcome parity from each lane's seat-0 wins
// over its finished games.
func sbOutcomeParityOf(nativeWin, nativeDone, v2Win, v2Done int) sbOutcomeParity {
	p := sbOutcomeParity{nativeWin: nativeWin, nativeDone: nativeDone, v2Win: v2Win, v2Done: v2Done}
	if nativeDone == 0 || v2Done == 0 {
		return p
	}
	pn, pv := float64(nativeWin)/float64(nativeDone), float64(v2Win)/float64(v2Done)
	se := math.Sqrt(pn*(1-pn)/float64(nativeDone) + pv*(1-pv)/float64(v2Done))
	d := pv - pn
	p.gapPP, p.loPP, p.hiPP = 100*d, 100*(d-1.959964*se), 100*(d+1.959964*se)
	p.ok = true
	return p
}

// fails reports whether the interval excludes zero and the gap exceeds
// maxGapPP percentage points. maxGapPP <= 0 disables the check.
func (p sbOutcomeParity) fails(maxGapPP float64) bool {
	if maxGapPP <= 0 || !p.ok {
		return false
	}
	return (p.loPP > 0 || p.hiPP < 0) && math.Abs(p.gapPP) > maxGapPP
}

func runSBOutcome(reg *cards.Registry, p *pool, o sbOptions, out io.Writer) sbOutcomeParity {
	jobs := sbJobs(p, o.seed, o.games)
	v2 := []bool{true, o.v2Both}
	pairs := make([]sbPair, 0, len(jobs))
	sbRun(jobs, o.workers,
		func(j sbJob) sbPair {
			return sbPair{native: playSBLane(reg, j.decks, j.seed, o, []bool{false, false}), v2: playSBLane(reg, j.decks, j.seed, o, v2)}
		},
		func(j sbJob, pr sbPair) {
			pairs = append(pairs, pr)
			fmt.Fprintf(os.Stderr, "cardfuzz: spellbench outcome %d/%d pairs\n", len(pairs), o.games)
		})
	fmt.Fprintf(out, "== spellbench outcome: policy=%s autopay=%s seed=%d pairs=%d v2 seats=%s ==\n", o.policy,
		map[bool]string{true: "all", false: "off"}[o.autopay], o.seed, o.games, map[bool]string{true: "both", false: "first"}[o.v2Both])
	var nDone, nWin, vDone, vWin, same, both, forfeits int
	var nTurns, vTurns, nLen, vLen int
	for i, pr := range pairs {
		fmt.Fprintf(out, "pair %3d seed %d\n  native: %s\n  v2:     %s\n", i, jobs[i].seed, pr.native, pr.v2)
		if pr.native.done {
			nDone++
			nTurns += int(pr.native.turns)
			nLen += pr.native.intents
			if pr.native.winner == 0 {
				nWin++
			}
		}
		if pr.v2.done {
			vDone++
			vTurns += int(pr.v2.turns)
			vLen += pr.v2.intents
			if pr.v2.winner == 0 {
				vWin++
			}
		}
		if pr.native.done && pr.v2.done {
			both++
			if pr.native.winner == pr.v2.winner {
				same++
			}
		}
		if pr.v2.forfeit[0] != "" || pr.v2.forfeit[1] != "" {
			forfeits++
		}
	}
	rate := func(k, n int) string {
		lo, hi := wilson(k, n)
		return fmt.Sprintf("%d/%d = %s [%.1f%%, %.1f%%]", k, n, sbPct(k, n), 100*lo, 100*hi)
	}
	mean := func(sum, n int) string {
		if n == 0 {
			return "n/a"
		}
		return fmt.Sprintf("%.1f", float64(sum)/float64(n))
	}
	fmt.Fprintf(out, "seat 0 win-rate, native seats: %s\n", rate(nWin, nDone))
	fmt.Fprintf(out, "seat 0 win-rate, v2 seats:     %s\n", rate(vWin, vDone))
	parity := sbOutcomeParityOf(nWin, nDone, vWin, vDone)
	if parity.ok {
		mark := ""
		if parity.fails(o.maxOutcomeGap) {
			mark = "  FAIL"
		}
		fmt.Fprintf(out, "outcome parity: v2 - native = %+.1f pp, 95%% CI [%+.1f, %+.1f] pp (unpaired normal)%s\n", parity.gapPP, parity.loPP, parity.hiPP, mark)
	} else {
		fmt.Fprintln(out, "outcome parity: n/a (no finished games)")
	}
	fmt.Fprintf(out, "same winner on %d/%d pairs both lanes finished; v2 lane forfeits: %d pairs\n", same, both, forfeits)
	fmt.Fprintf(out, "mean turns: native %s, v2 %s; mean intents: native %s, v2 %s\n", mean(nTurns, nDone), mean(vTurns, vDone), mean(nLen, nDone), mean(vLen, vDone))
	return parity
}

// runSpellbench is main's -spellbench entry; it returns the exit status.
func runSpellbench(reg *cards.Registry, p *pool, o sbOptions, failPath string, out io.Writer) int {
	switch o.mode {
	case "twin":
		sum, err := runSBTwin(reg, p, o, failPath, out)
		if err != nil {
			fmt.Fprintln(os.Stderr, "cardfuzz:", err)
			return 1
		}
		if sum.Errors > 0 {
			return 1
		}
		if o.minAgree > 0 {
			if kinds := sum.failingKinds(o.minAgree); len(kinds) > 0 {
				detail := make([]string, len(kinds))
				for i, k := range kinds {
					detail[i] = fmt.Sprintf("%s %s", k, sbPct(sum.Agree[k], sum.Total[k]))
				}
				fmt.Fprintf(os.Stderr, "cardfuzz: spellbench twin agreement below -min-agree %.1f%%: %s\n", o.minAgree, strings.Join(detail, ", "))
				return 1
			}
			return 0
		}
		if sum.Disagreements > 0 {
			return 1
		}
		return 0
	case "outcome":
		parity := runSBOutcome(reg, p, o, out)
		if parity.fails(o.maxOutcomeGap) {
			fmt.Fprintf(os.Stderr, "cardfuzz: spellbench outcome gap %+.1f pp exceeds -max-outcome-gap %.1f pp (95%% CI [%+.1f, %+.1f])\n", parity.gapPP, o.maxOutcomeGap, parity.loPP, parity.hiPP)
			return 1
		}
		return 0
	}
	fmt.Fprintf(os.Stderr, "cardfuzz: -spellbench %q: want twin or outcome\n", o.mode)
	return 1
}
