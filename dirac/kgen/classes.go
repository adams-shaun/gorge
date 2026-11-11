package kgen

import (
	"regexp"
	"sort"
	"strings"
)

// Refusal classes and tiers. refusalClasses normalises
// a refusal reason into classes: one class per filter
// property, param, API, keyword, trigger mode, static mode, cost token,
// value head, `faces`, ... (a port of an earlier
// closure script's normaliser). classTier maps
// a class to its tier: G generator-only, K keyword desugaring, O op or
// predicate arm, M<n> mechanism, ? unclassified. This file is the only
// place a refusal is classified.

var (
	reAPIPrefix   = regexp.MustCompile(`^((?:DB|SP|AB)\$ \w+): `)
	reUnread      = regexp.MustCompile(`^unread params \[(.*)\]`)
	reUnknownProp = regexp.MustCompile(`unknown property "([^"]+)"`)
	reUnknownNeg  = regexp.MustCompile(`unknown negation "([^"]+)"`)
	reValueHead   = regexp.MustCompile(`^value head (\w+)\$`)
	reCount       = regexp.MustCompile(`Count\$([A-Za-z]+)`)
	reAPI         = regexp.MustCompile(`^api (\w+)`)
	reKeyword     = regexp.MustCompile(`^keyword "([^":]+)`)
	reTrigMode    = regexp.MustCompile(`^trigger mode (\w+)`)
	reStatMode    = regexp.MustCompile(`^static mode (\w+)`)
	reReplHard    = regexp.MustCompile(`^repl-hard: (.*)`)
	reCostToken   = regexp.MustCompile(`^cost token "([^"<:]+)`)
	reCtxUnread   = regexp.MustCompile(`^(trigger|static|Tap|SVar) unread params \[(.*)\]`)
	reQuoted      = regexp.MustCompile(`"[^"]*"`)
	reDigits      = regexp.MustCompile(`\d+`)
)

// refusalClasses returns the classes a refusal reason carries (usually one;
// an unread-params reason carries one per param).
func refusalClasses(reason string) []string {
	r, api := reason, ""
	for i := 0; i < 2; i++ { // an ability prefix, then a nested (charm) one
		if m := reAPIPrefix.FindStringSubmatch(r); m != nil {
			api = strings.SplitN(m[1], " ", 2)[1]
			r = r[len(m[0]):]
		}
	}
	if strings.HasPrefix(r, "multi-face card") {
		return []string{"faces"}
	}
	if m := reUnread.FindStringSubmatch(r); m != nil {
		if api == "" {
			api = "x"
		}
		var out []string
		for _, p := range strings.Fields(m[1]) {
			out = append(out, "param:"+api+"."+p)
		}
		return out
	}
	if m := reUnknownProp.FindStringSubmatch(r); m != nil {
		return []string{"prop:" + reDigits.ReplaceAllString(m[1], "N")}
	}
	if m := reUnknownNeg.FindStringSubmatch(r); m != nil {
		return []string{"prop:" + reDigits.ReplaceAllString(m[1], "N")}
	}
	if m := reValueHead.FindStringSubmatch(r); m != nil {
		return []string{"head:" + m[1]}
	}
	if m := reCount.FindStringSubmatch(r); m != nil {
		return []string{"count:" + m[1]}
	}
	for _, p := range []struct {
		re     *regexp.Regexp
		prefix string
	}{{reAPI, "api:"}, {reKeyword, "kw:"}, {reTrigMode, "trig:"}, {reStatMode, "stat:"}, {reReplHard, "repl:"}, {reCostToken, "cost:"}} {
		if m := p.re.FindStringSubmatch(r); m != nil {
			return []string{p.prefix + m[1]}
		}
	}
	if m := reCtxUnread.FindStringSubmatch(r); m != nil {
		var out []string
		for _, p := range strings.Fields(m[2]) {
			out = append(out, "param:"+m[1]+"."+p)
		}
		return out
	}
	r = reDigits.ReplaceAllString(reQuoted.ReplaceAllString(r, `"_"`), "N")
	if rs := []rune(r); len(rs) > 60 {
		r = string(rs[:60])
	}
	return []string{"other:" + r}
}

