package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// pcard is a card's precomputed fast record.
type pcard struct {
	types   kop.TypeMask
	kw      kop.KwMask
	cost    kop.ManaCost
	total   uint8 // generic + all pips (X excluded)
	pipMask uint8 // colours (bit per W U B R G C) with a pip
	power   int8
	tough   int8
	timing  kop.Timing
	spellAb int8 // index of the AbSpell ability, -1 none
	manaAb  int8 // index of the first AbMana ability, -1 none
	manaCol kop.ColorMask
	// manaChosen: the mana ability makes one mana of the permanent's chosen
	// colour (perm.chosen; Heraldic Banner), not manaCol
	manaChosen bool
	// manaFixedChosen: the chosen-colour ability is a second bare {T} one
	// beside a fixed first ability (Quirion Elves): the source keeps its
	// fixed census position, unlike a single Combo Chosen ability
	manaFixedChosen bool
	manaN           uint8
	flashStat       bool // the card has a StatCastWithFlash static: castStatic skips every other source
	manaDyn         bool // a dynamic-amount variant (varPC): gorge's evaluated-amount layer
	manaOne         bool // Produced$ Any with an amount above one: all of it one colour, whichever is chosen
	manaTap         bool // the mana ability costs only {T}
	manaSac         bool // the mana ability costs {T} and sacrificing the source (Treasure)
	manaSacOnly     bool // the mana ability costs only sacrificing the source (Eldrazi Spawn)
	// manaIntrinsic: the view is a granted basic land type's intrinsic
	// "{T}: Add [c]" ability, not the card's own (static_landtype.go
	// blgView). A source whose own ability is a planner-deferred rider keeps
	// the intrinsic unit and the tap runs no rider.
	manaIntrinsic bool
	// castSrc: tapping this source for a cast is recorded (noteTapSrc):
	// csTreasure, the card's printed subtypes include Treasure (CastSpecTreasure;
	// set only when the cast ring is on); csObserved, the card carries a
	// Tf2ManaFromSelf SpellCast trigger (mana_spendtrigger.go)
	castSrc   uint8
	manaMulti bool          // several plain fixed-colour mana abilities folded into manaCol (mana_multicolour.go)
	manaIdent bool          // a commander-identity source's view (kop.Ability.ManaIdentity): gorge's planner appends its unit with the choice-shaped sources even when the identity is one colour (mana_reflected.go identityColours)
	manaRefl  bool          // an imprint mox's view: gorge's census keeps its unit at the source's battlefield position even when it is a colour choice (mana_imprint.go)
	altCol    uint8         // a granted basic land type's intrinsic alternative beside this view's own ability: 1 + its colour; the source taps for one of the two (static_landgrantalt.go); 0 none
	blgFirst  uint8         // a granted basic land type's view: 1 + its first ability's colour, the payment window's answer (static_landtype.go); 0 otherwise
	rankCol   kop.ColorMask // a restricted source's colours as gorge's rank keys read them (all its alternatives', admitted or not); 0: manaCol
	restrU    uint8         // a restricted source's planner view: 1 fixed colour, 2 widened {C} unit (mana_restrictedsource.go)
	// manaXv: the mana an all-at-once ability (kop.Ability.ManaEach) adds
	// beyond its first colour, which manaCol holds: a fixed vector the planner,
	// availability and taps read as a tap's extra (mana_tapextra.go srcExtra)
	manaXv [kop.NumManaKinds]uint8
	// wideN, wideCol: a widened restricted source's view whose restricted
	// ability makes wideN mana (of one colour of wideCol) where the census
	// ability makes manaN: the source's two alternatives differ in amount
	// (mana_restrictedsource.go restrPairUnit). Zero: they make the same.
	// wideChosen: wideCol holds only a printed colour; the alternative also
	// makes the permanent's chosen colour (Sunken Citadel), read per slot
	// (restrWideMask).
	wideN      uint8
	wideCol    kop.ColorMask
	wideChosen bool
	restrLife  uint8 // a restricted {T}, Pay N life ability's N: a last-resort unit that pays the life as it is used (mana_restrictedcost.go)
	restrFloat bool  // a restricted ability whose unit floats restricted when a plan leaves part of it unspent (rkExt; Steelswarm Operator's activation {U}{U}, Vodalian Arcanist's {C})
	cdRec      bool  // the card reads CtrlHitBySrc against itself: its combat hits on players are recorded (value_turntallies.go)
	food       bool  // the card's printed subtypes include Food (the sacFood tally; value_turntallies.go)
	manaTyped  bool  // gorge tallies the mana it adds as typed (an artifact, a Desert or a Cave: mana_artifact.go)
	manaSnow   bool  // gorge tallies the mana it adds as snow (the printed Snow supertype and no typed word: CR 106.3, effects.ManaProducerTag; mana_snow.go)
	// painCol, painN: the source's painful mana alternative, its colours and
	// its damage to the controller (a painland's second ability; for
	// painOnly, a pain-only source's sole one, manaAb itself), gorge's
	// last-resort Consequence{Damage} (mana_pain.go)
	painCol  kop.ColorMask
	painN    uint8
	painOnly bool
	// painLife: the painful alternative pays painN life as its cost
	// ({T}, Pay 1 life: Starting Town) instead of dealing damage (gorge's
	// Consequence{Life}: the same last-resort cost and lethal guard;
	// mana_pain.go)
	painLife bool
	// grantSac: a granted mana ability whose cost is sacrificing the source
	// alone (Lotus Ring's equipped creature; mana_grantcost.go). Only the
	// grant search, the gap readers and the manual window read it: the
	// printed sacrifice readers (manaSacOnly) never see a grant.
	grantSac bool
	// cantAct: the card prints a StatCantActivate static (cast_cantactivate.go):
	// actForbiddenBy walks only these cards plus emblems.
	cantAct     bool
	trigMask    uint32 // bit per kop.TrigEvent the card's triggers listen to
	hasStat     bool
	grantStat   bool  // an override static or a continuous static with a Grant
	nAct        uint8 // number of AbActivated abilities
	needsTgt    bool  // the spell ability has a mandatory target
	enterTap    bool  // ReplEntersTapped
	enterCtr    int8  // ReplEntersWithCounters count (+1/+1 kind assumed when C=0)
	enterCK     kop.CounterKind
	noCtrCond   kop.CondID // a ReplCantBeCountered with a Cond (Banefire: X is 5 or more), read with the entry; ahead of noCtr so the 16-bit enterCK leaves pcard no pad byte (92 bytes)
	noCtr       bool       // ReplCantBeCountered
	noUntap     uint8      // ReplDoesntUntap: untapPlain, or untapGated for one with a Cond (Alirios, Enraptured), read at the untap (repl_untapself.go)
	mayNotUntap bool       // ReplMayNotUntap: a tapped one asks "untap or keep tapped" at its controller's untap step (repl_untapchoice.go)
	land        bool
	altCost     bool // the card has its own mana alternative cost (genPrep.altFree; cast_altfree.go) or Impending: the hand walk's one-byte gate (offerAltHand)
	plotTrig    bool // a "when this card becomes plotted" trigger (TrigBecomesPlotted; kw_plot.go)
	creature    bool
	instant     bool  // castable at instant speed (instant type or flash)
	sagaN       uint8 // final chapter number of a Saga (0 = not a Saga; kw_saga.go)
	// ctrMana: the card's first mana ability costs only a counter put on
	// itself, once each turn (Wall of Roots; mana_countercost.go)
	ctrMana bool
}

