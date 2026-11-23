package rules

import (
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// castInfoEmitter is the one method the pay-time CastInfo emitters need.
type castInfoEmitter interface {
	emit(events.Event) events.Event
}

// faceWantsActivationSpend is the heads-safety gate for the activation-payment
// record: it reports whether the face reads what an ACTIVATION spent
// (Count$TotalManaSpent <Type> -- Jetmir's Fixer, Forsworn Paladin -- or
// Count$ManaColorsPaid -- Paragon of Modernity). Only such a face stamps its
// activated abilities' stack objects, so no other activation changes an event.
func faceWantsActivationSpend(f *cards.Face) bool {
	return f != nil && (f.Mentions("Count$TotalManaSpent") || f.Mentions("Count$ManaColorsPaid"))
}

// emitSpendCastInfos emits the TOTAL mana spent, its snow part and its typed
// (Treasure/Cave/Desert/Artifact) parts as trailing pay-time CastInfos on obj,
// one per count, each carrying every earlier flag so events.Apply's CastInfo
// switch routes each Amount to its own field (newest flag first). It returns
// the flags after the total event, the value the per-colour CastInfo extends.
// Spells (payCast) and activated abilities (emitActivationSpend) share it.
func emitSpendCastInfos(em castInfoEmitter, obj state.ObjID, flags string, total, snow int32, typed [4]int32) string {
	flags = events.FlagsString(events.FlagsFrom(flags) | state.FlagManaSpent)
	em.emit(events.Event{Kind: events.CastInfo, Obj: obj, Amount: total, Counter: flags})
	snowFlags := events.FlagsString(events.FlagsFrom(flags) | state.FlagManaSnowSpent)
	em.emit(events.Event{Kind: events.CastInfo, Obj: obj, Amount: snow, Counter: snowFlags})
	typedFlags := [4]uint64{state.FlagManaTreasureSpent, state.FlagManaCaveSpent, state.FlagManaDesertSpent, state.FlagManaArtifactSpent}
	acc := events.FlagsFrom(flags)
	for t := range typedFlags {
		acc |= typedFlags[t]
		em.emit(events.Event{Kind: events.CastInfo, Obj: obj, Amount: typed[t], Counter: events.FlagsString(acc)})
	}
	return flags
}

// typedSpend folds the per-unit-tag spent deltas into the four public typed
// counts, in state.TypedManaTags order: a composed Artifact+Treasure/Cave/
// Desert unit counts toward both of its types (once per head).
func typedSpend(spentTyped [8]state.Mana) [4]int32 {
	return [4]int32{
		manaSpentTotal(spentTyped[state.TypedTreasure]) + manaSpentTotal(spentTyped[state.TypedArtifactTreasure]),
		manaSpentTotal(spentTyped[state.TypedCave]) + manaSpentTotal(spentTyped[state.TypedArtifactCave]),
		manaSpentTotal(spentTyped[state.TypedDesert]) + manaSpentTotal(spentTyped[state.TypedArtifactDesert]),
		manaSpentTotal(spentTyped[state.TypedArtifact]) + manaSpentTotal(spentTyped[state.TypedSnowArtifact]) +
			manaSpentTotal(spentTyped[state.TypedArtifactTreasure]) + manaSpentTotal(spentTyped[state.TypedArtifactCave]) +
			manaSpentTotal(spentTyped[state.TypedArtifactDesert]),
	}
}

// emitActivationSpend records what an activation's payment spent on the
// ability's own stack object (CR 602.2b / 601.2h): the total, snow and typed
// counts plus the per-colour vector, through the same CastInfo folds a spell's
// payment uses. The stack object (never the source permanent) carries it, so
// the permanent's own cast record stays intact, and the Count$TotalManaSpent /
// ManaColorsPaid heads read it back off the resolving ability. It must run
// after the ability's X CastInfo (whose fold resets the flags) and is
// emitted only for a face that reads it (faceWantsActivationSpend).
func emitActivationSpend(em castInfoEmitter, stackObj state.ObjID, spentAll, spentSnow state.Mana, spentTyped [8]state.Mana) {
	flags := emitSpendCastInfos(em, stackObj, "", manaSpentTotal(spentAll), manaSpentTotal(spentSnow), typedSpend(spentTyped))
	csFlags := events.FlagsString(events.FlagsFrom(flags) | state.FlagManaColorSpent)
	em.emit(events.Event{Kind: events.CastInfo, Obj: stackObj, Text: events.ManaColorSpentText(spentAll), Counter: csFlags})
}
