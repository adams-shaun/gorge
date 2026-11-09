package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// Table.Emblems indexes this test builds by hand: 1-based, as State.g.emb
// stores them.
const (
	embDurEOT = iota + 1
	embDurNextTurn
	embDurEndNextTurn
	embDurWhileHostNoEOT
	embDurAsLongAsControl
)

// embDurScen is a vanilla table with one hand-built emblem per duration flag
// under test.
func embDurScen(t *testing.T) *scen {
	t.Helper()
	tbl := *v.Table
	tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	tbl.Exprs = make([]kop.Expr, 1) // a generated table: prepareGen keeps g.gen
	tbl.Emblems = append([]kop.Emblem{}, v.Table.Emblems...)
	tbl.Emblems = append(tbl.Emblems,
		kop.Emblem{UntilEOT: true},
		kop.Emblem{UntilNextTurn: true},
		kop.Emblem{UntilEndOfNextTurn: true},
		kop.Emblem{WhileHostNoEOT: true},
		kop.Emblem{AsLongAsControl: true},
	)
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	if !sc.k.gen.anyNTEndEmb || !sc.k.gen.anyWhileHost || !sc.k.gen.anyEmbCtrl {
		t.Fatal("prep: the duration flags did not reach genPrep")
	}
	return sc
}

// embDurPut puts Table.Emblems indexes into player p's slots, all hosted by
// host (NoObj for none), and clears the turn-start bits.
func embDurPut(sc *scen, p uint8, host Obj, idxs ...uint8) {
	for i, idx := range idxs {
		sc.s.g.emb[p][i] = idx
		sc.s.g.embC[p][i] = 0
		sc.s.g.embO[p][i] = host
		sc.s.g.embN[p][i] = 0
		sc.s.g.embT[p][i] = 1
	}
	sc.s.g.nemb[p] = uint8(len(idxs))
	sc.s.g.embF[p] = 0
}

