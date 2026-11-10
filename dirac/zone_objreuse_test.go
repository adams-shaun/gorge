package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestTokenObjReuse: two 100-card decks leave MaxObj-200 object ids for tokens.
// Six more tokens than that, made one after another, each dying (CR 704.5d: it ceases to exist)
// before the next turn's is made, never stall: once the table is full the
// lowest ceased id is minted again, with a ver no reference to the old
// object holds. A token that ceased this turn is not reused.
func TestTokenObjReuse(t *testing.T) {
	tbl := *v.Table
	tbl.Exprs = make([]kop.Expr, 1) // force the generated path (ceaseTokensGen)
	var lib [MaxDeck]CardID
	for i := range lib {
		lib[i] = v.Plains
	}
	s := new(State)
	s.Init(&tbl, lib[:], lib[:], 1, Config{})
	k := s.k()
	if s.nobj != 2*MaxDeck {
		t.Fatalf("genesis objects %d, want %d", s.nobj, 2*MaxDeck)
	}
	lastVer := map[Obj]uint8{}
	for i := 0; i < MaxObj-2*MaxDeck+6; i++ {
		if s.nearCapacity(k) {
			t.Fatalf("token %d: the object table reads full (nobj %d)", i+1, s.nobj)
		}
		o := s.newToken(v.GrizzlyBears, 0)
		want := Obj(2*MaxDeck + i)
		if i >= MaxObj-2*MaxDeck {
			want = 2 * MaxDeck // every earlier token ceased: the lowest id is free again
		}
		if o != want {
			t.Fatalf("token %d minted id %d, want %d", i+1, o, want)
		}
		if old, ok := lastVer[o]; ok && s.objs[o].ver == old {
			t.Fatalf("token %d reuses id %d at the ver its last object left with (%d)", i+1, o, old)
		}
		sl := s.enter(k, o, 0, false, kop.ZoneLibrary)
		s.npend = 0
		s.leave(k, sl, kop.ZoneGraveyard, false)
		s.npend = 0
		s.ceaseTokensGen()
		if s.objs[o].zone != kop.ZoneNone {
			t.Fatalf("token %d (id %d) did not cease: zone %d", i+1, o, s.objs[o].zone)
		}
		lastVer[o] = s.objs[o].ver
		if s.objFree(o) {
			t.Fatalf("token %d (id %d): a token that ceased this turn reads free", i+1, o)
		}
		s.turn++
		s.newTurnGen()
	}
	if s.Stalled() {
		t.Fatalf("stalled: reason %d", s.StallReason())
	}
	if int(s.nobj) != MaxObj {
		t.Fatalf("nobj %d, want the full table %d", s.nobj, MaxObj)
	}
}
