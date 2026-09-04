package rules

import (
	"math/rand/v2"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
)

// testBot is a line-for-line mirror of seat.Bot's policy (seat/bot.go's
// botDecide), duplicated here rather than imported: rules/fuzz_test.go is
// package rules, and importing seat -- which imports view -- runs the
// declared dependency order (cards -> state -> decision -> events ->
// effects -> rules -> view -> seat -> replay -> cmd/*) backwards into the
// package under test (Ruling F7). Task 26's acceptance harness drives the
// real seat.Bot instead; this copy exists solely so the rules package's own
// fuzz gate needs no import of anything above it. Keep the two in step: a
// policy change here without the matching change in seat/bot.go (or vice
// versa) is a bug.
type testBot struct {
	r *rand.Rand
}

func newTestBot(seed uint64) *testBot {
	return &testBot{r: rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15))}
}

// answer mirrors seat.Bot's botDecide exactly -- see that function's doc for
// the rationale of each case (Ruling P7): tap for mana, land, cast, then
// pass; attack with everything; target an opponent over yourself; block
// about half the time, never with the same blocker twice; order or
// accept/decline triggers with the bot's own rng; and, for anything else (or
// anything above that found nothing to pick), the first Min distinct
// indices, or pass when Min == 0. Every access into d.Options is guarded
// against the list being empty, and the intent this returns must always be
// one Decision.Validate accepts.
func (b *testBot) answer(d *decision.Decision) decision.Intent {
	in := decision.Intent{Seq: d.Seq, Player: d.Player}
	switch d.Kind {
	case decision.KPriority:
		for _, want := range [...]string{"activate", "play_land", "cast"} {
			for _, o := range d.Options {
				if o.Kind == want {
					in.Choices = []int{o.Index}
					return in
				}
			}
		}

	case decision.KTarget:
		for _, o := range d.Options {
			if o.Kind == "player" && o.Player != d.Player {
				in.Choices = []int{o.Index}
				return in
			}
		}
		if len(d.Options) > 0 {
			in.Choices = []int{d.Options[0].Index}
			return in
		}

	case decision.KAttackers:
		ch := make([]int, 0, len(d.Options))
		for _, o := range d.Options {
			ch = append(ch, o.Index)
		}
		in.Choices = ch
		return in

	case decision.KBlockers:
		var ch []int
		used := map[state.ObjID]bool{} // membership only -- never ranged.
		for _, o := range d.Options {
			if !used[o.Obj] && b.r.IntN(2) == 0 {
				used[o.Obj] = true
				ch = append(ch, o.Index)
			}
		}
		in.Choices = ch
		return in

	case decision.KTriggerOrder:
		if n := len(d.Options); n > 0 {
			perm := make([]int, n)
			for i := range perm {
				perm[i] = i
			}
			for i := n - 1; i > 0; i-- {
				j := b.r.IntN(i + 1)
				perm[i], perm[j] = perm[j], perm[i]
			}
			in.Choices = perm
			return in
		}

	case decision.KTriggerOptional:
		if idx := b.r.IntN(2); idx < len(d.Options) {
			in.Choices = []int{d.Options[idx].Index}
			return in
		}
	}

	// Last resort (Ruling P7): pass if Min == 0 and one is offered, else the
	// first Min distinct indices.
	if d.Min == 0 {
		for _, o := range d.Options {
			if o.Kind == "pass" {
				in.Choices = []int{o.Index}
				return in
			}
		}
		return in
	}
	n := d.Min
	if n > len(d.Options) {
		n = len(d.Options)
	}
	ch := make([]int, 0, n)
	for i := 0; i < n; i++ {
		ch = append(ch, d.Options[i].Index)
	}
	in.Choices = ch
	return in
}
