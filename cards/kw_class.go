// Keyword expansion split out of keywords.go so that tickets touching
// different keywords stop colliding on one file. Each expander is
// registered by head; a duplicate head panics (registerKeyword).

package cards

import (
	"strconv"
	"strings"
)

// kwClass expands the Class enchantment keyword (CR 702.118). The script form
// is one line per level above the first:
//
//	K:Class:<level>:<cost>:Add<Kind>$ <SVar-name> [ | Add<Kind>$ <SVar-name> ]...
//
// A Class enters with one level counter (CR 702.118a: "each Class ... has
// level 1"), and each line adds two things:
//
//   - a level-up activator, "<cost>: Gain the next level as a sorcery" --
//     the same ordinary sorcery-speed PutCounter | CounterType$ LEVEL shape
//     kw:Level up (CR 702.87) already expands to, gated on the Class's level
//     being BELOW this line's level (CR 702.118b: "you may activate a level
//     ability ... only if this Class's level is less than N"), so a Class at
//     level 2 is offered the level-3 activator and no longer the level-2 one;
//   - the granted ability itself, appended to the face with a level gate
//     `IsPresent$ Card.Self+counters_GE<N>_LEVEL` so it becomes live exactly
//     when the Class's level reaches N. AddStaticAbility$ names a static body
//     (Mode$ Continuous and Mode$ ReduceCost/RaiseCost/SetCost both occur in
//     the corpus), AddTrigger$ a trigger body and AddReplacementEffect$ a
//     replacement body; each is parsed with the SAME reader the printed line
//     uses (ParseStaticLine/ParseTriggerLine/parseParams), so the grant runs
//     through the ordinary static/trigger/replacement machinery with no
//     Class-specific consumer anywhere.
//
// Appending the parsed body directly (rather than wrapping it in the
// `Mode$ Continuous | AddStaticAbility$ ...` grant static the Exploration
// Broodship shape uses) is what makes a ReduceCost body work: layers.go's
// AddStaticAbility$ grant branch emits only an inner `Mode$ Continuous`
// static through staticEffects, while collectCostStatics reads the face's
// OWN statics -- so a directly-appended `Mode$ ReduceCost` is picked up by
// the cost-static collector and a directly-appended `Mode$ Continuous` by the
// layer walk.
//
// The keyword is idempotent per LINE (KeywordLine tag) and the entry counter
// once per face, so a second Link() of a cached face neither double-adds a
// level-up activator nor re-grants an ability.
func kwClass(f *Face, i int, k, head, param string, has func(kind, line string) bool) {
	// param is "<level>:<cost>:<body>"; more than three colon fields do not
	// occur (the body itself may carry further ` | Key$ value` segments).
	levelStr, rest, ok := strings.Cut(param, ":")
	if !ok {
		return
	}
	level, err := strconv.Atoi(strings.TrimSpace(levelStr))
	if err != nil || level < 2 {
		// A malformed level (or level 1, which the entry counter already
		// provides) grants nothing -- fail closed rather than guess a band.
		return
	}
	cost, body, _ := strings.Cut(rest, ":")
	cost = strings.TrimSpace(cost)
	if cost == "" {
		return
	}

	// 1. The Class enters at level 1 (CR 702.118a). One replacement per face,
	// keyed on its own canonical tag so several K:Class lines -- each of which
	// runs this expander -- share the single entry counter.
	const entryTag = "Class#entry"
	if !has("R", entryTag) {
		const sv = "__kwClassEntry"
		f.setSVar(sv, "DB$ PutCounter | Defined$ Self | CounterType$ LEVEL | CounterNum$ 1 | ETB$ True")
		rp := parseParams("Event$ Moved | Destination$ Battlefield | ValidCard$ Card.Self | ReplacementResult$ Updated | ReplaceWith$ " + sv +
			" | Keyword$ Class | Description$ (This Class enters at level 1.)")
		rp["KeywordLine"] = entryTag
		f.Repls = append(f.Repls, Repl{Event: "Moved", Params: rp})
	}

	// 2. The level-up activator: an ordinary sorcery-speed counter placement,
	// offered only while this Class's level is below N.
	if !has("A", k) {
		gate := "Card.Self+counters_LT" + strconv.Itoa(level) + "_LEVEL"
		sa, _ := parseSA("", "AB$ PutCounter | Cost$ "+cost+" | Defined$ Self | CounterType$ LEVEL | CounterNum$ 1 | IsPresent$ "+gate+
			" | SorcerySpeed$ True | Keyword$ Class | SpellDescription$ Level "+strconv.Itoa(level))
		if sa != nil {
			sa.Params["KeywordLine"] = k
			f.Abilities = append(f.Abilities, sa)
		}
	}

	// 3. The granted ability, live from level N on. The value of each Add* key
	// may name several bodies joined with " & " (SMayLook & SMayPlay), and a
	// line may carry several Add* keys at once (ProdigysWill | AddTrigger$ ...),
	// so every segment is read independently.
	grantGate := "Card.Self+counters_GE" + strconv.Itoa(level) + "_LEVEL"
	for _, seg := range strings.Split(body, "|") {
		key, val, ok := strings.Cut(strings.TrimSpace(seg), "$")
		if !ok {
			continue
		}
		key, val = strings.TrimSpace(key), strings.TrimSpace(val)
		if val == "" {
			continue
		}
		switch key {
		case "AddStaticAbility":
			for _, name := range splitGrantNames(val) {
				inner, ok := ParseStaticLine(f.SVars[name])
				if !ok {
					continue
				}
				tag := k + "#static#" + name
				if has("S", tag) {
					continue
				}
				addLevelGate(inner.Params, grantGate)
				inner.Params["KeywordLine"] = tag
				inner.Params["Keyword"] = "Class"
				f.Statics = append(f.Statics, inner)
			}
		case "AddTrigger":
			for _, name := range splitGrantNames(val) {
				tr, ok := ParseTriggerLine(f.SVars[name])
				if !ok {
					continue
				}
				tag := k + "#trigger#" + name
				if has("T", tag) {
					continue
				}
				addLevelGate(tr.Params, grantGate)
				tr.Params["KeywordLine"] = tag
				tr.Params["Keyword"] = "Class"
				f.Triggers = append(f.Triggers, tr)
			}
		case "AddReplacementEffect":
			for _, name := range splitGrantNames(val) {
				rp := parseParams(f.SVars[name])
				if ev := strings.TrimSpace(rp["Event"]); ev == "" {
					continue
				}
				tag := k + "#repl#" + name
				if has("R", tag) {
					continue
				}
				addLevelGate(rp, grantGate)
				rp["KeywordLine"] = tag
				rp["Keyword"] = "Class"
				f.Repls = append(f.Repls, Repl{Event: strings.TrimSpace(rp["Event"]), Params: rp})
			}
		}
	}
}

// splitGrantNames splits an Add*$ value into the one or more SVar names it
// names, joined by Forge's " & " list separator (SMayLook & SMayPlay, and the
// multi-part AddStaticAbility$ ProdigysWill line's single name). Empty members
// are dropped.
func splitGrantNames(v string) []string {
	var out []string
	for _, part := range strings.Split(v, "&") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

// addLevelGate attaches a Class level band as an intervening-if / continuous
// gate on a granted body. IsPresent$ is preferred; a body that already carries
// one gets the band as IsPresent2$, which every gate family
// (continuousGateHolds, triggerConditionHolds, replacementConditionHolds)
// evaluates alongside the first -- so a body with its own existence gate keeps
// it rather than having it overwritten.
func addLevelGate(params map[string]string, spec string) {
	if strings.TrimSpace(params["IsPresent"]) == "" {
		params["IsPresent"] = spec
	} else {
		params["IsPresent2"] = spec
	}
}

func init() { registerKeyword(kwClass, "Class") }
