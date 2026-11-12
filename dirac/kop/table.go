package kop

// CardID indexes Table.Cards. NoCard is "no card". It is 16 bits so one
// table can hold a whole card pool (the closed-list belief deals any card
// of the pool's decks, not only the two playing).
type CardID uint16

const NoCard CardID = 0xFFFF

// MaxSubtypes bounds Table.Subtypes: a SubMask holds that many.
const MaxSubtypes = 192

// SubMask is a set of subtype indexes (Table.Subtypes).
type SubMask [MaxSubtypes / 64]uint64

// Has reports whether subtype index i is in m.
func (m SubMask) Has(i int) bool { return uint(i) < MaxSubtypes && m[i>>6]&(1<<(i&63)) != 0 }

// With is m with subtype index i added (out of range: m).
func (m SubMask) With(i int) SubMask {
	if uint(i) < MaxSubtypes {
		m[i>>6] |= 1 << (i & 63)
	}
	return m
}

// Or is the union of m and o.
func (m SubMask) Or(o SubMask) SubMask { return SubMask{m[0] | o[0], m[1] | o[1], m[2] | o[2]} }

// And is the intersection of m and o.
func (m SubMask) And(o SubMask) SubMask { return SubMask{m[0] & o[0], m[1] & o[1], m[2] & o[2]} }

// Any reports whether m is non-empty.
func (m SubMask) Any() bool { return m[0]|m[1]|m[2] != 0 }

// Zone names a zone.
type Zone uint8

const (
	ZoneNone Zone = iota
	ZoneLibrary
	ZoneHand
	ZoneBattlefield
	ZoneGraveyard
	ZoneExile
	ZoneStack
	ZoneCommand
	// ZonePhased and ZonePhasedInd (CR 702.26; ext.go)
	// are the object zone of a phased-out permanent, phased out directly or
	// indirectly (because the permanent it is attached to phased out). The
	// permanent keeps its slot and perm record but is on no mask, so no
	// battlefield walk sees it; the oracle projects both as the battlefield.
	ZonePhased
	ZonePhasedInd
	// ZoneSideboard is the zone of a card outside the game (CR 400.11): a
	// seat's sideboard, which only a play permission naming it (MayPlayFrom
	// with StAlsoSideboard, The Holodeck) reads. It follows ZonePhasedInd
	// because the card tables pin the earlier values; see Phased.
	ZoneSideboard
	// ZoneAny is a filter's "any zone" marker, not a real zone: kgen sets it
	// on a SpellCast Tf2TargetsFilter2 chain lowered from a zone-free
	// TargetsValid$ object alternative (Danitha, Spear of Agony's
	// Creature.OppCtrl), and only kernel castTargetsAll reads it, matching an
	// off-battlefield target through the card matcher. Appended: the card
	// tables pin the earlier values.
	ZoneAny
)

// Phased reports whether z is a phased-out permanent's zone (ZonePhased or
// ZonePhasedInd).
func (z Zone) Phased() bool { return z == ZonePhased || z == ZonePhasedInd }

