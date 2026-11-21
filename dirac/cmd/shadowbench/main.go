// Command shadowbench plays the dirac shadow seat (dirac/shadowseat) against
// the heuristic bot in gorge games, gorge being the host, and prints one JSON
// line: how often the seat staged, why it fell back, and its win rate with a
// Wilson interval. See docs/site/architecture/dirac/shadow.md.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"sort"

	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/dirac/oracle"
	"github.com/adams-shaun/gorge/dirac/shadowseat"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/testkit/bench"
)

// maxIntents is the per-game intent cap.
const maxIntents = 10000

// topCardRefusals is how many card refusals the report names.
const topCardRefusals = 20

type options struct {
	pool                string
	games, sims, worlds int
	seed                uint64
	maxTurns            int
}

func main() { os.Exit(run(os.Args[1:], os.Stdout)) }

// run is the command: 0 on success, 1 when a run failed or a shadow intent
// was refused, 2 on a usage error (nothing on stdout).
func run(args []string, stdout io.Writer) int {
	o, err := parseFlags(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, "shadowbench:", err)
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	p, err := openPool(o.pool)
	if err != nil {
		fmt.Fprintln(os.Stderr, "shadowbench:", err)
		if errors.Is(err, errUsage) {
			return 2
		}
		return 1
	}
	var tot totals
	for g := 0; g < o.games; g++ {
		if err := playOne(p, o, g, &tot); err != nil {
			fmt.Fprintln(os.Stderr, "shadowbench:", err)
			return 1
		}
	}
	if p.redeals > 0 {
		fmt.Fprintf(os.Stderr, "shadowbench: %d matchups redrawn after a set guard refused them\n", p.redeals)
	}
	line, err := json.Marshal(tot.report(o))
	if err != nil {
		fmt.Fprintln(os.Stderr, "shadowbench:", err)
		return 1
	}
	fmt.Fprintf(stdout, "%s\n", line)
	if tot.refused > 0 {
		return 1
	}
	return 0
}

