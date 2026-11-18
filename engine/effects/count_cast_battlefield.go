package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/state"
)

// castBattlefieldHead is the Count$ head spelling whose carriers read the
// battlefield as it stood when the spell was cast.
const castBattlefieldHead = "LastStateBattlefield"

// SVarsReadCastBattlefield reports whether any SVar body counts the as-cast
// battlefield (Count$LastStateBattlefield or Count$LastStateBattlefieldWithFallback),
// i.e. whether casting the spell owning these SVars must freeze the
// battlefield. The WithFallback spelling contains the plain one, so one
// substring covers both. The map is only scanned for membership, so its
// iteration order reaches nothing.
func SVarsReadCastBattlefield(svars map[string]string) bool {
	for _, body := range svars {
		if strings.Contains(body, castBattlefieldHead) {
			return true
		}
	}
	return false
}

// frozenHost answers the derived-P/T reads of a frozen battlefield from the
// values captured at cast (readers that go through Host.Power/Toughness, e.g.
// zoneCountFold.extremePropertyValue and P/T comparisons in the filter
// grammar), while every other Host read is promoted from the live host. The
// frozen game itself is scanned for membership, controller and printed
// characteristics, so the eight carriers' clauses (printed type/colour +
// controller + modified + $GreatestCardPower) all read frozen data. A
// layer-derived characteristic beyond P/T (a granted type/colour/keyword) still
// reads the live host and is out of this fix's scope.
type frozenHost struct {
	Host
	snap *state.CastBattlefield
}

func (f frozenHost) Power(id state.ObjID) int32 {
	if p, _, ok := f.snap.DerivedPT(id); ok {
		return p
	}
	return f.Host.Power(id)
}

func (f frozenHost) Toughness(id state.ObjID) int32 {
	if _, t, ok := f.snap.DerivedPT(id); ok {
		return t
	}
	return f.Host.Toughness(id)
}

// evalCastBattlefieldCount is Count$LastStateBattlefieldWithFallback: the Valid
// scan of arg over the battlefield as the source spell was cast, and over the
// current battlefield only when the source carries no frozen battlefield (a
// bare context, an uncast source). A frozen EMPTY battlefield is an
// authoritative zero, never "missing". The scan is evalCountBodyZone itself
// run against the frozen game, so the filter grammar, $Property folds and
// extreme reductions cannot drift from Count$Valid. An empty filter is not a
// body this build can count, so it fails the head's own verdict.
func evalCastBattlefieldCount(h Host, c *Ctx, g *state.Game, arg string, depth int) (int32, bool, bool) {
	if arg == "" {
		return 0, false, true
	}
	if c != nil {
		if src := g.Obj(c.Source); src != nil && src.Zone == state.ZStack && src.CastBattlefield != nil {
			snap := src.CastBattlefield
			return evalCountBodyZone(frozenHost{Host: h, snap: snap}, c, snap.Game, "Valid", arg, depth)
		}
	}
	return evalCountBodyZone(h, c, g, "Valid", arg, depth)
}

// evalLastStateBattlefield is Count$LastStateBattlefield: the Valid scan of
// arg over the battlefield AS THE SOURCE WAS CAST, from the same frozen
// snapshot evalCastBattlefieldCount reads -- but with NO fallback. Forge's
// getLastStateBattlefield has no current-battlefield half: a source with no
// frozen battlefield (a bare context, an uncast source) reads an empty scan,
// a modelled zero. The corpus's two carriers (Boss's Chauffeur, Aeve,
// Progenitor Ooze) both read it in their own etbCounter bodies, where the
// cast's snapshot is present on the source; the read is
// evalCountBodyZone itself run against the frozen game, so the filter
// grammar, $Property folds and extreme reductions cannot drift from
// Count$Valid. An empty filter is not a body this build can count, so it
// fails the head's own verdict.
func evalLastStateBattlefield(h Host, c *Ctx, g *state.Game, arg string, depth int) (int32, bool, bool) {
	if arg == "" {
		return 0, false, true
	}
	if c != nil {
		if src := g.Obj(c.Source); src != nil && src.CastBattlefield != nil &&
			(src.Zone == state.ZStack || src.Zone == state.ZBattlefield) {
			snap := src.CastBattlefield
			return evalCountBodyZone(frozenHost{Host: h, snap: snap}, c, snap.Game, "Valid", arg, depth)
		}
	}
	return 0, true, true
}
