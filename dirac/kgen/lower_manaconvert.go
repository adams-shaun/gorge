package kgen

import "github.com/adams-shaun/gorge/dirac/kop"

// vizierConvert is the ManaConvert parameter set that lowers to
// kop.StatAnyTypeCreatures (Vizier of the Menagerie): the controller may spend
// mana as though it were mana of any type to cast creature spells.
var vizierConvert = map[string]string{
	"Mode": "ManaConvert", "ValidPlayer": "You", "ValidCard": "Creature.YouCtrl",
	"ValidSA": "Spell", "ManaConversion": "AnyType->AnyType",
}

// anyColorConvert is the keyset of the spell-scoped any-colour ManaConvert
// (Oath of Nissa, Case File Auditor): ValidCard$ is read into the static's
// Filter, so it is the one parameter that varies.
var anyColorConvert = map[string]string{
	"Mode": "ManaConvert", "ValidPlayer": "You",
	"ValidSA": "Spell", "ManaConversion": "AnyType->AnyColor",
}

// allConvert is the keyset of the unscoped any-colour ManaConvert (Chromatic
// Orrery): no ValidSA$ and no ValidCard$, so it covers every mana payment its
// controller makes (kop.StatAnyColorAll; gorge manaConversionParts applies a
// ValidSA$-less static to spells and activations alike).
var allConvert = map[string]string{
	"Mode": "ManaConvert", "ValidPlayer": "You", "ManaConversion": "AnyType->AnyColor",
}

// manaConvertAnyColorFlag marks the lStatic of the any-colour form, which
// build_ab.go lowers to kop.StatAnyColorSpells (Filter from ValidCard$).
const manaConvertAnyColorFlag = "AnyColorSpells"

// manaConvertAllFlag marks the lStatic of the unscoped form, which
// build_ab.go lowers to kop.StatAnyColorAll (no Filter).
const manaConvertAllFlag = "AnyColorAll"

// manaConvertStatic lowers S:Mode$ ManaConvert for exactly vizierConvert's
// parameters (kop.StatAnyTypeCreatures) or anyColorConvert's plus a ValidCard$
// (kop.StatAnyColorSpells; CR 609.4b; gorge rules/mana_convert.go
// manaConversionParts). Every other ManaConvert shape (an Effect-granted,
// Optional$ or colourless-only one) refuses as before.
func (sc *staticCtx) manaConvertStatic() error {
	sp := sc.sp
	want := vizierConvert
	anyColor := sp.str("ManaConversion") == anyColorConvert["ManaConversion"]
	_, scoped := sp.m["ValidSA"]
	_, carded := sp.m["ValidCard"]
	all := anyColor && !scoped && !carded
	if anyColor {
		want = anyColorConvert
	}
	if all {
		want = allConvert
		anyColor = false
	}
	n := 0
	for k, v := range sp.m {
		if ignoredParams[k] {
			continue
		}
		if k == "ValidCard" && anyColor {
			continue
		}
		if w, ok := want[k]; !ok || w != v {
			return refuse("static mode ManaConvert")
		}
		n++
	}
	if n != len(want) || anyColor && sp.str("ValidCard") == "" {
		return refuse("static mode ManaConvert")
	}
	for k := range want {
		sp.consume(k)
	}
	if all {
		sc.st.Flags = append(sc.st.Flags, manaConvertAllFlag)
		return nil
	}
	if !anyColor {
		return nil
	}
	f, err := sc.readF("ValidCard")
	if err != nil {
		return err
	}
	sc.st.F = f
	sc.st.Flags = append(sc.st.Flags, manaConvertAnyColorFlag)
	return nil
}

// manaConvertOp is the built form of a lowered ManaConvert static: the
// unscoped kind (kop.StatAnyColorAll) has no operand; the spell-scoped
// any-colour kind (kop.StatAnyColorSpells) carries ValidCard$ as its Filter, a
// cast card read by the kernel's matchCard with no object (so its zone is not
// tested); Vizier's any-type kind has no operand.
func (cb *cardBuilder) manaConvertOp(s lStatic) (kop.StaticKind, kop.FilterID, error) {
	if len(s.Flags) == 1 && s.Flags[0] == manaConvertAllFlag {
		return kop.StatAnyColorAll, 0, nil
	}
	if len(s.Flags) != 1 || s.Flags[0] != manaConvertAnyColorFlag {
		return kop.StatAnyTypeCreatures, 0, nil
	}
	f, err := cb.filter(s.F, kop.ZoneHand)
	return kop.StatAnyColorSpells, f, err
}
