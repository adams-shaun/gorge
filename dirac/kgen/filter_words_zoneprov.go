package kgen

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The zone, printed-layout and base-P/T filter words (T13c). Each is a kernel
// FilterWord literal (dirac/filter_<name>.go), emitted through the clause's
// spill program (filter_spill.go), so a filter that lowers today keeps its
// struct form and only a filter that refused gains a program. inRealZone<X>
// is not a word: gorge's wordInZone reads it exactly as inZone<X>
// (engine/effects/filter_word.go:401-415), so lower_filterwords.go's
// W7InZone arm covers both spellings.

func init() {
	fwName[kop.FwSuspended] = "FwSuspended"
	fwName[kop.FwTopLibrary] = "FwTopLibrary"
	fwName[kop.FwDoubleFaced] = "FwDoubleFaced"
	fwName[kop.FwBasePowerCmp] = "FwBasePowerCmp"
	fwName[kop.FwBaseToughnessCmp] = "FwBaseToughnessCmp"

	// suspended: gorge's compiled `suspended` read (predicates["suspended"],
	// engine/effects/filter.go:637, consulted before wordSuspended): in
	// exile with a positive time counter, a Suspend card. The kernel reads
	// the card's DurSuspend record (dirac/filter_suspended.go). Its manifest
	// context is "card": a battlefield or stack matcher refuses it.
	fwArms["suspended"] = func(c *lClause, _ string) error {
		return c.fwLit(lSpillLit{W: kop.FwSuspended})
	}
	// TopLibrary: gorge wordTopLibrary (filter_word.go:887), the top card of
	// its owner's library. Only the bare spelling lowers: gorge reads no
	// argument of it, and TopGraveyard*/TopLibrary2 have no reader at all
	// (the unknown-word census; a kernel true-read would diverge from
	// gorge's fail-closed matcher).
	fwArms["TopLibrary"] = func(c *lClause, _ string) error {
		return c.fwLit(lSpillLit{W: kop.FwTopLibrary})
	}
	// DoubleFaced: gorge wordDoubleFaced (filter_word.go:949), the printed
	// two-faced layout.
	fwArms["DoubleFaced"] = func(c *lClause, _ string) error {
		return c.fwLit(lSpillLit{W: kop.FwDoubleFaced})
	}
	// basePower<op><n> and baseToughness<op><n> are parsed in basePTArm,
	// reached from fwArmOrUnknown (the bound is part of the spelling, so no
	// arm-table key exists for them): gorge numericPred's basePower/
	// baseToughness fields (engine/effects/filter.go:1747), the layer-7b
	// base P/T (filter_value.go:292). A constant bound only: a variable
	// bound has no word (cmpLit refuses x, as every spill word does).
}

// wasCastOriginN maps the enter-side cast-provenance spellings to their
// cmpWasCast origin N (wasCastAny when the word is the bare wasCast or its
// negation). wasCastFromYourGraveyardByYou is gorge castOriginTokens' ByYou
// spelling, the same read the plain YourGraveyard one takes (both read
// by == you; cast_provenance.go:573). !wasCastFromYourHandByYou is ByYou's
// negation: cast, from anywhere but a hand by you.
func wasCastOriginN(p string) int {
	switch p {
	case "!wasCast":
		return wasCastNot
	case "wasCastFromGraveyard":
		return wasCastGrave
	case "wasCastFromYourGraveyard", "wasCastFromYourGraveyardByYou":
		return wasCastYourGrave
	case "!wasCastFromYourHandByYou":
		return wasCastHand | wasCastNot
	}
	return wasCastAny
}

// spellOriginN maps the spell-side cast-provenance spellings to their
// cmpWasCastSpell origin N (spellHand default: wasCastFromYourHand, any
// caster -- gorge reads wasCastFromTheirHand as the same predicate,
// cast_provenance.go). !wasCastFromYourHandByYou is the ByYou negation
// (gorge conditions.go's '!'): cast, from anywhere but a hand by you.
// wasCastFromYourLibrary is gorge castOriginTokens: the caster casts from
// their own library (a library cast is always from the caster's own
// library), so by == you.
func spellOriginN(p string) int {
	switch p {
	case "!wasCastFromYourHand", "!wasCastFromTheirHand":
		return spellHand | spellNot
	case "wasCastFromYourHandByYou":
		return spellHandBy
	case "!wasCastFromYourHandByYou":
		return spellHandBy | spellNot
	case "wasCastFromExile", "!wasCastFromExile":
		if p[0] == '!' {
			return spellExile | spellNot
		}
		return spellExile
	case "wasCastFromYourLibrary":
		return spellLibrary
	}
	return spellHand
}

// basePTArm lowers a layer-7b base power or toughness compare
// (basePower<op><n>, baseToughness<op><n>; pre names the field) to its word
// literal.
func (c *lClause) basePTArm(p, pre string) error {
	w := kop.FwBasePowerCmp
	if pre == "baseToughness" {
		w = kop.FwBaseToughnessCmp
	}
	rest, ok := strings.CutPrefix(p, pre)
	if !ok || len(rest) < 3 {
		return fmt.Errorf("unknown property %q", p)
	}
	op, ok := opNamesCmp[rest[:2]]
	if !ok {
		return fmt.Errorf("unknown property %q", p)
	}
	n, err := strconv.Atoi(rest[2:])
	if err != nil || n < -32768 || n > 32767 {
		return fmt.Errorf("unknown property %q", p)
	}
	return c.fwLit(lSpillLit{W: w, Op: op, N: n})
}
