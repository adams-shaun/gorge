package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestMutateCopyMerge is the kernel mirror of gorge's foldMutate with a spell
// copy as the mutating SOURCE (CR 730.2d, 608.3f, 707.10f): the token status
// travels with the cards. A pile a copy tops is a token, a copy that goes
// under parks as a token component, and a real card demoted by it stays a
// card that the cease pass leaves where the split put it.
func TestMutateCopyMerge(t *testing.T) {
	for _, tc := range []struct {
		name            string
		targetTok, top  bool
		pileTok, cmpTok bool
	}{
		{name: "copy on top of a token", targetTok: true, top: true, pileTok: true, cmpTok: true},
		{name: "copy on top of a card", top: true, pileTok: true},
		{name: "copy under a card", cmpTok: true},
		{name: "copy under a token", targetTok: true, pileTok: true, cmpTok: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tbl, mutator, bird, _, _ := mutateTable()
			sc := newScenTable(t, tbl, Config{NoElide: true})
			var surv Obj
			if tc.targetTok {
				surv = sc.s.newToken(bird, 0)
				sc.s.enter(sc.k, surv, 0, false, kop.ZoneNone)
				sc.s.npend = 0
			} else {
				surv = sc.perm(bird, 0)
			}
			if !sc.onBF(surv) || (sc.s.objs[surv].of&ofToken != 0) != tc.targetTok {
				t.Fatalf("precondition: target %d of %#x, battlefield %v", surv, sc.s.objs[surv].of, sc.onBF(surv))
			}
			// the mutating spell copy's object (copy_gen.go mintSpellCopy:
			// ofCopy alone)
			src := sc.s.newToken(mutator, 0)
			sc.s.objs[src].of = ofCopy
			sc.s.objs[src].zone = kop.ZoneNone
			if !sc.s.mergeFold(sc.k, surv, src, tc.top, 1) {
				t.Fatal("precondition: mergeFold refused the merge")
			}
			rec := sc.s.mergeRecOf(surv)
			if rec == nil || rec.n != 1 || Obj(rec.und[0]-1) != src {
				t.Fatalf("precondition: the pile did not form (%+v)", rec)
			}
			held := bird // the card the component object holds
			if !tc.top {
				held = mutator
			}
			if c := sc.s.objs[src].card; c != held {
				t.Fatalf("precondition: component holds card %d, want %d", c, held)
			}
			want := func(tok bool) uint8 {
				if tok {
					return ofToken
				}
				return 0
			}
			if got := sc.s.objs[surv].of & (ofToken | ofCopy); got != want(tc.pileTok) {
				t.Fatalf("pile token/copy bits %#x, want %#x (CR 730.2d)", got, want(tc.pileTok))
			}
			if got := sc.s.objs[src].of & (ofToken | ofCopy); got != want(tc.cmpTok) {
				t.Fatalf("component token/copy bits %#x, want %#x", got, want(tc.cmpTok))
			}
			sc.s.leave(sc.k, sc.slot(surv), kop.ZoneGraveyard, false)
			sc.s.ceaseTokensGen()
			for _, p := range []struct {
				o    Obj
				tok  bool
				what string
			}{{surv, tc.pileTok, "pile"}, {src, tc.cmpTok, "component"}} {
				z := kop.ZoneGraveyard
				if p.tok {
					z = kop.ZoneNone
				}
				if got := sc.s.objs[p.o].zone; got != z {
					t.Fatalf("%s %d is in zone %d after the pile died, want %d", p.what, p.o, got, z)
				}
			}
		})
	}
}