type prep struct {
	nsub          uint8 // len(t.Subtypes): the first outside-type code (chars_gen.go outsideSub)
	t             *kop.Table
	nameRep       []CardID // printed-name representative per card (nameRepOf); nil when every printed name is unique
	cards         []pcard
	srcs          []layerSrcCard // per card, the layer walk's ability masks (prepLayerSrc; static_layerwalk.go); on prep, not genPrep, so the pinned fixed-State inventory is unchanged
	gen           *genPrep       // generated-table extension (gen.go); nil for hand-written tables
	gyCont        bool           // some Continuous static works from its card's graveyard (tableHasGyCont; static_gycont.go): read only by the layer-6 walk and the cold landwalk/prowess readers
	cold          coldLayout
	handStat      []bool // card i has a static whose condition reads a hand size (Hellbent; prepHandStat, cast_handleaves.go); on prep, not genPrep, so the pinned fixed-State inventory is unchanged
	gainsOfAttach bool   // a GainsAbilitiesOf$ static and an Equip ability: a non-attachment can gain Equip and attach itself (CR 704.5p sba arm; static_unattachcreature.go); on prep, not genPrep, so the pinned fixed-State inventory is unchanged
	anyNoDmg      bool   // a card or emblem has a StatNoCombatDamage static (combat.go's combatAmount); on prep, not genPrep, so the pinned fixed-State inventory is unchanged
	anyAsUnblk    bool   // a card or emblem has a StatAssignCombatDamageAsUnblocked static (the as-unblocked election, combat_asgngrant.go); on prep, not genPrep, so the pinned fixed-State inventory is unchanged
	// embPrev: some emblem carries a static damage prevention
	// (tableEmbHasStaticPrev, repl_staticprevent_emblem.go); on prep, not
	// genPrep, so the pinned fixed-State inventory is unchanged
	embPrev bool
	// embExileLeave: some emblem carries a battlefield-origin dies-to-exile
	// replacement (tableEmbHasExileLeave, repl_exileleaveemb.go); on prep,
	// not genPrep, so the pinned fixed-State inventory is unchanged
	embExileLeave bool
	cda           bool // some card has a characteristic-defining P/T (StatCDAPT)
	// grantSrcCard: per card, a Continuous static grants a mana ability,
	// whoever it applies to (prepTokMana; contGrantLive); contGrantAlways: a
	// mana grant no battlefield card marks (an override record's, a printed
	// GainsAbilitiesOf one) reads as always on. On prep, not genPrep, so the
	// pinned fixed-State inventory is unchanged
	grantSrcCard    []bool
	contGrantAlways bool

	// landGrantPair: a granted basic land type beside a mana source whose own
	// ability is no plain {T} one (static_landgrantpair.go): StallLandGrant; on
	// prep, not genPrep, so the pinned fixed-State inventory is unchanged
	landGrantPair bool
	// gainsOfPair: a GainsAbilitiesOf$ static beside a creature ability its
	// walk does not model (static_gainsofpair.go): StallGainsOf; on prep, not
	// genPrep, so the pinned fixed-State inventory is unchanged
	gainsOfPair bool
	casualty    bool // some card has Casualty (kw_casualty.go): guards the casualty cast offer
	mayhem      bool // some card has Mayhem (kw_mayhem.go): guards the discard mark, the cast mark and the graveyard offer
	escape      bool // some card has Escape (kw_escape.go): guards the escape cast mark and the graveyard offer
	retrace     bool // some card has Retrace (kw_retrace.go): guards the graveyard offer
	bargain     bool // some card has Bargain (kw_bargain.go): guards the bargained cast offer
	entwine     bool // some card has Entwine (kw_entwine.go): guards the entwined cast's charm pick count
	teamwork    bool // some card has Teamwork (kw_teamwork.go): guards the teamworked cast offer and the charm pick count
	gift        bool // some card has Gift (kw_gift.go): guards the gift paths on the hot cast/trigger path
	// bargainRed: some card carries a StBargained cost static
	// (cost_reducespell.go): guards the bargained pricing
	bargainRed bool
	// offspring: some card has Offspring, Buyback or Entwine (kw_offspring.go,
	// kw_buyback.go, kw_entwine.go): guards the extra-cost cast offer on the priority walk
	offspring bool
	// anyGate: some ability has a threshold gate (Ability.Gate, kw_class.go):
	// guards the gated trigger/activation checks on the hot paths
	anyGate bool
	// anyStateCtr: some trigger is a counter-threshold state trigger
	// (kop.Tf4StateCtr, trig_statectr.go): guards the outstanding gate on
	// the counter-put path and the re-check at an instance's leave
	anyStateCtr bool
	// anyFbCard: some card prints flashback (kop.KwFlashback): with no
	// grant either, priorityOptions skips its graveyard walk
	anyFbCard bool
	// anyRoom: some card is a two-door Room (kop.FaceRoom;
	// face_rooms.go): guards the Room entry, leave and unlock paths
	anyRoom bool
	// anyFullyUnlock: some ability is a FullyUnlock watcher
	// (kop.TrigFullyUnlock; face_fullyunlock.go): guards the scan the
	// unlock action runs when a Room becomes fully unlocked
	anyFullyUnlock bool
	// anyDayNight: some card is daybound or nightbound (CR 702.145;
	// kw_daynight.go): guards the turn, untap, SBA and
	// entry hooks
	anyDayNight bool
	// phasing: some program is ExtPhases (kw_phasing.go):
	// guards the slot allocator, the untap and cleanup steps and a host's departure
	phasing bool
	// anyYourTurns: some value reads ExYourTurns' turn count:
	// beginTurn counts the active player's turns
	anyYourTurns bool
	// anySelfFlash: some card is a SelfFlash carrier (cast_selfflash.go):
	// the one guard castFlash adds beside the battlefield-static test
	anySelfFlash bool
	// anyEmbFlash: some emblem carries a StatCastWithFlash static (an
	// Effect-delivered "you may cast ... as though they had flash"): the
	// other guard castFlash adds
	anyEmbFlash bool
	// anyFlashStat: some card carries a StatCastWithFlash static (cast_selfflash.go):
	// guards castStatic's all-seats scan of the battlefield statics
	anyFlashStat bool
	// anyFlashAny: anyFlashStat, anySelfFlash or anyEmbFlash: castFlash's one inline guard
	anyFlashAny bool
	// anyEmbForget: some emblem carries a ForgetOnCast$ filter for a stack
	// keyword grant (kop.Emblem.Forget): the completed cast ends the emblem
	// (cast_selfflash.go embFlashForget, after the cascade triggers)
	anyEmbForget bool
	// anyRedNC: some value reads ExRedNCDmg: replDamage tallies
	// the noncombat damage red sources deal, beginTurn clears the tally
	anyRedNC bool
	// anyOppsAtt: some value reads ExOppsAttacked: finishAttacks records that the
	// declaration named a player defender (value_oppsattacked.go)
	anyOppsAtt bool
	// w8: some filter is a PredWord7 sub-kind >= W7FaceUp:
	// the one guard matchPermOne adds to pass ctrl and src
	w8 bool
	// anyCtrAdded: some value reads ExCtrAdded: every counter
	// placement notes the per-turn record (ctr_addedthisturn.go)
	anyCtrAdded bool
	// anySaga: some card is a Saga (CardDef.SagaN; kw_saga.go):
	// guards the precombat-main lore counter and the CR 714.4 SBA
	anySaga bool
	// anyInit: some card is a dungeon (CardDef.DungeonNext; the initiative,
	// act_initiative.go): a prep byte beside anySaga, so the
	// per-SBA-pass and per-upkeep gates read no genPrep
	anyInit bool
	// dungeons: the plain venture's candidate dungeons, in table (key)
	// order (prepInit; act_venture.go): every dungeon card of the table but
	// the initiative's Undercity (genPrep.dungeon). Prep data like cards,
	// never a pinned genPrep field (TestNoNewFixedStateFields).
	dungeons []CardID
	// cardKind: per card, its enters program asks for a card type, so its
	// chosen byte is a cardTypeChoice index (act_choosecardenter.go); nil when
	// no card does. Prep data, never a pinned genPrep field.
	cardKind []bool
	// anyAbilTgtPT: some activated ability reads its target's power or
	// toughness: the target's last known P/T is snapshot as it resolves
	// (value_abiltgtpt.go). A prep field, never a pinned genPrep one.
	anyAbilTgtPT bool
	// embalmCard / embalmAb: the granting card and ability index of the
	// latent embalm body a table's OpGrantEmbalmOwnCost names (prepEmbalmGrant;
	// kw_embalmgrant.go); anyEmbalmGrant is set when the table grants embalm.
	// Prep data, never a pinned genPrep field (TestNoNewFixedStateFields).
	embalmCard     CardID
	embalmAb       uint8
	anyEmbalmGrant bool
	// costObj: some card's cost forages or exiles cards from a graveyard
	// (kop.CostForage / kop.CostExileGrave; act_forage.go): guards their
	// payability and pick paths
	costObj bool
	// subAmong: some ability's cost removes counters from among permanents
	// (kop.CostSubAmong; kw_training.go): guards its pick
	subAmong bool
	// multiMana: some card has a second own mana ability (prepMultiMana
	// reads them once the card's loop is done)
	multiMana bool
	// manaEach: some card's mana ability adds several colours at once
	// (pcard.manaXv): its extra rides the tap-extra paths (genPrep.anyTapExtra)
	manaEach bool
	// manaOne: some card's printed mana ability makes several mana of one
	// chosen colour (pcard.manaOne): availFor's one-colour census can stall
	manaOne bool
	// anyTapPowerStatic: some card has a StatTapPower static
	// (value_tappower.go): guards the tap-power static walk without forcing
	// the generated extension on a table that has none
	anyTapPowerStatic bool
	// anyUncounter: some card carries ReplSpellsCantBeCountered; OpCounter
	// scans the battlefield for it (repl_uncounterable.go)
	anyUncounter bool
	// uncAnyCtrl: some ReplSpellsCantBeCountered names no controller scope
	// (ReplUncAnyCtrl): the scan widens to every battlefield permanent, each
	// line scoping itself (repl_uncounterable.go)
	uncAnyCtrl bool
	// sacMana: some card's mana ability sacrifices its source (a
	// Treasure; pcard.manaSac): guards the sacrifice-cost / mana
	// disjointness check on the activation walk (cost_disjointmana.go)
	sacMana bool
	// sacOnlyMana: some card's mana ability costs only sacrificing its
	// source (an Eldrazi Spawn token; pcard.manaSacOnly): guards the
	// last-resort planner and mana-window paths (mana_sacsource.go)
	sacOnlyMana bool
	// restrSrc: some card has a restricted mana source the planner counts
	// per payment (genPrep.anyRestrSrc; mana_restrictedsource.go): the one
	// guard of the offer gates and the payment
	restrSrc bool
	// artMana: some typed producer (an artifact, a Desert or a Cave) is on
	// the board: gorge tags the mana it adds by its source's face types,
	// printed ability or granted one alike (Ring of the Lucii, a Treasure,
	// a granted Waterskin under Great Divide Guide's grant; ManaProducerTag),
	// and a pool holding tagged mana is spent plain-first (mana_artifact.go)
	artMana bool
	// selfCast: some card has a "when you cast this spell" trigger
	// (TrigSpellCast + TfSelf; trig_selfcast.go): guards the stack
	// fire on the cast path
	selfCast bool
	// eachPay: some generated op is an any-player unless ask (OpIfPaid /
	// OpUnlessPaid / OpUnlessSwitched obj=EachPlayer; cost_anyplayerpays.go)
	eachPay bool
	// embComb: some emblem carries a CantBlockBy / MustAttack static
	// (act_support.go): guards the combat static checks that skip on an
	// empty statM
	embComb bool
	// embIdem: OpCreateEmblem reads idemEmblemHeld (embComb, or an emblem's
	// static prevention: repl_staticprevent_emblem.go)
	embIdem bool
	// anyPwVsSrc: some CantBlockBy static compares the blocker's power with
	// its host's (kop.Static.PwVsSrc; static_powercompare.go): the one guard
	// blockForbiddenGen adds
	anyPwVsSrc bool
	// speed: some card carries Start your engines! or changes speed
	// (OpChangeSpeed); engines: some card carries Start your engines!
	// (kw_speed.go): guard the life-loss and entry hooks
	speed, engines bool
	// ascend: some card prints Ascend: the city's blessing scans run
	// (kw_ascend.go); a prep bool, not a genPrep one, so the hot hooks
	// (enterNF, ctrlMove, sba, resolveTop) cost one byte load
	ascend bool
	// unleash: some card prints Unleash: the as-enters counter ask queues at
	// entry and the block gate reads it (kw_unleash.go); a prep bool like
	// ascend, for the same reason
	unleash bool
	// engAsc: engines || ascend, enterNF's single entry-hook guard
	engAsc bool
	// painMana: some card has a painful mana alternative (pcard.painCol;
	// mana_pain.go): guards the planner's last-resort pain
	// units and the window's pain riders
	painMana bool
	// phyMana: some card's printed cost carries a Phyrexian pip
	// (kop.ManaCost.Phyrexian; mana_phyrexian.go); availExt:
	// painMana || phyMana, availFor's cold extension
	phyMana, availExt bool
	// ctrMana: some card's mana ability costs only a counter put on itself,
	// once each turn (pcard.ctrMana; mana_countercost.go): guards the
	// prefix-search gap, the -0/-1 toughness read and its SBA
	ctrMana bool
	// anyForageCA: a Combo Any source pays a forage ({T}, Forage; mana_forage.go)
	anyForageCA bool
	// caMixForage: the table's Combo Any census gaps are all forage ones and the
	// mix stall (genPrep.caMix) stands (mana_forage.go forageMixIdle)
	caMixForage bool
	// tapMana: some card's mana ability taps another permanent (kop.TapXManaAb)
	// or sacrifices one (kop.SacXEachManaAb; mana_tapanother.go): a
	// prefix-search gap kind
	tapMana bool
	// tapXAb: per card, the index of its tap-another-permanent mana ability
	// (the first mana ability, or a later one beside a plain first,
	// kop.SecondTapXManaAb), -1 none: cold, read through tapXAbOf
	tapXAb []int8
	// ctrAb: per card, the index of its counter-cost mana ability (the first
	// mana ability, kop.CtrManaAb or the dead gap kop.CtrDeadManaAb, or a
	// later {T} one beside a plain {T} first, kop.SecondCtrTapManaAb), -1
	// none: cold, read through ctrAbOf (mana_countercost.go)
	ctrAb []int8
	// riderAb: per card, the index of its bounded mana-ability rider (the
	// first kop.ManaRiderAb ability), -1 none. Cold; read through riderAbOf
	// (mana_rider.go).
	riderAb []int8

	coldCmd coldLayout // the table's cold layout plus the partner record, a Commander game's (zone_cmdpartner.go)
}

