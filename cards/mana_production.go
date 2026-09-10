package cards

import (
	"strconv"
	"strings"
)

// ManaProduction is a face's mana-production capability summary, expressed
// as plain data so any later package (view, botpolicy, the seat adapter) can
// carry it without importing a package to its right. It is derived from the
// face's own abilities -- never from land subtypes. A face with several mana
// abilities lists their possible production here; rules selects exactly one
// when a shared tap cost is paid (rules/mana_activation.go).
//
// Colour is indexed the same way state.Mana is (W, U, B, R, G, then
// colourless), so a caller that also imports state can translate an index
// with state.ManaIndex. A dual land may list both possible colours here, but
// selecting one of its distinct mana abilities on activation adds only that
// selected ability's mana.
//
// Any reports that at least one mana ability's Produced$ was not a plain
// colour string: "Any"/"Combo Any", a listed "Combo X Y" choice, or a
// "Chosen"/"Special" word. Such a source is conditional in the card script,
// so a policy must not treat it as a dependable colour fixer. Colour still
// mirrors every rune effMana emits: its unrecognised runes become colourless
// through state.ManaIndex, including the words in Combo and Chosen.
type ManaProduction struct {
	Colour [6]int32 `json:"colour"`
	Any    bool     `json:"any"`
	// Indeterminate reports that at least one mana ability on the face
	// carries a non-literal Amount$ ("X", "Y", "UrzaAmount", a Count$
	// expression, "Sacrificed$...") whose value the projection cannot
	// statically price. Such an ability yields no amount the pool is
	// guaranteed to receive -- the executor's Num (effects/count.go)
	// resolves it to an SVar/count/X that is zero on a plain tap-for-mana
	// activation or unknown at projection time -- so the collector claims
	// no colour slot from it (every contribution is zero). The flag is what
	// lets a policy distinguish "this source produces mana I can count on"
	// from "this source might produce something"; chooseTap must never
	// PREFER an indeterminate source over one that demonstrably produces.
	// Indeterminate is a statement about the AMOUNT, orthogonal to Any,
	// which is a statement about the PRODUCED colour choice.
	//
	// It is a server-only field: the bot policy reads it, but it must not
	// ride the human wire (CardView) -- no web component consumes
	// CardView.produces at all, so surfacing it as `indeterminate?: boolean`
	// in protocol.ts is dead payload on every card view. json:"-" keeps it
	// out of tsgen's jsonName (internal/tsgen/tsgen.go) while the Go field
	// stays for the two adapters and the policy that reads it.
	Indeterminate bool `json:"-"`
}

// manaProductionForm normalises Produced$ exactly as effMana does. Like
// botpolicy's braceForm, it is package-scoped: strings.Replacer is immutable
// and safe for concurrent use, while building its trie in this hot collector
// once per mana ability creates needless garbage on every projected board.
var manaProductionForm = strings.NewReplacer("{", "", "}", "", " ", "")

// manaAbilityAmount is the Amount$ a mana ability produces. It returns the
// amount and whether the amount is known at projection time:
//
//   - a blank Amount is the executor's own default of 1 (Num's def), known;
//   - a literal integer is used directly, known (negative clamped to 0,
//     effMana's T14-f clamp);
//   - anything else ("X", "Y", "UrzaAmount", a Count$ expression,
//     "Sacrificed$...") is a value the projection cannot price -- the
//     executor's Num resolves it through the SVar/count/$X machinery to a
//     count the collector has no game context for -- so it is returned
//     as (0, false): NOT a guaranteed amount. The collector must not
//     record a source as producing mana the pool will never be promised.
//
// The contract mirrors effMana exactly in the cases that are statically
// decidable (blank -> 1, literal -> literal) and diverges only where effMana
// would need a live game, where claiming a count would be a lie.
func manaAbilityAmount(a *SA) (int32, bool) {
	raw := strings.TrimSpace(a.Params["Amount"])
	if raw == "" {
		return 1, true
	}
	if v, err := strconv.Atoi(raw); err == nil {
		if v < 0 {
			v = 0
		}
		return int32(v), true
	}
	return 0, false
}

// add folds one mana ability's production into the collector. It mirrors
// effMana exactly: blank / "Any" / "Combo Any" become one C; otherwise every
// brace/space-stripped rune adds its matching WUBRG colour, or colourless for
// an unrecognised rune. Non-plain productions remain flagged Any because
// their script-level choice is not modelled, even where effMana's degenerate
// rune walk happens to emit a listed colour.
func (mp *ManaProduction) add(a *SA) {
	amt, known := manaAbilityAmount(a)
	if !known {
		// The amount this ability would add is not statically known, so it
		// contributes nothing to the guaranteed-production counts the policy
		// relies on; the Indeterminate flag carries that it might produce
		// something. Keep off the wire-claim path: an unknown-amplitude
		// source must never make ProducesColour true.
		mp.Indeterminate = true
	}
	raw := strings.TrimSpace(a.Params["Produced"])
	if raw == "" || raw == "Any" || raw == "Combo Any" {
		raw = "C"
		mp.Any = true
	}
	s := manaProductionForm.Replace(raw)
	for _, r := range s {
		switch r {
		case 'W':
			mp.Colour[0] += amt // amt is 0 for an indeterminate amount
		case 'U':
			mp.Colour[1] += amt
		case 'B':
			mp.Colour[2] += amt
		case 'R':
			mp.Colour[3] += amt
		case 'G':
			mp.Colour[4] += amt
		default:
			mp.Colour[5] += amt
		}
	}
	if s != "" && strings.Trim(s, "WUBRGC") != "" {
		mp.Any = true
	}
}

// ManaProduction returns the production capabilities of every mana ability
// on the face, folded together. It is derived once at load from the face's
// abilities (ManaAbilities), refreshed after ApplyIntrinsics adds abilities,
// so a card the intrinsic layer granted its mana from subtypes (a Plains, a
// dual with a Plains and an Island half) is covered by the same path as a card
// whose script spells the ability out.
func (f *Face) ManaProduction() ManaProduction { return f.manaProduction }

// Distinguishes the five coloured pool slots from colourless: the index of a
// colour the policy can spend on a coloured pip. Index 5 (colourless) is not a
// colour.
const manaColourStart, manaColourEnd = 0, 5

// DistinctColours is the number of distinct coloured (WUBRG) mana kinds the
// production yields. An Any production reports all five: it is conditional in
// script, but still must sort after a basic rather than masquerading as a
// zero-flexibility source. Otherwise a source that produces exactly one colour
// (a basic Plains) reports 1 and is spent before a dual that reports 2.
func (mp ManaProduction) DistinctColours() int {
	if mp.Any {
		return manaColourEnd - manaColourStart
	}
	n := 0
	for i := manaColourStart; i < manaColourEnd; i++ {
		if mp.Colour[i] > 0 {
			n++
		}
	}
	return n
}

// ProducesColour reports whether the production yields at least one mana of
// colour i (a WUBRG index; index 5, colourless, reports false -- a colourless
// source never pays a coloured pip).
func (mp ManaProduction) ProducesColour(i int) bool {
	return i >= manaColourStart && i < manaColourEnd && mp.Colour[i] > 0
}

// Colourless reports whether the production yields any colourless mana.
func (mp ManaProduction) Colourless() bool { return mp.Colour[5] > 0 }

// IsZero reports whether the production is empty -- a card with no mana
// ability, or one whose abilities produce nothing this build can count. It is
// what lets a caller keep a zero production off the wire (omitempty) rather
// than pay for a six-entry array on every card.
func (mp ManaProduction) IsZero() bool { return mp == ManaProduction{} }
