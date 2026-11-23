package oracle

// The T13b relation and keyword filter words. Flash reads the KwMask bit the
// kernel already keeps (the printed mask for a card; off the battlefield, where
// kgen refuses it; Hexproof stays refused, see TestWithHexproofStaysRefused);
// ChosenCtrl and EnchantedControllerCtrl read the source permanent
// (kop.FwChosenCtrl, kop.FwEnchantedControllerCtrl), which the harness's one
// source per player (the first permanent it holds) is never a chooser or an
// attachment of, so neither is read true: both are in atomdiff-unproven.txt.
// The printed-keyword words (kop.FwWithKeyword) are proven by their own focus
// seed, atomdiff_relations_focus_test.go.
func init() {
	for _, w := range []string{"Card.withFlash", "Card.withoutFlash"} {
		RegisterFilterAtom(FilterAtom{Name: "fw:" + w, GorgeSpec: w, Ctx: AtomAnyZone &^ AtomBattlefield})
	}
	for _, w := range []string{"Card.ChosenCtrl", "Card.EnchantedControllerCtrl"} {
		RegisterFilterAtom(FilterAtom{Name: "fw:" + w, GorgeSpec: w, Ctx: AtomBattlefield})
	}
}
