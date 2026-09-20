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
			n, extra, _ := strings.Cut(rest, ":")
			sv := "__kwEtbCounter" + strconv.Itoa(i)
			f.setSVar(sv, "DB$ PutCounter | Defined$ Self | CounterType$ "+kind+" | CounterNum$ "+n+" | ETB$ True")
			p := parseParams("Event$ Moved | Destination$ Battlefield | ValidCard$ Card.Self | ReplacementResult$ Updated | ReplaceWith$ " + sv +
				" | Keyword$ etbCounter | Description$ CARDNAME enters with " + n + " " + kind + " counters.")
			// The FIRST extra colon field may be a condition gate: either a
			// bare `CheckSVar$ <name>` (Lupine Harbingers' "... since it was
			// foretold", Myojin of Night's Reach's "if you cast it from your
			// hand") or a `CheckSVar$ <name> | SVarCompare$ <op><N>` pair
			// (Hotheaded Giant, Freestrider Commando, Steel Exemplar). Split
			// the ` | `-separated tokens through to the shared
			// rules/replacementConditionHolds read (CheckSVar + optional
			// SVarCompare) instead of stuffing the whole field into one param:
			// a whole-stuffed CheckSVar resolves no SVar, the gate fails
			// closed, and those carriers' counters silently un-apply (the
			// round-2 review's measured regression). Only these two condition
			// params are passed through: a gate field can also carry real
			// match params (the Myojin-family lines carry `ValidCard$ ...`
			// here) that the replacement matcher honours, and passing those
			// through would widen every carrier's match. Everything else stays
			// dropped, exactly as before -- the later colon fields remain
			// display metadata.
			if first, _, _ := strings.Cut(extra, ":"); strings.Contains(first, "$") {
				for _, part := range strings.Split(first, " | ") {
					name, val, ok := strings.Cut(strings.TrimSpace(part), "$")
					if !ok {
						continue
					}
					switch strings.TrimSpace(name) {
					case "CheckSVar", "SVarCompare":
						if val = strings.TrimSpace(val); val != "" {
							p[strings.TrimSpace(name)] = val
						}
					case "ValidCard":
						// A gate field's ValidCard$ is a real match param the
						// replacement matcher honours -- epochrasite's
						// `Card.Self+!wasCastFromYourHandByYou` (task castprov1)
						// and the escape-counter family's `Card.Self+escaped`,
						// all 11 raw carriers spelled `Card.Self+<preds>`. It
						// replaces the default `Card.Self` ONLY when the gate's
						// own spec still constrains Self (a Self-less fragment
						// would widen the default's match, the pre-existing
						// drop's reason); a spec the filter fails closed on
						// (wasCastByYou's unknown predicate) keeps failing
						// closed. Measured: no carrier is in any repo deck.
						if val = strings.TrimSpace(val); val != "" && strings.Contains(val, "Self") {
							p["ValidCard"] = val
						}
					}
				}
			}
			p["KeywordLine"] = k
			f.Repls = append(f.Repls, Repl{Event: "Moved", Params: p})
		case "Devour":
			if has("R", k) {
				continue
			}
			// CR 702.83: "As <this> enters the battlefield, you may sacrifice
			// any number of <type>s. This enters the battlefield with a +1/+1
			// counter on it for each creature sacrificed this way." The
			// parameter is "<amount>[:<valid>[:display-text]]" -- amount 1/2/3/X,
			// valid defaults Creature (Feasting Hobbit's Food, Caprichrome's
			// Artifact, Famished Worldsire's Land are the corpus's typed
			// carriers); the trailing display fields are dropped. The expansion
			// is Forge's CardFactoryUtil Devour shape verbatim: one ETB
			// replacement whose body is an optional sacrifice ask (the batch is
			// remembered onto the source object), then the counter put reading
			// RememberedSize/Times.<amount>, then a cleanup clearing the memory.
			// Count$RememberedSize reads the event-backed Remembered list the
			// sacrifice primitive fills; /Times.N is the shared applyCountOp.
			// A Devour X (Thromok the Insatiable) carries only the bare count --
			// Times.X fails the op parser and leaves the count at one per
			// devoured permanent, which is exactly the CR 702.83 X read.
			amount, rest, _ := strings.Cut(param, ":")
			valid, _, _ := strings.Cut(rest, ":")
			amount, valid = strings.TrimSpace(amount), strings.TrimSpace(valid)
			if valid == "" {
				valid = "Creature"
			}
			sv := "__kwDevour" + strconv.Itoa(i)
			sacX := "__kwDevourSacX" + strconv.Itoa(i)
			cntX := "__kwDevourX" + strconv.Itoa(i)
			cn := "__kwDevourCounter" + strconv.Itoa(i)
			cl := "__kwDevourCleanup" + strconv.Itoa(i)
			f.setSVar(sacX, "Count$Valid "+valid+".YouCtrl+Other")
			f.setSVar(cntX, "Count$RememberedSize/Times."+amount)
			f.setSVar(sv, "DB$ Sacrifice | Defined$ You | Amount$ "+sacX+
				" | RememberSacrificed$ True | Optional$ True | SacValid$ "+valid+
				".Other | SubAbility$ "+cn)
			f.setSVar(cn, "DB$ PutCounter | ETB$ True | Defined$ Self | CounterType$ P1P1 | CounterNum$ "+cntX+
				" | SubAbility$ "+cl)
			f.setSVar(cl, "DB$ Cleanup | ClearRemembered$ True")
			p := parseParams("Event$ Moved | Destination$ Battlefield | ValidCard$ Card.Self" +
				" | ReplacementResult$ Updated | ReplaceWith$ " + sv + " | Keyword$ Devour")
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
		case "Persist":
			// CR 702.77, the mirror of Undying: the dies-condition reads
			// -1/-1 counters off the LKI and the return grants one.
			f.addKeywordTrigger(head, k, "Mode$ ChangesZone | Origin$ Battlefield | Destination$ Graveyard | ValidCard$ Card.Self+counters_EQ0_M1M1 | TriggerDescription$ Persist",
				"DB$ ChangeZone | Defined$ TriggeredNewCardLKICopy | Origin$ Graveyard | Destination$ Battlefield | WithCountersType$ M1M1 | WithCountersAmount$ 1", has)
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
		case "Afflict":
			// CR 702.130: "Whenever this creature becomes blocked, defending
			// player loses N life." The parameter is the life amount; every
			// corpus K:Afflict line carries one (measured 10/10). The engine's
			// become-blocked hook is trig:AttackerBlocked
			// (checkAttackerBlockedTriggers), which queues one instance per
			// blocked attacker and captures the defender as the trigger
			// context's DefendingPlayer -- exactly the Defined$ the body reads.
			// A keyword granted in a layer (AddKeyword$ Afflict:N, e.g. Lost
			// Monarch of Ifnir's Zombie grant) needs no expansion here: rules'
			// checkGrantedAfflictTriggers synthesizes the same trigger from the
			// derived keyword list.
			if strings.TrimSpace(param) == "" {
				continue
			}
			f.addKeywordTrigger(head, k, "Mode$ AttackerBlocked | ValidCard$ Card.Self | TriggerDescription$ Afflict",
				"DB$ LoseLife | Defined$ TriggeredDefendingPlayer | LifeAmount$ "+strings.TrimSpace(param), has)
		case "Flanking":
			// CR 702.25a: whenever this creature becomes blocked by a creature
			// without flanking, that blocker gets -1/-1 until end of turn (the
			// controller of the trigger is the attacker's controller, which the
			// queue's Source/Controller already are since the source IS the
			// attacker). One instance per (attacker, non-flanking blocker) pair,
			// mirrored from Forge's CardFactoryUtil expansion (Mode$
			// AttackerBlockedByCreature | ValidBlocker$ Creature.withoutFlanking);
			// the pair hook is rules/trigger_match.go's
			// attackerBlockedByPairCandidates, the per-attacker
			// attackerBlockedCandidates shape narrowed per blocker. CR 702.25b's
			// one-trigger-per-instance is out of measured scope: all 30 corpus
			// carriers are a bare single K:Flanking.
			f.addKeywordTrigger(head, k,
				"Mode$ AttackerBlockedByCreature | ValidCard$ Card.Self | ValidBlocker$ Creature.withoutFlanking | TriggerZones$ Battlefield | TriggerDescription$ Flanking",
				"DB$ Pump | Defined$ TriggeredBlockerLKICopy | NumAtt$ -1 | NumDef$ -1", has)
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
		case "Gravestorm":
			// CR 702.84: Storm's shape with a different amount -- one copy per
			// permanent put into a graveyard from the battlefield this turn
			// (Forge's CardFactoryUtil expansion). The count head resolves
			// through effects.countEntered's zone-aware spec match.
			f.addKeywordTrigger(head, k, "Mode$ SpellCast | ValidCard$ Card.Self | TriggerZones$ Stack | TriggerDescription$ Gravestorm",
				"DB$ CopySpellAbility | Defined$ TriggeredSpellAbility | Amount$ Count$ThisTurnEntered_Graveyard_from_Battlefield_Permanent | MayChooseTarget$ True", has)
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
		case "For Mirrodin":
			// CR 702.159: "For Mirrodin! — When this Equipment enters,
			// create a 2/2 red Rebel creature token, then attach this to
			// it." Exactly the Living Weapon shape (an enters-the-
			// battlefield trigger on the Equipment itself, remembering
			// the token it mints so the chained Attach can name it);
			// only the token script (r_2_2_rebel) and the display text
			// differ. Forge puts no trailing parameter on K:For Mirrodin.
			if has("T", k) {
				continue
			}
			f.setSVar("__kwFMAttach", "DB$ Attach | Defined$ Remembered | Object$ Self")
			f.addKeywordTrigger(head, k, "Mode$ ChangesZone | Destination$ Battlefield | ValidCard$ Card.Self | TriggerDescription$ For Mirrodin!",
				"DB$ Token | TokenScript$ r_2_2_rebel | TokenOwner$ You | RememberTokens$ True | SubAbility$ __kwFMAttach", has)
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
		case "Echo":
			// CR 702.35a: "At the beginning of your upkeep, if this permanent
			// came under your control since the beginning of your most recent
			// upkeep, you may pay {cost}. If you don't, sacrifice it." Expanded
			// into the ordinary Phase-trigger pipeline like Cumulative upkeep
			// (normal APNAP ordering, stack interaction, response windows). The
			// intervening-if rides the Echo$ True marker the same way
			// Annihilator$ rides its generated trigger: rules/trigger_match.go
			// checks it against the object's control-acquisition tuple
			// (Object.AcqTurn/AcqStep vs Player.LastUpkeepTurn) and suppresses
			// the trigger before it stacks when the gate is false. The election
			// itself (pay-or-sacrifice) is rules/echo.go's resolution-time flow.
			// param may include Forge's trailing display text after a colon;
			// only the first field is the echo cost.
			cost, _, _ := strings.Cut(param, ":")
			f.addKeywordTrigger(head, k,
				"Mode$ Phase | Phase$ Upkeep | ValidPlayer$ You | TriggerZones$ Battlefield | TriggerDescription$ Echo | Echo$ True",
				"DB$ Echo | Cost$ "+cost, has)
		case "Equip":
			if has("A", k) {
				continue
			}
			// param is "<cost>" followed by colon fields: a target
			// restriction ("3:Creature.YouCtrl+Legendary:legendary creature"),
			// rider parameters ("4:::ReduceCost$ Monarch:...",
			// "0:::ActivationLimit$ 1:...") and human-readable text. The first
			// field is exactly the cost: no corpus equip cost itself contains
			// a ":" (mana symbols, Sac<1/Creature>, PayLife<3> and so on are
			// all safe; measured over all 646 raw K:Equip lines at the corpus
			// pin -- the split-on-":" is a corpus invariant, not an
			// assumption to re-litigate per card).
			// The trailing fields are read, not dropped wholesale:
			//   - a "ReduceCost$ <v>" / "ActivationLimit$ <v>" field rides the
			//     minted SA verbatim; rules/legal.go's ownReduceCost and the
			//     offer loop's ActivationLimit gate already read both params.
			//   - the FIRST remaining field that is neither a rider nor a
			//     "Flavor " marker is the target restriction, a real filter
			//     spec passed through verbatim as ValidTgts$ (comma
			//     alternatives included); later fields are display text and
			//     stay dropped, as before. The space-free test separates spec
			//     from prose: every restriction spec in the corpus is
			//     space-free ("Creature.YouCtrl+Legendary",
			//     "Creature.YouCtrl+Shaman,..." — commas, never spaces), while
			//     every description field carries spaces ("legendary
			//     creature", "This ability costs {3} less to activate if
			//     you're the monarch"); the one-word descs ("Soldier",
			//     "commander") only ever trail a real restriction, so the
			//     first-real-field rule already claimed the slot.
			//   - any other "<Head>$ <value>" field is an unwired rider family
			//     (AlternateCost$, 4 raw lines) -- dropped, as today, but
			//     never mistaken for a restriction spec.
			fields := strings.Split(param, ":")
			cost := fields[0]
			restriction := ""
			var riders []string
			for _, fld := range fields[1:] {
				fld = strings.TrimSpace(fld)
				if fld == "" || strings.HasPrefix(fld, "Flavor ") {
					continue
				}
				head, _, isParam := strings.Cut(fld, " ")
				if isParam && strings.HasSuffix(head, "$") {
					if head == "ReduceCost$" || head == "ActivationLimit$" {
						riders = append(riders, fld)
					}
					continue
				}
				if restriction == "" && !strings.Contains(fld, " ") {
					restriction = fld
				}
			}
			tgts := "Creature.YouCtrl"
			if restriction != "" {
				tgts = restriction
			}
			saStr := "AB$ Attach | Cost$ " + cost + " | ValidTgts$ " + tgts + " | TgtPrompt$ Select target creature you control | SorcerySpeed$ True | Keyword$ Equip | SpellDescription$ Equip " + cost
			for _, r := range riders {
				saStr += " | " + r
			}
			sa, _ := parseSA("", saStr)
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
		case "TypeCycling":
			// CR 702.28d: typed cycling is an ordinary hand activation whose
			// resolution is a LIBRARY SEARCH, not a draw -- "[cost], Discard this
			// card: Search your library for a card with the [type] type, reveal
			// it, put it into your hand, then shuffle." The shape is therefore
			// Transmute's (search), never the plain Cycling case's AB$ Draw. The
			// reveal is the search's own default for a stated-quality
			// ChangeType$ (applyLibrarySearch's `spec != "Card"` arm), so no
			// Reveal$ is needed. param is "<type>:<cost>[...]"; the type is
			// fields[0] and the cost fields[1], with any trailing field a
			// human-readable description dropped -- the Landfall/etbCounter/
			// Equip trailing-field strip.
			if has("A", k) {
				continue
			}
			typeSpec, rest, _ := strings.Cut(param, ":")
			typeSpec = strings.TrimSpace(typeSpec)
			cost, _, _ := strings.Cut(rest, ":")
			cost = strings.TrimSpace(cost)
			sa, _ := parseSA("", "AB$ ChangeZone | Cost$ "+cost+" Discard<1/CARDNAME> | ActivationZone$ Hand | Origin$ Library | Destination$ Hand | ChangeType$ "+typeSpec+" | ChangeNum$ 1 | Keyword$ TypeCycling | SpellDescription$ "+typeSpec+"cycling "+cost)
			if sa != nil {
				sa.Params["KeywordLine"] = k
				f.Abilities = append(f.Abilities, sa)
			}
		case "Level up":
			if has("A", k) {
				continue
			}
			// CR 702.87a: "Level up [cost]" means "[cost]: Put a level counter
			// on this permanent. Activate only as a sorcery." It is an
			// ordinary activated ability, so the counter placement goes through
			// the existing PutCounter primitive and the level bands (ordinary
			// layer-7 SetPower$/SetToughness$/AddKeyword$ statics gated on
			// IsPresent$ Card.Self+counters_GE<n>_LEVEL) read the counter with
			// no further machinery. SorcerySpeed$ True is the CR 702.87a
			// sorcery-window restriction. param is the cost; any trailing
			// fields after a second colon are display text (none in the
			// measured 26-line corpus), exactly the trailing-field strip the
			// etbCounter and Affinity cases do.
			cost, _, _ := strings.Cut(param, ":")
			cost = strings.TrimSpace(cost)
			sa, _ := parseSA("", "AB$ PutCounter | Cost$ "+cost+" | Defined$ Self | CounterType$ LEVEL | CounterNum$ 1 | SorcerySpeed$ True | Keyword$ Level up | SpellDescription$ Level up "+cost)
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
		case "Afterlife":
			// CR 702.132a: "When this creature dies, create N 1/1 white and
			// black Spirit creature tokens with flying." One ChangesZone
			// death trigger (the Undying shape: Origin$ Battlefield,
			// Destination$ Graveyard, ValidCard$ Card.Self) whose effect mints
			// the Spirit tokens from the existing wb_1_1_spirit_flying token
			// script; effToken's default owner is the resolving controller,
			// so the tokens enter under the dying creature's controller. The
			// param is a bare literal count on every corpus line (measured:
			// 11 files, values 1/2/3, no trailing fields), so it is spliced
			// in verbatim as TokenAmount$. addKeywordTrigger already guards
			// idempotency via the KeywordLine tag, so a second Link() of a
			// cached face cannot double-add the trigger.
			f.addKeywordTrigger(head, k, "Mode$ ChangesZone | Origin$ Battlefield | Destination$ Graveyard | ValidCard$ Card.Self | TriggerDescription$ Afterlife",
				"DB$ Token | TokenScript$ wb_1_1_spirit_flying | TokenAmount$ "+param, has)
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
		case "Embalm", "Eternalize":
			if has("A", k) {
				continue
			}
			// CR 702.128 (Embalm) / CR 702.129 (Eternalize): one activated
			// ability the card offers from the graveyard, whose cost is the
			// printed mana cost plus exiling the card itself, and whose effect
			// is "create a token that's a copy of it, except ...". The two are
			// one family: Embalm's token is a white Zombie in addition to its
			// other types; Eternalize's is additionally a 4/4 black Zombie.
			// The body is DB$ CopyPermanent (api CopyPermanent), whose
			// characteristic modifications AddTypes$/SetColor$/SetPower$/
			// SetToughness$ this build applies to the minted copy.
			// ExileFromGrave<1/CARDNAME> is the shared graveyard self-exile
			// cost Encore already uses, settled by the ordinary cast-flow
			// exile stage; the whole keyword parameter is spliced verbatim
			// into Cost$ so an extra cost component (Sinuous Striker's and
			// Sunscourge Champion's "Discard<1/Card>") rides along. The
			// token's "no mana cost" (Forge's RemoveCost$) is NOT modelled --
			// this engine derives a copy's mana value from its printed card --
			// so RemoveCost$ is deliberately not emitted (an unread param
			// would only rot the parameter census); the divergence is recorded
			// in the task report's Issues section.
			body := "AB$ CopyPermanent | Cost$ " + param + " ExileFromGrave<1/CARDNAME> | ActivationZone$ Graveyard | SorcerySpeed$ True | Defined$ Self | SetColor$ "
			if head == "Eternalize" {
				body += "Black | AddTypes$ Zombie | SetPower$ 4 | SetToughness$ 4"
			} else {
				body += "White | AddTypes$ Zombie"
			}
			sa, _ := parseSA("", body+" | Keyword$ "+head+" | SpellDescription$ "+head+" "+param)
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
