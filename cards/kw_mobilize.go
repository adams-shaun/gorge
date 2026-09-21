// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

import "strings"

func kwMobilize(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	// CR 702.<mobilize>: "Whenever this creature attacks, create N
	// tapped and attacking 1/1 red Warrior creature tokens. Sacrifice
	// them at the beginning of the next end step." One Attacks
	// trigger (the Myriad shape, ValidCard$ Card.Self) whose effect
	// mints the Warrior tokens -- TokenTapped$ True is the ordinary
	// entry-tap path, TokenAttacking$ True the defender-marking path
	// (both in effects/token.go) -- and remembers every minted token
	// so ONE end-step delayed trigger can sacrifice the whole group
	// the way Encore's end-step sacrifice does. The registration
	// captures the resolving chain's Remembered, and the fired
	// ability's Defined$ DelayTriggerRememberedLKI resolves it, so a
	// token that already left the battlefield (killed in combat) is
	// simply not among the survivors Sacrifice moves. The trigger's
	// SVar names key on the full keyword line, the addKeywordTrigger
	// convention, so two Mobilize lines on one face cannot collide.
	if has("T", k) {
		return
	}
	sv := "__kw" + strings.ReplaceAll(k, " ", "")
	f.setSVar(sv+"Delay", "DB$ DelayedTrigger | Mode$ Phase | Phase$ End of Turn | Execute$ "+sv+"Sacrifice | RememberChain$ False")
	f.setSVar(sv+"Sacrifice", "DB$ Sacrifice | Defined$ DelayTriggerRememberedLKI")
	f.addKeywordTrigger(head, k, "Mode$ Attacks | ValidCard$ Card.Self | TriggerDescription$ Mobilize",
		"DB$ Token | TokenScript$ r_1_1_warrior | TokenTapped$ True | TokenAttacking$ True | TokenAmount$ "+param+" | RememberTokens$ True | SubAbility$ "+sv+"Delay", has)
}

func init() { registerKeyword(kwMobilize, "Mobilize") }
