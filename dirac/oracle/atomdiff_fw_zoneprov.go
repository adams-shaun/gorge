package oracle

// The T13c zone, layout and base-P/T filter words. TopLibrary reads only
// off the battlefield (gorge wordTopLibrary: the top card of its owner's
// library, engine/effects/filter_word.go:887) and kgen refuses it in a
// battlefield or stack matcher (its manifest context is "card"); the
// base-P/T compares read everywhere but the stack, whose spell entries carry
// no manifest context (no carrier names them there). The inRealZone
// spellings are gorge's own (wordInZone trims, filter_word.go:401) and lower
// exactly as inZone, so they ride the W7InZone word. DoubleFaced is not
// registered: no seed set holds a transforming card, so the registry's
// totals could only read it false -- its focus seed proves it
// (atomdiff_zoneprov_focus_test.go), the printed-keyword words' route.
func init() {
	RegisterFilterAtom(FilterAtom{Name: "fw:Card.TopLibrary", GorgeSpec: "Card.TopLibrary",
		Ctx: AtomAnyZone &^ (AtomBattlefield | AtomStack)})
	for _, w := range []string{"Card.basePowerGE2", "Card.basePowerEQ1", "Card.baseToughnessLE3"} {
		RegisterFilterAtom(FilterAtom{Name: "fw:" + w, GorgeSpec: w, Ctx: AtomAnyZone &^ AtomStack})
	}
	for _, w := range []string{"Card.inRealZoneBattlefield", "Card.inRealZoneGraveyard", "Card.inRealZone Graveyard"} {
		RegisterFilterAtom(FilterAtom{Name: "fw:" + w, GorgeSpec: w, Ctx: AtomAnyZone})
	}
}
