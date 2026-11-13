package oracle

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// pinCheck is one named condition a pin document's check field may name (kind
// "check"): a Go predicate over the gorge side that no pinCondition kind says.
// The registry is sorted by name, so a lookup is a binary search and a new
// check joins it in one place; validate refuses a name that is not here.
type pinCheck struct {
	name string
	// pred is the plain predicate. A check that reads the document's fields
	// (a card name, say) is a maker instead, and reports the field it got
	// wrong; exactly one of the two is set.
	pred  func(ref *GorgeSide) bool
	maker func(c pinCondition) (func(ref *GorgeSide) bool, error)
}

// pinChecks is the check registry, sorted by name.
var pinChecks = []pinCheck{
	{name: "anyActivated", pred: anyActivated},
	{name: "auraLeftBattlefield", pred: auraLeftBattlefield},
	{name: "castThroughPayLifeGrant", maker: payLifeGrantCheck},
	{name: "eachPlayerDiscarded", pred: eachPlayerDiscarded},
	{name: "hybridCast", pred: hybridCast},
	{name: "impendedCast", pred: impendedCast},
	{name: "madeRestrictedMana", pred: madeRestrictedMana},
	{name: "orderAskWhileResolving", pred: orderAskWhileResolving},
	{name: "parkedMoveSettled", pred: parkedMoveSettled},
	{name: "powerUpAfterRestrictedMana", pred: powerUpAfterRestrictedMana},
	{name: "preparedCopySpentRestrictedMana", pred: preparedCopySpentRestrictedMana},
	{name: "sagaLeftBattlefield", pred: sagaLeftBattlefield},
	{name: "threeWayAskWhileResolving", pred: threeWayAskWhileResolving},
	{name: "waterbendAsk", pred: waterbendAsk},
}

// pinCheckByName is the registry entry named name, found by binary search.
func pinCheckByName(name string) (pinCheck, bool) {
	i := sort.Search(len(pinChecks), func(i int) bool { return pinChecks[i].name >= name })
	if i < len(pinChecks) && pinChecks[i].name == name {
		return pinChecks[i], true
	}
	return pinCheck{}, false
}

// payLifeGrantCheck builds castThroughPayLifeGrant from the condition's card.
func payLifeGrantCheck(c pinCondition) (func(ref *GorgeSide) bool, error) {
	if c.Card == "" {
		return nil, fmt.Errorf("pin condition check castThroughPayLifeGrant: no card")
	}
	return castThroughPayLifeGrant(c.Card), nil
}

// madeRestrictedMana is the condition that mana carrying a spending restriction has
// been added to a pool.
func madeRestrictedMana(ref *GorgeSide) bool {
	for i := range ref.E.L.Events {
		ev := &ref.E.L.Events[i]
		if ev.Kind == events.ManaAdd && ev.Amount > 0 && strings.HasPrefix(ev.Text, "mana-restriction:") {
			return true
		}
	}
	return false
}

// impendedCast is the condition that a spell was cast for its impending cost (CR
// 702.176a): an object on the stack or battlefield carries FlagImpending.
func impendedCast(ref *GorgeSide) bool {
	g := ref.E.G
	zones := [][]state.ObjID{g.Zone(state.ZStack, 0)}
	for p := range g.Players {
		zones = append(zones, g.Zone(state.ZBattlefield, state.PlayerID(p)))
	}
	for _, ids := range zones {
		for _, id := range ids {
			if o := g.Obj(id); o != nil && o.CastFlags&state.FlagImpending != 0 {
				return true
			}
		}
	}
	return false
}

// hybridCast is the condition that a spell with a two-colour hybrid symbol in its
// mana cost (CR 107.4e) is on the stack.
func hybridCast(ref *GorgeSide) bool {
	for _, id := range ref.E.G.Zone(state.ZStack, 0) {
		o := ref.E.G.Obj(id)
		if o == nil || o.Face() == nil {
			continue
		}
		for _, sym := range strings.Fields(o.Face().ManaCost) {
			if len(sym) == 2 && strings.ContainsRune("WUBRG", rune(sym[0])) && strings.ContainsRune("WUBRG", rune(sym[1])) {
				return true
			}
		}
	}
	return false
}

// eachPlayerDiscarded is the condition that two players' discards moved together:
// discards of cards of different owners within a few events of each other (CR 101.4).
func eachPlayerDiscarded(ref *GorgeSide) bool {
	evs := ref.E.L.Events
	owner := func(i int) (state.PlayerID, bool) {
		if !events.IsDiscard(evs[i]) {
			return 0, false
		}
		o := ref.E.G.Obj(evs[i].Obj)
		if o == nil {
			return 0, false
		}
		return o.Owner, true
	}
	for i := range evs {
		a, ok := owner(i)
		if !ok {
			continue
		}
		for j := i + 1; j < min(len(evs), i+6); j++ {
			if b, ok := owner(j); ok && b != a {
				return true
			}
		}
	}
	return false
}

