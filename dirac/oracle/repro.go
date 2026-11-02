package oracle

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/rules"
)

// Repro is a replayable divergence: the matchup by card names, the seed, the
// run's knobs and the canonical action prefix that reached the divergence.
// It is the JSON a fuzz run writes and `kfuzz -repro` replays.
type Repro struct {
	Format      string      `json:"format"` // "koracle-repro-1"
	Decks       [2]string   `json:"decks"`
	Cards       [2][]string `json:"cards"` // each deck's card names in decklist order
	Seed        uint64      `json:"seed"`
	Mulligans   int         `json:"mulligans"`
	PassElision bool        `json:"pass_elision"`
	MaxTurns    int32       `json:"max_turns,omitempty"`
	// Actions is the canonical prefix; Readable is the same, rendered.
	Actions  []Canon  `json:"actions"`
	Readable []string `json:"readable,omitempty"`
	// Chance, when set, forces the gorge side's random outcomes
	// (rules.NewHypothetical): a kernel game recorded with its own draws.
	Chance []rules.ChanceDraw `json:"chance,omitempty"`
	// Divergence is what the run found (informational).
	Divergence *Divergence `json:"divergence,omitempty"`
	// Shrunk records the shrink pass: attempts made and the prefix length
	// before it.
	Shrunk *ShrinkInfo `json:"shrunk,omitempty"`
}

// ShrinkInfo is a repro's shrink record.
type ShrinkInfo struct {
	Attempts int `json:"attempts"`
	From     int `json:"from"`
}

const reproFormat = "koracle-repro-1"

// NewRepro records res (a lockstep game of m on seed) as a repro.
func NewRepro(m *Matchup, seed uint64, rc RunConfig, res *Result) *Repro {
	r := &Repro{Format: reproFormat, Decks: m.Names, Seed: seed, Mulligans: m.Mulligans,
		PassElision: rc.PassElision, MaxTurns: rc.MaxTurns, Divergence: res.Div}
	for s := 0; s < 2; s++ {
		for _, c := range m.Decks[s] {
			r.Cards[s] = append(r.Cards[s], cardName(c))
		}
	}
	r.setActions(res.Actions)
	return r
}

func (r *Repro) setActions(as []Canon) {
	r.Actions = append([]Canon(nil), as...)
	r.Readable = actionStrings(r.Actions)
}

// RunConfig is the repro's run knobs.
func (r *Repro) RunConfig() RunConfig {
	return RunConfig{Config: Config{PassElision: r.PassElision}, MaxTurns: r.MaxTurns}
}

// Matchup rebuilds the repro's matchup from its card names (Lookup against
// the corpus, the same resolution deck files get).
func (r *Repro) Matchup(reg *cards.Registry) (*Matchup, error) {
	var decks [2][]*cards.Card
	for s := 0; s < 2; s++ {
		for _, n := range r.Cards[s] {
			c, ok := reg.Lookup(n)
			if !ok {
				return nil, fmt.Errorf("repro: card %q not in the corpus", n)
			}
			decks[s] = append(decks[s], c)
		}
	}
	return NewMatchup(reg, r.Decks, decks, r.Mulligans)
}

// Write saves the repro as indented JSON.
func (r *Repro) Write(path string) error {
	b, err := json.MarshalIndent(r, "", " ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

// ReadRepro loads a repro.
func ReadRepro(path string) (*Repro, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var r Repro
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, err
	}
	if r.Format != reproFormat {
		return nil, fmt.Errorf("repro %s: format %q, want %q", path, r.Format, reproFormat)
	}
	return &r, nil
}

// ScriptPolicy serves actions in order, skipping any the decision does not
// offer, then plays tail (nil: FirstPolicy). Skipped counts the skips.
type ScriptPolicy struct {
	Actions []Canon
	Tail    Policy
	next    int
	Skipped int
}

func (s *ScriptPolicy) Pick(c *PickCtx) Canon {
	for s.next < len(s.Actions) {
		a := s.Actions[s.next]
		s.next++
		for _, o := range c.D.Options {
			if o == a {
				return a
			}
		}
		s.Skipped++
	}
	if s.Tail == nil {
		s.Tail = FirstPolicy()
	}
	return s.Tail.Pick(c)
}

// Exhausted reports whether every scripted action has been served or skipped.
func (s *ScriptPolicy) Exhausted() bool { return s.next >= len(s.Actions) }

// Rerun replays the repro in lockstep against mk: the scripted prefix, then
// FirstPolicy for at most tail more decisions.
func (r *Repro) Rerun(m *Matchup, mk SideFactory, tail int) Result {
	k, err := mk(m, r.Seed)
	if err != nil {
		return Result{Div: &Divergence{What: "kernel", Field: "setup", Detail: err.Error()}}
	}
	rc := r.RunConfig()
	rc.MaxDecisions = len(r.Actions) + tail
	return Lockstep(m, r.Seed, k, &ScriptPolicy{Actions: r.Actions}, rc)
}

// Shrink drops prefix actions while the repro still diverges in the same
// class (Divergence.fieldClass), trying chunks of halving size, at most
// attempts reruns. The shrunk prefix is the actions the last diverging rerun
// actually applied.
func Shrink(c *Corpus, r *Repro, mk SideFactory, attempts int) *Repro {
	if r.Divergence == nil {
		return r
	}
	m, err := r.Matchup(c.Reg)
	if err != nil {
		return r
	}
	class := r.Divergence.fieldClass()
	const tail = 200
	cur := append([]Canon(nil), r.Actions...)
	best := *r
	tries := 0
	try := func(cand []Canon) bool {
		tries++
		probe := *r
		probe.Actions = cand
		res := probe.Rerun(m, mk, tail)
		if res.Div == nil || res.Div.fieldClass() != class {
			return false
		}
		best.Divergence = res.Div
		best.setActions(res.Actions)
		return true
	}
	for chunk := len(cur) / 2; chunk >= 1 && tries < attempts; chunk /= 2 {
		for i := 0; i+chunk <= len(cur) && tries < attempts; {
			cand := append(append([]Canon(nil), cur[:i]...), cur[i+chunk:]...)
			if try(cand) {
				cur = best.Actions
			} else {
				i += chunk
			}
		}
	}
	best.Shrunk = &ShrinkInfo{Attempts: tries, From: len(r.Actions)}
	return &best
}
