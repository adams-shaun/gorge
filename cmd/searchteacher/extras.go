package main

import (
	"math/rand/v2"

	"github.com/adams-shaun/gorge/botpolicy"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/internal/policynet"
	"github.com/adams-shaun/gorge/rules"
	"github.com/adams-shaun/gorge/state"
	"github.com/adams-shaun/gorge/view"
)

// labelSchemaExtras is the schema a -label-extras record carries: schema 2
// plus the "extras" object (policynet.LabelExtras, ticket pn12). Without
// -label-extras the writer emits schema 2 byte for byte as before.
const labelSchemaExtras = 3

// libraryPeek is how many cards of each library the diagnostic extras
// record (the oracle's future draws).
const libraryPeek = 3

// labelExtras reads the pn12 extension at a labelled decision, off the REAL
// engine: the opponent's hand and both libraries' next cards (hidden
// information, flagged diagnostic in the corpus and never read by a
// checkpointable feature set), and for a priority decision every cast or
// activate option's follow-up target decision (on a clone; the game engine
// is never touched).
func labelExtras(e *rules.Engine, d *decision.Decision) *policynet.LabelExtras {
	x := &policynet.LabelExtras{DiagOppHand: []view.CardView{}, DiagOppLibraryTop: []string{}, DiagOwnLibraryTop: []string{}}
	for i := range e.G.Players {
		p := e.G.Players[i].ID
		if p == d.Player {
			x.DiagOwnLibraryTop = libraryTop(e.G, p)
			continue
		}
		ov := view.Project(e.G, e, p, nil)
		for j := range ov.Players {
			if ov.Players[j].ID == p {
				x.DiagOppHand = append(x.DiagOppHand, ov.Players[j].Hand...)
			}
		}
		x.DiagOppLibraryTop = append(x.DiagOppLibraryTop, libraryTop(e.G, p)...)
	}
	if d.Kind != decision.KPriority {
		return x
	}
	for i, o := range d.Options {
		if o.Kind != "cast" && o.Kind != "activate" {
			continue
		}
		if ft := followTarget(e, d, i); ft != nil {
			x.FollowTargets = append(x.FollowTargets, *ft)
		}
	}
	return x
}

// libraryTop names the first libraryPeek cards of p's library, top first.
func libraryTop(g *state.Game, p state.PlayerID) []string {
	out := []string{}
	for _, id := range g.Zone(state.ZLibrary, p) {
		if len(out) == libraryPeek {
			break
		}
		name := "?"
		if o := g.Obj(id); o != nil && o.Card != nil && len(o.Card.Faces) > 0 {
			name = o.Card.Faces[0].Name
		}
		out = append(out, name)
	}
	return out
}

// followTarget submits option i on a clone and walks the deciding seat's
// own follow-up decisions (answered by the default bot on a fixed,
// decision-derived rng) until a target decision appears, which it returns
// with the bot's answer. Any other outcome -- another seat to act, a
// priority window, an error, a panic, game over -- is "no target" (nil).
func followTarget(e *rules.Engine, d *decision.Decision, i int) (ft *policynet.FollowTarget) {
	defer func() {
		if r := recover(); r != nil {
			ft = nil
		}
	}()
	c := e.Clone()
	if err := c.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{i}}); err != nil {
		return nil
	}
	rng := rand.New(rand.NewPCG(d.Seq, 0x7a7267^uint64(i)))
	for step := 0; step < 8 && !c.G.Over; step++ {
		p := c.Pending()
		if p == nil || p.Player != d.Player || p.Kind == decision.KPriority {
			return nil
		}
		in := botpolicy.Decide(botpolicy.BoardFromGame(c.G, c, p.Player), p, rng)
		if p.Kind == decision.KTarget {
			return &policynet.FollowTarget{Option: i, Targets: append([]decision.Option(nil), p.Options...), Min: p.Min, Max: p.Max, Choices: append([]int(nil), in.Choices...)}
		}
		if err := c.Submit(in); err != nil {
			return nil
		}
	}
	return nil
}

// targetTracker records, for the last labelled priority decision, the
// target decision its answer went on to take in the actual game.
type targetTracker struct {
	label  int // index into rec.Labels, -1 when idle
	option int // the option the labelled decision played
}

// afterLabel arms the tracker when the decision just answered appended a
// label for a priority decision played as one option.
func (t *targetTracker) afterLabel(rec *GameRecord, before int, d *decision.Decision, in decision.Intent) {
	if len(rec.Labels) == before {
		return // no new label: an armed tracker stays armed
	}
	t.label = -1
	if d.Kind != decision.KPriority || len(in.Choices) != 1 || rec.Labels[len(rec.Labels)-1].Extras == nil {
		return
	}
	t.label, t.option = len(rec.Labels)-1, in.Choices[0]
}

// observe records the in-game target answer, or disarms at the next priority
// decision (the cast resolved into no target).
func (t *targetTracker) observe(rec *GameRecord, d *decision.Decision, in decision.Intent, actor state.PlayerID) {
	if t.label < 0 {
		return
	}
	if d.Kind == decision.KPriority || d.Player != actor {
		t.label = -1
		return
	}
	if d.Kind != decision.KTarget {
		return
	}
	rec.Labels[t.label].Extras.ChosenTarget = &policynet.FollowTarget{Option: t.option,
		Targets: append([]decision.Option(nil), d.Options...), Min: d.Min, Max: d.Max, Choices: append([]int(nil), in.Choices...)}
	t.label = -1
}
