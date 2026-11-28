package kgen

import (
	"strings"

	"github.com/adams-shaun/gorge/engine/cards"
)

// Opposition Agent's two lines (CR 723.2; gorge searchControlRedirect and
// replacementMatches' FoundSearchingLibrary$ scope, engine/rules):
//
//   - the static `Mode$ Continuous | Affected$ Opponent |
//     ControlOpponentsSearchingLibrary$ You` lowers to kop.StatSearchControl:
//     an opponent's library-search pick is answered by the static's
//     controller (dirac/zone_searchcontrol.go);
//   - the replacement `Event$ Moved | ValidCard$ Card.OppOwn |
//     FoundSearchingLibrary$ True | Origin$ Library | ActiveZones$
//     Battlefield` whose body exiles the replaced card and grants a permanent
//     while-exiled may-play with MayPlayIgnoreColor$ lowers to
//     kop.ReplFoundSearchExile: each card an opponent's search moves out of
//     the library is exiled instead, playable by the replacement's controller.
//
// Each shape is read whole, by its exact printed spelling; any other key or
// value refuses by name.

// searchControlStatic lowers the ControlOpponentsSearchingLibrary$ static.
func (lc *lowerCtx) searchControlStatic(s cards.Static) error {
	if bad := exactShape(s.Params, map[string]string{
		"Mode": "Continuous", "Affected": "Opponent", "ControlOpponentsSearchingLibrary": "You",
	}); bad != "" {
		return refuse("ControlOpponentsSearchingLibrary static %s", bad)
	}
	lc.out.Statics = append(lc.out.Statics, lStatic{Kind: "SearchControl"})
	return nil
}

// foundSearchExile lowers the FoundSearchingLibrary$ Moved replacement; rp's
// keys are all read on success.
func (lc *lowerCtx) foundSearchExile(rp *params, w *cards.SA, rl *lRepl) error {
	if bad := exactShape(rp.m, map[string]string{
		"Event": "Moved", "ValidCard": "Card.OppOwn", "FoundSearchingLibrary": "True",
		"Origin": "Library", "ActiveZones": "Battlefield", "ReplaceWith": "",
	}); bad != "" {
		return refuse("repl-hard: FoundSearchingLibrary %s", bad)
	}
	if w == nil || w.API != "ChangeZone" || w.Sub == nil || w.Sub.API != "Effect" || w.Sub.Sub != nil {
		return refuse("repl-hard: FoundSearchingLibrary body shape")
	}
	if bad := exactShape(w.Params, map[string]string{
		"Hidden": "True", "Origin": "All", "Destination": "Exile", "Defined": "ReplacedCard",
	}); bad != "" {
		return refuse("repl-hard: FoundSearchingLibrary body %s", bad)
	}
	if bad := exactShape(w.Sub.Params, map[string]string{
		"Duration": "Permanent", "StaticAbilities": "", "RememberObjects": "ReplacedCard", "ForgetOnMoved": "Exile",
	}); bad != "" {
		return refuse("repl-hard: FoundSearchingLibrary effect %s", bad)
	}
	ss, err := lc.effectStaticBodies(w.Sub.Params["StaticAbilities"])
	if err != nil {
		return err
	}
	if len(ss) != 1 {
		return refuse("repl-hard: FoundSearchingLibrary effect statics")
	}
	if bad := exactShape(ss[0].Params, map[string]string{
		"Mode": "Continuous", "MayPlay": "True", "Affected": "Card.IsRemembered",
		"AffectedZone": "Exile", "MayPlayIgnoreColor": "True",
	}); bad != "" {
		return refuse("repl-hard: FoundSearchingLibrary may-play %s", bad)
	}
	rp.consume("ValidCard", "FoundSearchingLibrary", "Origin", "ActiveZones")
	rl.Kind = "FoundSearchExile"
	return nil
}

// exactShape reports the first key of m (in sorted order) that want does not
// name, or whose value differs from want's ("" in want: any value), or a key
// of want m lacks; "" when m is exactly the shape. The description and
// sub-ability links are free text.
func exactShape(m, want map[string]string) string {
	for _, k := range sortedParamKeys(m) {
		switch k {
		case "Description", "SpellDescription", "StackDescription", "SubAbility":
			continue
		}
		w, ok := want[k]
		if !ok {
			return "unread " + k + "$"
		}
		if w != "" && strings.TrimSpace(m[k]) != w {
			return k + "$ " + strings.TrimSpace(m[k])
		}
	}
	for _, k := range sortedParamKeys(want) {
		if _, ok := m[k]; !ok {
			return "without " + k + "$"
		}
	}
	return ""
}
