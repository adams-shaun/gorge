package cards

// keywordRulesReads maps each registered keyword head whose rules meaning
// extends past its expansion lines to the place gorge's rules or effects code
// reads it by name (file:line, one line of reason). A head with an entry is
// NOT expansion-only: dirac's generator must model more than the lines
// registerKeyword's expander mints, because some rule elsewhere in the engine
// keys on the head itself.
//
// The table is a ratchet held complete by TestKeywordRulesReadsComplete in
// engine/rules, which parses the rules and effects sources and fails when a
// head named by code is missing here, or when an entry here names a head no
// code reads any more. The scan scope is engine/rules and engine/effects
// only: a head read in another package (e.g. engine/deck's commander pair
// seating reads "Partner with") is not a card-rules read for this table's
// purpose, and its head stays expansion-only. Keys are the exact registered
// heads (registerKeyword casing), interned to KeywordHeadID ordinals once
// here at the boundary; lookups go through KeywordExpansionOnly.
var keywordRulesReads = map[KeywordHeadID]string{
	InternKeywordHead("Affinity"):          "engine/effects/filter.go:1700 — affinity artifact-count cost filter",
	InternKeywordHead("Afflict"):           "engine/rules/trigger_granted.go:663 — afflict trigger granted by head",
	InternKeywordHead("Afterlife"):         "engine/rules/scriptfacts/granted_keyword_trigger.go:34 — afterlife grant trigger table",
	InternKeywordHead("Awaken"):            "engine/rules/awaken.go:81 — awaken cast mode",
	InternKeywordHead("Cipher"):            "engine/effects/cipher.go:10 — cipher cast-on-combat-damage",
	InternKeywordHead("Class"):             "engine/effects/typewords.go:117 — Class level type words",
	InternKeywordHead("Conspire"):          "engine/rules/cast_altcost.go:531 — conspire alt cost",
	InternKeywordHead("Crew"):              "engine/rules/pay/paidcost.go:246 — crew tap-power cost payment",
	InternKeywordHead("Cumulative upkeep"): "engine/rules/trigger_granted.go:1421 — cumulative-upkeep upkeep trigger grant",
	InternKeywordHead("Cycling"):           "engine/rules/trigger_cause.go:18 — cycling trigger cause",
	InternKeywordHead("Demonstrate"):       "engine/effects/demonstrate.go:10 — demonstrate shared-resolution half",
	InternKeywordHead("Dethrone"):          "engine/rules/trigger_granted.go:381 — dethrone trigger grant",
	InternKeywordHead("Echo"):              "engine/rules/stack.go:305 — echo upkeep choice",
	InternKeywordHead("Embalm"):            "engine/rules/quiet_facts.go:193 — embalm alt-cast facts",
	InternKeywordHead("Enchant"):           "engine/rules/aura_entry.go:134 — enchant aura legality",
	InternKeywordHead("Encore"):            "engine/effects/encore.go:31 — encore cast from graveyard",
	InternKeywordHead("Epic"):              "engine/rules/keyword_refs.go:15 — epic after-this-spell gate",
	InternKeywordHead("Equip"):             "engine/rules/attach.go:283 — equip attach legality",
	InternKeywordHead("ETBReplacement"):    "engine/rules/replacement.go:335 — ETB replacement dispatch by head",
	InternKeywordHead("Eternalize"):        "engine/rules/quiet_facts.go:193 — eternalize alt-cast facts",
	InternKeywordHead("Evolve"):            "engine/rules/static_condition.go:44 — evolve power/toughness condition",
	InternKeywordHead("Exalted"):           "engine/rules/melee.go:67 — exalted lone-attacker trigger",
	InternKeywordHead("Exploit"):           "engine/effects/exploit.go:61 — exploit sacrifice choice",
	InternKeywordHead("Extort"):            "engine/effects/extort.go:10 — extort drain on cast",
	InternKeywordHead("Firebending"):       "engine/rules/trigger_granted.go:567 — firebending trigger grant",
	InternKeywordHead("Flanking"):          "engine/rules/trigger_blocks.go:191 — flanking blocker debuff",
	InternKeywordHead("Fortify"):           "engine/rules/attach.go:286 — fortify attach legality",
	InternKeywordHead("Haunt"):             "engine/effects/changezone_params.go:328 — haunt exile-on-death",
	InternKeywordHead("Hideaway"):          "engine/effects/cardflow.go:28 — hideaway look-at-top-four",
	InternKeywordHead("Level up"):          "engine/rules/keyword_heads.go:89 — level-up activation head",
	InternKeywordHead("Living Weapon"):     "engine/rules/scriptfacts/granted_keyword_trigger.go:34 — living weapon grant trigger",
	InternKeywordHead("Melee"):             "engine/rules/melee.go:44 — melee power bonus",
	InternKeywordHead("Mentor"):            "engine/rules/keyword_refs.go:23 — mentor smaller-creature target",
	InternKeywordHead("Modular"):           "engine/rules/replacement_match.go:1354 — modular death transfer",
	InternKeywordHead("Myriad"):            "engine/effects/myriad.go:12 — myriad attack copies",
	InternKeywordHead("Ninjutsu"):          "engine/rules/cast_targets.go:1331 — ninjutsu return-attacker cost",
	InternKeywordHead("Offspring"):         "engine/rules/offspring.go:45 — offspring half-size token",
	InternKeywordHead("Outlast"):           "engine/rules/keyword_heads.go:89 — outlast activation head",
	InternKeywordHead("Persist"):           "engine/effects/filter.go:1015 — persist filter word",
	InternKeywordHead("Prevent all combat damage that would be dealt to and dealt by CARDNAME."): "engine/rules/replacement_prevent_kw.go:46 — prevent replacement",
	InternKeywordHead("Prevent all combat damage that would be dealt to CARDNAME."):              "engine/rules/replacement_prevent_kw.go:44 — prevent replacement",
	InternKeywordHead("Prevent all damage that would be dealt to CARDNAME."):                     "engine/rules/replacement_prevent_kw.go:47 — prevent replacement",
	InternKeywordHead("Prowess"):     "engine/rules/scriptfacts/granted_keyword_trigger.go:32 — prowess grant",
	InternKeywordHead("Renown"):      "engine/effects/putcounter_params.go:186 — renown counters",
	InternKeywordHead("Replicate"):   "engine/rules/cast_altcost.go:140 — replicate copy count",
	InternKeywordHead("Ripple"):      "engine/rules/trigger_granted.go:171 — granted-ripple cast trigger walk",
	InternKeywordHead("Saddle"):      "engine/rules/pay/paidcost.go:260 — saddle tap-power cost payment",
	InternKeywordHead("Scavenge"):    "engine/rules/quiet_facts.go:193 — scavenge cast facts",
	InternKeywordHead("Soulbond"):    "engine/effects/soulbond.go:65 — soulbond pairing",
	InternKeywordHead("Squad"):       "engine/rules/cast_altcost.go:179 — squad copy count",
	InternKeywordHead("Storm"):       "engine/rules/storm_granted.go:11 — storm copy count grant",
	InternKeywordHead("Sunburst"):    "engine/rules/replacement_etb.go:122 — sunburst enter counters",
	InternKeywordHead("Training"):    "engine/rules/trigger_granted.go:427 — training trigger grant",
	InternKeywordHead("Tribute"):     "engine/effects/counters_put_tribute.go:64 — tribute counter offer",
	InternKeywordHead("TypeCycling"): "engine/rules/trigger_cause.go:18 — typecycling trigger cause",
	InternKeywordHead("Unearth"):     "engine/effects/changezone_params.go:440 — unearth return",
	InternKeywordHead("Vanishing"):   "engine/rules/trigmatch/counters.go:205 — vanishing remove-counter trigger",
	InternKeywordHead("Ward"):        "engine/rules/trigger_granted.go:755 — ward cost trigger",
}

// KeywordExpansionOnly reports whether head's ENTIRE rules meaning is what
// its registered expander mints from the keyword line: gorge has no rule
// outside the expansion that reads the head by name, so a generator that
// emits exactly the expansion lines models the keyword completely. False for
// heads the rules or effects code reads by name (they carry an entry in
// keywordRulesReads) and for heads with no registered expander at all, which
// are not expansions in the first place.
func KeywordExpansionOnly(head string) bool {
	return KeywordExpandable(head) && keywordRulesReads[KeywordHeadIDOf(head)] == ""
}