// parseFlags reads and checks the command line.
func parseFlags(args []string) (options, error) {
	var o options
	fs := flag.NewFlagSet("shadowbench", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.StringVar(&o.pool, "pool", "vanilla", "card pool: vanilla, mbc-stage or set:<name>")
	fs.IntVar(&o.games, "games", 100, "games to play")
	fs.IntVar(&o.sims, "sims", shadowseat.DefaultSims, "simulations per search (a multiple of -worlds)")
	fs.IntVar(&o.worlds, "worlds", shadowseat.DefaultWorlds, "worlds per search")
	fs.Uint64Var(&o.seed, "seed", 1, "base seed; game g plays seed+g")
	fs.IntVar(&o.maxTurns, "max-turns", 60, "turn cap, past which a game is a stall")
	if err := fs.Parse(args); err != nil {
		return o, err
	}
	switch {
	case fs.NArg() != 0:
		return o, fmt.Errorf("%w: unexpected arguments %v", errUsage, fs.Args())
	case o.games <= 0:
		return o, fmt.Errorf("%w: -games %d must be positive", errUsage, o.games)
	case o.sims <= 0 || o.worlds <= 0:
		return o, fmt.Errorf("%w: -sims %d and -worlds %d must be positive", errUsage, o.sims, o.worlds)
	case o.sims%o.worlds != 0:
		return o, fmt.Errorf("%w: -sims %d is not a multiple of -worlds %d", errUsage, o.sims, o.worlds)
	case o.maxTurns <= 0:
		return o, fmt.Errorf("%w: -max-turns %d must be positive", errUsage, o.maxTurns)
	}
	return o, nil
}

// totals is the sum over every game.
type totals struct {
	games, shadowWins, heuristicWins, draws, stalls int
	refused                                         int
	stats                                           shadowseat.Stats
}

// playOne plays game g: the shadow seat in seat g%2 against the heuristic
// bot, and folds the result into tot.
func playOne(p *pool, o options, g int, tot *totals) error {
	seed := o.seed + uint64(g)
	tbl, decks, err := p.deal(seed)
	if err != nil {
		return err
	}
	m, err := oracle.KernelMatchup(p.reg, tbl, [2]string{"a", "b"}, decks, nil, 0)
	if err != nil {
		return fmt.Errorf("seed %d: %w", seed, err)
	}
	var lists [][]string
	for s := range decks {
		var l []string
		for _, id := range decks[s] {
			l = append(l, tbl.Cards[id].Name)
		}
		lists = append(lists, l)
	}
	sd, err := shadowseat.New(shadowseat.Config{
		Tbl: tbl, Set: p.set, Reg: p.reg, Lists: lists, Sims: o.sims, Worlds: o.worlds,
		Seed: seed, Fallback: heuristic.NewBot(seed ^ 0xfa11),
	})
	if err != nil {
		return fmt.Errorf("seed %d: %w", seed, err)
	}
	idx := g % 2
	seats := make([]seat.Seat, 2)
	seats[idx], seats[1-idx] = sd, heuristic.NewBot(seed^0xb07)
	refused := 0
	out, _, err := bench.PlayGame(m.Config(seed), seats, o.maxTurns, maxIntents, bench.Hooks{
		Submit: func(e *rules.Engine, seatIdx int, d *decision.Decision, in decision.Intent) (bool, error) {
			if seatIdx != idx {
				return false, nil
			}
			if err := e.Submit(in); err != nil {
				refused++
				fmt.Fprintf(os.Stderr, "shadowbench: seed %d: gorge refused the shadow seat's intent at %s decision %d: %v\n", seed, d.Kind, d.Seq, err)
				return false, nil // PlayGame re-submits, then re-answers
			}
			return true, nil
		},
	})
	if err != nil {
		return fmt.Errorf("seed %d: %w", seed, err)
	}
	tot.add(out, idx, refused, sd.Stats())
	return nil
}

// add folds one game.
func (t *totals) add(out bench.Outcome, shadowIdx, refused int, st shadowseat.Stats) {
	t.games++
	t.refused += refused
	switch {
	case out.IsStalled():
		t.stalls++
	case out.Draw:
		t.draws++
	case out.WinnerSeat == shadowIdx:
		t.shadowWins++
	default:
		t.heuristicWins++
	}
	t.stats.Decisions += st.Decisions
	t.stats.Searched += st.Searched
	t.stats.Continued += st.Continued
	t.stats.Fallbacks += st.Fallbacks
	t.stats.Reasons = sumInto(t.stats.Reasons, st.Reasons)
	t.stats.CardRefusals = sumInto(t.stats.CardRefusals, st.CardRefusals)
}

// sumInto adds src to dst, allocating dst on first use.
func sumInto(dst, src map[string]int) map[string]int {
	if dst == nil {
		dst = map[string]int{}
	}
	for _, k := range sortedKeys(src) {
		dst[k] += src[k]
	}
	return dst
}

// sortedKeys is m's keys in order.
func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// report is the one JSON line.
type report struct {
	Pool          string         `json:"pool"`
	Games         int            `json:"games"`
	ShadowWins    int            `json:"shadow_wins"`
	HeuristicWins int            `json:"heuristic_wins"`
	Draws         int            `json:"draws"`
	Stalls        int            `json:"stalls"`
	WinRate       float64        `json:"win_rate"`
	WilsonLo      float64        `json:"wilson_lo"`
	WilsonHi      float64        `json:"wilson_hi"`
	Decisions     int            `json:"decisions"`
	Searched      int            `json:"searched"`
	Continued     int            `json:"continued"`
	Fallbacks     int            `json:"fallbacks"`
	Refused       int            `json:"refused"`
	Reasons       map[string]int `json:"reasons"`
	CardRefusals  [][2]any       `json:"card_refusals"`
}

// report is t as the output line. Stalled games are outside the win rate's
// denominator (bench.Outcome.StallOn).
func (t *totals) report(o options) report {
	decided := t.shadowWins + t.heuristicWins + t.draws
	r := report{
		Pool: o.pool, Games: t.games, ShadowWins: t.shadowWins, HeuristicWins: t.heuristicWins,
		Draws: t.draws, Stalls: t.stalls, Decisions: t.stats.Decisions, Searched: t.stats.Searched,
		Continued: t.stats.Continued, Fallbacks: t.stats.Fallbacks, Refused: t.refused,
		Reasons: t.stats.Reasons, CardRefusals: topRefusals(t.stats.CardRefusals),
	}
	if r.Reasons == nil {
		r.Reasons = map[string]int{}
	}
	if decided > 0 {
		r.WinRate = float64(t.shadowWins) / float64(decided)
	}
	r.WilsonLo, r.WilsonHi = wilson(t.shadowWins, decided)
	return r
}

// wilson is the 95% Wilson score interval (z = 1.96) of w wins in n games,
// widened to hold the point estimate against rounding.
func wilson(w, n int) (lo, hi float64) {
	if n == 0 {
		return 0, 1
	}
	const z = 1.96
	nf := float64(n)
	p := float64(w) / nf
	den := 1 + z*z/nf
	centre := p + z*z/(2*nf)
	half := z * math.Sqrt(p*(1-p)/nf+z*z/(4*nf*nf))
	return math.Min((centre-half)/den, p), math.Max((centre+half)/den, p)
}

// topRefusals is the topCardRefusals card refusals with the most counts, by
// count descending then detail ascending. Never nil.
func topRefusals(m map[string]int) [][2]any {
	keys := sortedKeys(m)
	sort.SliceStable(keys, func(i, j int) bool {
		if m[keys[i]] != m[keys[j]] {
			return m[keys[i]] > m[keys[j]]
		}
		return keys[i] < keys[j]
	})
	if len(keys) > topCardRefusals {
		keys = keys[:topCardRefusals]
	}
	out := make([][2]any, 0, len(keys))
	for _, k := range keys {
		out = append(out, [2]any{k, m[k]})
	}
	return out
}
