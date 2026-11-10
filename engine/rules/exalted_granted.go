package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// grantedKeywordInstances counts the instances of an attack-triggered keyword
// (Melee, Exalted) a granted walk must synthesize: every derived entry with
// that head, less one per printed Attacks line carrying the keyword while the
// printed face's abilities exist (printedLive: not face down, CR 708.2, nor
// lost, CR 613.1f), since the ordinary trigger walk queues those. Derived
// keeps duplicates, so a printed line plus a grant, or two grants, is one
// instance each (CR 113.2c).
func grantedKeywordInstances(derived []string, printed []cards.Trigger, head string, printedLive bool) int {
	n := 0
	for _, kw := range derived {
		if cards.KeywordHead(kw) == head {
			n++
		}
	}
	if printedLive {
		for _, t := range printed {
			if t.Mode == "Attacks" && t.ParamStr(cards.PKKeyword) == head {
				n--
			}
		}
	}
	return n
}

// grantedExaltedTrigger is the printed Exalted expansion's trigger shape
// (engine/cards/kw_exalted.go): a creature its holder's controller controls
// attacks alone. A granted instance (CR 702.83a, a layer-6 grant such as
// Sublime Archangel's "Other creatures you control have exalted") matches
// and expands per attacker exactly as the printed line does.
func grantedExaltedTrigger() (cards.Trigger, bool) {
	return cards.ParseTriggerLine("Mode$ Attacks | ValidCard$ Creature.YouCtrl | Alone$ True")
}

// rememberedRefIDs encodes a trigger's Remembered list as event IDs: a player
// entry as state.PlayerRef, so events.Apply rebuilds {Player, IsPlayer}
// rather than an object 0 that playersOf would drop.
func rememberedRefIDs(rem []state.Target) []state.ObjID {
	ids := make([]state.ObjID, 0, len(rem))
	for _, tgt := range rem {
		if tgt.IsPlayer {
			ids = append(ids, state.PlayerRef(tgt.Player))
			continue
		}
		ids = append(ids, tgt.Obj)
	}
	return ids
}

// ctxKeywordTriggerPush is the KeywordTriggerPush of a granted keyword
// instance whose stack object carries the walk's captured trigger context:
//
//   - Ward (CR 702.21a via a layer-6 AddKeyword$ Ward:<cost>; the printed
//     K:Ward expansion is a face trigger and never lands here). The ability
//     is mandatory; its Counter payload is the cost text, which events.Apply
//     rebuilds into the printed DB$ Ward ability. effWard reads the targeting
//     spell off the captured TriggeredStack role at resolution.
//   - Exalted (CR 702.83a-b, no printed trigger index). Remembered (the
//     declaration's attackers and the defending player) rides IDs, so replay
//     and stack copies rebuild the printed Pump of the same attacker; the
//     attacker role rides the captured context.
func ctxKeywordTriggerPush(pt pendingTrigger) (events.Event, bool) {
	switch {
	case pt.Ward != "":
		return events.Event{Kind: events.KeywordTriggerPush, Player: pt.Controller,
			Obj: pt.Source, Counter: "__kwWard:" + pt.Ward, Text: "ward ability"}, true
	case pt.Exalted:
		return events.Event{Kind: events.KeywordTriggerPush, Player: pt.Controller,
			Obj: pt.Source, Counter: "__kwExaltedGranted", IDs: rememberedRefIDs(pt.Ctx.Remembered),
			Text: "exalted ability"}, true
	}
	return events.Event{}, false
}
