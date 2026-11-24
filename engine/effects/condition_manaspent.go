package effects

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// conditionManaSpentMet evaluates ConditionManaSpent$ / ConditionManaNotSpent$
// ("if {G} was spent to cast this spell", CR 601.2h / 608.2c) against the
// per-colour spend the payment recorded on the host (state.Object.
// ManaColorSpent, which survives stack -> battlefield for a trigger that
// resolves from the permanent). A value is a space-separated colour list
// meaning ALL listed colours were spent (Forge's hasAllColors). A source-less
// context or a zero vector means nothing was spent: ManaSpent is false and
// ManaNotSpent true, Forge's null-castSA arms. resolved is false for a token
// naming no colour.
func conditionManaSpentMet(c *Ctx, g *state.Game, spentKey, notSpentKey string) (met, resolved bool) {
	var spent state.Mana
	if o := g.Obj(c.Source); o != nil {
		spent = o.ManaColorSpent
	}
	return ManaColourSpendGate(spent, spentKey, notSpentKey)
}

// ManaColourSpendGate answers the shared per-colour read behind the
// ConditionManaSpent$ / ConditionManaNotSpent$ ability gate
// (conditionManaSpentMet) and the trigger-side ManaSpent$ / ManaNotSpent$
// clause (rules triggerConditionHolds): the ONE colour-list parser both
// spellings evaluate, so they cannot drift apart. spent is the recorded
// per-colour spend; a value is a space-separated colour list meaning ALL
// listed colours were spent (Forge's hasAllColors). A zero vector means
// nothing was spent: ManaSpent is false and ManaNotSpent true, Forge's
// null-castSA arms. ok is false for a token naming no colour.
func ManaColourSpendGate(spent state.Mana, spentKey, notSpentKey string) (met, ok bool) {
	met = true
	for _, k := range []struct {
		val string
		not bool
	}{{spentKey, false}, {notSpentKey, true}} {
		if k.val == "" {
			continue
		}
		all := true
		for _, tok := range strings.Fields(k.val) {
			l := colourLetter(tok)
			if l == 0 {
				return false, false
			}
			if spent[state.ManaIndex(l)] <= 0 {
				all = false
			}
		}
		if all == k.not {
			met = false
		}
	}
	return met, true
}

// SelfETBInterveningManaSpent reports the ConditionManaSpent$ /
// ConditionManaNotSpent$ gate carried on a self-enters trigger's Execute body
// when the trigger's own description spells it as an intervening-if ("When
// CARDNAME enters, if {C} was spent to cast it, ..."): CR 603.4 checks that
// condition when the event happens, not only as the ability resolves. Forge
// puts the gate on the Execute SVar for Ogre Savant, Steamcore Weird and
// Revenant Patriarch, so the trigger line itself carries no ManaSpent$; the
// resolution-time read stays in conditionMet. Bodies that say
// "sacrifice it unless {C} was spent" (Azorius Herald, Court Hussar,
// Plaxmanta, Patagia Viper, Squealing Devil) are not intervening-ifs and are
// deliberately not matched. The keys come from the compiled
// ConditionParams (ActivationOf), the record the resolution read uses.
func SelfETBInterveningManaSpent(t cards.Trigger) (spent, notSpent string, ok bool) {
	if t.ModeKind() != cards.TriggerChangesZone {
		return "", "", false
	}
	d, ok := t.ParamCode(cards.PKDestination)
	if z, sole := Destination(d).SoleZone(); !ok || !sole || z != state.ZBattlefield {
		return "", "", false
	}
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(t.ParamStr(cards.PKValidCard))), "card.self") {
		return "", "", false
	}
	desc := strings.ToLower(t.ParamStr(cards.PKTriggerDescription))
	if !strings.Contains(desc, "if {") || !strings.Contains(desc, "was spent to cast it") {
		return "", "", false
	}
	if t.Effect == nil {
		return "", "", false
	}
	cond := &ActivationOf(t.Effect).Cond
	spent, notSpent = cond.ManaSpent, cond.ManaNotSpent
	if spent == "" && notSpent == "" {
		return "", "", false
	}
	return spent, notSpent, true
}