// classTiers seeds the tier of the
// 64 classes carried by >= 100 corpus cards). Classes not listed fall to
// classTier's prefix heuristic.
var classTiers = map[string]string{
	`faces`:                              "M10b",
	`other:Defined$ "_"`:                 "G",
	`other:discard cost "_"`:             "G",
	`api:Scry`:                           "O",
	`kw:Cycling`:                         "K",
	`kw:TypeCycling`:                     "K",
	`cost:ExileFromGrave`:                "O",
	`other:ChangesZone Battlefield->Any`: "O",
	`api:Regenerate`:                     "O",
	`other:filter "_": unknown filter head "_"`: "G",
	`cost:tapXType`:                          "M13",
	`other:Effect without statics`:           "O",
	`trig:DamageDoneOnce`:                    "M5",
	`other:AffectedZone "_" without MayPlay`: "O/M11",
	`kw:Chapter`:                             "M14",
	`kw:Crew`:                                "M13",
	`stat:CantAttack`:                        "O",
	`api:PutCounterAll`:                      "O",
	`api:RepeatEach`:                         "O",
	`api:PreventDamage`:                      "O",
	`api:RollDice`:                           "O",
	`kw:Landwalk`:                            "K",
	`kw:Morph`:                               "M10b",
	`kw:Devoid`:                              "K",
	`trig:ChaosEnsues`:                       "O",
	`count:Valid`:                            "G",
	`other:token refused`:                    "G",
	`other:phase "_"`:                        "G",
	`prop:AttachedBy`:                        "G",
	`prop:Legendary`:                         "G",
	`prop:White`:                             "G",
	`prop:Blue`:                              "G",
	`prop:Black`:                             "G",
	`prop:Red`:                               "G",
	`prop:Green`:                             "G",
	`other:filter "_": empty filter`:         "G",
	`param:static.AddTrigger`:                "O",
	`param:static.AddSVar`:                   "O",
	`param:Token.RememberTokens`:             "G",
	`param:Sacrifice.RememberSacrificed`:     "G",
	`other:static Condition "_"`:             "G",
	`trig:AttackerBlocked`:                   "O",
	`trig:Blocks`:                            "O",
	`trig:Taps`:                              "O",
	`trig:Sacrificed`:                        "O",
	`trig:ChangesZoneAll`:                    "M5",
	`repl:DamageDone replacement shape`:      "O",
	`other:Untap replacement shape`:          "O",
	`other:ChangeType$ with ValidTgts$`:      "G",
	`other:Origin "_"`:                       "O",
	`other:Play shape`:                       "M11",
	`other:BecomesTarget shape`:              "M12",
	`other:CDA shape`:                        "M10a",
	`cost:Return`:                            "O",
	`kw:Convoke`:                             "K",
	`kw:Improvise`:                           "K",
	`kw:Protection`:                          "K",
	`count:ThisTurnCast`:                     "M12",
	`other:PutCounter without CounterType`:   "G",
	`other:discard mode "_"`:                 "G",
	`param:Tap.ConditionCompare`:             "G",
	`param:Tap.ConditionPresent`:             "G",
	`param:Animate.Colors`:                   "G",
	`trig:TurnFaceUp`:                        "M10b",

	// the Targeted.Other PumpAll pair outside lower_pumpallother.go's shape
	`other:PumpAll Targeted.Other beyond a constant pump of the one tar`: "O",
}

// genOtherPrefixes are the `other:` classes the design session's
// closure2.py#tier counts as generator-only.
var genOtherPrefixes = []string{
	"other:Defined", "other:filter", "other:SVar", "other:phase", "other:Origin", "other:discard",
	"other:Destination", "other:ChangeType", "other:static Condition", "other:token refused",
	"other:mana symbol", "other:kicker", "other:alternate", "other:both Valid", "other:AttackingPlayer",
	"other:PutCounter without", "other:library position", "other:Effect Duration", "other:ChooseType",
	"other:override", "other:AddType", "other:SetColor", "other:CopyPermanent", "other:GainControl",
	"other:counter cost", "other:ReduceCost", "other:resolution mana",
}