// CardDef is one card's static record.
type CardDef struct {
	Name      string // reports only
	Cost      ManaCost
	Types     TypeMask
	Subtypes  SubMask // indexes Table.Subtypes
	Keywords  KwMask
	Colors    ColorMask
	Power     int8
	Toughness int8
	Loyalty   int8
	WardN     uint8
	Timing    Timing
	Kicker    ManaCost // when Keywords has KwKicker
	AltCost   ManaCost // flashback cost when Keywords has KwFlashback
	Abilities []Ability
	// Enchant is the Aura's "enchant" target spec (the spell targets it).
	Enchant TargetSpec
	// EquipCost is the Equipment's equip cost (an activated ability is implied).
	EquipCost ManaCost

	// Appended by dirac/kgen.
	WardLife uint8 // ward—pay N life (KwWard with WardN = 0)
	// WardEvidence: ward—collect evidence N (KwWard with WardN = 0;
	// Axebane Ferox; kernel kw_wardcost.go)
	WardEvidence uint8
	// WardDiscard: ward—discard N cards (N = 1; KwWard with WardN 0;
	// gorge kw_ward.go's ward_discard pick; kernel
	// kw_wardcost.go, Graveyard Trespasser).
	WardDiscard uint8
	// WardLifePower: ward—pay life equal to its power (KwWard with WardN
	// 0; gorge kw_ward.go's PayLife<X/…> arm reads the ward permanent's power
	// at payment; kernel kw_wardcost.go; Raubahn).
	WardLifePower bool
	// WardSac: ward—sacrifice a permanent matching it (KwWard with WardN
	// 0; gorge kw_ward.go's ward_sac pick; kernel kw_wardcost.go;
	// Vein Ripper, Ygra).
	WardSac FilterID
	// WardDiscOrN: ward—discard a card or pay {N}, N here (KwWard with
	// WardN 0; gorge ward.go's ward_alt choice; kernel kw_wardcost.go;
	// Titania, Rugged Rumbler).
	WardDiscOrN uint8
	// WardWaterbend: ward—Waterbend {N}, N here (KwWard with WardN 0; gorge
	// ward.go's ward_waterbend pick; kernel cost_unlesswaterbend.go; The Unagi
	// of Kyoshi Island).
	WardWaterbend uint8
	// WardPoison: ward—AddCounterYou<N/POISON>, N here (KwWard with WardN 0;
	// gorge ward.go's AddCounterYou arm; kernel cost_restokens.go; The Serpent
	// Society).
	WardPoison uint8
	Leyline    bool // may begin the game on the battlefield from the opening hand
	// SelfFlash: a CastWithFlash EffectZone$ All self grant ("you may cast this
	// spell as though it had flash if ..."; Illusion Spinners). The card's own
	// StatCastWithFlash ability grants it from hand, never from the
	// battlefield (kernel cast_selfflash.go).
	SelfFlash  bool
	SpellCost  Cost // the spell's additional cost (sacrifice / discard; mana zero)
	AltAddCost Cost // AlternateAdditionalCost: may be paid instead of SpellCost (HasAltAdd)
	HasAltAdd  bool
	// KickerCost: the kicker's non-mana cost (one Sac<1/..> or Return<1/..>
	// part, Vayne's Treachery, Chocobo Kick; CR 702.33a). Kicker is then
	// zero mana; a kicked cast pays this through the cost stages (kernel
	// kw_kicker.go).
	KickerCost Cost
	// BeholdN, BeholdF: part one of the either-or additional cost is
	// "behold a <type>" (Behold<1/Type>, CR 701.4a): BeholdN (1) of the
	// payer's permanents or hand cards matching BeholdF. Table data, not a
	// CostFlags bit (none is free); kernel act_behold.go.
	BeholdN uint8
	BeholdF FilterID
	// BeholdExile: the behold is a mandatory additional cost that exiles what
	// it beholds (RaiseCost Cost$ BeholdExile<1/Type>, the Lorwyn Champions;
	// BeholdN is 1, no either-or): the pick is recorded, exiled at payment and
	// linked to the spell for the exiled-with reader (kernel
	// act_beholdexile.go).
	BeholdExile bool
	// SpellWaterbend: the spell's own additional cost is waterbend {N}
	// (RaiseCost Cost$ Waterbend<N>, CR 701.67a; 0 = none): N more generic,
	// payable by tapping artifacts and creatures (kernel act_waterbend.go).
	// Card.Cost stays the printed mana value.
	SpellWaterbend uint8
	// Casualty is K:Casualty:<N> (CR 702.153a, N >= 1; 0 = none): the caster
	// may sacrifice a creature with power N or greater as the spell is cast
	// (the casualty cast, AkCast mode bit 5; dirac/kw_casualty.go). The
	// keyword mask has no free bit, so the threshold is the whole carrier.
	Casualty uint8
	// Bargain is K:Bargain (CR 702.166; false = none): the caster may
	// sacrifice an artifact, an enchantment or a token as the spell is cast
	// (the bargained cast, the same AkCastCasualty action as Casualty; a
	// card never has both, so this field picks the row). The paid flag rides
	// the kicked provenance; dirac/kw_bargain.go.
	Bargain bool
	// Storm is K:Storm (CR 702.40a; false = none): the spell's cast trigger
	// (gorge kwStorm's expansion, engine/cards/kw_storm.go) copies it once
	// for each spell cast before it this turn, its controller choosing new
	// targets for the copies. The kernel enqueues its own native trigger on
	// the cast (abStorm; dirac/kw_storm.go) — no CardDef ability to index —
	// and a copy of a permanent spell resolves into a token
	// (genPrep.anyPermCopy; dirac/copy_permanentspell.go, CR 707.10f).
	Storm bool
	// OptCost, OptBehold: S:Mode$ OptionalCost ("as an additional cost to
	// cast this spell, you may blight N / behold a <type>", CR 601.2b): the
	// optional cast is the kicked cast (AkCast mode 1, gorge's
	// "optionalcost" option) and its paid flag rides the kicked provenance.
	// OptCost is a Blight<N> part (Flags/N/Filter); OptBehold marks the
	// card's BeholdN/BeholdF behold as the part. dirac/cost_optional.go.
	OptCost   Cost
	OptBehold bool
	// OptEvidence is the OptionalCost part CollectEvidence<N> (N 1..255, 0 =
	// none; CR 701.59a, 601.2b): the paid cast is the kicked cast and adds
	// the bsEvidence graveyard pick (gorge postTargetAsks evidenceAsk).
	// dirac/cost_optionalevidence.go.
	OptEvidence uint8
	// OptWaterbend is the OptionalCost part Waterbend<N> (N 1..255, 0 =
	// none; CR 701.67a, 601.2b): the paid cast is the kicked cast, Kicker
	// holds its {N} (no KwKicker, as Offspring's), and the build asks the
	// spell waterbend pick (kernel act_waterbend.go).
	OptWaterbend uint8
	// Teamwork is K:Teamwork:<N> (CR 702.194; 0 = none, N 1..15): the caster
	// may declare the teamworked cast (the same AkCastCasualty action as
	// Casualty/Bargain; a card never has two of them) and tap untapped
	// creatures with total power N or more. The paid flag rides the kicked
	// provenance; ModesPickTW (0 = none) is the charm's pick count under the
	// declared intent (Atlantis Attacks' "choose both"); dirac/kw_teamwork.go.
	Teamwork    uint8
	ModesPickTW uint8
	// ModesPickV (zero K = none) is a spell or triggered charm's
	// CharmNum$ over a Count$Compare with constant arms (Molten Collapse's
	// "choose both instead" once you descended), the paid-additional-cost
	// two-arm, or a general count beside MinCharmNum$ 0 ("choose up to V":
	// the graveyard count of Bumi, the paid X of the Ruinous crew): the pick
	// count, evaluated with the caster's ctx at the mode step and clamped to
	// 0..NModes (gorge effects.CharmModeBounds floors it at 1 only with
	// MinCharmNum$ absent, and no card's constant arms go below 1). Ability.ModesPick is its upper bound and
	// ModesFew the spread to MinCharmNum$; dirac/modal_charmcmp.go.
	ModesPickV ValueRef
	// ModesMinCount: the charm has no MinCharmNum$, so gorge's minimum is
	// the count evaluated at each ask (Pyrrhic Strike's paid additional cost
	// chooses both, the plain cast one). Ability.ModesFew then holds the
	// offer gate's minimum, the unpaid arm (the gate reads an unseeded
	// hand card), and the mode step's minimum is the evaluated cap.
	ModesMinCount bool
	// DungeonNext: a dungeon card (CR 309; the Undercity
	// token script), one entry per room in printed order, the room's
	// ability Abilities[i] (latent); the entry packs its NextRoom$ arrows
	// in printed order, each as room index + 1: low nibble the first,
	// high nibble the second (0 = none; both 0: the bottommost room).
	// dirac/act_initiative.go.
	DungeonNext []uint8
	// TypeValid: the as-enters OpChooseType with B = 1 (ValidTypes$,
	// Dawn-Blessed Pennant) offers only these creature-type vocabulary
	// indexes (Table.TypeVocab's), in vocabulary order (gorge
	// creatureTypeOptions + effects.FilterTypeWords); kernel
	// act_choosetype.go.
	TypeValid []uint16
	// SpentSelf: the card's own SVars read Count$CastTotalManaSpent (a spell's
	// chain or a self-ETB trigger body). gorge's payment
	// planner prices no cast of such a face ("shape:mana_spent_reader",
	// faceWantsCastSpend), so FeatPlannedCast is false for it.
	SpentSelf bool
	// DmgAmt: the card reads Count$TotalDamageReceivedThisTurn (its own
	// received damage); every damage site records the amount for such a card
	// (dirac/value_dmgamount.go).
	DmgAmt bool
	Gift   bool // K:Gift: the caster may promise the opponent a gift as the spell is cast (CR 702.174a; the build's bsGift stage)
	// Offspring: K:Offspring (CR 702.175a): Kicker holds the offspring cost
	// and the kicked cast (AkCast mode bit 0, sfKicked) is gorge's
	// "(offspring)" cast; Keywords has no KwKicker
	Offspring bool
	// Buyback: K:Buyback (CR 702.27a): Kicker holds the buyback cost (zero
	// mana with KickerCost for a Sac<1/..> part) and the kicked cast (AkCast
	// mode bit 0, sfKicked) is gorge's "buyback" cast; Keywords has no
	// KwKicker. The spell returns to its owner's hand as it resolves.
	Buyback bool
	// Landwalk is the printed landwalk keyword's land filter (CR 702.14,
	// 0 = none): the creature can't be blocked while the defending player
	// controls a land matching it.
	Landwalk FilterID
	// ProtColors are the printed "protection from <colour>" keywords' colours
	// (CR 702.16; 0 = none): damage from, blocking by, targeting by and
	// enchanting by a source of one of them are prevented / illegal.
	ProtColors ColorMask
	// HexColors are the printed "hexproof from <colour>" keywords' colours
	// (CR 702.11d; 0 = none): a spell or ability of one of them an opponent
	// controls can't target the permanent.
	HexColors ColorMask
	// ProtFrom is the printed K:Protection:<spec> keywords' source filter
	// (CR 702.16; type, subtype and colour terms only; 0 = none).
	ProtFrom FilterID
	// Infect is printed infect (CR 702.90): its damage is dealt to a player
	// as poison counters and to a creature as -1/-1 counters.
	Infect bool
	// Toxic is the printed toxic N, its instances summed (CR 702.164): a
	// player it deals combat damage to also gets N poison counters.
	Toxic uint8
	// Cascade is the number of printed K:Cascade instances (CR 702.85a/c):
	// casting the spell queues that many cascade triggers, each resolving
	// separately (dirac/kw_cascade.go; grants are not lowered).
	Cascade uint8
	// The printed K:Prevent damage sentences (CR 615.1; engine/cards/
	// kw_prevent.go). PreventDmgTo is "Prevent all damage that would be
	// dealt to CARDNAME." (combat and noncombat); PreventCombatTo and
	// PreventCombatBy are the combat-scoped to/dealt-by halves of the other
	// two sentences. The kernel reads them as per-card replacement bits
	// (genPrep.replBits rbPrev*; combat_prevention.go), the same way the
	// granted KwPreventCombatDamageTo/From bits are read.
	PreventDmgTo    bool
	PreventCombatTo bool
	PreventCombatBy bool
	// AllCreatureTypes is printed changeling (CR 702.73a): every creature
	// type, also one outside the matchup's subtypes (a chosen type).
	AllCreatureTypes bool
	// PrintedSubtypes is a changeling's printed subtypes, before kgen adds
	// every matchup creature type to Subtypes; zero for any other card, whose
	// Subtypes are its printed ones. gorge lists a creature-type choice's
	// options with the printed types its chooser owns first
	// (rules.creatureTypeOptions), which the heuristic bot reads.
	PrintedSubtypes SubMask
	// Faces is a multi-face card's faces (CR 712b): each face's
	// CardID, Faces[0] the front; every face's def carries the same list
	// and Face is its own index (gorge's Object.FaceIdx). FaceMode is the
	// layout (Face*). nil for a single-faced card. An object's CardID is
	// its current face's: the kernel turns a face up by rewriting it and
	// back to Faces[0] where gorge resets FaceIdx (dirac/face_multiface.go).
	//
	// A Room (FaceRoom) carries eight: Faces[0] and Faces[1] are the doors
	// as plain split halves (also the state "face f, door f unlocked" a
	// cast Room enters with), and Faces[2+3*f+j] the other battlefield
	// door states of a Room showing face f: j=0 no door unlocked, j=1 only
	// the other door, j=2 both (RoomState). RoomSplit is how many of a
	// Room def's leading abilities are door Face's lines; the rest are the
	// other door's.
	Faces     []CardID
	Face      uint8
	FaceMode  uint8
	RoomSplit uint8
	// FuseDef is, on a split card printed with fuse (CR 702.102a), the
	// fused split spell's def + 1 (0: none): the card cast from the hand as
	// both halves at once (gorge's "fuse" cast). That def is FusedSpell: its
	// record the halves combined (CR 702.102b-c, 709.4d) and its one spell
	// ability the left half's program then the right half's (CR 702.102d);
	// it shows the front (Face 0, Faces the card's) and exists only on the
	// stack, the card its front again as it leaves (dirac/face_fuse.go).
	FuseDef    CardID
	FusedSpell bool
	// Wither is printed wither (CR 702.80): its damage to a creature is
	// dealt as -1/-1 counters (infect's creature half).
	Wither bool
	// AloneAtk is printed "CARDNAME can't attack alone." (CR 506.5): it is
	// declared as an attacker only beside another attacker; not when it has
	// lost its abilities (dirac/combat_alone.go).
	AloneAtk bool
	// AloneBlk is printed "CARDNAME can't block alone." (CR 506.5), and the
	// block half of the compound "can't attack or block alone.": it is
	// declared as a blocker only beside another blocker; not when it has lost
	// its abilities (dirac/combat_alone.go).
	AloneBlk bool
	// Evasion holds printed blocking-restriction keywords without a KwMask
	// bit (Ev* bits): not when the creature has lost its abilities.
	Evasion uint8
	// TypedMana: the printed face is a Cave or a Desert, so the mana it
	// makes is typed in gorge's pool (state.Game TypedMana; an artifact's
	// is typed too, read from Types).
	TypedMana bool
	// Engines is printed Start your engines! (CR 702.179a): its controller's
	// speed starts at 1 when it has none (dirac/kw_speed.go).
	Engines bool
	// AtkFace: the printed face has a triggered ability gorge's trigger
	// prefilter lets observe an attack declaration (its compiled trigger
	// interests: a mode of the attack-declaration class, a Phase$ line or a
	// mode with no class). gorge then walks the face before its granted
	// keyword triggers, so a granted Melee instance (Ability.MeleeGrant)
	// lists after the triggers statics and effects grant by AddTrigger$;
	// without one, before them (rules/trigger_match.go, the printed-free
	// path). Read only where the table has a Melee grant (kw_melee.go).
	AtkFace bool
	// SagaN is the final chapter number of a Saga (CR 714.1, K:Chapter's
	// count; 0 = not a Saga). The chapter abilities themselves are printed
	// battlefield triggers on CtrLore, each gated at its chapter number.
	SagaN uint8
	// NoAbilities: the printed face has no ability of any kind, the card
	// read of the NoAbilities filter word (W7NoAbilities; gorge
	// FaceHasNoAbilities). Read off the battlefield only.
	NoAbilities bool
	// HasManaAbility: the printed face has a mana ability (gorge
	// effects.FaceHasManaAbility: an activated CR 605.1a ability or a
	// CR 605.1b triggered one; the intrinsic basic-land ability included).
	// Read by PredHasManaAbility on and off the battlefield.
	HasManaAbility bool
	// HasWarp is printed warp (CR 702.185a; no KwMask bit is free): the
	// hand offers the warp cast (its warp cost in WarpCost), and a permanent
	// spell cast that way is exiled at the beginning of the next end step
	// with a recast from exile on a later turn (dirac/kw_warp.go).
	HasWarp bool
	// WarpCost is the printed warp cost (HasWarp).
	WarpCost ManaCost
	// Impending is printed K:Impending:N:cost's N (CR 702.176a; 0 = none):
	// the hand offers the "impended" cast at ImpendingCost; cast that way
	// the permanent enters with N time counters and is not a creature while
	// it has one (kernel kw_impending.go).
	Impending     uint8
	ImpendingCost ManaCost
	// HasPlot is printed plot (CR 702.170a; no KwMask bit is free): the
	// hand offers the plot special action at PlotCost, exiling the card
	// with the plotted designation and a later-turn free cast from exile
	// (dirac/kw_plot.go).
	HasPlot bool
	// PlotCost is the printed plot cost (HasPlot).
	PlotCost ManaCost
	// TapPowerCrew / TapPowerSaddle are the static "this creature crews
	// Vehicles / saddles Mounts as though its power were N greater"
	// (graveexile; Mode$ TapPowerValue). Added only while paying a Crew or Saddle
	// tap-power cost, never Station.
	TapPowerCrew   int8
	TapPowerSaddle int8
	// Convoke is printed convoke (CR 702.51a; no KwMask bit is free):
	// the cast may tap creatures for its mana cost (dirac/kw_convoke.go).
	Convoke bool
	// Delve is printed delve (CR 702.66a; no KwMask bit is free): the cast
	// exiles graveyard cards toward its generic cost (dirac/kw_delve.go).
	// A granted delve reads the same path.
	Delve bool
	// ReboundAb is 1 + the latent ability index of a printed K:Rebound's
	// recast body (CR 702.88a; 0 = no Rebound): a spell cast from the hand
	// is exiled as it resolves and, at its controller's next upkeep, the
	// body offers the free cast of the exiled card (dirac/kw_rebound.go).
	ReboundAb uint8
	// Improvise is printed improvise (CR 702.126a): the cast may tap
	// untapped artifacts, each paying {1} of the generic part. Never set
	// with Convoke (kgen refuses both). Shares the convoke payment path.
	Improvise bool
	// Harmonize is printed harmonize (CR 702.180a): a graveyard cast for
	// AltCost, one untapped creature tapped to reduce its generic part by
	// the creature's power (dirac/kw_harmonize.go). Never set with
	// Convoke, Improvise or KwFlashback (kgen refuses them).
	Harmonize bool
	// Mayhem is printed mayhem (CR 702.187a): a graveyard cast for AltCost if
	// the card was discarded this turn (dirac/kw_mayhem.go). Never
	// set with Flashback or Harmonize (kgen refuses them).
	Mayhem bool
	// Aftermath marks the right half of a split card printed with aftermath
	// (CR 702.127a): this face is cast only from its owner's graveyard, for
	// its own mana cost (gorge's "aftermath" cast), never from the hand, and
	// the spell is exiled whenever it leaves the stack (dirac/face_aftermath.go).
	Aftermath bool
	// BeamMeUp is printed beam me up: a graveyard cast for AltCost that also
	// returns a creature you control to its owner's hand (FbCost), then exiles
	// the spell (dirac/kw_beammeup.go).
	BeamMeUp bool
	// Retrace is printed K:Retrace (CR 702.81a; false = none): a graveyard
	// cast paying the PRINTED cost (never a substitution) plus the additional
	// RetraceCost, a discard of one land card (CostDiscardFiltered); the
	// spell rests in its owner's graveyard on resolution (no sfFlashback
	// exile; beginCast marks the mode-2 retrace cast's entry trigPlRetrace,
	// and a copy never carries the mark, CR 707.10). Never set with
	// Flashback, Harmonize, Mayhem, BeamMeUp, Convoke, Improvise, Delve or a
	// spell additional cost (kgen refuses them; no corpus card carries one).
	Retrace     bool
	RetraceCost Cost
	// MorphCost is the turn-face-up cost of a printed Morph, Megamorph or
	// Disguise keyword (CR 702.37e, 702.168c, 702.169b; zero with
	// MorphFamily 0). MorphFamily says which (CardDef.Face*Family).
	MorphCost   ManaCost
	MorphFamily uint8
	// MorphReduce (zero K = none) is the turn-up cost's printed generic
	// reduction, evaluated for the permanent's controller and subtracted
	// from MorphCost's generic, floored at zero (dirac/face_morphreduce.go).
	MorphReduce ValueRef
	// CostEmpty is an EMPTY printed mana-cost text, the one case gorge's
	// manifestFaceUpCost rejects outright (strings.TrimSpace(f.ManaCost)=="");
	// Cost is also the zero ManaCost for {0} and for a printed "no cost", so
	// those two are told apart from empty text here. A manifested or cloaked
	// creature card turns up for its printed mana cost (CR 701.40b/701.58b):
	// {0} and "no cost" both price as a free cost and are offered.
	CostEmpty bool
	// Defense is a Battle's printed Defense (CR 310.4; 0 for any other
	// card): it enters with that many CtrDefense counters
	// (dirac/face_battle.go).
	Defense int8
	// HasSneak is printed sneak (CR 702.190a; no KwMask bit is free): in
	// its controller's declare-blockers step the hand offers the sneak cast
	// at SneakCost plus returning an unblocked attacker, and a permanent
	// spell cast that way enters tapped and attacking that attacker's
	// defender (CR 702.190b; dirac/kw_sneak.go).
	HasSneak bool
	// SneakCost is the printed sneak cost (HasSneak).
	SneakCost ManaCost
	// HasWebSling is printed web-slinging (CR 702.188a; no KwMask bit is
	// free): the hand offers the web-slinging cast (AkWebSling) at
	// WebSlingCost plus returning a tapped creature its caster controls to
	// its owner's hand (dirac/kw_websling.go).
	HasWebSling bool
	// WebSlingCost is the printed web-slinging cost (HasWebSling).
	WebSlingCost ManaCost
	// HasMayFlash is printed K:MayFlashCost with a mana parameter (CR
	// 702.8's "as though it had flash" permission bought for an extra cost:
	// "you may cast this as though it had flash if you pay {2} more"; no
	// KwMask bit is free): when the card's own timing does not allow the
	// cast, the hand offers the may-flash cast (AkMayFlash) at Cost plus
	// MayFlashCost (dirac/kw_mayflash.go).
	HasMayFlash bool
	// MayFlashCost is the extra mana the may-flash cast adds (HasMayFlash).
	MayFlashCost ManaCost
	// MayFlashBehold: the may-flash extra is Behold<1/Type> (Molten
	// Exhale), no mana: the cast is offered only with a candidate and owes
	// the behold (BeholdN/BeholdF) in its cost-pick stage (kw_mayflash.go).
	MayFlashBehold bool
	// FbBehold: the printed flashback cost carries Behold<N/Type> beside
	// its mana (Kindle the Inner Flame): the printed flashback cast is
	// offered only with BeholdN candidates and owes the behold (BeholdN/
	// BeholdF) in its cost-pick stage (dirac/act_behold.go).
	FbBehold bool
	// HasEvoke is printed evoke (CR 702.74a; no KwMask bit is free): the
	// hand offers the evoked cast at EvokeCost (gorge's "evoked" option,
	// FlagEvoked), and a permanent cast that way queues a mandatory
	// "sacrifice it" trigger as it enters (dirac/kw_evoke.go).
	HasEvoke bool
	// EvokeCost is the printed evoke cost (HasEvoke).
	EvokeCost ManaCost
	// HasDash is printed dash (CR 702.109a; no KwMask bit is free): the hand
	// offers the dashed cast at DashCost (gorge's "dashed" option,
	// FlagDashed); the permanent that cast produces gains haste and returns
	// to its owner's hand at the beginning of the next end step
	// (dirac/kw_dash.go).
	HasDash bool
	// DashCost is the printed dash cost (HasDash).
	DashCost ManaCost
	// HasForetell is printed foretell (CR 702.143a; no KwMask bit is free):
	// during its owner's turn the hand offers the {2} special action that
	// exiles the card face down, and on a later turn the exile offers the
	// cast for ForetellCost (dirac/kw_foretell.go).
	HasForetell bool
	// ForetellCost is the printed foretell cost (HasForetell): what the
	// later cast pays, never the {2} action's price.
	ForetellCost ManaCost
	// HasOverload is printed overload (CR 702.96a; no KwMask bit is free):
	// the hand offers the overloaded cast at OverloadCost, and the resolving
	// spell runs OverloadProg, the "each" variant of its printed program
	// (dirac/kw_overload.go).
	HasOverload bool
	// OverloadCost is the printed overload cost (HasOverload).
	OverloadCost ManaCost
	// OverloadProg is the overloaded cast's program (CR 702.96a): the
	// printed spell's targeted instructions rewritten to their each-sweeps.
	// Nil when the card cannot be modelled in bounded code.
	OverloadProg []Instr
	// HasBestow is printed bestow (CR 702.103; no KwMask bit is free): the
	// hand offers the bestowed cast (AkBestow) at BestowCost; the spell is
	// an Aura with enchant creature (BestowTgt) and the permanent an Aura,
	// not a creature, while it is attached (dirac/kw_bestow.go).
	HasBestow  bool
	BestowCost ManaCost
	BestowTgt  TargetSpec
	// HasModeCost: Spree (CR 702.172a): each chosen mode i adds
	// ModeCost[i] to the spell's total cost (kernel
	// kw_spree.go).
	HasModeCost bool
	ModeCost    [4]ManaCost
	// FbCost is a printed flashback cost's non-mana part (KwFlashback;
	// AltCost holds its mana, {0} when none): Sac<N/Spec> or
	// tapXType<N/Spec>, paid through the cast's pick stages in place of
	// the spell's additional cost (gorge flashbackCost).
	FbCost Cost
	// RemPersist: the card's enters trigger remembers objects its leaves
	// trigger reads (gorge's Remembered persisting on the source: Journey
	// to Nowhere, Mesmeric Fiend; dirac/zone_rememberpersist.go).
	RemPersist bool
	// RemSeed / RemKeep: with RemPersist, the abilities (bit i = Abilities[i])
	// whose resolution starts with its source incarnation's kept set, and
	// those whose finished set is kept. The enters/leaves pair sets both to
	// every ability; a Saga sets its reading and its writing chapters (The
	// Measure of a Man: chapter III returns what chapter I exiled).
	RemSeed, RemKeep uint32
	// ExiledWith: the card reads the exiled-with source link (PredExiledWith
	// or ExExiledWithN): its resolutions record the cards they exile
	// (dirac/zone_exiledwith.go, genPrep.exwRec).
	ExiledWith bool
	// CombatHitRec: the card reads CtrlHitBySrc against itself: a
	// combat hit on a player by one of its objects is recorded
	// (dirac/value_turntallies.go, genPrep.cdRec).
	CombatHitRec bool
	// ExiledWithRev: the card's text names ExiledWithSource, so gorge also
	// sets the reverse Object.ExiledWith scalar on what it exiles
	// (exileProvenanceNeeded); without it only the forward ExiledCards list
	// (and a Hideaway's marker) records the link.
	ExiledWithRev bool
	// MinBlockers is a printed S:Mode$ MinMaxBlocker Min$ N on the card
	// itself (CR 509.1b: "can't be blocked except by N or more creatures";
	// 0 = none): menace's floor generalised (dirac/combat.go minBlockers).
	// Not when the creature has lost its abilities. Troll of Khazad-dum.
	MinBlockers uint8
	// ExtraBlocks is a printed S:Mode$ Continuous | AffectedDefined$ Self |
	// CanBlockAmount$ 1 (CR 509.1a): the creature can block one additional
	// attacker each combat; 0 none. Not when it has lost its abilities.
	ExtraBlocks uint8
	// SpendRead: an SVar reads Count$CastTotalManaSpent (Memory Deluge):
	// gorge's cast planner prices no cast of it (faceWantsCastSpend,
	// "shape:mana_spent_reader"; kernel featview.go plannedCastCost).
	SpendRead bool
	// AltAddOne: SpellCost came from a one-part AlternateAdditionalCost
	// (Dusk Rose Reliquary): no ask, but gorge's cast planner prices no
	// cast of a card with the keyword ("shape:optional_cost").
	AltAddOne bool
	// Lure: "All creatures able to block CARDNAME do so." (CR 509.1c; gorge
	// kw:AllMustBlock): while it attacks, every creature able to block it
	// must block it or another lure attacker; not when it has lost its
	// abilities (kernel combat_lure.go).
	Lure bool
	// MustBeBlocked: "CARDNAME must be blocked if able." (CR 509.1c; gorge
	// hidden keyword MustBlock): while it attacks and some creature could
	// block it, the defender's declaration must block it, one blocker
	// satisfying it, as long as that does not cost a larger requirement
	// (kernel combat_mustbeblocked.go). Not when it has
	// lost its abilities.
	MustBeBlocked bool
	// Daybound / Nightbound (CR 702.145; gorge rules/daynight.go): the
	// face makes it day / night when neither is (CR 702.145d/g), a daybound
	// front face turns to its back as it becomes night and a nightbound
	// back face to its front as it becomes day, and a daybound front face
	// entering at night enters back face up (kernel
	// kw_daynight.go).
	Daybound, Nightbound bool
	// HasMadness is printed madness (CR 702.35a; no KwMask bit is free): a
	// discard exiles the card instead and queues the cast-or-graveyard
	// trigger; MadnessCost is its cast cost (dirac/kw_madness.go).
	HasMadness  bool
	MadnessCost ManaCost
	// HasMiracle is printed miracle (CR 702.94a): a first draw of the turn
	// offers the reveal, whose trigger casts the card from hand for
	// MiracleCost as it resolves (dirac/kw_miracle.go).
	HasMiracle  bool
	MiracleCost ManaCost
	Paradigm    uint8 // 1 + the table's Paradigm-name index; 0 = none
	// CastBF is the filter Count$LastStateBattlefieldWithFallback counts
	// the battlefield permanents matching it as the
	// spell was cast, frozen at beginCast into the spell entry's trigMV
	// (dirac/value_castbattlefield.go); 0 = the card does not read the head.
	CastBF FilterID
	// NoPT: the card prints no power or toughness, so a power or toughness
	// compare never matches it as a card (CR 208.3; gorge's P/T-less object,
	// dirac/filter_ptvalue.go).
	NoPT bool
	// TwoSeat: the lowering reads a two-player identity (the one opponent,
	// the one defending player, an opponents count of one) that is not the
	// card's meaning with three or more players. The two-seat game ignores
	// it; a game of more seats refuses a table carrying it (dirac
	// StallTwoSeat, stallclass.go). Set by dirac/kgen's lc.twoSeat.
	TwoSeat bool
	// ProduceCol is the colours the face's printed mana abilities could
	// produce ({C} dropped; gorge effects.producibleSymbols over
	// Face.ManaAbilities, the union a blank, "Any" or "Combo Any" Produced$
	// makes WUBRG, a "Combo <colours>" exactly those, a plain list its
	// coloured runes). A Produce-mode reflected ability (Ability.ManaProduce)
	// unions it over the matched permanents at each use (kernel
	// mana_reflected.go produceColours).
	ProduceCol ColorMask
	// Identity is the card's colour identity (cards.Card.ColourIdentity, every
	// face's union, WUBRG in bits 0..4 as ColorMask): a commander-identity mana
	// ability (Ability.ManaIdentity) unions it over its controller's commanders.
	Identity ColorMask
}

