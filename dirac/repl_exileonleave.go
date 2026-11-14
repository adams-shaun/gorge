package dirac

import (
	"sync/atomic"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// exileany: ReplExileOnLeave with Repl.C = 1, "if a card would be put into a
// graveyard from anywhere, exile it instead" (Rest in Peace, Leyline of the
// Void, Dryad Militant). gorge's replacementMatches takes a Moved line with
// Destination$ Graveyard and no Origin$ for a move from any zone, ValidCard$
// matched against the moving object. A battlefield death reaches it through
// exileOnLeaveN (dieZone); every other card reaches a graveyard through
// placeFrom (a resolved or countered spell, discard, mill, DigUntil's digMove
// and the pile moves), which asks exileAnyDest. Two matching replacements on
// one move (or one beside Progenitus's) pose gorge's CR 616.1 order ask: a
// parked move has settled it (repl_exilemark.go; a two-pile split's
// graveyard cards park in pilesToGraveyard), and any other is StallReplLeave,
// as dieZone.

// DebugExileAnyBattlefieldOnly skips the non-battlefield check (the behaviour
// before); TestExileanyMutation asserts the lockstep then
// diverges.
var DebugExileAnyBattlefieldOnly bool

// ExileAnyStats counts, for the lockstep test, the cards an any-zone source
// exiled instead of a graveyard move from a non-battlefield zone [0] and the
// creatures one exiled instead of dying [1]. Read-only elsewhere.
var ExileAnyStats [2]uint64

// exileAnyDied counts a creature exiled instead of dying (exileOnLeaveN).
func exileAnyDied() { atomic.AddUint64(&ExileAnyStats[1], 1) }

// exileAnyN counts the any-zone dies-to-exile sources (on the battlefield,
// abilities not lost) whose filter matches card c (object o, owned by owner)
// moving to a graveyard.
func (s *State) exileAnyN(k *prep, o Obj, c CardID, owner uint8) int {
	g := k.gen
	n := 0
	for m := s.statM; m.any(); m = m.rest() {
		b := m.first()
		sc := s.cardAt(b)
		// a departed seat's effects no longer apply (CR 800.4a; gorge's
		// replacement walk visits only living seats' zones)
		if g.replBits[sc]&rbExileOnLeave == 0 || !g.exileAnyC[sc] || s.genLost(k, b) || s.out(s.perms[b].ctrl) {
			continue
		}
		if s.matchCardGen(k, g.exileOnLeave[sc], c, owner, s.perms[b].ctrl, o) {
			n++
		}
	}
	return n
}

// exileAnyDest is the zone a card o headed for a graveyard really goes to:
// the exile zone when an any-zone source applies. Called with the table's
// anyExileAny set.
//
//go:noinline
func (s *State) exileAnyDest(k *prep, o Obj) kop.Zone {
	if DebugExileAnyBattlefieldOnly {
		return kop.ZoneGraveyard
	}
	ob := &s.objs[o]
	n := s.exileAnyN(k, o, ob.card, ob.owner())
	if n == 0 {
		return kop.ZoneGraveyard
	}
	if k.gen.anyRepl&rbGraveLib != 0 && s.graveRedirect(k, ob.card, kop.ZoneGraveyard) == kop.ZoneLibrary {
		n++
	}
	if n > 1 && !s.takeAnswered(o) {
		s.capFull(StallReplLeave)
	}
	atomic.AddUint64(&ExileAnyStats[0], 1)
	return kop.ZoneExile
}
