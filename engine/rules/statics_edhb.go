package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The S:Mode$ CantTransform, CantAttach, CantDiscard, MaxCounter and
// ManaBurn statics. Each is one check at a known point, registered here
// beside its reader:
//
//   - CantTransform (CR 701.28f; Immerwolf, Bound by Moonsilver): an
//     effect's transform of a battlefield permanent (SetState Mode$ Transform,
//     through Engine.Emit's staticForbidsHostEvent) and the CR 702.145
//     day/night face sync (dayNightFlips) leave a matching permanent on its
//     face.
//   - CantAttach (CR 301.5c / 701.3b / 704.5n; O-Naginata, Konda's Banner, Gate
//     Smasher, Goblin Brawler): the Equipment shape (cards.CantAttachEquipmentShape)
//     is refused by attachRefused and detached by attachmentSBAs; the Aura shapes
//     stay registered as stat:CantAttach.OtherShape.
//   - CantDiscard (Tamiyo, Collector of Tales): an effect's discard (the
//     events.Discard move an effect emits through Engine.Emit) of a protected
//     player's card is withheld; cost discards (events.DiscardCost, and the
//     unless-cost payment, which emits through the pay engine) are not.
//   - MaxCounter (CR 704.5r; Rasputin Dreamweaver): the SBA removes all but N.
//   - ManaBurn (Yurlok of Scorch Thrash): a player losing unspent mana -- the
//     CR 500.4 step boundary or a DB$ DrainMana -- loses that much life.
func init() {
	effects.RegisterNonAPI("stat:CantTransform", "stat:CantAttach", "stat:CantDiscard", "stat:MaxCounter", "stat:ManaBurn")
}

// edhbStaticBoard is the narrow engine surface these readers need: the
// static collector, the spec matcher, the cause classifier, the board and the
// emit.
type edhbStaticBoard interface {
	Game() *state.Game
	activeStatics(mode string) []staticView
	matchesSpecFrom(spec string, id state.ObjID, you state.PlayerID, source state.ObjID) bool
	causeSpecAdmits(spec string, source state.ObjID) bool
	emit(ev events.Event) events.Event
}

// transformForbidden reports whether a live CantTransform static's
// ValidCard$ (read with the static's controller as "you" and its source's
// host as EnchantedBy) names the permanent id.
func transformForbidden(e edhbStaticBoard, id state.ObjID) bool {
	for _, sv := range e.activeStatics("CantTransform") {
		if spec := sv.ParamStr(cards.PKValidCard); spec != "" && e.matchesSpecFrom(spec, id, sv.Controller, sv.Source) {
			return true
		}
	}
	return false
}

// cantAttachForbids reports whether a live CantAttach static forbids
// attaching the Equipment o to bearer: its ValidCard$ names o and its Target$
// names bearer, both read from the static's source and controller. Only the
// Equipment arms call it, so the Aura shapes (Aura.Other, a ValidCardToTarget$
// line without ValidCard$) never match here; cards.CantAttachEquipmentShape
// keeps them registered as stat:CantAttach.OtherShape.
func cantAttachForbids(e edhbStaticBoard, o, bearer *state.Object) bool {
	for _, sv := range e.activeStatics("CantAttach") {
		vc, tg := sv.ParamStr(cards.PKValidCard), sv.ParamStr(cards.PKTarget)
		if vc == "" || tg == "" {
			continue
		}
		if e.matchesSpecFrom(vc, o.ID, sv.Controller, sv.Source) &&
			e.matchesSpecFrom(tg, bearer.ID, sv.Controller, sv.Source) {
			return true
		}
	}
	return false
}

// discardForbidden reports whether a live CantDiscard static protects player
// p from discarding by the cause in flight. ForCost$ True restricts only a
// cost discard and ForCost$ False only an effect's; ValidCause$ is evaluated
// against the resolving wrapper (causeSpecAdmits), which only the effect path
// has, so a cause-scoped line never restricts a cost. ValidPlayer$ names the
// protected player.
func discardForbidden(e edhbStaticBoard, p state.PlayerID, forCost bool) bool {
	for _, sv := range e.activeStatics("CantDiscard") {
		switch sacrificeBlockedCodes.Code(string(sv.ParamStr(cards.PKForCost))) {
		case sacrificeBlockedTrue:
			if !forCost {
				continue
			}
		case sacrificeBlockedFalse:
			if forCost {
				continue
			}
		}
		if spec := sv.ParamStr(cards.PKValidCause); spec != "" && (forCost || !e.causeSpecAdmits(spec, sv.Source)) {
			continue
		}
		if spec := sv.ParamStr(cards.PKValidPlayer); spec != "" && !effects.MatchesPlayerSpec(e.Game(), spec, p, sv.Controller) {
			continue
		}
		return true
	}
	return false
}

