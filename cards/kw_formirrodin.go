// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

func kwForMirrodin(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	// CR 702.159: "For Mirrodin! — When this Equipment enters,
	// create a 2/2 red Rebel creature token, then attach this to
	// it." Exactly the Living Weapon shape (an enters-the-
	// battlefield trigger on the Equipment itself, remembering
	// the token it mints so the chained Attach can name it);
	// only the token script (r_2_2_rebel) and the display text
	// differ. Forge puts no trailing parameter on K:For Mirrodin.
	if has("T", k) {
		return
	}
	f.setSVar("__kwFMAttach", "DB$ Attach | Defined$ Remembered | Object$ Self")
	f.addKeywordTrigger(head, k, "Mode$ ChangesZone | Destination$ Battlefield | ValidCard$ Card.Self | TriggerDescription$ For Mirrodin!",
		"DB$ Token | TokenScript$ r_2_2_rebel | TokenOwner$ You | RememberTokens$ True | SubAbility$ __kwFMAttach", has)
}

func init() { registerKeyword(kwForMirrodin, "For Mirrodin") }
