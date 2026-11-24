package lockstep

import (
	"errors"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/eventdiff"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/replay"
	"github.com/adams-shaun/gorge/engine/rules"
)

// Direction says which side plays and which replays.
type Direction uint8

const (
	Forward Direction = iota + 1 // play on Forge definitions, replay on swapped
	Reverse                      // play on swapped, replay on Forge
)

func (d Direction) String() string {
	switch d {
	case Forward:
		return "forward"
	case Reverse:
		return "reverse"
	}
	return "unknown"
}

// Swap maps a Forge-resolved deck to the deck to compare against.
type Swap func([]*cards.Card) []*cards.Card

// Game is one seated game on Forge definitions.
type Game struct {
	ID  string       // "fdn/<line>" or "legacy/<seats>"
	Cfg rules.Config // Decks hold Forge cards
}

// Result is one game in one direction.
type Result struct {
	ID      string
	Dir     Direction
	Swapped int    // distinct cards whose pointer the swap changed
	Outcome string // "clean", "diverged", "rejected" or "unfinished"
	Seq     uint64 // the first differing event, when diverged
	Detail  string // the error text, when not clean
}

// Run plays g in direction dir with newBot answering, replays the log, and reports.
func Run(g Game, dir Direction, swap Swap, newBot func(seed uint64) rules.Answerer) Result {
	forge := g.Cfg
	swapped := g.Cfg
	swapped.Decks = make([][]*cards.Card, len(g.Cfg.Decks))
	changed := map[*cards.Card]bool{}
	for i, d := range g.Cfg.Decks {
		swapped.Decks[i] = swap(d)
		for j, c := range d {
			if j < len(swapped.Decks[i]) && swapped.Decks[i][j] != c {
				changed[c] = true
			}
		}
	}
	res := Result{ID: g.ID, Dir: dir, Swapped: len(changed)}
	play, other := forge, swapped
	if dir == Reverse {
		play, other = swapped, forge
	}
	e, _, err := rules.PlayAcceptance(play, newBot, nil)
	if err != nil {
		res.Outcome, res.Detail = "unfinished", err.Error()
		return res
	}
	if _, err := replayIgnoringNames(e.L, other); err != nil {
		var dv *replay.Divergence
		if errors.As(err, &dv) {
			res.Outcome, res.Seq, res.Detail = "diverged", dv.Seq, err.Error()
		} else {
			res.Outcome, res.Detail = "rejected", err.Error()
		}
		return res
	}
	res.Outcome = "clean"
	return res
}

// replayIgnoringNames replays l on cfg, treating two fields as display or naming only:
//   - the text of a mode_chosen marker, Forge's per-mode description label (or, with none, the mode's
//     SVar name), which cardc's scripts do not carry and which no decision or later event reads;
//   - the Counter of a granted or gained trigger push, and the trigger name in a delayed registration's
//     text: SVar names. Each side resolves a name in its own card's table, so a different name that
//     reached a different body would still show in the events that follow.
//
// A divergence that differs from the recording in such a field alone is taken as agreement (the
// recorded copy takes the replayed value) and the replay resumes; every other difference is still a
// divergence.
func replayIgnoringNames(l *events.Log, cfg rules.Config) (*rules.Engine, error) {
	log := &events.Log{Seed: l.Seed, Events: append([]events.Event(nil), l.Events...), Intents: l.Intents}
	for {
		e, err := replay.Replay(log, cfg)
		var dv *replay.Divergence
		if !errors.As(err, &dv) || dv.Missing || dv.Short || int(dv.Seq) >= len(log.Events) {
			return e, err
		}
		fixed, ok := namingOnly(dv.Want, dv.Got)
		if !ok {
			return e, err
		}
		log.Events[dv.Seq] = fixed
	}
}

// namingOnly reports whether want and got differ in a display or naming field alone, and returns want
// with got's value in that field.
func namingOnly(want, got events.Event) (events.Event, bool) {
	if want.Kind != got.Kind {
		return want, false
	}
	switch want.Kind {
	case events.ModeChosen:
		want.Text = got.Text
	case events.GrantTriggerPush, events.GainedTriggerPush:
		want.Counter = got.Counter
	case events.DelayedRegister:
		// "<mode>:<trigger SVar name>|<rest>": the name is the only part that may differ.
		if wm, wr, ok := splitRegister(want.Text); ok {
			if gm, gr, ok := splitRegister(got.Text); ok && wm == gm && wr == gr {
				want.Text = got.Text
			}
		}
	default:
		return want, false
	}
	return want, eventdiff.Same(want, got)
}

// splitRegister splits a delayed registration's text "<mode>:<name>|<rest>" into the mode and the rest.
func splitRegister(text string) (mode, rest string, ok bool) {
	head, rest, ok := strings.Cut(text, "|")
	if !ok {
		return "", "", false
	}
	mode, _, ok = strings.Cut(head, ":")
	return mode, rest, ok
}