// maxCounterSBA queues CR 704.5r: a permanent a live MaxCounter static names
// (ValidCard$) holding more than MaxNum$ counters of CounterType$ loses all
// but MaxNum$ of them. Walked in fixed seat and battlefield order; a removal
// already queued this check for the same counter is counted, so two statics
// capping one counter remove the excess once. An unreadable MaxNum$ or a
// line without CounterType$/ValidCard$ caps nothing.
func maxCounterSBA(e edhbStaticBoard, b *sbaBatch) {
	statics := e.activeStatics("MaxCounter")
	if len(statics) == 0 {
		return
	}
	g := e.Game()
	for _, sv := range statics {
		kind := sv.ParamStr(cards.PKCounterType)
		limit, ok := effects.MaxCounterValueOK(sv.ParamStr(cards.PKMaxNum))
		spec := sv.ParamStr(cards.PKValidCard)
		if kind == "" || !ok || spec == "" {
			continue
		}
		for _, p := range g.AliveFrom(0) {
			for _, id := range g.Zone(state.ZBattlefield, p) {
				o := g.Obj(id)
				if o == nil || o.PhasedOut || !e.matchesSpecFrom(spec, id, sv.Controller, sv.Source) {
					continue
				}
				if excess := o.Counter(kind) - queuedCounterRemoval(b, id, kind) - limit; excess > 0 {
					b.counters = append(b.counters, events.Event{Kind: events.CounterChange, Obj: id, Counter: kind, Amount: -excess})
				}
			}
		}
	}
}

// queuedCounterRemoval sums the removals of kind from id already queued in b.
func queuedCounterRemoval(b *sbaBatch, id state.ObjID, kind string) int32 {
	var n int32
	for _, ev := range b.counters {
		if ev.Obj == id && ev.Counter == kind && ev.Amount < 0 {
			n -= ev.Amount
		}
	}
	return n
}

// emitManaClearBurning emits one ManaClear and, under a live ManaBurn static,
// makes its player lose as much life as the clear took from the pool. The
// loss is measured, so mana a stat:UnspentMana keep or a persistent unit
// kept, or a clear a replacement converted or parked, burns nothing.
func emitManaClearBurning(e edhbStaticBoard, ev events.Event) {
	g := e.Game()
	if int(ev.Player) < 0 || int(ev.Player) >= len(g.Players) || len(e.activeStatics("ManaBurn")) == 0 {
		e.emit(ev)
		return
	}
	before := g.Players[ev.Player].Pool.Total()
	e.emit(ev)
	if lost := before - g.Players[ev.Player].Pool.Total(); lost > 0 {
		e.emit(events.Event{Kind: events.LifeChange, Player: ev.Player, Amount: -int32(lost)})
	}
}

// staticForbidsHostEvent reports whether a live static forbids an action an
// effect emits through Engine.Emit, with the Note recorded in its place:
//   - CantTransform: a transform (FlipFace "Transformed") of a battlefield
//     permanent. Entering transformed is not a transform and is not judged.
//   - CantDiscard: an effect's discard (events.Discard, never the cost form)
//     of a protected player's card.
func staticForbidsHostEvent(e edhbStaticBoard, ev events.Event) (string, bool) {
	if ev.Kind == events.FlipFace && ev.Text == "Transformed" {
		if o := e.Game().Obj(ev.Obj); o != nil && o.Zone == state.ZBattlefield && transformForbidden(e, ev.Obj) {
			return "cannot transform (CantTransform)", true
		}
		return "", false
	}
	if events.IsDiscard(ev) && !events.IsDiscardCost(ev) && discardForbidden(e, ev.Player, false) {
		return "cannot discard (CantDiscard)", true
	}
	return "", false
}
