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
// Idempotent: an already-expanded keyword *line* (the full "Head:param:..."
// text, not just the head) is never added twice, so a face with two
// distinct K:Equip: lines (different costs or restrictions) still expands
// both -- ruling FL-13. Keywords whose meaning is a casting option (Kicker,
// Surge, Flashback, Delve, Flash, Miracle) or a static property (Protection,
// Indestructible, Devoid) are not expanded: rules reads them directly.
func (f *Face) expandKeywords() {
	// has reports whether the exact keyword line k (head and every param,
	// verbatim) already produced a T:/R:/A: entry of the given kind, via
	// the KeywordLine tag every case below sets alongside the head-only
	// Keyword tag. Reading f.Triggers/f.Repls/f.Abilities live means a
	// second Link() call -- or two equal lines in the same pass -- can
	// never double-add.
	has := func(kind, k string) bool {
		switch kind {
		case "T":
			for _, t := range f.Triggers {
				if t.Params["KeywordLine"] == k {
					return true
				}
			}
		case "R":
			for _, r := range f.Repls {
				if r.Params["KeywordLine"] == k {
					return true
				}
			}
		case "A":
			for _, a := range f.Abilities {
				if a.Params["KeywordLine"] == k {
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
			if has("R", k) {
				continue
			}
			// param is "<KIND>:<N>", occasionally followed by further
			// colon-separated fields real Forge reads for its own
			// bookkeeping (a CheckSVar$ condition, a human-readable
			// description) -- those are not part of <N> and are dropped
			// rather than spliced into CounterNum$ or the Repl body (some
			// contain their own "|", which would otherwise inject a
			// spurious param into both).
			kind, rest, _ := strings.Cut(param, ":")
			n, _, _ := strings.Cut(rest, ":")
			sv := "__kwEtbCounter" + strconv.Itoa(i)
			f.setSVar(sv, "DB$ PutCounter | Defined$ Self | CounterType$ "+kind+" | CounterNum$ "+n+" | ETB$ True")
			p := parseParams("Event$ Moved | Destination$ Battlefield | ValidCard$ Card.Self | ReplacementResult$ Updated | ReplaceWith$ " + sv +
				" | Keyword$ etbCounter | Description$ CARDNAME enters with " + n + " " + kind + " counters.")
			p["KeywordLine"] = k
			f.Repls = append(f.Repls, Repl{Event: "Moved", Params: p})
		case "ETBReplacement":
			if has("R", k) {
				continue
			}
			// param is "Copy:<SVar>" or "Other:<SVar>", occasionally
			// followed by further colon-separated fields real Forge reads
			// for its own bookkeeping (Mandatory/Optional, a valid-zone
			// spec, a filter): those are not part of the SVar name, so only
			// the field right after the layer tag is taken. The layer tag
			// itself (Copy vs Other) and the Optional/Mandatory field are
			// parsed past, not modeled: both are expanded identically here
			// (Ledger: replacement-semantics task owns telling a Copy-layer
			// or Optional replacement apart from a mandatory Other one).
			_, rest, _ := strings.Cut(param, ":")
			sv, _, _ := strings.Cut(rest, ":")
			p := parseParams("Event$ Moved | Destination$ Battlefield | ValidCard$ Card.Self | ReplacementResult$ Updated | ReplaceWith$ " + sv + " | Keyword$ ETBReplacement")
			p["KeywordLine"] = k
			f.Repls = append(f.Repls, Repl{Event: "Moved", Params: p})
		case "Undying":
			f.addKeywordTrigger(head, k, "Mode$ ChangesZone | Origin$ Battlefield | Destination$ Graveyard | ValidCard$ Card.Self+counters_EQ0_P1P1 | TriggerDescription$ Undying",
				"DB$ ChangeZone | Defined$ TriggeredNewCardLKICopy | Origin$ Graveyard | Destination$ Battlefield | WithCountersType$ P1P1 | WithCountersAmount$ 1", has)
		case "Evolve":
			f.addKeywordTrigger(head, k, "Mode$ ChangesZone | Destination$ Battlefield | ValidCard$ Creature.YouCtrl+Other | Evolve$ True | TriggerDescription$ Evolve",
				"DB$ PutCounter | Defined$ Self | CounterType$ P1P1 | CounterNum$ 1", has)
		case "Exalted":
			f.addKeywordTrigger(head, k, "Mode$ Attacks | ValidCard$ Creature.YouCtrl | Alone$ True | TriggerDescription$ Exalted",
				"DB$ Pump | Defined$ TriggeredAttacker | NumAtt$ +1 | NumDef$ +1", has)
		case "Dethrone":
			// CR 702.105's event-relative life comparison is in attacksMatches.
			f.addKeywordTrigger(head, k, "Mode$ Attacks | ValidCard$ Card.Self | Dethrone$ True | TriggerDescription$ Dethrone",
				"DB$ PutCounter | Defined$ Self | CounterType$ P1P1 | CounterNum$ 1", has)
		case "Hideaway":
			if has("R", k) {
				continue
			}
			// Hideaway is an enters-the-battlefield replacement. Keep the
			// keyword parameter as data so its varying N is not lost.
			n := strings.TrimSpace(param)
			if n == "" {
				n = "4"
			}
			sv := "__kwHideaway" + strconv.Itoa(i)
			f.setSVar(sv, "DB$ Hideaway | Amount$ "+n)
			p := parseParams("Event$ Moved | Destination$ Battlefield | ValidCard$ Card.Self | ReplacementResult$ Updated | ReplaceWith$ " + sv + " | Keyword$ Hideaway")
			p["KeywordLine"] = k
			f.Repls = append(f.Repls, Repl{Event: "Moved", Params: p})
		case "Prowess":
			f.addKeywordTrigger(head, k, "Mode$ SpellCast | ValidCard$ Card.nonCreature | ValidActivatingPlayer$ You | TriggerDescription$ Prowess",
				"DB$ Pump | Defined$ Self | NumAtt$ +1 | NumDef$ +1", has)
		case "Extort":
			// CR 702.100: "Whenever you cast a spell, you may pay {W/B}. If you
			// do, each opponent loses 1 life and you gain that much life." A
			// SpellCast trigger on the controller; the optional {W/B} payment is
			// efectively asked in effExtort (the mid-resolution KModes ask) and
			// the drain runs per spell cast.
			f.addKeywordTrigger(head, k, "Mode$ SpellCast | ValidActivatingPlayer$ You | TriggerDescription$ Extort",
				"DB$ Extort", has)
		case "Soulbond":
			// CR 702.103 has two independently-triggering cases: this creature
			// enters, and another unpaired creature its controller controls
			// enters. The two synthetic KeywordLine suffixes retain idempotency
			// for both expansions while Keyword remains the printed keyword.
			f.addKeywordTrigger(head, k+"#self", "Mode$ ChangesZone | Destination$ Battlefield | ValidCard$ Creature.Self | TriggerDescription$ Soulbond",
				"DB$ Pair", has)
			f.addKeywordTrigger(head, k+"#other", "Mode$ ChangesZone | Destination$ Battlefield | ValidCard$ Creature.YouCtrl+Other | TriggerDescription$ Soulbond",
				"DB$ Pair", has)
		case "Myriad":
			// CR 702.109: "Whenever this creature attacks, for each opponent other
			// than the defending player, you may create a token that's a copy of
			// this creature tapped and attacking that player." An Attacks trigger
			// whose body creates the myriad per-other-opponent token copies; the
			// token copy creation is the engine's Myriad effect (a focused
			// implementation: the copies are minted and attack the respective
			// opponent).
			f.addKeywordTrigger(head, k, "Mode$ Attacks | ValidCard$ Card.Self | Myriad$ True | TriggerDescription$ Myriad",
				"DB$ Myriad", has)
		case "Storm":
			f.addKeywordTrigger(head, k, "Mode$ SpellCast | ValidCard$ Card.Self | TriggerZones$ Stack | TriggerDescription$ Storm",
				"DB$ CopySpellAbility | Defined$ TriggeredSpellAbility | Amount$ Count$ThisTurnCast/Minus1 | MayChooseTarget$ True", has)
		case "Living Weapon":
			if has("T", k) {
				continue
			}
			f.setSVar("__kwLWAttach", "DB$ Attach | Defined$ Remembered | Object$ Self")
			f.addKeywordTrigger(head, k, "Mode$ ChangesZone | Destination$ Battlefield | ValidCard$ Card.Self | TriggerDescription$ Living weapon",
				"DB$ Token | TokenScript$ b_0_0_phyrexian_germ | TokenOwner$ You | RememberTokens$ True | SubAbility$ __kwLWAttach", has)
		case "Equip":
			if has("A", k) {
				continue
			}
			// param is "<cost>", occasionally followed by a creature-type
			// restriction and/or a human-readable description ("3:Creature.
			// YouCtrl+Legendary:legendary creature") or trailing ability
			// modifiers ("0:::ActivationLimit$ 1:..."). No corpus equip cost
			// itself contains a ":" (mana symbols, Sac<1/Creature>, PayLife
			// <3> and so on are all safe), so the first field is exactly the
			// cost; anything after is dropped for now -- restrictions are a
			// later Equip task's job, not this one's (Ledger).
			cost, _, _ := strings.Cut(param, ":")
			sa, _ := parseSA("", "AB$ Attach | Cost$ "+cost+" | ValidTgts$ Creature.YouCtrl | TgtPrompt$ Select target creature you control | SorcerySpeed$ True | Keyword$ Equip | SpellDescription$ Equip "+cost)
			if sa != nil {
				sa.Params["KeywordLine"] = k
				f.Abilities = append(f.Abilities, sa)
			}
		case "Enchant":
			if has("A", k) || f.SpellAbility() != nil {
				continue
			}
			// param is "<spec>", occasionally followed by the human-
			// readable prompt Forge itself shows ("Creature.YouCtrl:
			// creature you control"). When present, that third field is
			// used verbatim as the prompt; when absent, one is generated
			// from the spec the same way the brief's table describes.
			spec, prompt, hasPrompt := strings.Cut(param, ":")
			if !hasPrompt || prompt == "" {
				prompt = strings.ToLower(spec)
			}
			sa, _ := parseSA("", "SP$ Attach | ValidTgts$ "+spec+" | TgtPrompt$ Select target "+prompt+" | Object$ Self | Keyword$ Enchant")
			if sa != nil {
				sa.Params["KeywordLine"] = k
				f.Abilities = append(f.Abilities, sa)
			}
		}
	}
}

// addKeywordTrigger appends one tagged T: line whose Execute$ is an SVar
// this function creates, unless the exact keyword line was already
// expanded (kw is the head, used for the Keyword$ tag and the __kw SVar
// name; line is the full keyword text, used only for idempotency).
func (f *Face) addKeywordTrigger(kw, line, trigger, effect string, has func(kind, line string) bool) {
	if has("T", line) {
		return
	}
	sv := "__kw" + strings.ReplaceAll(kw, " ", "")
	f.setSVar(sv, effect)
	p := parseParams(trigger + " | Execute$ " + sv + " | Keyword$ " + kw)
	p["KeywordLine"] = line
	f.Triggers = append(f.Triggers, Trigger{Mode: p["Mode"], Params: p})
}

// setSVar lazily initializes f.SVars before writing name/body. Most compiled
// faces never call this -- only ones with an SVar-based keyword expansion
// do -- so allocating unconditionally in expandKeywords would put a throwaway
// empty map into every face the IR cache stores for nothing.
func (f *Face) setSVar(name, body string) {
	if f.SVars == nil {
		f.SVars = map[string]string{}
	}
	f.SVars[name] = body
}