// anyActivated is the condition that some activated ability was put on the stack.
func anyActivated(ref *GorgeSide) bool {
	for i := range ref.E.L.Events {
		if ref.E.L.Events[i].Kind == events.AbilityPush {
			return true
		}
	}
	return false
}

// sagaLeftBattlefield is the condition that a Saga left the battlefield (CR 714.4).
func sagaLeftBattlefield(ref *GorgeSide) bool { return leftBattlefield(ref, "Saga") }

// auraLeftBattlefield is the condition that an Aura left the battlefield.
func auraLeftBattlefield(ref *GorgeSide) bool { return leftBattlefield(ref, "Aura") }

// leftBattlefield reports whether a permanent with the given type left the battlefield.
func leftBattlefield(ref *GorgeSide, typ string) bool {
	for i := range ref.E.L.Events {
		ev := &ref.E.L.Events[i]
		if ev.Kind != events.MoveZone || ev.From != state.ZBattlefield {
			continue
		}
		if o := ref.E.G.Obj(ev.Obj); o != nil && o.Face() != nil && slices.Contains(o.Face().Types, typ) {
			return true
		}
	}
	return false
}

// waterbendAsk is the condition that a waterbend ask is pending (CR 601.2h,
// 701.67a).
func waterbendAsk(ref *GorgeSide) bool {
	if d := ref.Pending(); d != nil {
		for _, o := range d.Options {
			if strings.HasPrefix(o.Kind, "waterbend") {
				return true
			}
		}
	}
	return false
}

// powerUpAfterRestrictedMana is the condition that a power-up was activated after
// restricted mana had been added to a pool (CR 106.6, 702.193b).
func powerUpAfterRestrictedMana(ref *GorgeSide) bool {
	armed := false
	for i := range ref.E.L.Events {
		ev := &ref.E.L.Events[i]
		switch ev.Kind {
		case events.ManaAdd:
			if ev.Amount > 0 && strings.HasPrefix(ev.Text, "mana-restriction:") {
				armed = true
			}
		case events.AbilityPush:
			if o := ref.E.G.Obj(ev.Obj); armed && o != nil && o.Face() != nil && int(ev.Amount) < len(o.Face().Abilities) &&
				o.Face().Abilities[ev.Amount].Params["PowerUp"] == "True" {
				return true
			}
		}
	}
	return false
}

// orderAskWhileResolving: a CR 616.1 order ask is pending with an object on the stack,
// so a resolving effect (not a land play) moves the land.
func orderAskWhileResolving(ref *GorgeSide) bool {
	d := ref.Pending()
	return d != nil && d.Kind == decision.KReplacement && len(ref.E.G.Stack) > 0
}

// threeWayAskWhileResolving: the order ask has three options with an object on the
// stack: two untap bodies against a tap, whose later picks are asked again after the
// land has entered (CR 616.1f).
func threeWayAskWhileResolving(ref *GorgeSide) bool {
	d := ref.Pending()
	return d != nil && d.Kind == decision.KReplacement && len(d.Options) == 3 && len(ref.E.G.Stack) > 0
}

// parkedMoveSettled is the condition that the board is a priority with the stack
// empty and two Rests in Peace out (gravesAfterAnswers, parkedmove_test.go): the
// board a parked move's answers settle into.
func parkedMoveSettled(ref *GorgeSide) bool {
	_, ok := gravesAfterAnswers(ref)
	return ok
}

// castThroughPayLifeGrant is the condition that the named card is in a
// graveyard and a spell is on the stack that was cast through a may-play
// grant: the card it exiled was cast for life, not mana. The spell must be
// one its caster does not own: Inside Information exiles the opponent's
// library, so this excludes Snowslope Hunter's own-library permission (which
// carries no pay-life bit and never reaches the canon pay-life arm).
func castThroughPayLifeGrant(name string) func(ref *GorgeSide) bool {
	return func(ref *GorgeSide) bool {
		g := ref.E.G
		seen := false
		for p := range g.Players {
			seen = seen || inZoneNamed(g, state.ZGraveyard, state.PlayerID(p), name)
		}
		if !seen {
			return false
		}
		for _, id := range g.Zone(state.ZStack, 0) {
			if o := g.Obj(id); o != nil && o.CastFlags&state.FlagMayPlay != 0 && o.Owner != o.Controller {
				return true
			}
		}
		return false
	}
}

// preparedCopySpentRestrictedMana is the condition that a prepared copy's cast
// spent mana carrying a spend restriction: a restricted ManaAdd with a negative
// amount in the same cast as the prepared copy's CastInfo (gorge emits the
// spend before the cast commits). It is the interaction the pin exists for.
func preparedCopySpentRestrictedMana(ref *GorgeSide) bool {
	spent := false
	evs := ref.E.L.Events
	for i := range evs {
		e := &evs[i]
		switch e.Kind {
		case events.ManaAdd:
			if e.Amount < 0 && strings.HasPrefix(e.Text, "mana-restriction:") {
				spent = true
			}
		case events.CastInfo:
			if spent && events.FlagsFrom(e.Counter)&state.FlagPreparedCopy != 0 {
				return true
			}
			spent = false
		}
	}
	return false
}
