package rules

import (
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The S:Mode$ CantTransform, CantAttach, CantDiscard, MaxCounter and
// ManaBurn statics. Each is one check at a known point, registered here
// beside its reader:
//
//   - CantTransform (CR 701.28f; Immerwolf, Bound by Moonsilver): SetState
//     Mode$ Transform (effects' transformBlocked through Host.TransformBlocked)
//     and the CR 702.145 day/night face sync (dayNightFlips) leave a matching
//     permanent on its face.
//   - CantAttach (CR 301.5c / 701.3b / 704.5n; O-Naginata, Konda's Banner, Gate
//     Smasher, Goblin Brawler): the Equipment shape (cards.CantAttachEquipmentShape)
//     is refused by attachRefused and detached by attachmentSBAs; the Aura shapes
//     stay registered as stat:CantAttach.OtherShape.
//   - CantDiscard (Tamiyo, Collector of Tales): api:Discard's flush keeps a
//     protected player's cards in hand (Host.DiscardBlocked).
//   - MaxCounter (CR 704.5r; Rasputin Dreamweaver): the SBA removes all but N.
//   - ManaBurn (Yurlok of Scorch Thrash): a player losing unspent mana -- the
//     CR 500.4 step boundary or a DB$ DrainMana -- loses that much life.
func init() {
	effects.RegisterNonAPI("stat:CantTransform", "stat:CantAttach", "stat:CantDiscard", "stat:MaxCounter", "stat:ManaBurn")
}

var (
	cantTransformParamsKnown = state.NewNameSet("Mode", "ValidCard", "Description")
	cantDiscardParamsKnown   = state.NewNameSet("Mode", "ValidPlayer", "ValidCause", "ForCost", "Description")
	maxCounterParamsKnown    = state.NewNameSet("Mode", "ValidCard", "CounterType", "MaxNum", "Description")
	manaBurnParamsKnown      = state.NewNameSet("Mode", "Description")
)

// staticParamsKnown reports whether every parameter of a static line is one
// its reader evaluates: a line naming any other scoping is skipped (the
// permissive direction) rather than enforced blanket.
func staticParamsKnown(params map[string]string, known state.NameSet) bool {
	for k := range params {
		if !known.Has(k) {
			return false
		}
	}
	return true
}

// TransformBlocked implements effects.Host for the CantTransform static.
func (e *Engine) TransformBlocked(id state.ObjID) bool { return transformForbidden(e, id) }

// transformForbidden reports whether a live CantTransform static's
// ValidCard$ (read with the static's controller as "you" and its source's
// host as EnchantedBy) names the permanent id.
func transformForbidden(e *Engine, id state.ObjID) bool {
	for _, sv := range e.activeStatics("CantTransform") {
		if !staticParamsKnown(sv.Params, cantTransformParamsKnown) {
			continue
		}
		if spec := sv.ParamStr(cards.PKValidCard); spec != "" && e.matchesSpecFrom(spec, id, sv.Controller, sv.Source) {
			return true
		}
	}
	return false
}

// cantAttachForbids reports whether a live CantAttach static of the Equipment
// shape forbids attaching o to bearer: its ValidCard$ names o and its Target$
// names bearer, both read from the static's source and controller.
func cantAttachForbids(e *Engine, o, bearer *state.Object) bool {
	for _, sv := range e.activeStatics("CantAttach") {
		if !cards.CantAttachEquipmentShape(sv.Params) {
			continue
		}
		if e.matchesSpecFrom(sv.ParamStr(cards.PKValidCard), o.ID, sv.Controller, sv.Source) &&
			e.matchesSpecFrom(sv.ParamStr(cards.PKTarget), bearer.ID, sv.Controller, sv.Source) {
			return true
		}
	}
	return false
}

// DiscardBlocked implements effects.Host for the CantDiscard static: whether
// player p is protected from discarding by the cause in flight. ForCost$
// True restricts only a cost discard and ForCost$ False only an effect's;
// ValidCause$ is evaluated against the resolving wrapper (causeSpecAdmits),
// which only the effect path has, so a cause-scoped line never restricts a
// cost. ValidPlayer$ names the protected player.
func (e *Engine) DiscardBlocked(p state.PlayerID, forCost bool) bool {
	for _, sv := range e.activeStatics("CantDiscard") {
		if !staticParamsKnown(sv.Params, cantDiscardParamsKnown) {
			continue
		}
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
		if spec := sv.ParamStr(cards.PKValidPlayer); spec != "" && !effects.MatchesPlayerSpec(e.G, spec, p, sv.Controller) {
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
// capping one counter remove the excess once.
func maxCounterSBA(e *Engine, b *sbaBatch) {
	statics := e.activeStatics("MaxCounter")
	if len(statics) == 0 {
		return
	}
	for _, sv := range statics {
		if !staticParamsKnown(sv.Params, maxCounterParamsKnown) {
			continue
		}
		kind := strings.TrimSpace(sv.ParamStr(cards.PKCounterType))
		limit, err := strconv.Atoi(strings.TrimSpace(sv.Params["MaxNum"]))
		spec := sv.ParamStr(cards.PKValidCard)
		if kind == "" || err != nil || limit < 0 || spec == "" {
			continue
		}
		for _, p := range e.G.AliveFrom(0) {
			for _, id := range e.G.Zone(state.ZBattlefield, p) {
				o := e.G.Obj(id)
				if o == nil || o.PhasedOut || !e.matchesSpecFrom(spec, id, sv.Controller, sv.Source) {
					continue
				}
				if excess := o.Counter(kind) - queuedCounterRemoval(b, id, kind) - int32(limit); excess > 0 {
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

// manaBurnActive reports whether a live ManaBurn static is on the battlefield.
func manaBurnActive(e *Engine) bool {
	for _, sv := range e.activeStatics("ManaBurn") {
		if staticParamsKnown(sv.Params, manaBurnParamsKnown) {
			return true
		}
	}
	return false
}

// emitManaClearBurning emits one ManaClear and, under a live ManaBurn static,
// makes its player lose as much life as the clear took from the pool (mana
// a stat:UnspentMana keep or a persistent unit kept is not lost). A clear
// that parked a replacement order choice burns nothing here: the loss is not
// settled yet.
func emitManaClearBurning(e *Engine, ev events.Event) {
	if !validManaClearPlayer(e, ev.Player) || !manaBurnActive(e) {
		e.emit(ev)
		return
	}
	before := e.G.Players[ev.Player].Pool.Total()
	choicesBefore := len(e.replChoices)
	e.emit(ev)
	if len(e.replChoices) > choicesBefore {
		return
	}
	if lost := before - e.G.Players[ev.Player].Pool.Total(); lost > 0 {
		e.emit(events.Event{Kind: events.LifeChange, Player: ev.Player, Amount: -int32(lost)})
	}
}

// validManaClearPlayer reports whether p names a seat of the game.
func validManaClearPlayer(e *Engine, p state.PlayerID) bool {
	return int(p) >= 0 && int(p) < len(e.G.Players)
}
