package events

import (
	"sort"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// ResolvedAbilityKey identifies "this ability" for Forge's
// Count$ResolvedThisTurn -- the per-ability resolution tally that gates
// Sephiroth, Fabled SOLDIER's "if this is the fourth time this ability has
// resolved this turn, transform" (and Prowl's, Victor's, Nissa's, ...).
//
// The key is (source permanent ObjID, the resolving root Ability$ body's
// CONTENT), never a *cards.SA pointer: cards.Link resolves an Execute$ SVar
// body by PARSING THE TEXT FRESH, so one card's two T: lines that share an
// Execute$ (Victor, Valgavoth's Seneschal's ChangesZone and FullyUnlock lines
// both Execute$ TrigSurveil) carry two structurally-equal but pointer-distinct
// SAs -- and a recorded log re-parses the body a third time. Content therefore
// is the only identity that merges the abilities the oracle calls one, and it
// is stable across a live game and its replay because the script text is
// static. The walk is a fixed pre-order with sorted parameter names, so it
// never depends on map iteration order.
//
// The source's ZoneGen is part of the key (CR 400.7): a permanent that left
// and returned is a new object whose ability has not resolved this turn, as
// XMage's AbilityResolvedWatcher (zone-change-counter key) and Forge both
// read it (farm window BLB-s111 xe:RAND:4:1:w11, Harvestrite Host blinked).
// ZoneGen is folded only by events.Apply, so a log-only replay rebuilds it and
// no new Kind or field is needed.
//
// source is part of the key because two permanents can carry the same body
// (two Sephiroths, or a Sephiroth and a copied one) and each has its own tally.
func ResolvedAbilityKey(g *state.Game, source state.ObjID, sa *cards.SA) string {
	return strconv.FormatUint(uint64(source), 10) + "." + strconv.FormatUint(uint64(sourceZoneGen(g, source)), 10) +
		"|" + saKeyContent(sa)
}

// sourceZoneGen is the source object's current CR 400.7 identity stamp (0 for
// an object the game no longer holds).
func sourceZoneGen(g *state.Game, source state.ObjID) uint32 {
	if o := g.Obj(source); o != nil {
		return o.ZoneGen
	}
	return 0
}

func writeSAKey(b *strings.Builder, sa *cards.SA, depth int) {
	if sa == nil || depth > 32 {
		return
	}
	b.WriteString(sa.Kind)
	b.WriteByte(':')
	b.WriteString(sa.API)
	keys := make([]string, 0, len(sa.Params))
	for k := range sa.Params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		b.WriteByte(';')
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(sa.Params[k])
	}
	if sa.Sub != nil {
		b.WriteByte('>')
		writeSAKey(b, sa.Sub, depth+1)
	}
}
