package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// chooseColorResSub reports whether a mid-resolution ChooseColor's one
// sub-ability (nothing after it) reads the chosen colour in a shape the
// kernel models over the resolution's answer (ExtChooseColorRes: res.chosen
// and res.col): ProtectionAll Gains$ ChosenColor (Akroma's Blessing, Brave
// the Elements, Reverent Mantra; protectionAllChosen), or a ChosenColor
// filter word in a DestroyAll's ValidCards$ (Root Greevil), a
// ChangeZoneAll's ChangeType$ (Wash Out) or a Discard's DiscardValid$
// (Addle). gorge effChooseColor records the answer on the source, and
// chosenColorMatches / resolveGains read the source's, so the answer of
// this resolution is what each reads; the sub's own lowering refuses any
// other parameter.
func chooseColorResSub(n *cards.SA) bool {
	if n == nil || n.Sub != nil {
		return false
	}
	switch n.API {
	case "ProtectionAll":
		return strings.EqualFold(strings.TrimSpace(n.Params["Gains"]), "ChosenColor")
	case "DestroyAll":
		return readsChosenColor(n.Params["ValidCards"])
	case "ChangeZoneAll":
		return readsChosenColor(n.Params["ChangeType"])
	case "Discard":
		return readsChosenColor(n.Params["DiscardValid"])
	}
	return false
}

// readsChosenColor: some "."/"+"/","-separated word of the filter spec is
// ChosenColor.
func readsChosenColor(spec string) bool {
	for _, w := range strings.FieldsFunc(spec, func(r rune) bool { return r == '.' || r == '+' || r == ',' }) {
		if strings.TrimSpace(w) == "ChosenColor" {
			return true
		}
	}
	return false
}

// protectionAllChosen lowers ProtectionAll Gains$ ChosenColor after
// ChooseColorRes (gorge effProtectionAll, grantProtectionUntilEOT): every
// battlefield permanent matching ValidCards$ as it resolves gains
// protection from the chosen colour until end of turn: OpExt ExtColorGrant
// over the filter, C bit 2. Any other Gains$, a player half
// (ValidPlayers$) or a Choices$ list refuses.
func (lc *lowerCtx) protectionAllChosen(ps *params) error {
	if !lc.colGrant || !strings.EqualFold(ps.str("Gains"), "ChosenColor") {
		return refuse("ProtectionAll Gains$ %q outside a chosen-colour grant", ps.str("Gains"))
	}
	for _, k := range [...]string{"ValidPlayers", "Choices", "Duration", "Defined"} {
		if _, ok := ps.get(k); ok {
			return refuse("chosen-colour ProtectionAll %s$", k)
		}
	}
	vc := ps.str("ValidCards")
	if vc == "" {
		return refuse("chosen-colour ProtectionAll without ValidCards$")
	}
	f, err := lc.filter(vc)
	if err != nil {
		return err
	}
	if f.Players != 0 {
		return refuse("chosen-colour ProtectionAll ValidCards$ %q names players", vc)
	}
	ps.consume("AILogic")
	lc.emit(lInstr{Op: "ColorGrant", F: f, A: cst(0), C: 4, Note: "until end of turn: each matching permanent has protection from the chosen colour"})
	return nil
}

// chooseColorRefusal names the sub-ability shape a mid-resolution
// ChooseColor refuses over: a Mana of the chosen colour (Nykthos, Shrine to
// Nyx: the mana ability's colour choice and its devotion amount), an
// Effect other than Prismatic Strands' prevention (Zombie Boa's
// chosen-colour blocked trigger), a chain longer than one, or another API.
func chooseColorRefusal(n *cards.SA) string {
	switch {
	case n == nil:
		return ""
	case n.API == "Mana":
		return ": a Mana ability of the chosen colour"
	case n.API == "Effect" && n.Sub == nil:
		return ": an Effect other than the chosen-colour prevention"
	case n.Sub != nil:
		return ": a " + n.API + " sub-ability with more after it"
	}
	return ": a " + n.API + " sub-ability"
}