// TestEmblemDurations: each emblem duration flag ends at the same boundary as
// gorge's Effect registration.
func TestEmblemDurations(t *testing.T) {
	t.Run("UntilEOT", func(t *testing.T) {
		sc := embDurScen(t)
		embDurPut(sc, 0, NoObj, embDurEOT)
		if sc.s.g.nemb[0] != 1 {
			t.Fatal("precondition: the emblem is not in player 0's slot")
		}
		sc.s.cleanupGen(sc.k)
		if sc.s.g.nemb[0] != 0 {
			t.Fatalf("%d emblems after the cleanup, want 0", sc.s.g.nemb[0])
		}
	})

	t.Run("UntilNextTurn", func(t *testing.T) {
		sc := embDurScen(t)
		embDurPut(sc, 0, NoObj, embDurNextTurn)
		sc.s.active = 0
		sc.s.cleanupGen(sc.k) // its controller's own cleanup is not the boundary
		if sc.s.g.nemb[0] != 1 {
			t.Fatalf("%d emblems after its controller's cleanup, want 1", sc.s.g.nemb[0])
		}
		sc.s.active = 1
		sc.s.cleanupGen(sc.k) // the cleanup before its controller's next turn
		if sc.s.g.nemb[0] != 0 {
			t.Fatalf("%d emblems after the preceding cleanup, want 0", sc.s.g.nemb[0])
		}
	})

	t.Run("UntilEndOfNextTurn", func(t *testing.T) {
		sc := embDurScen(t)
		embDurPut(sc, 0, NoObj, embDurEndNextTurn)
		if sc.s.g.embF[0] != 0 {
			t.Fatalf("precondition: turn-start bits %b, want none", sc.s.g.embF[0])
		}
		sc.s.cleanupGen(sc.k) // the turn it was created in ends
		if sc.s.g.nemb[0] != 1 || sc.s.g.embF[0] != 0 {
			t.Fatalf("end of its creation turn: %d emblems, bits %b, want 1 and none", sc.s.g.nemb[0], sc.s.g.embF[0])
		}
		sc.s.active = 1
		sc.s.cleanupGen(sc.k) // the opponent's cleanup is not the boundary
		if sc.s.g.nemb[0] != 1 {
			t.Fatalf("%d emblems after the opponent's cleanup, want 1", sc.s.g.nemb[0])
		}
		sc.s.beginTurn(0) // its controller's next turn begins
		if sc.s.g.embF[0]&(1<<0) == 0 {
			t.Fatalf("turn-start bits %b after the controller's next turn began, want bit 0", sc.s.g.embF[0])
		}
		sc.s.cleanupGen(sc.k) // that next turn's cleanup is the boundary
		if sc.s.g.nemb[0] != 0 || sc.s.g.embF[0] != 0 {
			t.Fatalf("after the next turn's cleanup: %d emblems, bits %b, want 0 and none", sc.s.g.nemb[0], sc.s.g.embF[0])
		}
	})

	t.Run("WhileHostNoEOT", func(t *testing.T) {
		sc := embDurScen(t)
		host := sc.perm(v.GrizzlyBears, 0)
		if !sc.onBF(host) {
			t.Fatal("precondition: the host is not on the battlefield")
		}
		embDurPut(sc, 0, host, embDurWhileHostNoEOT)
		sc.s.cleanupGen(sc.k)
		if sc.s.g.nemb[0] != 1 {
			t.Fatalf("%d emblems after a cleanup, want 1 (this flag has no cleanup end)", sc.s.g.nemb[0])
		}
		sc.s.active = 1
		sc.s.cleanupGen(sc.k)
		if sc.s.g.nemb[0] != 1 {
			t.Fatalf("%d emblems after the opponent's cleanup, want 1", sc.s.g.nemb[0])
		}
		sc.s.hostLeft(sc.k, host)
		if sc.s.g.nemb[0] != 0 {
			t.Fatalf("%d emblems after the host left, want 0", sc.s.g.nemb[0])
		}
	})

	t.Run("AsLongAsControl", func(t *testing.T) {
		t.Run("host leaves", func(t *testing.T) {
			sc := embDurScen(t)
			host := sc.perm(v.GrizzlyBears, 0)
			embDurPut(sc, 0, host, embDurAsLongAsControl)
			sc.s.cleanupGen(sc.k)
			if sc.s.g.nemb[0] != 1 {
				t.Fatalf("%d emblems after a cleanup, want 1", sc.s.g.nemb[0])
			}
			sc.s.hostLeft(sc.k, host)
			if sc.s.g.nemb[0] != 0 {
				t.Fatalf("%d emblems after the host left, want 0", sc.s.g.nemb[0])
			}
		})
		t.Run("controller changes", func(t *testing.T) {
			sc := embDurScen(t)
			host := sc.perm(v.GrizzlyBears, 0)
			embDurPut(sc, 0, host, embDurAsLongAsControl)
			sl := sc.slot(host)
			if sc.s.perms[sl].ctrl != 0 {
				t.Fatalf("precondition: the host is controlled by %d, want 0", sc.s.perms[sl].ctrl)
			}
			sc.s.ctrlMove(sl, 1)
			if sc.s.g.nemb[0] != 0 {
				t.Fatalf("%d emblems after the host's controller changed, want 0", sc.s.g.nemb[0])
			}
		})
	})

	// An AsLongAsControl effect whose host is already under another player's
	// control when it would start creates nothing (CR 611.2b).
	t.Run("AsLongAsControl at creation", func(t *testing.T) {
		sc := embDurScen(t)
		host := sc.perm(v.GrizzlyBears, 0)
		e := &stackEntry{obj: host, srcVer: sc.s.objs[host].ver}
		if sc.s.perms[sc.s.objs[host].slot].ctrl != 0 {
			t.Fatalf("precondition: the host is controlled by %d, want 0", sc.s.perms[sc.s.objs[host].slot].ctrl)
		}
		if sc.s.hostEmblemDone(e, 0, embDurAsLongAsControl) {
			t.Fatal("the host's controller is refused its own emblem")
		}
		if !sc.s.hostEmblemDone(e, 1, embDurAsLongAsControl) {
			t.Fatal("a player who does not control the host got an emblem")
		}
		if sc.s.hostEmblemDone(e, 1, embDurWhileHostNoEOT) {
			t.Fatal("the control check applied to an emblem without the flag")
		}
	})

	t.Run("flags move with survivors", func(t *testing.T) {
		sc := embDurScen(t)
		host := sc.perm(v.GrizzlyBears, 0)
		embDurPut(sc, 0, host, embDurWhileHostNoEOT, embDurEndNextTurn)
		sc.s.g.embF[0] = 1 << 1 // the UntilEndOfNextTurn emblem has begun its turn
		sc.s.hostLeft(sc.k, host)
		if sc.s.g.nemb[0] != 1 || sc.s.g.emb[0][0] != embDurEndNextTurn || sc.s.g.embF[0] != 1<<0 {
			t.Fatalf("after the host left: %d emblems, index %d, bits %b, want 1, %d, %b",
				sc.s.g.nemb[0], sc.s.g.emb[0][0], sc.s.g.embF[0], embDurEndNextTurn, 1<<0)
		}
		sc.s.cleanupGen(sc.k)
		if sc.s.g.nemb[0] != 0 {
			t.Fatalf("%d emblems after the surviving flag's boundary, want 0", sc.s.g.nemb[0])
		}
	})
}
