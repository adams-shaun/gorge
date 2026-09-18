package searchprobe

import (
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"sort"
	"strings"

	"github.com/adams-shaun/gorge/botpolicy"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/rules"
	"github.com/adams-shaun/gorge/state"
	"github.com/adams-shaun/gorge/view"
)

func Candidates(d *ObservedDecision, baseline Action, limit int) []Action {
	if d == nil || d.Kind != decision.KPriority || limit < 2 || !candidateKind(baseline.Kind) {
		return nil
	}
	var pool []Action
	var pass Action
	hasPass, hasBaseline := false, false
	for _, o := range d.Options {
		if candidateKind(o.Action.Kind) {
			pool = append(pool, o.Action)
		}
		if o.Action.Kind == "pass" {
			pass = o.Action
			hasPass = true
		}
		if o.Action == baseline {
			hasBaseline = true
		}
	}
	if len(pool) < 2 || !hasPass || !hasBaseline {
		return nil
	}
	sort.SliceStable(pool, func(i, j int) bool {
		a, _ := json.Marshal(pool[i])
		b, _ := json.Marshal(pool[j])
		return string(a) < string(b)
	})
	out := []Action{baseline}
	if baseline != pass {
		out = append(out, pass)
	}
	for _, a := range pool {
		if len(out) >= limit {
			break
		}
		if a != baseline && a != pass {
			out = append(out, a)
		}
	}
	return out
}
func candidateKind(kind string) bool { return kind == "cast" || kind == "ability" || kind == "pass" }

// Frozen calibration scorer v1: terminal +/-100000 (draw 0); otherwise
// life + 2*hand-count + material. Land=3; other permanent=10; creatures add
// 2*(power+toughness). Only the projected public board/counts are read.
func LeafScore(v view.View, actor state.PlayerID) float64 {
	if v.Over {
		if v.Draw {
			return 0
		}
		if v.Winner != nil && *v.Winner == actor {
			return 100000
		}
		return -100000
	}
	value := 0.0
	for _, p := range v.Players {
		score := float64(p.Life) + 2*float64(p.HandSize)
		for _, c := range p.Battlefield {
			score += material(c)
		}
		if p.ID == actor {
			value += score
		} else {
			value -= score
		}
	}
	return value
}
func material(c view.CardView) float64 {
	if strings.Contains(c.Types, "Land") {
		return 3
	}
	score := 10.0
	if strings.Contains(c.Types, "Creature") {
		score += 2 * float64(c.Power+c.Toughness)
	}
	return score
}

// Frozen static challenger v1: pass=0, activated ability=1, cast=printed
// material score. No simulated outcomes or hidden state enter this ranking.
func StaticChoice(frame Frame, candidates []Action) (int, error) {
	if len(candidates) == 0 {
		return 0, fmt.Errorf("empty candidates")
	}
	var v view.View
	if err := json.Unmarshal(frame.Board, &v); err != nil {
		return 0, err
	}
	best, bestScore := 0, -1.0
	for i, a := range candidates {
		score := 0.0
		if a.Kind == "ability" {
			score = 1
		}
		if a.Kind == "cast" {
			for _, p := range v.Players {
				for _, zone := range [][]view.CardView{p.Hand, p.Graveyard, p.Exile, p.Command, p.Battlefield} {
					for _, c := range zone {
						if uint32(c.ID) == a.Obj {
							score = material(c)
						}
					}
				}
			}
		}
		if score > bestScore {
			best, bestScore = i, score
		}
	}
	return best, nil
}

type SearchResult struct {
	Index, Replays, Submits int
	Scores                  []float64
	Fallback                string
}

// SearchChoice never receives the actual engine. Every candidate uses the
// same sampled worlds and independent continuation seeds, through turn end.
func SearchChoice(worlds []World, candidates []Action, seed uint64, maxSubmits int) (SearchResult, error) {
	result := SearchResult{Scores: make([]float64, len(candidates))}
	if len(candidates) == 0 || maxSubmits < 1 {
		return result, fmt.Errorf("invalid search budget/candidates")
	}
	if len(worlds) == 0 {
		result.Fallback = "insufficient sampled worlds/ESS"
		return result, nil
	}
	for wi, w := range worlds {
		if err := VerifyWorld(w); err != nil {
			return result, err
		}
		result.Replays++
		for i, a := range candidates {
			e := w.Engine.Clone()
			in, err := w.Observer.Match(e.Pending(), []Action{a})
			if err != nil {
				return result, fmt.Errorf("root action mismatch: %w", err)
			}
			actor, turn := e.Pending().Player, e.G.Turn
			if err := e.SubmitHypothetical(in); err != nil {
				return result, err
			}
			submits := 1
			rngs := BotRandoms(seed^uint64(wi+1), len(e.G.Players))
			board := botpolicy.NewBoard(len(rngs))
			for !e.G.Over && e.G.Turn == turn && submits < maxSubmits {
				d := e.Pending()
				if d == nil {
					return result, fmt.Errorf("rollout has no pending decision")
				}
				in := botpolicy.Decide(botpolicy.BoardFromGameInto(e.G, e, d.Player, &board), d, rngs[d.Player])
				if err := e.SubmitHypothetical(in); err != nil {
					return result, err
				}
				submits++
			}
			result.Submits += submits
			if !e.G.Over && e.G.Turn == turn {
				result.Fallback = "turn-end submit budget exhausted"
				return result, nil
			}
			result.Scores[i] += LeafScore(view.Project(e.G, e, actor, e.Pending()), actor) / float64(len(worlds))
		}
	}
	for i := 1; i < len(result.Scores); i++ {
		if result.Scores[i] > result.Scores[result.Index] {
			result.Index = i
		}
	}
	return result, nil
}

func VerifyWorld(w World) error {
	e, err := rules.NewHypothetical(w.Config, w.Engine.ChanceTranscript())
	if err != nil {
		return err
	}
	if err := e.AdvanceHypothetical(); err != nil {
		return err
	}
	for _, in := range w.Engine.L.Intents {
		if err := e.SubmitHypothetical(in); err != nil {
			return err
		}
	}
	if e.L.Head() != w.Engine.L.Head() || e.RNGDraws() != w.Engine.RNGDraws() || len(e.L.Events) != len(w.Engine.L.Events) {
		return fmt.Errorf("sampled-world replay diverged")
	}
	return nil
}

func BotRandoms(seed uint64, seats int) []*rand.Rand {
	out := make([]*rand.Rand, seats)
	for i := range out {
		s := seed ^ uint64(i+1)
		out[i] = rand.New(rand.NewPCG(s, s^0x9e3779b97f4a7c15))
	}
	return out
}
