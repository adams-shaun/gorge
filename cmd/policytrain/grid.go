package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/internal/policynet"
)

// The pn12 grid surface: which feature set and action encoding a run trains
// under, a nested game-subset filter for the data axis, and an evaluation on
// a separate held-out corpus (a disjoint seed block) with the override-only
// readout. Every flag defaults to today's behaviour exactly: with none set,
// the corpus loads through policynet.Load and nothing below runs.

// gridFlags is the parsed pn12 flag set.
type gridFlags struct {
	features     string
	actions      string
	maxGameIndex int
	evalCorpus   string
	evalJSON     string
	arm          string
}

// active reports whether any pn12 flag moves the load away from Load.
func (g gridFlags) loadOptions() (policynet.LoadOptions, bool, error) {
	var lo policynet.LoadOptions
	fs, err := policynet.ParseFeatureSet(g.features)
	if err != nil {
		return lo, false, err
	}
	lo.Features = fs
	switch g.actions {
	case "", "split":
	case "joint":
		lo.Joint = true
	default:
		return lo, false, fmt.Errorf("unknown -actions %q (want split or joint)", g.actions)
	}
	if g.maxGameIndex > 0 {
		k := g.maxGameIndex
		lo.Keep = func(_ string, gameIndex int) bool { return gameIndex < k }
	}
	return lo, fs != policynet.FeaturesV1 || lo.Joint || lo.Keep != nil, nil
}

// loadCorpus loads one corpus path under the grid options: policynet.Load
// when no grid option is set (today's path, byte for byte), LoadWith
// otherwise.
func loadCorpus(path string, lo policynet.LoadOptions, grid bool) ([]policynet.Example, policynet.Stats, error) {
	if !grid {
		return policynet.Load(path)
	}
	return policynet.LoadWith(path, lo)
}

// GridKind is one decision kind's held-out readout. Every top-1 is at the
// CARD level: under the joint encoding the argmax option's card
// (Example.JointCard) is what is compared with the teacher's choice, so the
// split and joint arms are scored on the same question.
type GridKind struct {
	Kind          string  `json:"kind"`
	N             int     `json:"n"`
	BotTop1       float64 `json:"bot_top1"` // the teacher kept the bot
	ModelTop1     float64 `json:"model_top1"`
	OverrideN     int     `json:"override_n"`
	OverrideTop1  float64 `json:"override_top1"`
	KeepN         int     `json:"keep_n"`
	KeepTop1      float64 `json:"keep_top1"`
	ModelPicksBot float64 `json:"model_picks_bot"`
	// Subset kinds (attackers): exact set match of the admitted set with the
	// teacher's chosen set, under the sign rule (score > 0) and the seat's
	// default auto rule (sign when the scores straddle 0, else the mean;
	// an exact tie keeps the bot's set).
	SetN            int     `json:"set_n,omitempty"`
	SetOverrideN    int     `json:"set_override_n,omitempty"`
	SetSign         float64 `json:"set_sign,omitempty"`
	SetSignOverride float64 `json:"set_sign_override,omitempty"`
	SetSignKeep     float64 `json:"set_sign_keep,omitempty"`
	SetAuto         float64 `json:"set_auto,omitempty"`
	SetAutoOverride float64 `json:"set_auto_override,omitempty"`
	SetAutoKeep     float64 `json:"set_auto_keep,omitempty"`
}

// GridReport is the -eval-json document.
type GridReport struct {
	Arm           string     `json:"arm"`
	Features      string     `json:"features"`
	Actions       string     `json:"actions"`
	MaxGameIndex  int        `json:"max_game_index"`
	TrainExamples int        `json:"train_examples"`
	TrainGames    int        `json:"train_games"`
	TrainOverride int        `json:"train_overrides"`
	ResidualInit  float64    `json:"residual_init"`
	Epochs        int        `json:"epochs"`
	EvalExamples  int        `json:"eval_examples"`
	Kinds         []GridKind `json:"kinds"`
}

// countGames counts distinct (pair, seed, game index) games and override
// examples.
func countGames(exs []policynet.Example) (games, overrides int) {
	seen := map[gameKey]bool{}
	for i := range exs {
		k := gameKey{exs[i].Pair, exs[i].Seed, exs[i].GameIndex}
		if !seen[k] {
			seen[k] = true
			games++
		}
		if exs[i].Override() {
			overrides++
		}
	}
	return games, overrides
}

func cardOf(ex *policynet.Example, k int) int {
	if ex.JointCard != nil {
		return ex.JointCard[k]
	}
	return k
}

