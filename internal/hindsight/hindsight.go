// Package hindsight contains the deterministic mechanics for hindsight branch
// measurement. It does not own a clock or files; cmd/hindsight supplies both.
package hindsight

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"

	"github.com/adams-shaun/gorge/botpolicy"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/internal/searchprobe"
	"github.com/adams-shaun/gorge/rules"
)

// Candidate is one complete semantic answer. Choices and Rest use the
// deciding seat's observer-local identities, so they can be mapped into every
// sampled world without exposing that world's raw object ids.
type Candidate struct {
	Choices []searchprobe.Action
	Rest    []searchprobe.Action
	Summary string
}

// Candidates puts the bot's recorded answer first, then deterministic legal
// alternatives. Combat uses searchprobe's shared whole-declaration builders;
// all other kinds use single-option substitutions, order changes and the
// shared bot clamp. The returned capHit says legal alternatives remained when
// the output reached limit.
func Candidates(c *searchprobe.Collector, e *rules.Engine, d *decision.Decision, bot decision.Intent, limit int) (out []Candidate, capHit bool, err error) {
	if c == nil || e == nil || d == nil || limit < 2 {
		return nil, false, fmt.Errorf("invalid candidate input")
	}
	var intents []decision.Intent
	switch d.Kind {
	case decision.KAttackers:
		intents = searchprobe.AttackCandidates(d, bot, limit+1)
	case decision.KBlockers:
		b := botpolicy.BoardFromGame(e.G, e, d.Player)
		legal := func(choices []int) []int { return botpolicy.LegalBlockChoices(b, d, choices) }
		intents = searchprobe.BlockCandidates(d, bot, limit+1, legal)
	case decision.KPriority:
		intents = priorityCandidates(d, bot, limit+1)
	default:
		intents = genericCandidates(d, bot, limit+1)
	}
	if len(intents) > limit {
		intents = intents[:limit]
		capHit = true
	}
	for _, in := range intents {
		choices, rest, xerr := c.IntentActions(d, in)
		if xerr != nil {
			return nil, false, xerr
		}
		out = append(out, Candidate{Choices: choices, Rest: rest, Summary: summarizeIntent(d, in)})
	}
	return out, capHit, nil
}

func priorityCandidates(d *decision.Decision, bot decision.Intent, limit int) []decision.Intent {
	if len(bot.Choices) != 1 || bot.Choices[0] < 0 || bot.Choices[0] >= len(d.Options) {
		return nil
	}
	allowed := func(kind string) bool { return kind == "cast" || kind == "ability" || kind == "pass" }
	if !allowed(d.Options[bot.Choices[0]].Kind) {
		return []decision.Intent{bot}
	}
	out := []decision.Intent{bot}
	seen := map[int]bool{bot.Choices[0]: true}
	// Keep pass second, matching searchprobe.Candidates' common baseline arm.
	for _, o := range d.Options {
		if o.Kind == "pass" && !seen[o.Index] {
			out = append(out, decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{o.Index}})
			seen[o.Index] = true
			break
		}
	}
	for _, o := range d.Options {
		if len(out) >= limit {
			break
		}
		if allowed(o.Kind) && !seen[o.Index] {
			out = append(out, decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{o.Index}})
			seen[o.Index] = true
		}
	}
	return out
}

func genericCandidates(d *decision.Decision, bot decision.Intent, limit int) []decision.Intent {
	var out []decision.Intent
	seen := make(map[string]bool)
	add := func(in decision.Intent) {
		if len(out) >= limit {
			return
		}
		in.Seq, in.Player = d.Seq, d.Player
		in = botpolicy.Clamp(d, in)
		if err := d.Validate(in); err != nil {
			return
		}
		keyBytes, _ := json.Marshal(struct {
			Choices []int
			Rest    []int
		}{in.Choices, in.Rest})
		key := string(keyBytes)
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, in)
	}
	add(bot)

	// Scalar asks and priority windows: every offered option is an arm.
	for _, o := range d.Options {
		add(decision.Intent{Choices: []int{o.Index}})
	}
	// Minimum and maximum offered-order sets exercise optional and multi-pick
	// asks. Clamp derives requirements, group caps and cumulative budgets from
	// the same helpers as the production bot.
	add(decision.Intent{})
	var all []int
	for _, o := range d.Options {
		all = append(all, o.Index)
	}
	add(decision.Intent{Choices: all})

	// Preserve the bot's set while testing order-sensitive asks.
	rev := append([]int(nil), bot.Choices...)
	reverse(rev)
	rest := append([]int(nil), bot.Rest...)
	reverse(rest)
	add(decision.Intent{Choices: rev, Rest: rest})
	for i := 0; i+1 < len(bot.Choices); i++ {
		next := append([]int(nil), bot.Choices...)
		next[i], next[i+1] = next[i+1], next[i]
		add(decision.Intent{Choices: next})
	}

	// One edit away from the recorded answer covers multi-target, choose and
	// modes decisions without enumerating an exponential power set.
	for pos := range bot.Choices {
		for _, o := range d.Options {
			next := append([]int(nil), bot.Choices...)
			next[pos] = o.Index
			add(decision.Intent{Choices: next})
		}
	}
	return out
}

