package azmcts

import (
	"github.com/adams-shaun/gorge/bots/searchprobe"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// The principal line (Options.Line): below the chosen root child, the
// most-visited child at each node, for up to Line plies. It is read off the
// tree after the search, so it never moves a choice, a visit or a value.
//
// One tree node is shared by every world that reaches it by the same keys,
// so a child's visits are summed across the sampled worlds, and a node whose
// worlds disagree about the next decision splits its visits among keys: the
// step's Share says how much of the node's visits the line's key holds.
//
// A step is kept only if the player it is shown to could name its action.
// A candidate that names an object the root observation did not show (a card
// drawn in the line, or in an opponent's re-dealt hand) is keyed by an
// observer-local reference that means a different card in each world, and an
// opponent's choice is public only for attackers and blockers. The line
// stops before the first such step, because everything after it depends on
// it.

// Why a principal line ended before Options.Line plies.
const (
	// LineEndHidden: the next step would name an object the player cannot see.
	LineEndHidden = "hidden"
	// LineEndUnexplored: the search never expanded the next decision.
	LineEndUnexplored = "unexplored"
)

// LineStep is one ply of the principal line.
type LineStep struct {
	// Opp marks an opponent's decision (Options.OpponentNodes).
	Opp bool
	// Label is CandidateLabel of the step's candidate, as the world that
	// first reached it offered it.
	Label string
	// Visits is the step's visit count and Share its fraction of the visits
	// its node's children hold.
	Visits int
	Share  float64
	// Q is the step's mean backed-up value, the searching seat's win
	// probability in [0,1].
	Q float64
}

// principalLine follows the most-visited child (ties to the lower index)
// from edge e for at most n steps.
func principalLine(e *edge, n int) ([]LineStep, string) {
	var out []LineStep
	for nd := e.next; len(out) < n; {
		if nd == nil {
			return out, LineEndUnexplored
		}
		var best *edge
		total := 0
		for _, k := range nd.kids {
			total += k.n
			if k.n > 0 && (best == nil || k.n > best.n) {
				best = k
			}
		}
		switch {
		case best == nil:
			return out, LineEndUnexplored
		case best.hidden:
			return out, LineEndHidden
		}
		out = append(out, LineStep{Opp: best.opp, Label: best.label, Visits: best.n, Share: float64(best.n) / float64(total), Q: best.w / float64(best.n)})
		nd = best.next
	}
	return out, ""
}

// lineInfos describes cands for the principal line, nil unless Options.Line
// is on. opp marks the candidates of an opponent's decision.
func (e *engineEnv) lineInfos(obs *searchprobe.Collector, d *decision.Decision, cands []cand, opp bool) []lineInfo {
	if e.cfg.line <= 0 {
		return nil
	}
	out := make([]lineInfo, len(cands))
	for i, c := range cands {
		hidden := false
		if opp {
			hidden = d.Kind != decision.KAttackers && d.Kind != decision.KBlockers
		} else {
			hidden = candLate(obs, d, c, e.cfg.rootRefs)
		}
		out[i] = lineInfo{key: c.key, label: CandidateLabel(d, c.in), hidden: hidden}
	}
	return out
}

// candLate reports whether c names an object past the first rootRefs the
// root observation introduced: nameKeys's test of the same objects.
func candLate(obs *searchprobe.Collector, d *decision.Decision, c cand, rootRefs int) bool {
	in := c.keyIntent()
	acts := c.acts
	if acts == nil && in.Payment == nil {
		a, err := obs.Actions(d, in)
		if err != nil {
			return true
		}
		acts = a
	}
	late := func(ref uint32, id state.ObjID) bool {
		if _, player := id.PlayerRef(); player || id == 0 {
			return false
		}
		return int(ref) > rootRefs
	}
	for k := range acts {
		src, obj, atk := d.Source, state.ObjID(0), state.ObjID(0)
		switch {
		case in.Payment != nil:
			for _, a := range d.PaymentActions {
				if a.ID == in.Payment.ActionID {
					obj = a.Cast.Object
				}
			}
		case k < len(in.Choices) && in.Choices[k] >= 0 && in.Choices[k] < len(d.Options):
			o := d.Options[in.Choices[k]]
			obj, atk = o.Obj, o.Attacker
		}
		if late(acts[k].Source, src) || late(acts[k].Obj, obj) || late(acts[k].Attacker, atk) {
			return true
		}
	}
	return false
}