// evalGrid scores every held-out example. Kinds are reported in sorted
// order; no map iteration reaches the output.
func evalGrid(m *policynet.Model, exs []policynet.Example) []GridKind {
	type acc struct {
		n, bot, model, ovr, ovrHit, keepHit, picksBot int
		subset                                        bool
		setN, setOvrN                                 int
		signHit, signOvr, signKeep                    int
		autoHit, autoOvr, autoKeep                    int
	}
	by := map[decision.Kind]*acc{}
	get := func(k decision.Kind) *acc {
		a := by[k]
		if a == nil {
			a = &acc{}
			by[k] = a
		}
		return a
	}
	for i := range exs {
		ex := &exs[i]
		scores := m.Score(ex.State, ex.Options)
		// Card-level sets.
		prefCard := map[int]bool{}
		botCard := map[int]bool{}
		best := -1
		for k := range ex.Options {
			o := ex.Options[k]
			if o.Target.Preferred {
				prefCard[cardOf(ex, k)] = true
			}
			if o.BotPick {
				botCard[cardOf(ex, k)] = true
			}
			if o.Target.Labelled && (best < 0 || scores[k] > scores[best]) {
				best = k
			}
		}
		if best < 0 {
			continue
		}
		ovr := ex.Override()
		if ex.Kind == decision.KAttackers || ex.Kind == decision.KBlockers {
			// The subset readout covers every labelled decision, the teacher's
			// empty declaration ("attack with nothing") included.
			a := get(ex.Kind)
			a.subset = true
			a.setN++
			if ovr {
				a.setOvrN++
			}
			sign := setMatch(ex, scores, func(s []float32) (float32, bool) { return 0, true })
			auto := setMatch(ex, scores, autoThreshold)
			tally := func(ok bool, all, o, kp *int) {
				if !ok {
					return
				}
				*all++
				if ovr {
					*o++
				} else {
					*kp++
				}
			}
			tally(sign, &a.signHit, &a.signOvr, &a.signKeep)
			tally(auto, &a.autoHit, &a.autoOvr, &a.autoKeep)
		}
		if len(prefCard) == 0 {
			continue
		}
		a := get(ex.Kind)
		a.n++
		if !ovr {
			a.bot++
		}
		hit := prefCard[cardOf(ex, best)]
		if hit {
			a.model++
		}
		if botCard[cardOf(ex, best)] {
			a.picksBot++
		}
		if ovr {
			a.ovr++
			if hit {
				a.ovrHit++
			}
		} else if hit {
			a.keepHit++
		}
	}
	kinds := make([]decision.Kind, 0, len(by))
	for k := range by {
		kinds = append(kinds, k)
	}
	sort.Slice(kinds, func(i, j int) bool { return kinds[i] < kinds[j] })
	var out []GridKind
	for _, k := range kinds {
		a := by[k]
		g := GridKind{Kind: string(k), N: a.n, BotTop1: ratio(a.bot, a.n), ModelTop1: ratio(a.model, a.n),
			OverrideN: a.ovr, OverrideTop1: ratio(a.ovrHit, a.ovr), KeepN: a.n - a.ovr, KeepTop1: ratio(a.keepHit, a.n-a.ovr),
			ModelPicksBot: ratio(a.picksBot, a.n)}
		if a.subset {
			g.SetN, g.SetOverrideN = a.setN, a.setOvrN
			g.SetSign, g.SetSignOverride, g.SetSignKeep = ratio(a.signHit, a.setN), ratio(a.signOvr, a.setOvrN), ratio(a.signKeep, a.setN-a.setOvrN)
			g.SetAuto, g.SetAutoOverride, g.SetAutoKeep = ratio(a.autoHit, a.setN), ratio(a.autoOvr, a.setOvrN), ratio(a.autoKeep, a.setN-a.setOvrN)
		}
		out = append(out, g)
	}
	return out
}

// autoThreshold mirrors the seat's default admission reference: 0 when the
// scores straddle it, else the decision's mean; ok false on an exact tie
// (the seat then keeps the bot's declaration).
func autoThreshold(s []float32) (float32, bool) {
	pos, neg := 0, 0
	var sum float64
	for _, v := range s {
		if v > 0 {
			pos++
		} else if v < 0 {
			neg++
		}
		sum += float64(v)
	}
	if pos > 0 && neg > 0 {
		return 0, true
	}
	allEq := true
	for _, v := range s[1:] {
		if v != s[0] {
			allEq = false
			break
		}
	}
	if allEq {
		return 0, false
	}
	return float32(sum / float64(len(s))), true
}

// setMatch reports whether the admitted option set equals the teacher's
// chosen set (a refused decision admits the bot's own set).
func setMatch(ex *policynet.Example, scores []float32, ref func([]float32) (float32, bool)) bool {
	t, ok := ref(scores)
	for k := range ex.Options {
		in := ex.Options[k].BotPick
		if ok {
			in = scores[k] > t
		}
		if in != ex.Options[k].Target.Preferred {
			return false
		}
	}
	return true
}

// writeGridReport prints the table and writes the JSON document.
func writeGridReport(w io.Writer, path string, rep GridReport) error {
	fmt.Fprintf(w, "pn12 eval (%s): %d held-out examples; top-1 at the card level\n", rep.Arm, rep.EvalExamples)
	fmt.Fprintf(w, "  %-10s %7s %7s %7s %6s %7s %7s %7s | %6s %6s %7s %7s %7s %7s\n", "kind", "n", "bot", "model", "ovr-n", "ovr-t1", "keep-t1", "m-bot",
		"set-n", "ovr-n", "sgn", "sgn-ovr", "auto", "aut-ovr")
	for _, k := range rep.Kinds {
		fmt.Fprintf(w, "  %-10s %7d %7.3f %7.3f %6d %7.3f %7.3f %7.3f | %6d %6d %7.3f %7.3f %7.3f %7.3f\n", k.Kind, k.N, k.BotTop1, k.ModelTop1,
			k.OverrideN, k.OverrideTop1, k.KeepTop1, k.ModelPicksBot, k.SetN, k.SetOverrideN, k.SetSign, k.SetSignOverride, k.SetAuto, k.SetAutoOverride)
	}
	if path == "" {
		return nil
	}
	b, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}