func reverse(xs []int) {
	for i, j := 0, len(xs)-1; i < j; i, j = i+1, j-1 {
		xs[i], xs[j] = xs[j], xs[i]
	}
}

func summarizeIntent(d *decision.Decision, in decision.Intent) string {
	if len(in.Choices) == 0 {
		return "choose none"
	}
	parts := make([]string, 0, len(in.Choices))
	for _, i := range in.Choices {
		if i < 0 || i >= len(d.Options) {
			continue
		}
		o := d.Options[i]
		label := o.Label
		if label == "" {
			label = o.Kind
		}
		parts = append(parts, label)
	}
	b, _ := json.Marshal(parts)
	return string(b)
}

// Backward returns [n-1..0], the order a finished game's decisions are mined.
func Backward(n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = n - 1 - i
	}
	return out
}

// Wilson95 is the Wilson score interval for a binomial rate.
func Wilson95(wins, n int) (lo, hi float64) {
	if n <= 0 {
		return 0, 1
	}
	const z = 1.959963984540054
	p := float64(wins) / float64(n)
	z2 := z * z
	den := 1 + z2/float64(n)
	center := (p + z2/(2*float64(n))) / den
	half := z * math.Sqrt((p*(1-p)+z2/(4*float64(n)))/float64(n)) / den
	return math.Max(0, center-half), math.Min(1, center+half)
}

// PairedDiff95 is a normal interval over paired {-1,0,+1} win differences.
// positive means the alternative won while the recorded choice did not;
// negative is the converse. It is the interval used by ClearMargin.
func PairedDiff95(positive, negative, n int) (mean, lo, hi float64) {
	if n <= 0 {
		return 0, -1, 1
	}
	mean = float64(positive-negative) / float64(n)
	if n == 1 {
		return mean, -1, 1
	}
	sumSq := float64(positive + negative)
	variance := (sumSq - float64(n)*mean*mean) / float64(n-1)
	if variance < 0 {
		variance = 0
	}
	half := 1.959963984540054 * math.Sqrt(variance/float64(n))
	return mean, math.Max(-1, mean-half), math.Min(1, mean+half)
}

type OptionResult struct {
	Index               int     `json:"index"`
	Summary             string  `json:"summary"`
	Wins                int     `json:"wins"`
	Rollouts            int     `json:"rollouts"`
	Rate                float64 `json:"rate"`
	WilsonLow           float64 `json:"wilson_low"`
	WilsonHigh          float64 `json:"wilson_high"`
	PairedWins          int     `json:"paired_wins_vs_chosen,omitempty"`
	PairedLosses        int     `json:"paired_losses_vs_chosen,omitempty"`
	PairedDifference    float64 `json:"paired_difference,omitempty"`
	PairedDifferenceLow float64 `json:"paired_difference_low,omitempty"`
	PairedDifferenceHi  float64 `json:"paired_difference_high,omitempty"`
}

type Evaluation struct {
	Options         []OptionResult `json:"options"`
	BestAlternative int            `json:"best_alternative"`
	Delta           float64        `json:"delta"`
	Clear           bool           `json:"clear"`
	RolloutsUsed    int            `json:"rollouts_used"`
	SamplerStatus   string         `json:"sampler_status"`
	Attempts        int            `json:"sampler_attempts,omitempty"`
	Accepted        int            `json:"sampler_accepted,omitempty"`
	ESS             float64        `json:"sampler_ess,omitempty"`
	// Redealt counts the worlds the sampler's redeal fallback supplied
	// (searchprobe.SampleOptions.Redeal) and RedealRefused keeps the first
	// reason a starved block could not be redealt. Both stay empty when the
	// fallback is off.
	Redealt       int    `json:"sampler_redealt,omitempty"`
	RedealRefused string `json:"sampler_redeal_refused,omitempty"`
}

type EvalOptions struct {
	Block, MaxRollouts, MaxSubmits int
	Parallelism                    int
	Clairvoyant                    bool
}

// WorldSource returns one fresh common-random-number block. Honest callers
// invoke searchprobe.Sample with a new deterministic seed; the leaked arm
// returns repeated clones of the actual branch state.
type WorldSource func(block, worlds int) ([]searchprobe.World, searchprobe.SampleResult, error)