// CardDef.MorphFamily values: the face-down keyword the card was printed with.
const (
	MorphNone uint8 = iota
	MorphMorph
	MorphMegamorph
	MorphDisguise
)

// CardDef.FaceMode layouts (gorge's Card.AlternateMode).
const (
	FaceNone  = iota
	FaceModal // a modal double-faced card (CR 712.4d): the back face is a land played from the hand (gorge's "modal_land") or a nonland face cast from it (gorge's "modal_spell")
	// FaceAdventure is an adventurer card (CR 715): face 1 is the
	// Adventure instant or sorcery, cast from the hand (gorge's
	// "adventure_alt"); it resolves into exile, from where the front is
	// cast (gorge's "adventure_recast"), dirac/face_castface.go.
	FaceAdventure
	// FaceSplit is a split card (CR 709), no Room: face 1 is the right
	// half, cast on its own from the hand (gorge's "split_alt"), or from the
	// graveyard only when it carries aftermath (CardDef.Aftermath, gorge's
	// "aftermath"); a card printed with fuse is cast as both halves too
	// (CardDef.FuseDef, gorge's "fuse"); off the stack the mana value is the
	// sum of the halves (CR 709.4b).
	FaceSplit
	// FaceTransform is a transforming double-faced card or token (gorge's
	// "DoubleFaced", CR 712): a SetState Mode$ Transform flips it to its
	// back face in place (CR 701.27a) and restamps the layer timestamp
	// (CR 613.7g); a ChangeZone Transformed$ True return enters the back
	// face (CR 712.14a) and a Mode$ Transformed trigger follows the flip.
	// A card any of whose faces is a Battle (CR 310) is refused.
	FaceTransform
	// FacePrepare is a preparation card (CR 722): face 1 is the prepare
	// spell, an instant or sorcery never cast from the hand; a prepared
	// permanent's exile copy is cast as gorge's "prepared_copy"
	// (dirac/face_prepare.go).
	FacePrepare
	// FaceRoom is a two-door Room (CR 709.5, gorge rules face_rooms.go): either
	// door is cast (gorge's room_alt casts the other); the face persists in
	// every zone; on the battlefield the CardID names the door state
	// (CardDef.Faces, dirac/face_rooms.go).
	FaceRoom
	// FaceOmen is an omen card (CR 720): face 1 is the Omen instant or
	// sorcery, cast from the hand (gorge's "omen_alt"); a resolved Omen goes
	// to the bottom of its owner's library, which is then shuffled;
	// countered or fizzled, it goes to the graveyard; off the stack it is
	// its front (dirac/face_omen.go).
	FaceOmen
)

