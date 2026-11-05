package kgen

// vizierConvert is the one ManaConvert parameter set kgen lowers (Vizier of
// the Menagerie): the controller may spend mana as though it were mana of any
// type to cast creature spells.
var vizierConvert = map[string]string{
	"Mode": "ManaConvert", "ValidPlayer": "You", "ValidCard": "Creature.YouCtrl",
	"ValidSA": "Spell", "ManaConversion": "AnyType->AnyType",
}

// manaConvertStatic lowers S:Mode$ ManaConvert for exactly vizierConvert's
// parameters (CR 609.4b; gorge rules/mana_convert.go manaConversionParts);
// kop.StatAnyTypeCreatures carries it. Every other ManaConvert shape (an
// Effect-granted, Optional$ or colourless-only one) refuses as before.
func manaConvertStatic(sp *params) error {
	n := 0
	for k, v := range sp.m {
		if ignoredParams[k] {
			continue
		}
		if w, ok := vizierConvert[k]; !ok || w != v {
			return refuse("static mode ManaConvert")
		}
		n++
	}
	if n != len(vizierConvert) {
		return refuse("static mode ManaConvert")
	}
	for k := range vizierConvert {
		sp.consume(k)
	}
	return nil
}
