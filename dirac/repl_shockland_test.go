package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestShockElectionBeforeEffectTap: a shock land an effect puts onto the
// battlefield tapped is untapped while its election is asked, as in gorge, whose
// Tapped$ Tap follows the suspended move, and ends tapped on either answer.
func TestShockElectionBeforeEffectTap(t *testing.T) {
	for _, mode := range []uint8{0, 1} {
		set := cardsettest.Set(t, "shock")
		deck := []cardset.Entry{{Name: "Blood Crypt", Count: 1}, {Name: "Mountain", Count: 8}}
		if i, ok := set.Lookup("Blood Crypt"); !ok || !set.Supported()[i] {
			t.Skip("Blood Crypt is not in the shock table")
		}
		tbl, da, _, err := set.Matchup(deck, deck)
		if err != nil {
			t.Fatal(err)
		}
		var pool, mountain CardID
		for _, c := range da {
			switch tbl.Cards[c].Name {
			case "Blood Crypt":
				pool = c
			case "Mountain":
				mountain = c
			}
		}
		var lib [20]CardID
		for i := range lib {
			lib[i] = mountain
		}
		lib[0] = pool
		s := new(State)
		s.Init(tbl, lib[:], lib[:], 1, Config{NoElide: true})
		k := s.k()
		var o Obj = NoObj
		for i := range s.objs {
			if s.objs[i].card == pool && s.objs[i].owner() == 0 {
				o = Obj(i)
				break
			}
		}
		if o == NoObj {
			t.Fatal("no Blood Crypt object")
		}
		from := s.detach(o)
		sl := s.enter(k, o, 0, true, from)
		if s.g.nask != 1 || s.g.ask[0].kind != akPayLife {
			t.Fatalf("mode %d: %d asks queued, want the election", mode, s.g.nask)
		}
		if s.perms[sl].flags&pfTapped != 0 {
			t.Errorf("mode %d: tapped while the election is asked", mode)
		}
		s.answerQueuedAsk(k, MakeAction(AkChoose, 0, 0, mode))
		if s.perms[sl].flags&pfTapped == 0 {
			t.Errorf("mode %d: not tapped after the answer", mode)
		}
	}
}
