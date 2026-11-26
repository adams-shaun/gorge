package kgen

import (
	"strconv"
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
// DB$ Cleanup is the flag, not a program. The cost is a tap with an optional
// generic fee (Nykthos's "{2}, {T}"); every other shape of the chain (Rhystic
// Cave's UnlessCost$, Foraging Wickermaw's Animate, a cost beyond a tap and a
// generic fee, a second asking ability, a granted ability) is not matched here
// and refuses where it always did (chooseColorPrevent, manaEffect).

// askColourHeadParams, askColourManaParams and askColourCleanupParams are the
// only params the three nodes may carry. The unless form (Rhystic Cave) also
// allows InstantSpeed$ True on the head -- presentation on a CR 605.1a mana
// ability, which gorge ignores (its timing rule is moot) -- and the two unless
// params on the Mana node.
var (
	askColourHeadParams       = []string{"AB", "Cost", "AILogic", "AINoRecursiveCheck", "SubAbility", "SpellDescription"}
	askColourHeadUnlessParams = append(append([]string(nil), askColourHeadParams...), "InstantSpeed")
	askColourManaParams       = []string{"DB", "Produced", "Amount", "SubAbility"}
	askColourManaUnlessParams = append(append([]string(nil), askColourManaParams...), "UnlessCost", "UnlessPayer")
	askColourCleanupParams    = []string{"DB", "ClearChosenColor"}
)

// askColourUnless reports whether the Mana node carries the one unless shape
// kgen lowers: UnlessCost$ 1 | UnlessPayer$ Player (Rhystic Cave's "add one
// mana of that colour unless any player pays {1}"). Both params must be
// present; any other amount or payer is not this shape.
func askColourUnless(mana *cards.SA) bool {
	if mana == nil {
		return false
	}
	return strings.TrimSpace(mana.Params["UnlessCost"]) == "1" &&
		strings.TrimSpace(mana.Params["UnlessPayer"]) == "Player"
}

// askColourHasUnlessParams reports whether the Mana node carries either unless
// param at all, so the chain walk can refuse a partial or widened shape rather
// than silently lowering it as the devotion form.
func askColourHasUnlessParams(mana *cards.SA) bool {
	if mana == nil {
		return false
	}
	_, cost := mana.Params["UnlessCost"]
	_, payer := mana.Params["UnlessPayer"]
	return cost || payer
}

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

// askColourCost reports whether cost is a tap with an optional generic fee
// (a plain "{T}", or Nykthos's "{2}, {T}"): the kernel's colour-asking source
// prices the fee like a filter's activation and pays it before the ask
// (dirac/mana_askcolour.go askColFee, askColDo).
func askColourCost(cost string) bool {
	taps := 0
	for _, f := range strings.Fields(cost) {
		if f == "T" {
			taps++
			continue
		}
		for _, r := range f {
			if r < '0' || r > '9' {
				return false
			}
		}
		if f == "" {
			return false
		}
		n, err := strconv.Atoi(f)
		if err != nil || n > 255 {
			return false // the generic rides kop.ManaCost.Generic
		}
	}
	return taps == 1
}

// askColourManaChain reports whether sa is the colour-asking mana chain: the
// exact three nodes above, a plain cost, and a chain gorge classes as a mana
// ability (none of dbManaExcluded's reasons: no loyalty marker, no target, no
// library-moving cost or effect).
func askColourManaChain(sa *cards.SA) bool {
	if sa == nil || sa.Kind != "AB" || sa.API != "ChooseColor" || sa.Sub == nil || sa.Sub.API != "Mana" {
		return false
	}
	mana, clr := sa.Sub, sa.Sub.Sub
	if clr == nil || clr.API != "Cleanup" || clr.Sub != nil ||
		strings.TrimSpace(mana.Params["Produced"]) != "Chosen" || strings.TrimSpace(clr.Params["ClearChosenColor"]) != "True" ||
		!askColourCost(sa.Params["Cost"]) || dbManaExcluded(sa) || !cards.IsManaAbilitySA(sa) {
		return false
	}
	if askColourHasUnlessParams(mana) {
		// Rhystic Cave's shape only: the constant one-mana production and the
		// exact unless cost/payer. The devotion forms keep their own arm.
		if !askColourUnless(mana) || !paramsWithin(sa.Params, askColourHeadUnlessParams) ||
			!paramsWithin(mana.Params, askColourManaUnlessParams) || !paramsWithin(clr.Params, askColourCleanupParams) {
			return false
		}
		if amt := strings.TrimSpace(mana.Params["Amount"]); amt != "" && amt != "1" {
			return false
		}
		return true
	}
	return paramsWithin(sa.Params, askColourHeadParams) && paramsWithin(mana.Params, askColourManaParams) &&
		paramsWithin(clr.Params, askColourCleanupParams)
}

// askColourManaHead reports whether the AB sa being lowered is the
// colour-asking mana chain of a printed (not granted) ability. The ask may sit
// on a later mana ability (Nykthos's {2}, {T} beside its {T}: {C}), but a card
// carries at most one: the kernel records one asking ability index.
func (lc *lowerCtx) askColourManaHead(sa *cards.SA) bool {
	if lc.granted != 0 || !askColourManaChain(sa) {
		return false
	}
	for _, o := range lc.out.Abilities {
		if o.Kind == akMana && o.Mana != nil && o.Mana.AskColour {
			return false
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
// the colour chosen; or, for the unless form, one mana of it that is added
// only when no player pays {1} (Rhystic Cave; kop.Ability.ManaUnless).
func (lc *lowerCtx) lowerAskColourMana(sa *cards.SA, ps *params, ab *lAbility) error {
	ps.consume("AILogic", "AINoRecursiveCheck", "SubAbility")
	if askColourHasUnlessParams(sa.Sub) {
		if !askColourUnless(sa.Sub) {
			return refuse("ChooseColor mana ability UnlessCost$ %q | UnlessPayer$ %q", sa.Sub.Params["UnlessCost"], sa.Sub.Params["UnlessPayer"])
		}
		if v, ok := ps.get("InstantSpeed"); ok && strings.TrimSpace(v) != "" && !strings.EqualFold(strings.TrimSpace(v), "True") {
			return refuse("ChooseColor InstantSpeed$ %q", v)
		}
		if v, ok := sa.Sub.Params["Amount"]; ok && strings.TrimSpace(v) != "1" {
			return refuse("ChooseColor mana ability Amount$ %q beside UnlessCost$", v)
		}
		ab.Mana = &lMana{Colors: "WUBRG", AskColour: true, Amount: cst(1), Unless: true}
		if left := ps.unread(); len(left) > 0 {
			return refuse("ChooseColor unread params %v", left)
		}
		return nil
	}
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