// RoomState is the Faces index of a Room's battlefield state showing face
// f with door mask m unlocked (bit i = door i).
func RoomState(f, m uint8) int {
	switch {
	case m == 1<<f:
		return int(f)
	case m == 0:
		return 2 + 3*int(f)
	case m == 3:
		return 4 + 3*int(f)
	}
	return 3 + 3*int(f)
}

// Table is a matchup's whole card universe: every card a deck can hold plus
// every token those cards can create. Filters[0] must be the zero Filter.
type Table struct {
	Name     string
	Cards    []CardDef
	Filters  []Filter
	Subtypes []string

	// Appended by dirac/kgen: the side tables generated programs index.
	Exprs     []Expr     // VExpr ValueRefs index this; Exprs[0] is unused
	Conds     []Cond     // CondIDs index this; Conds[0] is "always"
	Overrides []Override // OpOverride / StatOverride index this
	Emblems   []Emblem   // OpCreateEmblem indexes this
	// TypeVocab maps each entry of gorge's creature-type vocabulary (the
	// option list of OpChooseType, in vocabulary order) to the matchup's
	// subtype index, 255 when no object in the matchup can have it.
	TypeVocab []uint8
	// NameCards are the CardIDs of the matchup's real nonland cards, ascending
	// (the order of gorge's sorted name list over the same cards): the options
	// of an as-enters ExtChooseName. Set only when a card of the matchup
	// chooses a name; dirac/repl_choosename.go.
	NameCards []CardID
	// NameCardsAny are the CardIDs of every real card of the matchup, lands
	// included, ascending: the options of an ExtChooseName with B >= 1 (a
	// NameCard with no ValidCards$, Sorcerous Spyglass).
	NameCardsAny []CardID
}

// Filter returns the filter with id f.
func (t *Table) Filter(f FilterID) *Filter { return &t.Filters[f] }

// TapPays reports whether the card's cast announces tapped permanents toward
// its mana cost (printed Convoke or Improvise; dirac/kw_convoke.go).
func (c *CardDef) TapPays() bool { return c.Convoke || c.Improvise }

// AndNot is m without the members of o.
func (m SubMask) AndNot(o SubMask) SubMask { return SubMask{m[0] &^ o[0], m[1] &^ o[1], m[2] &^ o[2]} }
