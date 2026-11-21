package shadow_test

import (
	"errors"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/shadow"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

// departedInput is a four-seat observation of seat 0's main phase in which seat 3
// has left the game: it shows nothing, and the caller holds its twenty cards as
// Gone.
func departedInput() *shadow.Input {
	in := reasonsInput()
	for p := 2; p < 4; p++ {
		pv := view.PlayerView{ID: state.PlayerID(p), Life: 20, HandSize: 7, LibrarySize: 13}
		if p == 3 {
			pv = view.PlayerView{ID: 3, Life: 0, Lost: true}
			for i := 0; i < 20; i++ {
				in.Hidden.Gone[3] = append(in.Hidden.Gone[3], shadow.Card{Obj: dirac.Obj(60 + i), Name: "Plains"})
			}
		} else {
			for i := 0; i < 7; i++ {
				in.Hidden.Hand[p] = append(in.Hidden.Hand[p], shadow.Card{Obj: dirac.Obj(p*20 + i), Name: "Plains"})
			}
			for i := 0; i < 13; i++ {
				in.Hidden.Library[p] = append(in.Hidden.Library[p], shadow.Card{Obj: dirac.Obj(p*20 + 7 + i), Name: "Savannah Lions"})
			}
		}
		in.View.Players = append(in.View.Players, pv)
	}
	return in
}

// TestWorldFromViewDeparted: a seat that has left stages with its cards ceased,
// and each way the view or the position contradicts that is refused with its
// reason.
func TestWorldFromViewDeparted(t *testing.T) {
	t.Parallel()
	in := departedInput()
	w, err := shadow.WorldFromView(in)
	if err != nil {
		t.Fatalf("the departed seat does not stage: %v", err)
	}
	if !w.Lost[3] || w.Lost[0] || w.Lost[1] || w.Lost[2] {
		t.Fatalf("lost marks %v, want seat 3 only", w.Lost)
	}
	if _, err := dirac.NewFromWorld(&w); err != nil {
		t.Fatalf("the departed world does not stage: %v", err)
	}
	cases := map[string]struct {
		reason string
		mut    func(*shadow.Input)
	}{
		"shows a library size": {shadow.ReasonSeatLeft, func(in *shadow.Input) { in.View.Players[3].LibrarySize = 1 }},
		"shows a graveyard":    {shadow.ReasonSeatLeft, func(in *shadow.Input) { in.View.Players[3].Graveyard = []view.CardView{{ID: 60, Name: "Plains"}} }},
		"active seat left":     {dirac.ReasonSeats, func(in *shadow.Input) { in.View.Active, in.View.Priority = 3, 3 }},
		"priority holder left": {dirac.ReasonSeats, func(in *shadow.Input) { in.View.Priority = 3 }},
		"short deck":           {dirac.ReasonLayout, func(in *shadow.Input) { in.Hidden.Gone[3] = in.Hidden.Gone[3][:3] }},
	}
	for name, c := range cases {
		in := departedInput()
		c.mut(in)
		_, err := shadow.StageFromView(in)
		var u dirac.ErrUnstageable
		if !errors.As(err, &u) || u.Reason != c.reason {
			t.Errorf("%s: got %v, want reason %q", name, err, c.reason)
		}
	}
}
