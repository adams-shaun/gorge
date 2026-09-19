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
			//
			// The second case's partner choice must be restricted to the
			// SPECIFIC creature that triggered it (CR 702.103a: "you may pair
			// this creature with that creature"), not any unpaired creature
			// the controller happens to have -- a bystander unpaired creature
			// must never be offered just because a third, unrelated creature
			// entered. RestrictToRemembered$ True tells effPair (Ctx.Remembered
			// already carries the triggering entrant, via triggerRemembered) to
			// narrow its candidate scan to that one object; the #self trigger
			// omits it and keeps the broad "any unpaired creature I control"
			// scan CR 702.103a's other half calls for.
			f.addKeywordTrigger(head, k+"#self", "Mode$ ChangesZone | Destination$ Battlefield | ValidCard$ Creature.Self | TriggerDescription$ Soulbond",
				"DB$ Pair", has)
			f.addKeywordTrigger(head, k+"#other", "Mode$ ChangesZone | Destination$ Battlefield | ValidCard$ Creature.YouCtrl+Other | TriggerDescription$ Soulbond",
				"DB$ Pair | RestrictToRemembered$ True", has)
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
		case "Annihilator":
			// CR 702.86: each time this creature attacks, its defending
			// player sacrifices the stated number of permanents. The count
			// rides the Annihilator$ marker itself rather than Amount$: the
			// generated trigger is this repo's own shape (no raw corpus card
			// carries an Annihilator$ param), and keeping Amount$ off the
			// expansion leaves api:Sacrifice.Amount genuinely unread for the
			// ordinary Sacrifice lines the parameter census still labels.
			f.addKeywordTrigger(head, k, "Mode$ Attacks | ValidCard$ Card.Self | TriggerDescription$ Annihilator",
				"DB$ Sacrifice | Defined$ TriggeredDefendingPlayer | SacValid$ Permanent | Annihilator$ "+param, has)
		case "Ward":
			// Ward is a becomes-target trigger. Ward$ lets the matcher exclude
			// the permanent's controller; the effect counters the targeting
			// spell or ability unless that player pays the printed cost.
			f.addKeywordTrigger(head, k, "Mode$ BecomesTarget | ValidTarget$ Card.Self | Ward$ True | TriggerDescription$ Ward",
				"DB$ Ward | UnlessCost$ "+param, has)
		case "Storm":
			f.addKeywordTrigger(head, k, "Mode$ SpellCast | ValidCard$ Card.Self | TriggerZones$ Stack | TriggerDescription$ Storm",
				"DB$ CopySpellAbility | Defined$ TriggeredSpellAbility | Amount$ Count$ThisTurnCast/Minus1 | MayChooseTarget$ True", has)
		case "Replicate":
			// CR 702.55a: "you may pay an additional [cost] any number of
			// times as you cast this spell. If you do, copy it for each time
			// you paid its replicate cost." The cast flow poses the count ask
			// (rules/cast.go's replicateAsk, one KChoose before the payment
			// window) and records the count on the pay-time CastInfo
			// (FlagReplicated's Amount); this trigger reads Count$ReplicatePaid
			// off the cast spell, so a DECLINED replicate resolves the trigger
			// with Amount 0 and effCopySpellAbility's loop emits nothing. The
			// copies keep their targets (MayChooseTarget$), the same
			// Storm-shaped stand-in the M4 copy-target task owns.
			f.addKeywordTrigger(head, k, "Mode$ SpellCast | ValidCard$ Card.Self | TriggerZones$ Stack | TriggerDescription$ Replicate",
				"DB$ CopySpellAbility | Defined$ TriggeredSpellAbility | Amount$ Count$ReplicatePaid | MayChooseTarget$ True", has)
		case "Living Weapon":
			if has("T", k) {
				continue
			}
			f.setSVar("__kwLWAttach", "DB$ Attach | Defined$ Remembered | Object$ Self")
			f.addKeywordTrigger(head, k, "Mode$ ChangesZone | Destination$ Battlefield | ValidCard$ Card.Self | TriggerDescription$ Living weapon",
				"DB$ Token | TokenScript$ b_0_0_phyrexian_germ | TokenOwner$ You | RememberTokens$ True | SubAbility$ __kwLWAttach", has)
		case "Cumulative upkeep":
			// CR 702.46a is a triggered ability, not an upkeep turn action.
			// Expanding it into the ordinary Phase-trigger pipeline gives it
			// normal APNAP ordering, stack interaction and response windows.
			// param may include Forge's trailing display text after a colon;
			// only the first field is the actual upkeep cost.
			cost, _, _ := strings.Cut(param, ":")
			f.addKeywordTrigger(head, k,
				"Mode$ Phase | Phase$ Upkeep | ValidPlayer$ You | TriggerZones$ Battlefield | TriggerDescription$ Cumulative upkeep",
				"DB$ CumulativeUpkeep | Cost$ "+cost, has)
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
		case "Transmute":
			if has("A", k) {
				continue
			}
			// CR 702.53: transmute is a sorcery-speed hand activation. The
			// searched card has the source card's printed mana value.
			cost := strings.TrimSpace(param)
			sa, _ := parseSA("", "AB$ ChangeZone | Cost$ "+cost+" Discard<1/CARDNAME> | ActivationZone$ Hand | SorcerySpeed$ True | Origin$ Library | Destination$ Hand | ChangeType$ Card.cmcEQ"+strconv.Itoa(int(f.Cmc()))+" | ChangeNum$ 1 | Keyword$ Transmute | SpellDescription$ Transmute "+cost)
			if sa != nil {
				sa.Params["KeywordLine"] = k
				f.Abilities = append(f.Abilities, sa)
			}
		case "Cycling":
			if has("A", k) {
				continue
			}
			cost := strings.TrimSpace(param)
			sa, _ := parseSA("", "AB$ Draw | Cost$ "+cost+" Discard<1/CARDNAME> | ActivationZone$ Hand | NumCards$ 1 | Keyword$ Cycling | SpellDescription$ Cycling "+cost)
			if sa != nil {
				sa.Params["KeywordLine"] = k
				f.Abilities = append(f.Abilities, sa)
			}
		case "Affinity":
			// CR 702.41a: affinity for <spec> is a cost-reduction static, not an
			// ability, so its idempotence key cannot use has() (which reads only
			// Triggers/Repls/Abilities) -- it keys on the minted static itself,
			// carrying the same KeywordLine tag the other cases set. Without it a
			// second Link() (cards/registry.go re-runs f.link() on cached faces
			// that predate a newly added expansion) would append a SECOND
			// reduction and double the discount, replay-visibly.
			dup := false
			for _, st := range f.Statics {
				if st.Params["KeywordLine"] == k {
					dup = true
					break
				}
			}
			if dup {
				continue
			}
			// param is "<spec>", occasionally followed by a human-readable
			// description after a second colon ("Land.Snow:snow land",
			// "Permanent.token:token", "Creature.Artifact:artifact creature" --
			// 3 corpus lines): only the first field is the count spec, exactly
			// the trailing-field strip the etbCounter and Equip cases do.
			spec, desc, _ := strings.Cut(param, ":")
			if desc == "" {
				desc = spec
			}
			sv := "__kwAffinity" + strconv.Itoa(i)
			// Count$Valid counts BATTLEFIELD objects (effects/count.go's countZone
			// maps "Valid" to ZBattlefield), so the "you control" qualifier lives
			// inside the spec. The joining separator is load-bearing: the matcher
			// splits base from predicates on the FIRST dot
			// (effects/filter.go MatchesObjectCtx), so a dot-less spec joined with
			// '+' ("Food+YouCtrl") reads the whole thing as one base type word and
			// fails closed to 0 -- but a spec that already carries a dot
			// ("Land.Snow", "Permanent.token") must join with '+' (the corpus's
			// measured-working "Swamp.Snow+YouCtrl" shape), because a second dot
			// would glue "YouCtrl" onto the previous predicate token, which is
			// unknown and fails closed just as hard.
			sep := "."
			if strings.ContainsRune(spec, '.') {
				sep = "+"
			}
			f.setSVar(sv, "Count$Valid "+spec+sep+"YouCtrl")
			p := parseParams("Mode$ ReduceCost | ValidCard$ Card.Self | Type$ Spell | EffectZone$ All | Amount$ " + sv +
				" | Description$ This spell costs {1} less to cast for each " + desc + " you control.")
			// EffectZone$ All keeps the reduction live from hand/library (the
			// Ghalta precedent in rules/statics.go's collectCostStatics doc);
			// no Color$ (affinity reduces generic only -- CR 702.41a) and no
			// Relative$ (that flag is for X-dependent amounts).
			p["KeywordLine"] = k
			f.Statics = append(f.Statics, Static{Mode: "ReduceCost", Params: p})
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
		case "Mobilize":
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
				continue
			}
			sv := "__kw" + strings.ReplaceAll(k, " ", "")
			f.setSVar(sv+"Delay", "DB$ DelayedTrigger | Mode$ Phase | Phase$ End of Turn | Execute$ "+sv+"Sacrifice | RememberChain$ False")
			f.setSVar(sv+"Sacrifice", "DB$ Sacrifice | Defined$ DelayTriggerRememberedLKI")
			f.addKeywordTrigger(head, k, "Mode$ Attacks | ValidCard$ Card.Self | TriggerDescription$ Mobilize",
				"DB$ Token | TokenScript$ r_1_1_warrior | TokenTapped$ True | TokenAttacking$ True | TokenAmount$ "+param+" | RememberTokens$ True | SubAbility$ "+sv+"Delay", has)
		case "Encore":
			if has("A", k) {
				continue
			}
			// param is "<cost>" ("3 R"), occasionally followed by further
			// colon-separated fields no corpus line carries; the first field is
			// the cost. The expansion mirrors the C21 oracle shape ("Encore
			// <cost> (<cost>, Exile this card from your graveyard: For each
			// opponent, create a token copy that attacks that opponent this turn
			// if able. They gain haste. Sacrifice them at the beginning of the
			// next end step. Activate only as a sorcery.)"): one AB$ ability in
			// the graveyard whose cost is the printed cost plus exiling the card
			// itself (ExileFromGrave<1/CARDNAME>), resolved by effects.Encore
			// (encore.go): one CardToken copy per opponent, haste granted, and
			// one end-step delayed sacrifice per copy. The tokens' "attacks that
			// opponent this turn if able" is NOT enforced -- this build has no
			// attack-requirement machinery for it (recorded in the ticket
			// report's Issues), the copy is otherwise exact.
			cost, _, _ := strings.Cut(param, ":")
			sa, _ := parseSA("", "AB$ Encore | Cost$ "+cost+" ExileFromGrave<1/CARDNAME> | ActivationZone$ Graveyard | SorcerySpeed$ True | Keyword$ Encore | SpellDescription$ Encore "+cost)
			if sa != nil {
				sa.Params["KeywordLine"] = k
				f.Abilities = append(f.Abilities, sa)
			}
		}
	}
}

// addKeywordTrigger appends one tagged T: line whose Execute$ is an SVar
// this function creates, unless the exact keyword line was already
// expanded (kw is the head, used only for the Keyword$ tag; line is the full
// keyword text, used both for idempotency and -- since it, unlike kw, is
// unique per call -- for the __kw SVar name. Soulbond calls this twice with
// the same kw ("Soulbond") but two different lines ("Soulbond#self" and
// "Soulbond#other"): keying the SVar name on kw alone would collide the two
// calls onto one shared SVar, silently letting the second call's effect body
// overwrite the first's).
func (f *Face) addKeywordTrigger(kw, line, trigger, effect string, has func(kind, line string) bool) {
	if has("T", line) {
		return
	}
	sv := "__kw" + strings.ReplaceAll(line, " ", "")
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
