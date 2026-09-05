package cards

import (
	"strconv"
	"strings"
)

// expandKeywords turns each keyword the engine implements through ordinary
// machinery into the triggered ability, replacement effect or activated
// ability Forge itself expands it to (CardFactoryUtil, in spirit), tagged
// Params["Keyword"] so nothing downstream needs to know the difference. The
// SVars it adds start with "__kw" and cannot collide with a script's own.
// Idempotent: an already-tagged entry is never added twice. Keywords whose
// meaning is a casting option (Kicker, Surge, Flashback, Delve, Flash,
// Miracle) or a static property (Protection, Indestructible, Devoid) are
// not expanded: rules reads them directly.
func (f *Face) expandKeywords() {
	if f.SVars == nil {
		f.SVars = map[string]string{}
	}
	has := func(kind, kw string) bool {
		switch kind {
		case "T":
			for _, t := range f.Triggers {
				if t.Params["Keyword"] == kw {
					return true
				}
			}
		case "R":
			for _, r := range f.Repls {
				if r.Params["Keyword"] == kw {
					return true
				}
			}
		case "A":
			for _, a := range f.Abilities {
				if a.Params["Keyword"] == kw {
					return true
				}
			}
		}
		return false
	}
	for i, k := range f.Keywords {
		head := KeywordHead(k)
		param := ""
		if j := strings.IndexByte(k, ':'); j >= 0 {
			param = strings.TrimSpace(k[j+1:])
		}
		switch head {
		case "etbCounter":
			if has("R", head) {
				continue
			}
			kind, n, _ := strings.Cut(param, ":")
			sv := "__kwEtbCounter" + strconv.Itoa(i)
			f.SVars[sv] = "DB$ PutCounter | Defined$ Self | CounterType$ " + kind + " | CounterNum$ " + n + " | ETB$ True"
			f.Repls = append(f.Repls, Repl{Event: "Moved", Params: parseParams(
				"Event$ Moved | Destination$ Battlefield | ValidCard$ Card.Self | ReplacementResult$ Updated | ReplaceWith$ " + sv +
					" | Keyword$ etbCounter | Description$ CARDNAME enters with " + n + " " + kind + " counters.")})
		case "ETBReplacement":
			if has("R", head) {
				continue
			}
			// param is "Copy:<SVar>" or "Other:<SVar>", occasionally
			// followed by further colon-separated fields real Forge reads
			// for its own bookkeeping (Mandatory/Optional, a valid-zone
			// spec, a filter): those are not part of the SVar name, so only
			// the field right after the layer tag is taken.
			_, rest, _ := strings.Cut(param, ":")
			sv, _, _ := strings.Cut(rest, ":")
			f.Repls = append(f.Repls, Repl{Event: "Moved", Params: parseParams(
				"Event$ Moved | Destination$ Battlefield | ValidCard$ Card.Self | ReplacementResult$ Updated | ReplaceWith$ " + sv + " | Keyword$ ETBReplacement")})
		case "Undying":
			f.addKeywordTrigger(head, "Mode$ ChangesZone | Origin$ Battlefield | Destination$ Graveyard | ValidCard$ Card.Self+counters_EQ0_P1P1 | TriggerDescription$ Undying",
				"DB$ ChangeZone | Defined$ TriggeredNewCardLKICopy | Origin$ Graveyard | Destination$ Battlefield | WithCountersType$ P1P1 | WithCountersAmount$ 1", has)
		case "Evolve":
			f.addKeywordTrigger(head, "Mode$ ChangesZone | Destination$ Battlefield | ValidCard$ Creature.YouCtrl+Other | Evolve$ True | TriggerDescription$ Evolve",
				"DB$ PutCounter | Defined$ Self | CounterType$ P1P1 | CounterNum$ 1", has)
		case "Exalted":
			f.addKeywordTrigger(head, "Mode$ Attacks | ValidCard$ Creature.YouCtrl | Alone$ True | TriggerDescription$ Exalted",
				"DB$ Pump | Defined$ TriggeredAttacker | NumAtt$ +1 | NumDef$ +1", has)
		case "Prowess":
			f.addKeywordTrigger(head, "Mode$ SpellCast | ValidCard$ Card.nonCreature | ValidActivatingPlayer$ You | TriggerDescription$ Prowess",
				"DB$ Pump | Defined$ Self | NumAtt$ +1 | NumDef$ +1", has)
		case "Storm":
			f.addKeywordTrigger(head, "Mode$ SpellCast | ValidCard$ Card.Self | TriggerZones$ Stack | TriggerDescription$ Storm",
				"DB$ CopySpellAbility | Defined$ TriggeredSpellAbility | Amount$ Count$ThisTurnCast/Minus1 | MayChooseTarget$ True", has)
		case "Living Weapon":
			if has("T", head) {
				continue
			}
			f.SVars["__kwLWAttach"] = "DB$ Attach | Defined$ Remembered | Object$ Self"
			f.addKeywordTrigger(head, "Mode$ ChangesZone | Destination$ Battlefield | ValidCard$ Card.Self | TriggerDescription$ Living weapon",
				"DB$ Token | TokenScript$ b_0_0_phyrexian_germ | TokenOwner$ You | RememberTokens$ True | SubAbility$ __kwLWAttach", has)
		case "Equip":
			if has("A", head) {
				continue
			}
			sa, _ := parseSA("", "AB$ Attach | Cost$ "+param+" | ValidTgts$ Creature.YouCtrl | TgtPrompt$ Select target creature you control | SorcerySpeed$ True | Keyword$ Equip | SpellDescription$ Equip "+param)
			if sa != nil {
				f.Abilities = append(f.Abilities, sa)
			}
		case "Enchant":
			if has("A", head) || f.SpellAbility() != nil {
				continue
			}
			sa, _ := parseSA("", "SP$ Attach | ValidTgts$ "+param+" | TgtPrompt$ Select target "+strings.ToLower(param)+" | Object$ Self | Keyword$ Enchant")
			if sa != nil {
				f.Abilities = append(f.Abilities, sa)
			}
		}
	}
}

// addKeywordTrigger appends one tagged T: line whose Execute$ is an SVar
// this function creates, unless the keyword was already expanded.
func (f *Face) addKeywordTrigger(kw, trigger, effect string, has func(kind, kw string) bool) {
	if has("T", kw) {
		return
	}
	sv := "__kw" + strings.ReplaceAll(kw, " ", "")
	f.SVars[sv] = effect
	p := parseParams(trigger + " | Execute$ " + sv + " | Keyword$ " + kw)
	f.Triggers = append(f.Triggers, Trigger{Mode: p["Mode"], Params: p})
}