func prepare(t *kop.Table) *prep {
	if len(t.Filters) == 0 {
		t.Filters = []kop.Filter{{}}
	}
	p := &prep{t: t, nsub: uint8(len(t.Subtypes)), cards: make([]pcard, len(t.Cards)), selfCast: prepSelfCast(t), eachPay: prepEachPay(t), tapXAb: make([]int8, len(t.Cards)), ctrAb: make([]int8, len(t.Cards)), riderAb: make([]int8, len(t.Cards))}
	p.w8 = tableWord8(t) // filter_props.go
	p.nameRep = nameRepOf(t)
	typedSub := typedManaSubtypes(t)
	p.speed, p.engines = prepSpeed(t)
	p.ascend = tableHasAscend(t)
	p.anyFbCard = tableHasFlashback(t)
	p.unleash = tableHasUnleash(t)
	p.engAsc = p.engines || p.ascend
	for i := range t.Cards {
		c := &t.Cards[i]
		pc := &p.cards[i]
		pc.types = c.Types
		pc.kw = c.Keywords
		pc.cost = c.Cost
		pc.sagaN = c.SagaN
		p.anySaga = p.anySaga || c.SagaN != 0
		p.anyInit = p.anyInit || len(c.DungeonNext) != 0
		p.anyPwVsSrc = p.anyPwVsSrc || cardPwVsSrc(c) // static_powercompare.go
		pc.total = uint8(c.Cost.MV())
		// CR 712.8e: gorge state/mana_value.go gives a transforming DFC
		// back face its front face's mana value. Table data, no read-site
		// changes. A token back face (Incubator) keeps its printed value;
		// the front face is always Face 0.
		if c.FaceMode == kop.FaceTransform && c.Face != 0 && c.Types&kop.TToken == 0 && len(c.Faces) > 0 && int(c.Faces[0]) < len(t.Cards) {
			pc.total = uint8(t.Cards[c.Faces[0]].Cost.MV())
		}
		for k, n := range c.Cost.Pips {
			if n > 0 {
				pc.pipMask |= 1 << k
			}
		}
		pc.power, pc.tough = c.Power, c.Toughness
		pc.timing = c.Timing
		p.gift = p.gift || c.Gift
		p.casualty = p.casualty || c.Casualty != 0
		p.bargain = p.bargain || c.Bargain
		p.mayhem = p.mayhem || c.Mayhem
		p.escape = p.escape || c.Escape
		p.retrace = p.retrace || c.Retrace
		p.teamwork = p.teamwork || c.Teamwork != 0
		p.entwine = p.entwine || c.Entwine
		p.offspring = p.offspring || c.Offspring || c.Buyback || c.Entwine
		p.costObj = p.costObj || (c.SpellCost.Flags|c.AltAddCost.Flags|c.OptCost.Flags|c.KickerCost.Flags|c.FbCost.Flags|c.EscCost.Flags|c.EscBfCost.Flags|c.Repeated.Flags)&costObjFlags != 0 || c.BeholdN != 0 || c.HasSneak || c.HasWebSling // a behold part's ask (act_behold.go); a sneak or web-slinging cast's return pick (kw_sneak.go, kw_websling.go); a Squad/Replicate payment's exile part (kw_squadreplicate.go)
		pc.spellAb, pc.manaAb = -1, -1
		p.tapXAb[i], p.ctrAb[i], p.riderAb[i] = -1, -1, -1
		// the card's mana is typed by its types (mana_artifact.go) whether
		// the producing ability is printed or granted: the tag rides the
		// source, not the ability (effects.ManaProducerTag)
		typedWord := (c.Subtypes[0]&typedSub[0] | c.Subtypes[1]&typedSub[1]) != 0
		pc.manaTyped = c.Types&kop.TArtifact != 0 || typedWord || c.TypedMana
		pc.manaSnow = snowMana(c, typedWord)
		p.artMana = p.artMana || pc.manaTyped
		pc.land = c.Types&kop.TLand != 0
		pc.creature = c.Types&kop.TCreature != 0
		pc.instant = c.Types&kop.TInstant != 0 || c.Keywords&kop.KwFlash != 0
		for j := range c.Abilities {
			a := &c.Abilities[j]
			p.anyGate = p.anyGate || a.Gate.Ctr != 0
			p.anyStateCtr = p.anyStateCtr || a.Kind == kop.AbTriggered && a.Trig.Flags4&kop.Tf4StateCtr != 0
			p.costObj = p.costObj || a.Cost.Flags&costObjFlags != 0 || a.Cost.PutLibSelf
			p.subAmong = p.subAmong || a.Cost.Flags&kop.CostSubAmong != 0
			p.bargainRed = p.bargainRed || a.Kind == kop.AbStatic && a.Static.StFlags&kop.StBargained != 0
			switch a.Kind {
			case kop.AbSpell:
				if pc.spellAb < 0 {
					pc.spellAb = int8(j)
					for ti := 0; ti < int(a.NTargets); ti++ {
						if !a.Targets[ti].UpTo {
							pc.needsTgt = true
						}
					}
				}
			case kop.AbMana:
				if a.ManaRider != kop.ManaRiderNone && a.AFlags&kop.AfLatent == 0 && p.riderAb[i] < 0 {
					p.riderAb[i] = int8(j)
				}
				if pc.manaAb < 0 {
					pc.manaAb = int8(j)
					pc.manaCol = a.ManaColors
					pc.manaN = a.ManaN
					if pc.manaN == 0 {
						pc.manaN = 1
					}
					pc.manaOne = manaOneColour(a, pc.manaN)
					p.manaOne = p.manaOne || pc.manaOne
					if a.ManaEach && !kop.CtrEachManaAb(a) && !kop.FilterEachManaAb(a) && !kop.SacXEachManaAb(a) { // (a counter-cost one keeps its whole mask: mana_countercost.go; a filter's is mana_filtereach.go's; a sacrifice-another one's mana_tapanother.go's)
						prepEachMana(a, pc)
						p.manaEach = true
					}
					pc.manaTap = (a.Cost.Flags == kop.CostTap || a.Cost.Flags == kop.CostTap|kop.CostOncePerGame) && a.Cost.Mana.MV() == 0 && a.Cost.Mana.X == 0
					pc.manaSac = a.Cost.Flags == kop.CostTap|kop.CostSacSelf && a.Cost.Mana.MV() == 0 && a.Cost.Mana.X == 0
					// (a latent one is another permanent's: Lotus Ring's sacrifice-only ability is its equipped creature's, mana_grantcost.go)
					pc.manaSacOnly = !DebugIgnoreSpawnMana && a.AFlags&kop.AfLatent == 0 && a.Cost.Flags == kop.CostSacSelf && a.Cost.Mana.MV() == 0 && a.Cost.Mana.X == 0 && !a.Cost.DiscardHand // (Lion's Eye Diamond is no planner unit: mana_discardhand.go)
					p.sacMana = p.sacMana || pc.manaSac || pc.manaSacOnly
					p.sacOnlyMana = p.sacOnlyMana || pc.manaSacOnly
					if ctrGapAb(a) {
						pc.ctrMana = true
						p.ctrMana = true
						p.ctrAb[i] = int8(j)
					}
					if kop.TapXSearchMana(a) { // (a sacrifice-another or exile-from-graveyard one is the tap-another search's too, mana_exilegrave.go)
						p.tapMana = true // (read per card through p.tapXAb, not a pcard field: the hot record stays as it was)
						p.tapXAb[i] = int8(j)
					}
					if d, ok := manaPain(a); ok {
						pc.painCol, pc.painN, pc.painOnly = a.ManaColors, d, true
						p.painMana = true
					}
				} else if a.AFlags&kop.AfLatent == 0 {
					p.multiMana = true
					if pk := int(pc.manaAb); pk >= 0 && kop.ChosenSecondManaAb(c, pk, j) {
						// a later bare {T}: Add one mana of the chosen colour
						// beside a fixed {T} first one (Quirion Elves): the
						// source's unit reads the chosen colour too, and stays
						// in the fixed-production census pass
						pc.manaChosen, pc.manaFixedChosen = true, true
					}
					if p.tapXAb[i] < 0 && kop.SecondTapAnotherManaAb(c, int(pc.manaAb), j) {
						p.tapMana = true // a second tap-another ability: the card's tapX gap (mana_tapanother.go)
						p.tapXAb[i] = int8(j)
					}
					if p.ctrAb[i] < 0 && (kop.SecondCtrTapManaAb(c, int(pc.manaAb), j) || kop.StoreManaAb(a)) {
						p.ctrMana = true // a second counter-cost ability: the card's counter gap (mana_countercost.go)
						p.ctrAb[i] = int8(j)
					}
				}
			case kop.AbTriggered:
				pc.trigMask |= 1 << a.Trig.Event & (1<<nSubEvents - 1) // TrigGiveGift: no subscription (kw_gift.go)
				if a.Trig.Event == kop.TrigCounterAddedAll {
					pc.trigMask |= 1 << kop.TrigCounterAddedOnce // the counter sites' gate; fireObj's hook runs the watcher (trig_counteraddedall.go)
				}
			case kop.AbStatic:
				pc.hasStat = battlefieldStatic(pc.hasStat, a) // a graveyard static is none (static_gycont.go)
				if a.Static.Kind == kop.StatOverride || (a.Static.Kind == kop.StatContinuous && a.Static.Grant != 0) {
					pc.grantStat = true // a source grantedAbilitiesN's static walk can reach
				}
				if a.Static.Kind == kop.StatCDAPT {
					p.cda = true
				}
			case kop.AbActivated:
				pc.nAct++
			case kop.AbReplacement:
				switch a.Repl.Kind {
				case kop.ReplEntersTapped:
					if a.Repl.C != 0 {
						// no FDN pool card has a conditional enters-tapped
						// (census 2026-10-17: the gain lands and Diregraf
						// Ghoul are unconditional); refuse loudly
						panic("kernel: conditional enters-tapped replacement not implemented (card " + c.Name + ")")
					}
					pc.enterTap = true
				case kop.ReplEntersWithCounters:
					pc.enterCtr = int8(a.Repl.A)
					pc.enterCK = kop.CounterKind(a.Repl.C)
				case kop.ReplCantBeCountered:
					if a.Repl.Cond != 0 {
						pc.noCtrCond = a.Repl.Cond
					} else {
						pc.noCtr = true
					}
				case kop.ReplSpellsCantBeCountered:
					pc.hasStat = true // scanned through statM (repl_uncounterable.go)
					p.anyUncounter = true
				case kop.ReplDoesntUntap:
					pc.noUntap = noUntapKind(pc.noUntap, a.Repl.Cond) // repl_untapself.go
				case kop.ReplMayNotUntap:
					pc.mayNotUntap = true
				}
			}
		}
	}
	p.uncAnyCtrl = tableHasUncAnyCtrl(t) // repl_uncounterable.go
	if p.multiMana {
		for i := range t.Cards {
			if p.cards[i].manaAb >= 0 {
				prepMultiMana(&t.Cards[i], &p.cards[i])
				if !p.cards[i].manaMulti && prepPainMana(&t.Cards[i], &p.cards[i]) {
					p.painMana = true
				}
			}
		}
	}
	for i := range t.Cards {
		if prepPainSource(&t.Cards[i], &p.cards[i]) {
			p.painMana = true
		}
		p.phyMana = p.phyMana || t.Cards[i].Cost.Phyrexian()
		for j := range t.Cards[i].Abilities {
			p.phyMana = p.phyMana || t.Cards[i].Abilities[j].Cost.Mana.Phyrexian() // an activation's {W/P}
		}
	}
	// the TapPowerValue static walk reads this even for a table with no
	// generated extension: forcing prepareGen here would switch every static
	// and cost path to the generated forms (value_tappower.go)
	p.anyTapPowerStatic = tableHasStatic(t, kop.StatTapPower)
	p.embPrev = tableEmbHasStaticPrev(t)
	p.embExileLeave = tableEmbHasExileLeave(t)
	p.gen = prepareGen(t, p)
	prepLayerSrc(t, p) // static_layerwalk.go: the walk's per-card ability masks
	prepHandStat(t, p) // cast_handleaves.go
	p.gyCont = tableHasGyCont(t)
	p.anyNoDmg = tableHasStatic(t, kop.StatNoCombatDamage)
	p.anyAsUnblk = tableHasStatic(t, kop.StatAssignCombatDamageAsUnblocked)
	p.painMana = p.painMana || p.gen != nil && p.gen.anyTapsPain  // a Pain-class Taps watcher prices its tapped source (mana_tapsdefer.go)
	p.costObj = p.costObj || p.gen != nil && p.gen.anyEscapeGrant // an escape cast's exile-fodder picks (kw_escape.go)
	p.availExt = p.painMana || p.phyMana
	p.restrSrc = p.gen != nil && p.gen.anyRestrSrc
	p.embComb = prepEmbComb(t, p)
	p.embIdem = prepEmbIdem(p) // static_emblemcombat.go
	p.cold.size()
	// Only a Commander game can seat a partner pair, so only its layout
	// carries the record: a non-Commander State's cold buffer stays empty,
	// as InitN's equality test compares every State byte, slice header
	// included. Both layouts are table-level, so Init and InitN of the same
	// table with the same Config select the same pointer.
	p.coldCmd = p.cold
	coldPartner.set(&p.coldCmd, 1)
	p.coldCmd.size()
	return p
}
