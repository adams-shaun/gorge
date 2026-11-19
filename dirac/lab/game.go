package lab

import (
	"fmt"

	"github.com/adams-shaun/gorge/dirac"
)

// Game is one game in progress: a dirac.State and the decision it is waiting on.
type Game struct {
	su    *Setup
	s     dirac.State
	d     dirac.Decision
	seed  uint64
	steps int
	owns  bool // Close also closes the Setup (NewGame's)
}

// NewGame opens cfg and deals the game of cfg.Seed. Close the game to release its table.
func NewGame(cfg Config) (*Game, error) {
	su, err := Open(cfg)
	if err != nil {
		return nil, err
	}
	g := su.Game(cfg.Seed)
	g.owns = true
	return g, nil
}

// Game deals the game of seed, advanced to its first decision.
func (su *Setup) Game(seed uint64) *Game {
	g := &Game{su: su, seed: seed}
	var decks [dirac.MaxSeats][]dirac.CardID
	copy(decks[:], su.ids)
	g.s.UseForkPool(dirac.NewForkPool())
	g.s.InitN(su.tbl, decks, uint8(len(su.ids)), seed, su.cfg)
	g.s.StepInto(&g.d)
	return g
}

// Close releases the Setup if NewGame made it; otherwise it does nothing.
func (g *Game) Close() {
	if g.owns {
		g.su.Close()
	}
}

// Seed is the seed the game was dealt from.
func (g *Game) Seed() uint64 { return g.seed }

// Seats is the number of seats.
func (g *Game) Seats() int { return len(g.su.ids) }

// State is the underlying dirac.State, for a policy or a probe that reads it.
// Mutating it other than through Apply voids the game's step count.
func (g *Game) State() *dirac.State { return &g.s }

// Decision is the pending decision, or nil when the game is Done. It is valid until the next Apply.
func (g *Game) Decision() *dirac.Decision {
	if g.d.Kind == dirac.DkNone {
		return nil
	}
	return &g.d
}

// Option is one legal action and what it does.
type Option struct {
	Index  int // position in Decision().Options
	Action dirac.Action
	Text   string
}

// Actions lists the pending decision's legal actions, in dirac's order. Nil when Done.
func (g *Game) Actions() []Option {
	d := g.Decision()
	if d == nil {
		return nil
	}
	out := make([]Option, d.N)
	for i := range out {
		out[i] = Option{i, d.Options[i], g.Describe(d.Options[i])}
	}
	return out
}

// Apply answers the pending decision with a, which must be one of its options.
func (g *Game) Apply(a dirac.Action) error {
	d := g.Decision()
	if d == nil {
		return fmt.Errorf("lab: game is over")
	}
	ok := false
	for _, o := range d.Options[:d.N] {
		ok = ok || o == a
	}
	if !ok {
		return fmt.Errorf("lab: action %#x is not one of the %d options", uint32(a), d.N)
	}
	g.apply(a)
	return nil
}

func (g *Game) apply(a dirac.Action) {
	g.s.Apply(a)
	g.steps++
	g.s.StepInto(&g.d)
}

// Done reports that no decision is pending: the game ended or stalled. Outcome says which.
func (g *Game) Done() bool { return g.d.Kind == dirac.DkNone }

// Outcome is how a game stands.
type Outcome uint8

const (
	// Running: a decision is pending.
	Running Outcome = iota
	// Won: the game ended with a winner.
	Won
	// Drawn: the game ended with no winner.
	Drawn
	// Stalled: dirac stopped at a capacity or a rule gap. The position has no result.
	Stalled
	// Truncated: Play ran out of steps. The position has no result.
	Truncated
	// Failed: a policy panicked or answered with an action not offered.
	Failed
)

var outcomeNames = [...]string{"running", "won", "drawn", "stalled", "truncated", "failed"}

func (o Outcome) String() string { return outcomeNames[o] }

// Outcome is Running while a decision is pending, else Won, Drawn or Stalled.
func (g *Game) Outcome() Outcome {
	switch {
	case !g.Done():
		return Running
	case g.s.Stalled():
		return Stalled
	}
	if over, w := g.s.Terminal(); over {
		if w < 0 {
			return Drawn
		}
		return Won
	}
	return Stalled // no decision and not over: nothing more can be played
}

// Winner is the winning seat, or -1 with no winner (still running, drawn, stalled).
func (g *Game) Winner() int {
	if g.Outcome() != Won {
		return -1
	}
	_, w := g.s.Terminal()
	return int(w)
}

// StallReason names why a stalled game stopped and its class (capacity or
// unmodelled), or "" when the game has not stalled.
func (g *Game) StallReason() (reason, class string) {
	if g.Outcome() != Stalled {
		return "", ""
	}
	why := g.s.StallReason()
	if why == dirac.StallNone {
		return "unfinished", "unmodelled"
	}
	if dirac.StallIsUnmodelled(why) {
		return dirac.StallName(why), "unmodelled"
	}
	return dirac.StallName(why), "capacity"
}

// Turn is the current turn number.
func (g *Game) Turn() int { return g.s.Turn() }

// Steps is the number of decisions answered so far.
func (g *Game) Steps() int { return g.steps }

// Fork is an independent copy of the game at this position: applying to either
// leaves the other as it was. It shares the Setup, so Close the original only
// after the forks are done.
func (g *Game) Fork() *Game {
	f := &Game{su: g.su, seed: g.seed, steps: g.steps, d: g.d}
	f.s.UseForkPool(dirac.NewForkPool())
	f.s.CopyFrom(&g.s)
	return f
}

// Hash identifies the position: every seat's view of it (the turn, life, mana,
// zones, stack and the objects each seat sees) and the pending decision. Equal
// seeds and answers give equal hashes; it does not read library order.
func (g *Game) Hash() uint64 {
	h := mix(0x6c6162, uint64(g.s.Turn()))
	for p := 0; p < g.Seats(); p++ {
		h = mix(h, g.s.ViewHash(uint8(p), nil))
	}
	h = mix(h, g.s.DecisionHash(&g.d))
	return mix(h, uint64(g.steps))
}

func mix(h, v uint64) uint64 {
	h ^= v + 0x9e3779b97f4a7c15 + h<<6 + h>>2
	h *= 0xbf58476d1ce4e5b9
	return h ^ h>>31
}
