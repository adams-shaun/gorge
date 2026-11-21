package dirac_test

import (
	"errors"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestStageCeasedToken: a token that has ceased to exist (ZoneNone) keeps its
// object index and holds no zone; one carrying a mark, or in a zone other than
// the battlefield and none, is refused as a token.
func TestStageCeasedToken(t *testing.T) {
	t.Parallel()
	ceased := func(w *dirac.World, owner uint8) {
		w.Objs = append(w.Objs, dirac.WorldObj{Card: vanilla.SoldierToken, Owner: owner, Ctrl: owner, Zone: kop.ZoneNone, Token: true})
	}
	w := stageBase()
	ceased(&w, 0)
	w.Objs = append(w.Objs, dirac.WorldObj{Card: vanilla.SoldierToken, Owner: 1, Ctrl: 1, Zone: kop.ZoneBattlefield, Token: true})
	s, err := dirac.NewFromWorld(&w)
	if err != nil {
		t.Fatalf("a world with a ceased token does not stage: %v", err)
	}
	if got := s.NumObjs(); got != 42 {
		t.Fatalf("staged %d objects, want 42", got)
	}
	if !s.ObjToken(40) || s.ObjZone(40) != kop.ZoneNone || s.ObjZone(41) != kop.ZoneBattlefield {
		t.Errorf("token 40 zone %d (token %v), token 41 zone %d, want none and battlefield", s.ObjZone(40), s.ObjToken(40), s.ObjZone(41))
	}
	for name, mut := range map[string]func(*dirac.WorldObj){
		"tapped":    func(o *dirac.WorldObj) { o.Tapped = true },
		"damaged":   func(o *dirac.WorldObj) { o.Damage = 1 },
		"graveyard": func(o *dirac.WorldObj) { o.Zone = kop.ZoneGraveyard },
	} {
		w := stageBase()
		ceased(&w, 1)
		mut(&w.Objs[40])
		_, err := dirac.NewFromWorld(&w)
		var u dirac.ErrUnstageable
		if !errors.As(err, &u) || u.Reason != dirac.ReasonToken {
			t.Errorf("ceased token %s: got %v, want reason %q", name, err, dirac.ReasonToken)
		}
	}
}
