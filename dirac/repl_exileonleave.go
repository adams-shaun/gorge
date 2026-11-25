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
// moving to a graveyard. The second and third results are the counter a
// matching source leaves on the exiled card (0 when none).
func (s *State) exileAnyN(k *prep, o Obj, c CardID, owner uint8) (int, kop.CounterKind, uint8) {
	g := k.gen
	n := s.embExileAnyN(k, o, c, owner) // an emblem's any-zone arm joins the count (repl_exileleaveemb.go)
	var ctr kop.CounterKind
	var ctrN uint8
	for m := s.statM; m.any(); m = m.rest() {
		b := m.first()
		sc := s.cardAt(b)
		// a departed seat's effects no longer apply (CR 800.4a; gorge's
		// replacement walk visits only living seats' zones)
		if g.replBits[sc]&rbExileOnLeave == 0 || g.exileAnyC[sc]&eaAny == 0 || s.genLost(k, b) || s.out(s.perms[b].ctrl) {
			continue
		}
		if g.exileAnyC[sc]&eaSelf != 0 {
			continue // a self filter is not a card matcher: the card exiles only itself (exileAnyDest)
		}
		if s.matchCardGen(k, g.exileOnLeave[sc], c, owner, s.perms[b].ctrl, o) {
			n++
			if kind, amount := exileCounterKind(k.t, sc, g.exileOnLeave[sc]); kind != 0 {
				ctr, ctrN = kind, amount
			}
		}
	}
	return n, ctr, ctrN
}

// exileAnyDest is the zone a card o headed for a graveyard really goes to:
// the exile zone when an any-zone source applies, including the moving card's
// own "from anywhere" replacement (a ReplExileOnLeave with a self filter,
// whose source is the card itself and so is not a battlefield permanent when
// it moves from another zone). from is the move's origin.
//
//go:noinline
func (s *State) exileAnyDest(k *prep, o Obj, from kop.Zone) kop.Zone {
	if DebugExileAnyBattlefieldOnly {
		return kop.ZoneGraveyard
	}
	ob := &s.objs[o]
	n, ctr, ctrN := s.exileAnyN(k, o, ob.card, ob.owner())
	if from != kop.ZoneBattlefield && k.gen.exileAnyC != nil && k.gen.exileAnyC[ob.card]&eaSelf != 0 {
		n++ // the card's own self any-zone replacement
		if kind, amount := exileCounterKind(k.t, ob.card, k.gen.exileOnLeave[ob.card]); kind != 0 {
			ctr, ctrN = kind, amount
		}
	}
	if n == 0 {
		return kop.ZoneGraveyard
	}
	if k.gen.anyRepl&rbGraveLib != 0 && s.graveRedirect(k, ob.card, kop.ZoneGraveyard, from) == kop.ZoneLibrary {
		n++
	}
	if n > 1 {
		// two or more effects compete: a stall, or settled by the ask, whose
		// answer applyExileOrder stamps the counter of (not a re-scan here)
		if !s.takeAnswered(o) {
			s.capFull(StallReplLeave)
		}
		ctr = 0
	}
	if ctr != 0 {
		// the move is to exile: place the counter anchored one bump back, the
		// ver the object had when placeFromK brought it here
		s.stampExileCounterMoved(o, ctr, int(ctrN))
	}
	atomic.AddUint64(&ExileAnyStats[0], 1)
	return kop.ZoneExile
}
