// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

func kwLivingWeapon(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	if has("T", k) {
		return
	}
	f.setSVar("__kwLWAttach", "DB$ Attach | Defined$ Remembered | Object$ Self")
	f.addKeywordTrigger(head, k, "Mode$ ChangesZone | Destination$ Battlefield | ValidCard$ Card.Self | TriggerDescription$ Living weapon",
		"DB$ Token | TokenScript$ b_0_0_phyrexian_germ | TokenOwner$ You | RememberTokens$ True | SubAbility$ __kwLWAttach", has)
}

func init() { registerKeyword(kwLivingWeapon, "Living Weapon") }
