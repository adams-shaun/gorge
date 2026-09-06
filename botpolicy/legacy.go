package botpolicy

import (
	"math/rand/v2"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
)

// LegacyDecide is the pre-B2 bot policy, frozen for one purpose: the
// botbench head-to-head that measures whether the combat heuristic is any
// better than the fuzz driver it replaced. It is the old Decide body
// verbatim — KAttackers attacks with every legal attacker, KBlockers takes
// roughly half the legal pairs on a per-option coin with one blocker per
// attacker, everything else unchanged — and it deliberately has no Board
// facts to read, matching what the policy was before B2.
//
// Ruling F7's one-copy rule governs the production policy (Decide), whose
// two adapter halves must answer the same; this is not a second production
// policy but the historical snapshot the benchmark compares against, so it
// carries no adapters of its own — the bench's legacy seat feeds it the
// plain IsMain board the old policy knew and nothing else. Anything that
// ships in the game (seats, acceptance, fuzz) calls Decide, never this.
func LegacyDecide(b Board, d *decision.Decision, r *rand.Rand) decision.Intent {
	in := decision.Intent{Seq: d.Seq, Player: d.Player}
	switch d.Kind {
	case decision.KPriority:
		if b.IsMain {
			for _, o := range d.Options {
				if o.Kind == "activate" {
					in.Choices = []int{o.Index}
					return clamp(d, in)
				}
			}
		}
		for _, want := range [...]string{"play_land", "cast"} {
			for _, o := range d.Options {
				if o.Kind == want {
					in.Choices = []int{o.Index}
					return clamp(d, in)
				}
			}
		}
		if b.IsMain {
			for _, o := range d.Options {
				if o.Kind == "ability" {
					in.Choices = []int{o.Index}
					return clamp(d, in)
				}
			}
		}
		for _, o := range d.Options {
			if o.Kind == "pass" {
				in.Choices = []int{o.Index}
				return clamp(d, in)
			}
		}

	case decision.KTarget:
		for _, o := range d.Options {
			if o.Kind == "player" && o.Player != d.Player {
				in.Choices = []int{o.Index}
				return clamp(d, in)
			}
		}
		if len(d.Options) > 0 {
			in.Choices = []int{d.Options[0].Index}
			return clamp(d, in)
		}

	case decision.KAttackers:
		ch := make([]int, 0, len(d.Options))
		for _, o := range d.Options {
			ch = append(ch, o.Index)
		}
		in.Choices = ch
		return clamp(d, in)

	case decision.KBlockers:
		var ch []int
		used := map[state.ObjID]bool{} // membership only -- never ranged.
		for _, o := range d.Options {
			if !used[o.Obj] && r.IntN(2) == 0 {
				used[o.Obj] = true
				ch = append(ch, o.Index)
			}
		}
		in.Choices = ch
		return clamp(d, in)

	case decision.KTriggerOrder:
		if n := len(d.Options); n > 0 {
			perm := make([]int, n)
			for i, o := range d.Options {
				perm[i] = o.Index
			}
			for i := n - 1; i > 0; i-- {
				j := r.IntN(i + 1)
				perm[i], perm[j] = perm[j], perm[i]
			}
			in.Choices = perm
			return clamp(d, in)
		}

	case decision.KTriggerOptional:
		if idx := r.IntN(2); idx < len(d.Options) {
			in.Choices = []int{d.Options[idx].Index}
			return clamp(d, in)
		}

	case decision.KChoose:
		if len(d.Options) == 0 {
			break
		}
		switch d.Options[0].Kind {
		case "x":
			in.Choices = []int{d.Options[len(d.Options)-1].Index}
		case "exile", "sacrifice":
			for i := 0; i < len(d.Options) && i < d.Max; i++ {
				in.Choices = append(in.Choices, d.Options[i].Index)
			}
		default:
			in.Choices = []int{d.Options[0].Index}
		}
		return clamp(d, in)

	case decision.KMulligan:
		if len(d.Options) > 0 && d.Options[0].Kind == "bottom" {
			for j := 0; j < len(d.Options) && j < d.Min; j++ {
				in.Choices = append(in.Choices, d.Options[j].Index)
			}
			return clamp(d, in)
		}
		if len(d.Options) > 1 {
			for _, o := range d.Options {
				if o.Kind == "mulligan" && r.IntN(3) == 0 {
					in.Choices = []int{o.Index}
					return clamp(d, in)
				}
			}
		}
		if len(d.Options) > 0 {
			in.Choices = []int{d.Options[0].Index}
			return clamp(d, in)
		}

	case decision.KModes:
		for j := 0; j < len(d.Options) && j < d.Min; j++ {
			in.Choices = append(in.Choices, d.Options[j].Index)
		}
		return clamp(d, in)
	}

	if d.Min == 0 {
		for _, o := range d.Options {
			if o.Kind == "pass" {
				in.Choices = []int{o.Index}
				return clamp(d, in)
			}
		}
	}
	return clamp(d, in)
}