// classTier returns a class's tier: the seeded table first, then
// closure2.py's heuristic (face mechanics; property, count, param and
// head classes and the listed `other:` shapes G; keywords K; API, trigger,
// static, cost and replacement classes O). Anything else is "?".
func classTier(class string) string {
	if t, ok := classTiers[class]; ok {
		return t
	}
	if class == "faces" || strings.HasPrefix(class, "prop:faceDown") || strings.Contains(class, "TurnFaceUp") ||
		strings.Contains(class, "Morph") || strings.Contains(class, "Disguise") || strings.Contains(class, "Transformed") {
		return "M10b"
	}
	for _, p := range []string{"prop:", "count:", "param:", "head:"} {
		if strings.HasPrefix(class, p) {
			return "G"
		}
	}
	for _, p := range genOtherPrefixes {
		if strings.HasPrefix(class, p) {
			return "G"
		}
	}
	if strings.HasPrefix(class, "kw:") {
		return "K"
	}
	for _, p := range []string{"api:", "trig:", "stat:", "cost:", "repl:"} {
		if strings.HasPrefix(class, p) {
			return "O"
		}
	}
	return "?"
}

// stdTier is classTier plus the Standard-census tier seeds. It is used by
// the -census-std report only; coverage.md keeps classTier.
func stdTier(class string) string {
	if t := classTier(class); t != "?" {
		return t
	}
	for _, p := range stdTierO {
		if strings.HasPrefix(class, p) {
			return "O"
		}
	}
	for _, p := range stdTierG {
		if strings.HasPrefix(class, p) {
			return "G"
		}
	}
	return "?"
}

// stdTierO seeds the Standard-census classes whose refusal class is an op or
// predicate arm. Trailing spaces are significant.
var stdTierO = []string{
	"other:Effect ", "other:RepeatEach", "other:AlterAttribute", "other:GenericChoice", "other:PutCounterAll",
	"other:AnimateAll", "other:Play ", "other:PeekAndReveal", "other:TwoPiles", "other:SetState", "other:DigUntil", "other:Dig ",
	"other:ChangesZoneAll", "other:ChangesZone ", "other:DamageDoneOnce", "other:StaticEffect", "other:Attach ",
	"other:Counter ", "other:CopySpellAbility", "other:ETB ", "other:delayed ", "other:replacement ", "other:Room door",
	"other:Token ", "other:Pump ", "other:ChooseColor", "other:MayPlay", "other:MayLookAt", "other:Drawn ", "other:Taps ",
	"other:player static", "other:Condition$", "other:ExcessSVar", "other:DamageSource$", "other:Activation ",
	"other:MaxSpeed", "other:waterbend", "other:CastWithFlash", "other:activation limit", "other:two activate-only-once",
	"other:N mana abilities", "other:gated mana ability", "other:granted mana ability", "other:mana ability with",
	"other:DB$ Mana", "other:AddTrigger", "other:equip shape",
}

// stdTierG seeds the Standard-census classes whose refusal class is a
// generator value, property, count, param or head. Trailing spaces are
// significant.
var stdTierG = []string{
	"other:mana ", "other:life cost", "other:ward cost", "other:flashback cost", "other:disguise cost", "other:warp cost",
	"other:exile cost", "other:return cost", "other:XXX cost", "other:resolution ", "other:instr ", "other:counter kind",
	"other:amount ", "other:empty amount", "other:power compare", "other:target count", "other:charm ", "other:multi-pick charm",
	"other:hybrid mana", "other:set P/T", "other:Remembered$", "other:ValidPlayer", "other:ValidActivatingPlayer",
	"other:MinMaxBlocker", "other:Hidden$",
	"other:ValidAttackersAmount", "other:PresentZone", "other:RaiseCost", "other:no target", "other:graveyard pick",
	"other:Graveyard,Exile pick", "other:hand move filter", "other:shuffled move", "other:search with", "other:optional ChooseCard",
	"other:two intervening-if", "other:DiscardValid$", "other:ThisTargetedController", "other:reads Remembered", "other:RememberTargets",
}

// classRow is one line of coverage.md's "Refusals by class and tier".
type classRow struct {
	class, tier string
	cards       int
}

// refusalClassRows counts, per class, the distinct refused cards carrying
// it, most-carried first (ties by class name).
func refusalClassRows(refused []refusal) []classRow {
	byClass := map[string]map[string]bool{}
	for _, r := range refused {
		for _, c := range refusalClasses(r.Reason) {
			if byClass[c] == nil {
				byClass[c] = map[string]bool{}
			}
			byClass[c][r.Card] = true
		}
	}
	rows := make([]classRow, 0, len(byClass))
	for c, cs := range byClass {
		rows = append(rows, classRow{class: c, tier: classTier(c), cards: len(cs)})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].cards != rows[j].cards {
			return rows[i].cards > rows[j].cards
		}
		return rows[i].class < rows[j].class
	})
	return rows
}
