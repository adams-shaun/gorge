package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// CR 111.8: a token that has left the battlefield can't move to another zone
// or come back onto the battlefield. It stays where it is and ceases at the
// next state-based action (ceaseTokensGen). No state-based action runs inside
// one resolution, so a "then return it" sub-ability (Lilysplash Mentor, Splash
// Portal) finds the exiled token still there; every move of an object path
// carries this test, as gorge's changeZoneMoveSkip and effChangeZoneAll's
// emitMove do (engine/effects/zone_change.go tokenCannotChangeZone).
//
// A spell copy is ofCopy until it enters the battlefield and a prepared copy
// stays ofCopy in exile, so neither has the bit; a token copy has both.

// DebugTokenMoves turns the test off: the token moves again, the earlier
// gorge. Test-only (TestTokenmoveMutation).
var DebugTokenMoves bool

// tokenLeftBattlefield reports whether o is a token CR 111.8 holds in place.
func (s *State) tokenLeftBattlefield(o Obj) bool {
	ob := &s.objs[o]
	return ob.of&ofToken != 0 && ob.zone != kop.ZoneBattlefield && ob.zone != kop.ZoneStack && !DebugTokenMoves
}

// instantStays reports whether o is an instant or sorcery card bound for the
// battlefield: it remains in its previous zone (CR 304.4, 307.4), as gorge's
// movers leave it (effects.instantCannotEnter). A token has no printed type.
func (s *State) instantStays(k *prep, o Obj, z kop.Zone) bool {
	ob := &s.objs[o]
	return z == kop.ZoneBattlefield && ob.of&ofToken == 0 && k.cards[ob.card].types&(kop.TInstant|kop.TSorcery) != 0
}
