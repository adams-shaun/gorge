package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/vanilla"
)

// playActions expands a self-play action log (elision on, so the single-option
// decisions are absent) to one action per decision: replayed with every
// decision posed, the expansion reaches self-play's final position, and it
// holds exactly the decisions the elision answered in place on top of
// the logged ones.
func TestPlayActionsExpandsElided(t *testing.T) {
	t.Parallel()

	for g := 0; g < 20; g++ {
		decks := [2][]kop.CardID{vanilla.Decks[g%4], vanilla.Decks[(g/4)%4]}
		play := dirac.Config{Mulligans: uint8(g % 3)}
		s := dirac.NewConfig(vanilla.Table, decks[0], decks[1], uint64(g), play)
		pol := dirac.NewRandomPolicy(uint64(g)*7919 + 1)
		var acts []dirac.Action
		var d dirac.Decision
		for len(acts) < 20000 {
			s.StepInto(&d)
			if d.Kind == dirac.DkNone {
				break
			}
			a := pol.Pick(&d)
			acts = append(acts, a)
			s.Apply(a)
		}
		full, subs, err := playActions(vanilla.Table, decks, uint64(g), play, acts)
		if err != nil {
			t.Fatalf("game %d: %v", g, err)
		}
		if len(subs) != len(full) {
			t.Fatalf("game %d: %d kinds for %d actions", g, len(subs), len(full))
		}
		if want := len(acts) + int(s.Skipped[0]+s.Skipped[1]); len(full) != want {
			t.Fatalf("game %d: %d expanded actions, want %d logged + %d elided", g, len(full), len(acts), want-len(acts))
		}
		posed := play
		posed.NoElide = true
		r := dirac.NewConfig(vanilla.Table, decks[0], decks[1], uint64(g), posed)
		for _, a := range full {
			r.StepInto(&d)
			r.Apply(a)
		}
		r.StepInto(&d)
		var pa, pb dirac.Projection
		s.Project(&pa)
		r.Project(&pb)
		if d.Kind != dirac.DkNone || pa != pb {
			t.Fatalf("game %d: the expansion ends elsewhere (decision kind %d)", g, d.Kind)
		}
	}
}
