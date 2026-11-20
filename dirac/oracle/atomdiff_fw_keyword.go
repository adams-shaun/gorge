package oracle

// The keyword filter family: with<Keyword>, without<Keyword> and the '!'
// negation of with<Keyword>, which kgen lowers over a permanent's derived
// keywords (kop.PredLacksKw for the negations), and hasKeywordFlash, which
// gorge registers as withFlash. withFlying has its own flag in kgen and is in
// atomdiff_fw_base.go.
func init() {
	for _, k := range []string{"Defender", "Deathtouch", "Trample", "Haste", "Menace", "Vigilance", "Reach", "Lifelink", "Indestructible", "FirstStrike", "DoubleStrike", "Shadow"} {
		for _, w := range []string{"Card.with" + k, "Card.without" + k, "Card.!with" + k} {
			RegisterFilterAtom(FilterAtom{Name: "fw:" + w, GorgeSpec: w, Ctx: AtomBattlefield})
		}
	}
	RegisterFilterAtom(FilterAtom{Name: "fw:Card.!withFlying", GorgeSpec: "Card.!withFlying", Ctx: AtomBattlefield})
	RegisterFilterAtom(FilterAtom{Name: "fw:Card.hasKeywordFlash", GorgeSpec: "Card.hasKeywordFlash", Ctx: AtomAnyZone &^ AtomBattlefield})
	// the printed-keyword reads (kop.W7WithWarp ...): a card's printed
	// keyword wherever it is, so every zone
	for _, w := range []string{"Card.withWarp", "Card.withDisguise", "Card.withFlashback", "Card.withInfect"} {
		RegisterFilterAtom(FilterAtom{Name: "fw:" + w, GorgeSpec: w, Ctx: AtomAnyZone})
	}
}
