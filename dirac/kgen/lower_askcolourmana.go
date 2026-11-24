package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// A colour-asking mana ability (kop.Ability.ManaAskColour): Nyx Lotus's "{T}:
// Choose a color. Add an amount of mana of that color equal to your devotion
// to that color" and Nykthos, Shrine to Nyx's "{2}, {T}: ...". Forge scripts
// the chain AB$ ChooseColor -> DB$ Mana (Produced$ Chosen, Amount$
// Count$Devotion.Chosen) -> DB$ Cleanup (ClearChosenColor$ True); gorge's
// classifier takes it for a CR 605.1a mana ability (cards.IsManaAbilitySA:
// the chain reaches a DB$ Mana and none of the exclusions of
// dbManaExcluded holds) and resolves it off the stack, asking the colour at
// activation (lower_dbmanacontext.go).
//
// The chain lowers as ONE mana ability: the head's cost, the ask flag, ManaV
// the devotion-of-chosen amount, and no program. The record on the source and
// its clearing are the kernel's own (dirac/mana_askcolour.go askColDo), so the
// DB$ Cleanup is the flag, not a program. Every other shape of the chain
// (Rhystic Cave's UnlessCost$, Foraging Wickermaw's Animate, a cost beyond
// {T} such as Nykthos's "{2}, {T}", an ability that is not the card's first
// mana ability, a granted ability) is not matched here and refuses where it
// always did (chooseColorPrevent, manaEffect).

// askColourHeadParams, askColourManaParams and askColourCleanupParams are the
// only params the three nodes may carry.
var (
	askColourHeadParams    = []string{"AB", "Cost", "AILogic", "AINoRecursiveCheck", "SubAbility", "SpellDescription"}
	askColourManaParams    = []string{"DB", "Produced", "Amount", "SubAbility"}
	askColourCleanupParams = []string{"DB", "ClearChosenColor"}
)

// paramsWithin reports whether every key of m is among keys.
func paramsWithin(m map[string]string, keys []string) bool {
	for k := range m {
		found := false
		for _, w := range keys {
			found = found || k == w
		}
		if !found {
			return false
		}
	}
	return true
}

// askColourCost reports whether cost is the plain {T} the kernel's
// colour-asking source models (kop.Ability.ManaAskColour; prepManaAsk reads
// the card's first mana ability and a plain tap). Nykthos's "{2}, {T}"
// ability is a fee beside its first mana ability: the kernel prices it as a
// filter and lockstep diverges, so it stays refused.
func askColourCost(cost string) bool { return strings.TrimSpace(cost) == "T" }

// askColourManaChain reports whether sa is the colour-asking mana chain: the
// exact three nodes above, a plain cost, and a chain gorge classes as a mana
// ability (none of dbManaExcluded's reasons: no loyalty marker, no target, no
// library-moving cost or effect).
func askColourManaChain(sa *cards.SA) bool {
	if sa == nil || sa.Kind != "AB" || sa.API != "ChooseColor" || sa.Sub == nil || sa.Sub.API != "Mana" {
		return false
	}
	mana, clr := sa.Sub, sa.Sub.Sub
	return clr != nil && clr.API == "Cleanup" && clr.Sub == nil &&
		paramsWithin(sa.Params, askColourHeadParams) && paramsWithin(mana.Params, askColourManaParams) &&
		paramsWithin(clr.Params, askColourCleanupParams) &&
		strings.TrimSpace(mana.Params["Produced"]) == "Chosen" && strings.TrimSpace(clr.Params["ClearChosenColor"]) == "True" &&
		askColourCost(sa.Params["Cost"]) && !dbManaExcluded(sa) && cards.IsManaAbilitySA(sa)
}

// askColourManaHead reports whether the AB sa being lowered is the
// colour-asking mana chain of a printed (not granted) ability.
func (lc *lowerCtx) askColourManaHead(sa *cards.SA) bool {
	if lc.granted != 0 || !askColourManaChain(sa) {
		return false
	}
	for _, o := range lc.out.Abilities {
		if o.Kind == akMana {
			return false // the kernel reads the colour ask on the card's first mana ability only
		}
	}
	return true
}

// manaAbilityHead reports whether the AB sa lowers as a mana ability: an AB$
// Mana, an AB$ ManaReflected, the colour-asking chain or the chosen-colour
// mana ability (chosenColorManaShape).
func (lc *lowerCtx) manaAbilityHead(sa *cards.SA) bool {
	return sa.API == "Mana" || sa.API == "ManaReflected" || lc.askColourManaHead(sa) || chosenColorManaShape(sa)
}

// otherManaHead reports whether ab (kind akMana) has a head lowerManaHead
// lowers: no AB$ Mana, but ManaReflected or the colour-asking chain. A
// ChooseColor head with ab.Kind akMana is the chain: manaAbilityHead
// (askColourManaHead) is the only place that gives a ChooseColor head akMana.
func (lc *lowerCtx) otherManaHead(sa *cards.SA, ab *lAbility) bool {
	return ab.Kind == akMana && (sa.API == "ManaReflected" || sa.API == "ChooseColor")
}

// dynamicFlag is the ability flag a counted Amount$ sets: AfManaDynamic, but
// not for the colour-asking source (its amount reads the record its own ask
// makes, ExDevotionChosen; AfManaDynamic would put the ability in the
// planner's per-amount variants and take it out of the kernel's colour-asking
// route, dirac/mana_askcolour.go prepManaAsk).
func (m *lMana) dynamicFlag() kop.AbilityFlags {
	if m.AskColour {
		return 0
	}
	return kop.AfManaDynamic
}

// lowerManaHead lowers a mana ability whose head is no AB$ Mana: a
// ManaReflected one (lower_reflectedmana.go) or the colour-asking chain.
func (lc *lowerCtx) lowerManaHead(sa *cards.SA, ps *params, ab *lAbility) error {
	if sa.API == "ManaReflected" {
		return lc.lowerManaReflected(sa, ps, ab)
	}
	if chosenColorManaShape(sa) {
		return lc.lowerChooseColorMana(sa, ps, ab) // lower_choosecolmana.go
	}
	return lc.lowerAskColourMana(sa, ps, ab)
}

// lowerAskColourMana lowers the chain askColourManaHead matched into ab (kind
// akMana): the mana of any colour, asked at activation, ManaV the devotion to
// the colour chosen.
func (lc *lowerCtx) lowerAskColourMana(sa *cards.SA, ps *params, ab *lAbility) error {
	ps.consume("AILogic", "AINoRecursiveCheck", "SubAbility")
	amount := newParams(sa.Sub.Params)
	amt, err := lc.amount(amount, "Amount", 1)
	if err != nil {
		return err
	}
	if amt.Kind != vDevotionChosen {
		return refuse("ChooseColor mana ability Amount$ %q beyond Count$Devotion.Chosen", sa.Sub.Params["Amount"])
	}
	ab.Mana = &lMana{Colors: "WUBRG", AskColour: true, Amount: amt}
	if left := ps.unread(); len(left) > 0 {
		return refuse("unread params %v", left)
	}
	return nil
}
