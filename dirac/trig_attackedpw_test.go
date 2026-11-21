package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestAttackedMatchPlaneswalkerOnly: Attacked$ Planeswalker.YouCtrl
// (Tf2AtkYourPW) matches an object defender only when it is a planeswalker
// the listener's controller controls; a battle (CR 506.2) never does, and a
// creature-animated planeswalker (a type override) still does.
func TestAttackedMatchPlaneswalkerOnly(t *testing.T) {
	const pw, battle = CardID(0), CardID(1)
	for _, tc := range []struct {
		name  string
		card  CardID
		ovr   kop.TypeMask // type override, 0 = none
		owner uint8
		zone  kop.Zone
		want  bool
	}{
		{"planeswalker I control", pw, 0, 0, kop.ZoneBattlefield, true},
		{"battle I control", battle, 0, 0, kop.ZoneBattlefield, false},
		{"animated planeswalker I control", pw, kop.TPlaneswalker | kop.TCreature, 0, kop.ZoneBattlefield, true},
		{"planeswalker that stopped being one", pw, kop.TCreature, 0, kop.ZoneBattlefield, false},
		{"planeswalker the other seat controls", pw, 0, 1, kop.ZoneBattlefield, false},
		{"planeswalker I control, off the battlefield", pw, 0, 0, kop.ZoneGraveyard, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			k := &prep{cards: []pcard{{types: kop.TPlaneswalker}, {types: kop.TBattle}}}
			s := &State{}
			const sl, o = Slot(0), Obj(0)
			s.objs[o].zone = tc.zone
			s.objs[o].slot = sl
			s.perms[sl].card = tc.card
			s.perms[sl].ctrl = tc.owner
			if tc.ovr != 0 {
				s.perms[sl].ovr.flags |= ovTypes
				s.perms[sl].ovr.types = tc.ovr
			}
			// Precondition: the types the gate reads are what the case names.
			wantTy := k.cards[tc.card].types
			if tc.ovr != 0 {
				wantTy = tc.ovr
			}
			if got := s.typesAt(k, sl); got != wantTy {
				t.Fatalf("typesAt = %#x, want %#x", got, wantTy)
			}
			f := kop.Tf2Attacked | kop.Tf2AtkYou | kop.Tf2AtkYourPW
			if got := s.attackedMatch(k, f, 0, TCode(o)); got != tc.want {
				t.Errorf("attackedMatch = %v, want %v", got, tc.want)
			}
			if got := s.attacksGate(k, f, 0, TCode(o)); got != tc.want {
				t.Errorf("attacksGate = %v, want %v", got, tc.want)
			}
		})
	}
}
