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
	Format string      `json:"format"` // "koracle-repro-2"
	Decks  [2]string   `json:"decks"`
	Cards  [2][]string `json:"cards"` // each deck's card names in decklist order
	// MoreDecks and MoreCards are a game of more than two seats' further
	// seats, as Decks and Cards; a two-seat repro has neither.
	MoreDecks   []string   `json:"more_decks,omitempty"`
	MoreCards   [][]string `json:"more_cards,omitempty"`
	Seed        uint64     `json:"seed"`
	Mulligans   int        `json:"mulligans"`
	PassElision bool       `json:"pass_elision"`
	MaxTurns    int32      `json:"max_turns,omitempty"`
	// Commander and Commanders are Matchup.Commander and Commanders: a repro
	// written without them is a Constructed game.
	Commander  bool  `json:"commander,omitempty"`
	Commanders []int `json:"commanders,omitempty"`
	// Partners is Matchup.Partners, absent when neither seat has one.
	Partners []int `json:"partners,omitempty"`
	// Surface is the option surface both sides offered ("autopay" or "full"):
	// the same actions on the other surface are another game. Kernel is the side
	// the game was played against, as kfuzz names it ("synth std-trk", "set
	// blb", "vanilla", "gorge"). Repros written before either was recorded
	// leave it empty.
	Surface string `json:"surface,omitempty"`
	Kernel  string `json:"kernel,omitempty"`
	// Pool is the synthetic pool the game's decks came from ("std-trk",
	// "custom", "standard-meta", "one-off"): the tests glob testdata/repro
	// and replay every file whose pool is theirs. Why is the one-line reason
	// the game is pinned, as the repro tables used to carry it. Stall marks a
	// replay that ends at a declared kernel stall.
	Pool  string `json:"pool,omitempty"`
	Why   string `json:"why,omitempty"`
	Stall bool   `json:"stall,omitempty"`
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

const (
	reproFormat = "koracle-repro-2"
	// reproFormat1 is the format before the target byte was repacked
	// (RefStack 0xE0, RefPlayer 0xF0, MaxInst 224): ReadRepro maps its
	// actions to the current bytes.
	reproFormat1 = "koracle-repro-1"
)

// format1Ref maps a Ref byte of a format-1 recording to the current Refs:
// stack ability i (0xE0+i) is RefStack+i and seat p (0xF0+p) RefPlayer+p, in
// whichever size class reads it (0xEC and 0xFC in the default one). A format-1 recording never names an
// instance at or above 224, and RefNone (0xFF) did not move, so every other
// byte is unchanged.
func format1Ref(r Ref) Ref {
	switch {
	case r >= 0xE0 && r < 0xF0:
		return RefStack + (r - 0xE0)
	case r == 0xF0 || r == 0xF1:
		return RefPlayer + (r - 0xF0)
	}
	return r
}

// UnmarshalJSON decodes a repro and maps a format-1 one's actions to the
// current Refs (format1Ref), so every reader of a recorded repro (ReadRepro,
// the pinned-game files) sees the same bytes. The file keeps its format-1
// bytes; the decoded repro says koracle-repro-2.
func (r *Repro) UnmarshalJSON(b []byte) error {
	type plain Repro // no methods: no recursion into this one
	var p plain
	if err := json.Unmarshal(b, &p); err != nil {
		return err
	}
	if p.Format == reproFormat1 {
		p.Actions = format1Actions(p.Actions)
		p.Format = reproFormat
	}
	*r = Repro(p)
	return nil
}

// format1Actions returns as with each action's source and target mapped by
// format1Ref (kind and mode are not Refs).
func format1Actions(as []Canon) []Canon {
	out := make([]Canon, len(as))
	for i, a := range as {
		out[i] = Pack(a.Kind(), format1Ref(a.Src()), format1Ref(a.Tgt()), a.Mode())
	}
	return out
}

// NewRepro records res (a lockstep game of m on seed) as a repro.
func NewRepro(m *Matchup, seed uint64, rc RunConfig, res *Result) *Repro {
	r := &Repro{Format: reproFormat, Decks: m.Names, Seed: seed, Mulligans: m.Mulligans,
		PassElision: rc.PassElision, MaxTurns: rc.MaxTurns, Surface: SurfaceName(rc.Config), Divergence: res.Div}
	r.Commander, r.Commanders, r.Partners = m.commanderFields()
	for s := 0; s < 2; s++ {
		for _, c := range m.Decks[s] {
			r.Cards[s] = append(r.Cards[s], cardName(c))
		}
	}
	for s := 2; s < m.NSeat(); s++ {
		r.MoreDecks = append(r.MoreDecks, m.SeatName(s))
		var names []string
		for _, c := range m.SeatDeck(s) {
			names = append(names, cardName(c))
		}
		r.MoreCards = append(r.MoreCards, names)
	}
	r.setActions(res.Actions)
	return r
}

func (r *Repro) setActions(as []Canon) {
	r.Actions = append([]Canon(nil), as...)
	r.Readable = actionStrings(r.Actions)
}

// SurfaceName is the option surface cfg offers: "autopay" or "full".
func SurfaceName(cfg Config) string {
	if cfg.GorgeAutopaySurface {
		return "autopay"
	}
	return "full"
}

// RunConfig is the repro's run knobs, its surface among them.
func (r *Repro) RunConfig() RunConfig {
	return RunConfig{Config: Config{PassElision: r.PassElision, GorgeAutopaySurface: r.Surface == "autopay"}, MaxTurns: r.MaxTurns}
}

// Matchup rebuilds the repro's matchup from its card names (Lookup against
// the corpus, the same resolution deck files get).
func (r *Repro) Matchup(reg *cards.Registry) (*Matchup, error) {
	if len(r.MoreCards) != len(r.MoreDecks) {
		return nil, fmt.Errorf("repro: %d further decks, %d further card lists", len(r.MoreDecks), len(r.MoreCards))
	}
	names := append([]string{r.Decks[0], r.Decks[1]}, r.MoreDecks...)
	lists := append([][]string{r.Cards[0], r.Cards[1]}, r.MoreCards...)
	decks := make([][]*cards.Card, len(lists))
	for s, l := range lists {
		for _, n := range l {
			c, ok := reg.Lookup(n)
			if !ok {
				return nil, fmt.Errorf("repro: card %q not in the corpus", n)
			}
			decks[s] = append(decks[s], c)
		}
	}
	m, err := NewMatchupN(reg, names, decks, r.Mulligans)
	if err != nil {
		return nil, err
	}
	m.setCommander(r.Commander, r.Commanders, r.Partners)
	return m, nil
}

// commanderFields is m's commanders as a repro or game record writes them
// (nil Commanders for a Constructed matchup, nil partners when neither seat
// has one, so a one-commander record reads as it did).
func (m *Matchup) commanderFields() (bool, []int, []int) {
	if !m.Commander {
		return false, nil, nil
	}
	var partners []int
	if m.Partners[0] >= 0 || m.Partners[1] >= 0 {
		partners = []int{m.Partners[0], m.Partners[1]}
	}
	return true, []int{m.Commanders[0], m.Commanders[1]}, partners
}

// setCommander restores what commanderFields recorded.
func (m *Matchup) setCommander(on bool, ix, partners []int) {
	m.Commander, m.Commanders, m.Partners = on, [2]int{-1, -1}, [2]int{-1, -1}
	for s := 0; s < 2 && s < len(ix); s++ {
		m.Commanders[s] = ix[s]
	}
	for s := 0; s < 2 && s < len(partners); s++ {
		m.Partners[s] = partners[s]
	}
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