// Evaluate adaptively rolls all candidates in blocks while the top two Wilson
// intervals overlap. Candidate 0 is always the recorded bot answer.
func Evaluate(candidates []Candidate, source WorldSource, seed uint64, opts EvalOptions) (Evaluation, error) {
	out := Evaluation{BestAlternative: -1, SamplerStatus: "ok"}
	if len(candidates) < 2 || opts.Block < 1 || opts.MaxRollouts < opts.Block || opts.MaxSubmits < 1 {
		return out, fmt.Errorf("invalid hindsight evaluation")
	}
	wins := make([]int, len(candidates))
	rollouts := make([]int, len(candidates))
	pairedWins := make([]int, len(candidates))
	pairedLosses := make([]int, len(candidates))
	semantic := make([]searchprobe.SemanticIntent, len(candidates))
	for i, c := range candidates {
		semantic[i] = searchprobe.SemanticIntent{Choices: c.Choices, Rest: c.Rest}
	}
	for block, used := 0, 0; used < opts.MaxRollouts; block++ {
		n := min(opts.Block, opts.MaxRollouts-used)
		worlds, sr, err := source(block, n)
		out.Attempts += sr.Attempts
		out.Accepted += sr.Accepted
		out.ESS = sr.ESS
		out.Redealt += sr.Redealt
		if out.RedealRefused == "" {
			out.RedealRefused = sr.RedealRefused
		}
		if err != nil {
			out.SamplerStatus = "no_world"
			return out, nil
		}
		if len(worlds) != n {
			out.SamplerStatus = "no_world"
			return out, nil
		}
		res, err := searchprobe.TeacherIntentChoice(worlds, semantic, searchprobe.TeacherOptions{
			Seed: seed ^ mix64(uint64(block)+0x726f6c6c6f7574), MaxSubmits: opts.MaxSubmits, Parallelism: opts.Parallelism, Clairvoyant: opts.Clairvoyant,
		})
		if err != nil {
			return out, err
		}
		if res.Capped > 0 || res.Terminal != res.Rollouts {
			out.SamplerStatus = "rollout_not_terminal"
			return out, nil
		}
		for i := range candidates {
			wins[i] += res.WinsByCandidate[i]
			rollouts[i] += n
			pairedWins[i] += res.WinsOverBaseline[i]
			pairedLosses[i] += res.LossesToBaseline[i]
		}
		used += n
		out.RolloutsUsed = used
		if !topTwoOverlap(wins, rollouts) {
			break
		}
	}
	out.Options = make([]OptionResult, len(candidates))
	for i := range candidates {
		lo, hi := Wilson95(wins[i], rollouts[i])
		mean, dlo, dhi := PairedDiff95(pairedWins[i], pairedLosses[i], rollouts[i])
		out.Options[i] = OptionResult{Index: i, Summary: candidates[i].Summary, Wins: wins[i], Rollouts: rollouts[i], Rate: float64(wins[i]) / float64(rollouts[i]), WilsonLow: lo, WilsonHigh: hi,
			PairedWins: pairedWins[i], PairedLosses: pairedLosses[i], PairedDifference: mean, PairedDifferenceLow: dlo, PairedDifferenceHi: dhi}
	}
	best := 1
	for i := 2; i < len(out.Options); i++ {
		if out.Options[i].Rate > out.Options[best].Rate {
			best = i
		}
	}
	out.BestAlternative = best
	out.Delta, out.Clear = ClearMargin(out.Options[0], out.Options[best])
	return out, nil
}

// ClearMargin applies pn20's label rule to a chosen and best-alternative
// result: at least ten percentage points and a paired 95% lower bound above
// zero.
func ClearMargin(chosen, best OptionResult) (delta float64, clear bool) {
	delta = best.Rate - chosen.Rate
	return delta, delta >= 0.10 && best.PairedDifferenceLow > 0
}

func topTwoOverlap(wins, n []int) bool {
	indices := make([]int, len(wins))
	for i := range indices {
		indices[i] = i
	}
	sort.SliceStable(indices, func(i, j int) bool {
		a, b := indices[i], indices[j]
		return float64(wins[a])/float64(n[a]) > float64(wins[b])/float64(n[b])
	})
	aLo, aHi := Wilson95(wins[indices[0]], n[indices[0]])
	bLo, bHi := Wilson95(wins[indices[1]], n[indices[1]])
	return aLo <= bHi && bLo <= aHi
}

// Seed derives independent deterministic streams without depending on worker
// order. Every component is included before SplitMix64's avalanche.
func Seed(base, game, decision, block uint64) uint64 {
	x := base ^ mix64(game+0x67616d65) ^ mix64(decision+0x6465636973696f6e) ^ mix64(block+0x626c6f636b)
	return mix64(x)
}

func mix64(x uint64) uint64 {
	x += 0x9e3779b97f4a7c15
	x = (x ^ (x >> 30)) * 0xbf58476d1ce4e5b9
	x = (x ^ (x >> 27)) * 0x94d049bb133111eb
	return x ^ (x >> 31)
}
