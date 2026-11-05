package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// The three printed K:Prevent sentences engine/cards/kw_prevent.go registers
// (its init list). The sentence has no colon, so KeywordHead keeps the whole
// line as the head; matching it here is what turns the printed keyword into a
// CardDef fact instead of the default arm's refusal.
const (
	preventKWCombatTo      = "Prevent all combat damage that would be dealt to CARDNAME."
	preventKWCombatToAndBy = "Prevent all combat damage that would be dealt to and dealt by CARDNAME."
	preventKWAnyTo         = "Prevent all damage that would be dealt to CARDNAME."
)

// lowerPreventKeyword records one of the printed K:Prevent sentences as the
// card-def facts the kernel reads. The keyword also mints bodyless DamageDone
// prevention R: lines (engine/cards/kw_prevent.go); those are consumed by
// printedPreventRepl below, not lowered as replacements, because the kernel's
// per-card prevention bits replace this event.
func (lc *lowerCtx) lowerPreventKeyword(head string) bool {
	switch head {
	case preventKWCombatTo:
		lc.out.PrevCmbTo = true
	case preventKWCombatToAndBy:
		lc.out.PrevCmbTo, lc.out.PrevCmbBy = true, true
	case preventKWAnyTo:
		lc.out.PreventDmgTo = true
	default:
		return false
	}
	return true
}

// printedPreventRepl reports whether r is one of the bodyless DamageDone
// replacements kwPrevent mints. They carry Keyword$ Prevent and the sentence
// that produced them in KeywordLine$; the keyword's own CardDef fact already
// models the prevention, so the line must not also lower as a replacement
// (damageRepl would refuse it for having no ActiveZones$).
func printedPreventRepl(r cards.Repl) bool {
	if r.Event != "DamageDone" || r.With != nil ||
		strings.TrimSpace(r.Params["Keyword"]) != "Prevent" ||
		strings.TrimSpace(r.Params["Prevent"]) != "True" {
		return false
	}
	vt := strings.TrimSpace(r.Params["ValidTarget"])
	vs := strings.TrimSpace(r.Params["ValidSource"])
	combat := strings.TrimSpace(r.Params["IsCombat"]) == "True"
	switch strings.TrimSpace(r.Params["KeywordLine"]) {
	case preventKWCombatTo:
		return combat && vt == "Card.Self" && vs == ""
	case preventKWCombatToAndBy:
		return combat && (vt == "Card.Self" && vs == "" || vt == "" && vs == "Card.Self")
	case preventKWAnyTo:
		return !combat && vt == "Card.Self" && vs == ""
	}
	return false
}
