package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestEncoreGroupSacrificeIsSimultaneous: the end-step trigger that sacrifices
// a group of tokens (Encore, "sacrifice them" over several copies) sacrifices
// them together, so each Lightless Evangel token's "whenever you sacrifice
// another creature" sees the other one go (CR 603.10a): two triggers, one per
// token, not one only for the token sacrificed second.
func TestEncoreGroupSacrificeIsSimultaneous(t *testing.T) {
	set := cardsettest.Synth(t, "std-eoe")
	deck := []cardset.Entry{{Name: "Lightless Evangel", Count: 2}}
	if i, ok := set.Lookup("Lightless Evangel"); !ok || !set.Supported()[i] {
		t.Skip("Lightless Evangel not in the std-eoe table")
	}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	if int(v.Plains) >= len(tbl.Cards) {
		t.Skip("table too small for the scenario library")
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	a, b := sc.perm(id["Lightless Evangel"], 0), sc.perm(id["Lightless Evangel"], 0)
	s := sc.s
	if !s.subs[kop.TrigSacrificed].any() {
		t.Fatal("the Evangel does not listen for sacrifices: the test sees nothing")
	}
	// a's end-step trigger is on the stack; b joined it as a follower
	s.dly[0] = delayed{act: uint8(kop.OpSacrifice), ctrl: 0, obj: b, ver: s.objs[b].ver, when: dlyGrpFollow, creator: a}
	s.ndly = 1
	s.npend = 0
	s.resolveSacUntracked(sc.k, &stackEntry{ctrl: 0, obj: a, trigObj: NoObj})
	if s.objs[a].zone == kop.ZoneBattlefield || s.objs[b].zone == kop.ZoneBattlefield {
		t.Fatalf("both tokens must be sacrificed: zones %v %v", s.objs[a].zone, s.objs[b].zone)
	}
	var srcs [2]int
	for i := 0; i < int(s.npend); i++ {
		switch s.pend[i].obj {
		case a:
			srcs[0]++
		case b:
			srcs[1]++
		}
	}
	if srcs != [2]int{1, 1} || s.npend != 2 {
		t.Fatalf("pending Sacrificed triggers: a %d, b %d, total %d; want one each", srcs[0], srcs[1], s.npend)
	}
}
