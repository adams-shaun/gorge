package kgen

import (
	"fmt"
	"math/bits"
	"slices"
	"sort"
	"strings"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/effects"
)

// The build step: LIR -> kop values plus the pool's side tables. Every LIR
// shape kop cannot carry refuses here (an emission refusal), so the
// generated table only ever holds what both lowering and kop can express.

// poolTables are the pool-wide side tables generated programs index.
type poolTables struct {
	filters   []kop.Filter
	filterKey map[string]kop.FilterID
	exprs     []kop.Expr
	exprKey   map[string]int16
	conds     []kop.Cond
	condKey   map[string]kop.CondID
	overrides []kop.Override
	ovSubs    []ovSub
	emblems   []kop.Emblem
	subtypes  []string // pool subtype names (index = pool subtype id)
	subIdx    map[string]int
	tokenIdx  map[string]int   // token script key -> pool index
	cardIdx   map[string]int   // pool card name -> pool index (multi-face links)
	named     map[string][]int // printed name -> pool indexes (cards, tokens, faces; a named-other clause)
	missing   map[string]bool  // subtypes a build looked up that the table lacks
	paradigm  map[string]int   // Paradigm card name -> the table's Paradigm-name index
}

// ovSub carries an override's subtype sets as pool subtype indexes (the
// matchup builder turns them into matchup masks).
type ovSub struct {
	set, add, keep []uint8
}

// built is one card's kop form.
type built struct {
	def      kop.CardDef
	subs     []uint8        // pool subtype indexes of the printed type line
	stAddSub [][]uint8      // per ability: the static's granted subtypes (pool indexes)
	tokens   []int          // pool indexes of the tokens it creates
	refSubs  []uint8        // pool subtypes the card's filters name
	ovVals   []kop.ValueRef // the Power$ and Toughness$ values of the overrides it applies
	origins  []uint8        // per kop ability: gorge Face.Abilities index, 0x80|Face.Triggers index, or 0xFF
	refusals []refusal
	// readsName: one of the card's filters reads a name (F2SameNameAsSource,
	// PredNamed); a matchup that also holds a Room refuses (roomNameGuard).
	readsName bool
}

// newPoolTables starts the side tables; the subtype table holds the pool
// cards' and tokens' printed subtypes plus extra (the subtypes an earlier
// build pass found effects and filters naming).
func newPoolTables(lw *lowered, extra map[string]bool) *poolTables {
	pt := &poolTables{filterKey: map[string]kop.FilterID{}, exprKey: map[string]int16{}, condKey: map[string]kop.CondID{}, subIdx: map[string]int{}, tokenIdx: map[string]int{}, missing: map[string]bool{}}
	pt.filters = append(pt.filters, kop.Filter{}) // FilterID 0: the zero filter
	pt.exprs = append(pt.exprs, kop.Expr{})       // Exprs[0] unused
	pt.conds = append(pt.conds, kop.Cond{})       // CondID 0: always
	set := map[string]bool{}
	for w := range extra {
		set[w] = true
	}
	for _, c := range append(append([]*lCard{}, lw.cards...), lw.tokens...) {
		for _, s := range c.Subtypes {
			set[s] = true
		}
	}
	for w := range set {
		pt.subtypes = append(pt.subtypes, w)
	}
	sort.Strings(pt.subtypes)
	for i, w := range pt.subtypes {
		pt.subIdx[w] = i
	}
	for i, t := range lw.tokens {
		pt.tokenIdx[t.Name] = len(lw.cards) + i
	}
	pt.cardIdx = map[string]int{}
	pt.named = map[string][]int{}
	for i, c := range lw.cards {
		pt.cardIdx[c.Name] = i
		pt.named[c.Name] = append(pt.named[c.Name], i)
	}
	pt.paradigm = map[string]int{}
	for _, c := range lw.cards {
		// the table's distinct Paradigm names in pool order: CardDef.Paradigm
		// is 1 + the index, the kernel's genState.parad bit
		if _, ok := pt.paradigm[c.Name]; c.Paradigm && len(c.Refusals) == 0 && !ok {
			pt.paradigm[c.Name] = len(pt.paradigm)
		}
	}
	for i, t := range lw.tokens {
		if t.printed != "" {
			pt.named[t.printed] = append(pt.named[t.printed], len(lw.cards)+i)
		}
	}
	return pt
}

// sub returns a subtype's pool index, recording a miss for the next pass.
func (pt *poolTables) sub(w string) (int, bool) {
	i, ok := pt.subIdx[w]
	if !ok {
		pt.missing[w] = true
		return 0, true
	}
	return i, true
}

type buildErr struct{ node, reason string }

func (e buildErr) Error() string { return e.reason }

func bfail(node, format string, a ...any) error {
	return buildErr{node, fmt.Sprintf(format, a...)}
}

// cardBuilder builds one card.
type cardBuilder struct {
	pt          *poolTables
	c           *lCard
	out         built
	abMap       map[int]int // LIR ability index -> kop ability index
	node        string
	kicked      bool // the card has kicker (TgfKickedRange)
	bargain     bool // the card has bargain (the same kicked provenance)
	gift        bool // the card has gift (TgfGiftRange)
	inDamageMap bool
	spellCost   kop.Cost // an instant's/sorcery's additional cost (its AbSpell's Cost) // inside a damage map (DealDamage nodes until ResolveDamageMap)
	ctrGate     bool     // building the operand of a >= 1 condition: vCtrAdded may lower (value_countersadded.go)
	markPerm    bool     // ThisTurnEntered's dest=Battlefield card form: mark Permanent-base clauses
	keepOwner   bool     // a filter over this turn's departures: "you own" stays Owner, apart from "you control"
	selfCast    bool     // building a SpellCast self trigger's stack filter: sneaked is readable
}

func buildCard(pt *poolTables, c *lCard) built {
	cb := &cardBuilder{pt: pt, c: c, abMap: map[int]int{}}
	if err := cb.record(); err != nil {
		cb.out.refusals = append(cb.out.refusals, refusal{Card: c.Name, Node: "record", Reason: err.Error()})
	}
	if len(c.Refusals) > 0 {
		// A refused card keeps its static record only.
		cb.out.def.Abilities = nil
		return cb.out
	}
	if err := cb.abilities(); err != nil {
		node := cb.node
		if be, ok := err.(buildErr); ok && be.node != "" {
			node = be.node
		}
		cb.out.refusals = append(cb.out.refusals, refusal{Card: c.Name, Node: "emit " + node, Reason: err.Error()})
		cb.out.def.Abilities = nil
	} else if c.Rebound != nil {
		// CR 702.88a: the latent recast body, run by the upkeep delayed
		// program the kernel registers as the hand-cast spell resolves
		// (dirac/kw_rebound.go); the ability map is filled by abilities()
		if j, ok := cb.abMap[c.Rebound.Index]; ok && j+1 <= 255 {
			cb.out.def.ReboundAb = uint8(j + 1)
		} else {
			cb.out.refusals = append(cb.out.refusals, refusal{Card: c.Name, Node: "emit K:Rebound", Reason: "the recast body was not emitted"})
			cb.out.def.Abilities = nil
		}
	}
	if len(c.Refusals) == 0 && c.Overload != nil && c.OverloadProg != nil {
		p, err := cb.prog(c.OverloadProg)
		if err != nil {
			cb.out.refusals = append(cb.out.refusals, refusal{Card: c.Name, Node: "emit K:Overload", Reason: err.Error()})
			cb.out.def.Abilities = nil
		} else {
			cb.out.def.OverloadProg = p
		}
	}
	if c.Impending == nil && putsTimeCounters(&cb.out.def) {
		// the kernel's impending dormancy reads the time counters, not
		// gorge's cast provenance (FlagImpending): no other card may put
		// them (kernel kw_impending.go)
		cb.out.refusals = append(cb.out.refusals, refusal{Card: c.Name, Node: "emit", Reason: "time counters outside Impending"})
		cb.out.def.Abilities = nil
	}
	for j := range cb.out.def.Abilities {
		// the kernel's mana walk silently dropped a
		// mana ability whose cost it does not read; refuse it instead
		if a := &cb.out.def.Abilities[j]; !kop.ManaAbilityHonoured(&cb.out.def, j) {
			cb.out.refusals = append(cb.out.refusals, refusal{Card: c.Name, Node: "emit mana ability",
				Reason: "mana ability cost " + kop.ManaCostShape(a) + ": the kernel's mana walk cannot activate it"})
			cb.out.def.Abilities = nil
			break
		}
	}
	if c.FaceMode == kop.FaceRoom {
		// the leading door's abilities as kop indexes: a folded keyword
		// ability (Equip, Ward, ...) is not in the kop list
		n := 0
		for li, ki := range cb.abMap {
			if li < c.RoomSplit && ki >= n {
				n = ki + 1
			}
		}
		cb.out.def.RoomSplit = uint8(n)
	}
	if kop.CardTwoSeat(&cb.out.def) {
		// an operand naming the one opponent marks the card as its lowering
		// would (lc.twoSeat), so the support count sees it too
		if !c.TwoSeat {
			c.TwoSeatWhy = append(c.TwoSeatWhy, twoSeatOperand)
		}
		cb.out.def.TwoSeat, c.TwoSeat = true, true
	}
	return cb.out
}

func manaCostOf(m manaCost) (kop.ManaCost, error) {
	var out kop.ManaCost
	if m.None {
		return out, nil
	}
	if len(m.Hybrid) > len(out.Hyb) {
		return out, fmt.Errorf("hybrid mana %s: more than %d pips (gorge planHybridLimit)", m, len(out.Hyb))
	}
	for i, h := range m.Hybrid {
		// The kernel offers a pip's lower colour first unless the lowering
		// records the printed order (kop.ManaCost.HybOrd): gorge's
		// pay.AnnouncePip offers Hybrid[i].A first, so a pair printed out of
		// WUBRG order ({G/W}) records the reversal rather than being refused.
		a, b := strings.IndexByte("WUBRG", h[0]), strings.IndexByte("WUBRG", h[1])
		if a < 0 || b < 0 || a == b {
			return out, fmt.Errorf("hybrid mana %s", m)
		}
		out.Hyb[i] = kop.ColorMask(1<<a | 1<<b)
		if a > b {
			out.HybOrd |= uint8(1) << i
		}
	}
	if len(m.Twobrid) != 0 || len(m.Phyrexian) != 0 {
		if err := encodeTwobrid(m, &out); err != nil {
			return out, err
		}
	}
	out.Generic = uint8(m.Generic)
	for i := 0; i < 5; i++ {
		out.Pips[i] = uint8(m.Pips[i])
	}
	out.Pips[kop.C] = uint8(m.C)
	if m.X {
		out.X = 1
	}
	if m.XX {
		out.X = 2
	}
	if m.XXX {
		out.X = 3
	}
	if m.XColor != 0 {
		i := strings.IndexByte("WUBRG", m.XColor)
		if i < 0 || out.X == 0 {
			return out, fmt.Errorf("XColor mana %s", m)
		}
		out.XPip = uint8(i + 1)
	}
	return out, nil
}

// manaColOrd packs the colour letters of s (W U B R G only) into
// kop.Ability.ManaColOrd: three bits per printed position, the WUBRG index.
func manaColOrd(s string) uint16 {
	var o uint16
	for i, r := range s {
		o |= uint16(strings.IndexRune("WUBRG", r)) << (3 * uint(i))
	}
	return o
}

func colorMask(s string) kop.ColorMask {
	var m kop.ColorMask
	for _, r := range s {
		switch r {
		case 'W':
			m |= kop.ColW
		case 'U':
			m |= kop.ColU
		case 'B':
			m |= kop.ColB
		case 'R':
			m |= kop.ColR
		case 'G':
			m |= kop.ColG
		case 'C':
			m |= kop.ColC
		}
	}
	return m
}

// typeMask maps the LIR type bits (whose low byte matches kop's) and the
// supertypes.
// subtypeTypeBits is the kernel type bits a subtype list carries: Aura and
// Equipment are TypeMask bits as well as subtypes.
func subtypeTypeBits(subs []string) kop.TypeMask {
	var m kop.TypeMask
	for _, s := range subs {
		switch s {
		case "Aura":
			m |= kop.TAura
		case "Equipment":
			m |= kop.TEquipment
		}
	}
	return m
}

func typeMask(t uint16) kop.TypeMask {
	m := kop.TypeMask(t & 0xff)
	if t&tLegendary != 0 {
		m |= kop.TLegendary
	}
	if t&tBasic != 0 {
		m |= kop.TBasic
	}
	if t&tKindred != 0 {
		m |= kop.TKindred
	}
	return m
}

var kwBits = map[string]kop.KwMask{
	"Flying": kop.KwFlying, "Reach": kop.KwReach, "First Strike": kop.KwFirstStrike, "Double Strike": kop.KwDoubleStrike,
	"Deathtouch": kop.KwDeathtouch, "Lifelink": kop.KwLifelink, "Trample": kop.KwTrample, "Vigilance": kop.KwVigilance,
	"Haste": kop.KwHaste, "Menace": kop.KwMenace, "Defender": kop.KwDefender, "Hexproof": kop.KwHexproof,
	"Indestructible": kop.KwIndestructible, "Protection from everything": kop.KwProtectionAll,
	"Unblockable": kop.KwUnblockable, "CantBlock": kop.KwCantBlock, "HexproofFromInstants": kop.KwHexproofFromInstants,
	"ProtectionFromBlack": kop.KwProtBlack, "ProtectionFromRed": kop.KwProtRed,
	"PreventCombatDamageTo": kop.KwPreventCombatDamageTo, "FlashbackOwnCost": kop.KwFlashbackOwnCost,
	"PreventCombatDamageFrom": kop.KwPreventCombatDamageFrom, "Shroud": kop.KwShroud,
	"Fear": kop.KwFear, "Shadow": kop.KwShadow, "Prowess": kop.KwProwess, "Wither": kop.KwWither, // a GRANTED wither only (printed: CardDef.Wither)
	"Ward": kop.KwWard, // a static's granted ward only (lStatic.Ward; splitWardGrant)
}

func kwMask(ks []string) (kop.KwMask, error) {
	var m kop.KwMask
	for _, k := range ks {
		b, ok := kwBits[k]
		if !ok {
			return 0, fmt.Errorf("keyword %q has no kop bit", k)
		}
		m |= b
	}
	return m, nil
}

var counterKinds = map[string]kop.CounterKind{
	"P1P1": kop.CtrP1P1, "M1M1": kop.CtrM1M1, "LOYALTY": kop.CtrLoyalty, "Stun": kop.CtrStun, "STUN": kop.CtrStun,
	"INCUBATION": kop.CtrIncubation, "BAIT": kop.CtrBait, "SOUL": kop.CtrSoul, "FELLOWSHIP": kop.CtrFellowship,
	// classLevelCounter: a Class's level (lower_delayed.go classBandCond), never a Forge counter name
	"REVIVAL": kop.CtrRevival, "STASH": kop.CtrStash, classLevelCounter: kop.CtrLevel,
	"CHARGE": kop.CtrCharge, "OIL": kop.CtrOil, "SPORE": kop.CtrSpore, "QUEST": kop.CtrQuest,
	"VERSE": kop.CtrVerse, "GROWTH": kop.CtrGrowth, "FUSE": kop.CtrFuse, "STORY": kop.CtrStory,
	"Flying":       kop.CtrFlying, // CR 122.1b keyword counter (the featurized kind)
	"First Strike": kop.CtrFirstStrike, "Double Strike": kop.CtrDoubleStrike, "Deathtouch": kop.CtrDeathtouch,
	"Haste": kop.CtrHaste, "Hexproof": kop.CtrHexproof, "Indestructible": kop.CtrIndestructible,
	"Lifelink": kop.CtrLifelink, "Menace": kop.CtrMenace, "Reach": kop.CtrReach, "Shadow": kop.CtrShadow,
	"Trample": kop.CtrTrample, "Vigilance": kop.CtrVigilance, "SUPPLY": kop.CtrSupply,
	"SHIELD":   kop.CtrShield,   // CR 122.1c: read by the kernel's damage and destroy replacements (ctr_shield.go)
	"LORE":     kop.CtrLore,     // CR 702.151a Saga chapter counter (kw_saga.go)
	"LANDMARK": kop.CtrLandmark, // Treasure Map's named counter (CR 122.1)
	"DREAD":    kop.CtrDread,    // Grasping Shadows named counter (CR 122.1)
	"CHORUS":   kop.CtrChorus,   // Malcolm, Alluring Scoundrel's named counter (CR 122.1)
	"FINALITY": kop.CtrFinality, // CR 122.1: exiled instead of dying (kernel dieZone)
	"Dread":    kop.CtrDread,    // the remove-cost spelling (SubCtrKnd)
	"NET":      kop.CtrNet,      // Braided Net's named counter (CR 122.1)
	"PLAN":     kop.CtrPlan,     // the MSH Plan enchantments' named counter (CR 122.1)
	"VALOR":    kop.CtrValor,    // Intrepid Adversary's named counter (CR 122.1)
	"TIME":     kop.CtrTime,     // Impending's time counters (CR 702.176a; only an Impending card may put them: buildCard)
	// the named counters; the scripts spell them upper case.
	// All but HONE have no inherent rules meaning; HONE is CR 122.1j (+1/+0
	// on the equipped creature), read by the kernel's ptGen.
	"INVASION":   kop.CtrInvasion,   // Alien Invasion's named counter (CR 122.1)
	"LOOT":       kop.CtrLoot,       // Bandit's Haul's named counter (CR 122.1)
	"REV":        kop.CtrRev,        // Chainsaw's named counter (CR 122.1)
	"PAGE":       kop.CtrPage,       // Diary of Dreams' named counter (CR 122.1)
	"INGENUITY":  kop.CtrIngenuity,  // Lady Octopus, Inspired Inventor's named counter (CR 122.1)
	"FILM":       kop.CtrFilm,       // Peter Parker's Camera's named counter (CR 122.1)
	"FIRE":       kop.CtrFire,       // War Balloon's named counter (CR 122.1)
	"HONE":       kop.CtrHone,       // Dwalin, Weaponmaster's hone counter (CR 122.1j; kernel honePower)
	"COIN":       kop.CtrCoin,       // Wishing Well's named counter (CR 122.1)
	"DIVINITY":   kop.CtrDivinity,   // Myojin of Night's Reach's named counter (CR 122.1)
	"IMPOSTOR":   kop.CtrImpostor,   // Illicit Masquerade's named counter (CR 122.1)
	"BORE":       kop.CtrBore,       // Brass's Tunnel-Grinder's named counter (CR 122.1)
	"POINT":      kop.CtrPoint,      // Contested Game Ball's named counter (CR 122.1)
	"BLOODSTAIN": kop.CtrBloodstain, // Blood Spatter Analysis's named counter (CR 122.1)
	"POSSESSION": kop.CtrPossession, // Unwilling Vessel's named counter (CR 122.1)
	"PROMOTION":  kop.CtrPromotion,  // Beckett Mariner's named counter (CR 122.1)
	"PEACE":      kop.CtrPeace,      // Peace in Our Time's named counter (CR 122.1)
	"WISH":       kop.CtrWish,       // Wishclaw Talisman's named counter (CR 122.1)
	"SKEWER":     kop.CtrSkewer,     // Rotisserie Elemental's named counter (CR 122.1)
	"NEST":       kop.CtrNest,       // Twitching Doll's named counter (CR 122.1)
	"AGE":        kop.CtrAge,        // cumulative upkeep's age counter (CR 702.24a; Mystic Remora)
	"FADE":       kop.CtrFade,       // fading's fade counter (CR 702.32a; Blastoderm)
	"LUCK":       kop.CtrLuck,       // Gemstone Caverns' named counter (CR 122.1; the Condition-split mana gate, lower_condsplit.go)
	"DEPLETION":  kop.CtrDepletion,  // Peat Bog's depletion counter (CR 122.1; mana_rider.go)
	"MINING":     kop.CtrMining,     // Gemstone Mine's mining counter (CR 122.1; mana_rider.go)
	"BURDEN":     kop.CtrBurden,     // The One Ring's burden counter (CR 122.1; its count reads in Count$CardCounters.BURDEN)
}

func counterKind(s string) (kop.CounterKind, error) {
	k, ok := counterKinds[s]
	if !ok {
		return 0, fmt.Errorf("counter kind %q", s)
	}
	return k, nil
}

var zoneMap = [...]kop.Zone{zNone: kop.ZoneNone, zBattlefield: kop.ZoneBattlefield, zHand: kop.ZoneHand,
	zGraveyard: kop.ZoneGraveyard, zLibrary: kop.ZoneLibrary, zExile: kop.ZoneExile, zStack: kop.ZoneStack,
	zCommand: kop.ZoneCommand, zAny: kop.ZoneNone}

func (cb *cardBuilder) subIndexes(ws []string) ([]uint8, error) {
	var out []uint8
	for _, w := range ws {
		if w == "AllBasicLandType" || w == "AllNonBasicLandType" {
			// gorge expands the token to every land type (chars appendLandTypes)
			// and a land's type carries its mana ability (CR 305.6): the kernel
			// would add a subtype literally named after the token
			return nil, fmt.Errorf("land-type grant %s (not expanded to the land types)", w)
		}
		i, ok := cb.pt.sub(w)
		if !ok {
			return nil, fmt.Errorf("subtype %q not in the pool table", w)
		}
		out = append(out, uint8(i))
	}
	return out, nil
}

func (cb *cardBuilder) record() error {
	c := cb.c
	d := &cb.out.def
	d.Name = c.Name
	costErr := error(nil)
	var err error
	if d.Cost, err = manaCostOf(c.Cost); err != nil {
		costErr = err
	}
	// {0} and "no cost" both lower to the zero ManaCost; CostEmpty keeps the
	// distinction the turn-face-up gate needs (CR 701.40b): gorge rejects
	// only an empty printed cost, so "no cost" is offered as free.
	d.CostEmpty = c.CostEmpty
	if (len(c.Cost.Twobrid) != 0 || len(c.Cost.Phyrexian) != 0) && costErr == nil {
		costErr = twobridConflict(c)
	}
	defer func() {
		if costErr != nil {
			cb.out.refusals = append(cb.out.refusals, refusal{Card: c.Name, Node: "record", Reason: costErr.Error()})
		}
	}()
	d.Types = typeMask(c.Types)
	if c.Token {
		d.Types |= kop.TToken
	}
	d.Types |= subtypeTypeBits(c.Subtypes)
	if cb.out.subs, err = cb.subIndexes(c.Subtypes); err != nil {
		return err
	}
	if c.Changeling {
		for _, i := range cb.out.subs {
			d.PrintedSubtypes = d.PrintedSubtypes.With(int(i))
		}
		cb.out.subs = changelingSubs(cb.pt, cb.out.subs)
	}
	if d.Keywords, err = kwMask(c.Kw); err != nil {
		return err
	}
	d.PreventDmgTo, d.PreventCombatTo, d.PreventCombatBy = c.PreventDmgTo, c.PrevCmbTo, c.PrevCmbBy
	d.Colors = colorMask(c.Colors)
	d.Power, d.Toughness, d.Loyalty = int8(c.P), int8(c.T), int8(c.Loyalty)
	d.NoPT = c.NoPT
	d.TwoSeat = c.TwoSeat
	d.Defense = int8(c.Defense)
	switch {
	case c.Types&tLand != 0:
		d.Timing = kop.TimingLand
	case c.Flash || c.Types&tInstant != 0:
		d.Timing = kop.TimingInstant
	default:
		d.Timing = kop.TimingSorcery
	}
	if c.Flash {
		d.Keywords |= kop.KwFlash
	}
	cb.gift = c.Gift
	d.Casualty = uint8(c.Casualty)
	if c.Bargain {
		// Bargain rides sfKicked too, and reuses Casualty's cast action: a
		// card with either has no free bit (kw_bargain.go)
		if c.Kicker != nil || c.Offspring != nil || c.Casualty != 0 {
			return fmt.Errorf("bargain with a kicker/offspring/casualty")
		}
		d.Bargain = true
		cb.bargain = true
	}
	if c.Storm {
		// the printed K:Storm trigger is the kernel's own native trigger
		// (dirac/kw_storm.go); no ability, no cast-action bit
		d.Storm = true
	}
	if c.OptCost != nil {
		// an OptionalCost part rides sfKicked and the kicked cast action
		// (dirac/cost_optional.go): no other optional cost may share it
		if c.Kicker != nil || c.Offspring != nil || c.Casualty != 0 || c.Bargain || c.Teamwork != 0 || c.SpellCost != nil || c.AltAddCost != nil {
			return fmt.Errorf("optional cost with another optional cost")
		}
		oc := *c.OptCost
		if oc.BeholdN != 0 {
			d.OptBehold, d.BeholdN = true, uint8(oc.BeholdN)
			if d.BeholdF, err = cb.filter(oc.BeholdF, kop.ZoneBattlefield); err != nil {
				return err
			}
			oc.BeholdN, oc.BeholdF = 0, nil
		}
		if oc.EvidenceN != 0 {
			d.OptEvidence = uint8(min(oc.EvidenceN, 255)) // a table byte (kernel cost_optionalevidence.go)
			oc.EvidenceN = 0
		}
		if oc.Waterbend != 0 {
			// the kicked cast's {N} rides Kicker (no KwKicker, as Offspring's);
			// the taps are the spell waterbend pick (kernel act_waterbend.go)
			if oc.Waterbend > 255 {
				return fmt.Errorf("optional waterbend %d", oc.Waterbend)
			}
			d.OptWaterbend = uint8(oc.Waterbend)
			d.Kicker = kop.ManaCost{Generic: uint8(oc.Waterbend)}
			oc.Waterbend, oc.Mana = 0, manaCost{None: true}
		}
		if d.OptCost, err = cb.cost(oc); err != nil {
			return err
		}
	}
	if c.Teamwork != 0 {
		// Teamwork rides sfKicked and the casualty cast action too (kw_teamwork.go)
		if c.Kicker != nil || c.Offspring != nil || c.Casualty != 0 || c.Bargain {
			return fmt.Errorf("teamwork with a kicker/offspring/casualty/bargain")
		}
		for _, sc := range []*lCost{c.SpellCost, c.AltAddCost} {
			if sc != nil && (sc.TapN != 0 || sc.TapPowN != 0) {
				// the Taps teamwork gate reads every pkTapX pick of the build as a teamwork tap
				return fmt.Errorf("Teamwork beside a tap-creatures cost")
			}
		}
		d.Teamwork = uint8(c.Teamwork)
		d.ModesPickTW = uint8(c.ModesPickTW)
		cb.bargain = true // the kicked-range target slots (Tear Asunder's), read by the same provenance
	}
	d.ModesMinCount = c.ModesMinCount != 0
	if c.ModesPickV.Kind != vNone {
		if d.ModesPickV, err = cb.value(c.ModesPickV); err != nil {
			return err
		}
	}
	d.DungeonNext = c.DungeonNext // lower_initiative.go
	d.TypeValid = c.TypeValid     // lower_choosetype.go
	if c.Offspring != nil {
		// Offspring rides the kicked cast (sfKicked, CardDef.Kicker) with
		// CardDef.Offspring naming it; a card with both has no free bit
		if c.Kicker != nil {
			return fmt.Errorf("offspring with a kicker")
		}
		d.Offspring = true
		if d.Kicker, err = manaCostOf(*c.Offspring); err != nil {
			return err
		}
	}
	if c.Buyback != nil {
		// Buyback rides the kicked cast (sfKicked, CardDef.Kicker) with
		// CardDef.Buyback naming it (kw_buyback.go); refuseBuybackCombos
		// keeps every other optional cost off the card
		if c.Kicker != nil || c.Offspring != nil {
			return fmt.Errorf("buyback with a kicker/offspring")
		}
		d.Buyback = true
		if d.Kicker, err = manaCostOf(*c.Buyback); err != nil {
			return err
		}
		if c.KickerCost != nil {
			if d.KickerCost, err = cb.cost(*c.KickerCost); err != nil {
				return err
			}
		}
	}
	if c.Spree {
		d.HasModeCost = true
		for i := range c.ModeCost {
			if d.ModeCost[i], err = manaCostOf(c.ModeCost[i]); err != nil {
				return err
			}
		}
	}
	if c.Kicker != nil {
		d.Keywords |= kop.KwKicker
		cb.kicked = true
		if d.Kicker, err = manaCostOf(*c.Kicker); err != nil {
			return err
		}
		if c.KickerCost != nil {
			if c.SpellCost != nil {
				return fmt.Errorf("non-mana kicker beside an additional cost") // one pick cost per cast (kernel buildPickCost)
			}
			if d.KickerCost, err = cb.cost(*c.KickerCost); err != nil {
				return err
			}
		}
	}
	if c.MorphCost != nil {
		d.MorphFamily = c.MorphFamily
		if d.MorphCost, err = manaCostOf(*c.MorphCost); err != nil {
			return err
		}
		if d.MorphReduce, err = cb.value(c.MorphReduce); err != nil {
			return err
		}
	}
	if c.Flashback != nil {
		d.Keywords |= kop.KwFlashback
		if d.AltCost, err = manaCostOf(*c.Flashback); err != nil {
			return err
		}
	}
	d.RemPersist, d.RemSeed, d.RemKeep = c.RemPersist, c.RemSeed, c.RemKeep
	d.ExiledWith = c.ExiledWith
	d.CombatHitRec = c.CombatHitRec
	d.ProduceCol = c.ProduceCol
	d.ExiledWithRev = c.ExwWord
	d.SpendRead = c.SpendRead
	if c.FbCost != nil {
		if c.SpellCost != nil {
			// one pick cost per cast: the spell's own additional cost
			// beside a non-mana flashback cost is not composed
			return fmt.Errorf("non-mana flashback cost beside a spell additional cost")
		}
		if d.FbCost, err = cb.cost(*c.FbCost); err != nil {
			return err
		}
	}
	if c.Madness != nil {
		d.HasMadness = true // CR 702.35a (dirac/kw_madness.go)
		if d.MadnessCost, err = manaCostOf(*c.Madness); err != nil {
			return err
		}
	}
	if c.Miracle != nil {
		d.HasMiracle = true // CR 702.94a (dirac/kw_miracle.go)
		if d.MiracleCost, err = manaCostOf(*c.Miracle); err != nil {
			return err
		}
	}
	d.NoAbilities = c.NoAbilities
	d.HasManaAbility = c.HasManaAbility
	if c.Warp != nil {
		// CR 702.185a: no KwMask bit is free, so the warp marker is its own
		// field; the cost is the hand's alternate cost (dirac/kw_warp.go)
		d.HasWarp = true
		if d.WarpCost, err = manaCostOf(*c.Warp); err != nil {
			return err
		}
	}
	if c.Impending != nil {
		// CR 702.176a (kernel kw_impending.go)
		d.Impending = uint8(c.ImpendingN)
		if d.ImpendingCost, err = manaCostOf(*c.Impending); err != nil {
			return err
		}
	}
	if c.Paradigm {
		// the name's bit in the kernel's 8-bit genState.parad
		i := cb.pt.paradigm[c.Name]
		if i >= 8 {
			return fmt.Errorf("more than 8 Paradigm names in one table")
		}
		d.Paradigm = uint8(i + 1)
	}
	if c.Plot != nil {
		// CR 702.170a: no KwMask bit is free, so the plot marker is its own
		// field; the cost is the special action's price
		// (dirac/kw_plot.go)
		d.HasPlot = true
		if d.PlotCost, err = manaCostOf(*c.Plot); err != nil {
			return err
		}
	}
	if c.Sneak != nil {
		// CR 702.190a: no KwMask bit is free, so the sneak marker is its own
		// field; the cost is the declare-blockers alternative cost
		// (dirac/kw_sneak.go)
		d.HasSneak = true
		if d.SneakCost, err = manaCostOf(*c.Sneak); err != nil {
			return err
		}
	}
	if c.WebSling != nil {
		// CR 702.188a: no KwMask bit is free, so the web-slinging marker is
		// its own field; the cost is the hand's alternative cost
		// (dirac/kw_websling.go)
		d.HasWebSling = true
		if d.WebSlingCost, err = manaCostOf(*c.WebSling); err != nil {
			return err
		}
	}
	if c.MayFlash != nil {
		// CR 702.8: no KwMask bit is free, so the may-flash marker is its
		// own field; the cost is the extra mana the instant-timing cast adds
		// to the mana cost (dirac/kw_mayflash.go)
		d.HasMayFlash = true
		if d.MayFlashCost, err = manaCostOf(*c.MayFlash); err != nil {
			return err
		}
		if c.MayFlashBehold != nil {
			// the extra is a behold (kernel kw_mayflash.go): the card's
			// behold fields, which no other behold cost may share
			if d.BeholdN != 0 || c.BeholdExile || c.SpellCost != nil || c.OptCost != nil {
				return fmt.Errorf("may-flash behold beside another behold or additional cost")
			}
			d.MayFlashBehold, d.BeholdN = true, 1
			if d.BeholdF, err = cb.filter(c.MayFlashBehold, kop.ZoneBattlefield); err != nil {
				return err
			}
		}
	}
	if c.FbBeholdN != 0 {
		// the flashback cost's behold part (kernel act_behold.go): the
		// card's behold fields, which no other behold cost may share
		if d.BeholdN != 0 || c.BeholdExile || c.SpellCost != nil || c.OptCost != nil || c.FbBeholdN > 255 {
			return fmt.Errorf("flashback behold beside another behold or additional cost")
		}
		d.FbBehold, d.BeholdN = true, uint8(c.FbBeholdN)
		if d.BeholdF, err = cb.filter(c.FbBeholdF, kop.ZoneBattlefield); err != nil {
			return err
		}
	}
	if c.Evoke != nil {
		// CR 702.74a: no KwMask bit is free, so the evoke marker is its own
		// field; the cost is the hand's alternative cost, and the permanent
		// is sacrificed as it enters (dirac/kw_evoke.go)
		d.HasEvoke = true
		if d.EvokeCost, err = manaCostOf(*c.Evoke); err != nil {
			return err
		}
	}
	if c.Dash != nil {
		// CR 702.109a: no KwMask bit is free, so the dash marker is its own
		// field; the cost is the hand's alternative cost, and the permanent
		// gains haste and returns to hand at the next end step
		// (dirac/kw_dash.go)
		d.HasDash = true
		if d.DashCost, err = manaCostOf(*c.Dash); err != nil {
			return err
		}
	}
	if c.Overload != nil {
		// CR 702.96a: the hand's alternative cost and the resolving program's
		// each-variant (dirac/kw_overload.go)
		d.HasOverload = true
		if d.OverloadCost, err = manaCostOf(*c.Overload); err != nil {
			return err
		}
	}
	if c.Bestow != nil {
		// CR 702.103a: no KwMask bit is free, so the bestow marker is its own
		// field; the cost is the hand's alternative cost of the Aura cast and
		// BestowTgt its enchant creature target (dirac/kw_bestow.go)
		d.HasBestow = true
		if d.BestowCost, err = manaCostOf(*c.Bestow); err != nil {
			return err
		}
		if d.BestowTgt, err = cb.target(*c.BestowT); err != nil {
			return err
		}
	}
	if c.Convoke {
		// CR 702.51a: printed convoke; the cast taps creatures for its
		// mana cost (dirac/kw_convoke.go)
		d.Convoke = true
	}
	if c.Delve {
		// CR 702.66a: printed delve; the cast exiles graveyard cards for
		// its generic cost (dirac/kw_delve.go)
		d.Delve = true
	}
	if c.Harmonize != nil {
		// CR 702.180a: a graveyard cast for the harmonize cost; the one
		// announced creature pays its power (dirac/kw_harmonize.go)
		if c.Flashback != nil {
			return fmt.Errorf("Harmonize beside Flashback (both claim CardDef.AltCost)")
		}
		d.Harmonize = true
		if d.AltCost, err = manaCostOf(*c.Harmonize); err != nil {
			return err
		}
	}
	if c.BeamMeUp != nil {
		d.BeamMeUp = true // dirac/kw_beammeup.go; FbCost holds the return
		if d.AltCost, err = manaCostOf(*c.BeamMeUp); err != nil {
			return err
		}
	}
	if c.Mayhem != nil {
		// CR 702.187a: a graveyard cast for the mayhem cost if the card was
		// discarded this turn (dirac/kw_mayhem.go)
		if c.Flashback != nil || c.Harmonize != nil {
			return fmt.Errorf("Mayhem beside Flashback or Harmonize (all claim CardDef.AltCost)")
		}
		d.Mayhem = true
		if d.AltCost, err = manaCostOf(*c.Mayhem); err != nil {
			return err
		}
	}
	if c.Retrace {
		// CR 702.81a: a graveyard cast paying the printed cost plus the
		// additional discard-a-land cost (dirac/kw_retrace.go); lower.go's
		// lowerRetrace has refused every collision, so the CardDef carries
		// only Retrace and RetraceCost.
		if c.RetraceCost == nil {
			return fmt.Errorf("Retrace without its discard cost")
		}
		d.Retrace = true
		if d.RetraceCost, err = cb.cost(*c.RetraceCost); err != nil {
			return err
		}
	}
	if c.Improvise {
		// CR 702.126a: printed improvise; the cast taps artifacts for its
		// generic cost (dirac/kw_convoke.go)
		d.Improvise = true
	}
	if c.BeholdExile {
		// a self RaiseCost's mandatory behold-and-exile (lower_beholdexile.go):
		// the either-or and OptionalCost beholds share these fields
		if d.BeholdN != 0 {
			return fmt.Errorf("BeholdExile beside another behold cost")
		}
		d.BeholdN, d.BeholdExile = uint8(c.BeholdN), true
		if d.BeholdF, err = cb.filter(c.BeholdF, kop.ZoneBattlefield); err != nil {
			return err
		}
	}
	if c.SpellWaterbend != 0 {
		d.SpellWaterbend = uint8(c.SpellWaterbend)
	}
	if c.SpellCost != nil {
		spc := *c.SpellCost
		if spc.BeholdN != 0 {
			// the either-or's behold part is CardDef table data (kernel
			// act_behold.go): both zones' candidates match it
			d.BeholdN = uint8(spc.BeholdN)
			if d.BeholdF, err = cb.filter(spc.BeholdF, kop.ZoneBattlefield); err != nil {
				return err
			}
			spc.BeholdN, spc.BeholdF = 0, nil
		}
		sc, err := cb.cost(spc)
		if err != nil {
			return err
		}
		if c.Types&(tInstant|tSorcery) != 0 {
			// An instant's or sorcery's additional cost rides on its
			// AbSpell ability (dirac/vanilla's Thrill of Possibility).
			cb.spellCost = sc
		} else {
			d.SpellCost = sc
		}
	}
	if c.AltAddCost != nil {
		if d.AltAddCost, err = cb.cost(*c.AltAddCost); err != nil {
			return err
		}
		d.HasAltAdd = true
	}
	d.Leyline = c.Leyline
	d.SelfFlash = c.SelfFlash
	d.Gift = c.Gift
	d.SpentSelf = c.SpentSelf
	d.DmgAmt = c.DmgAmt
	if c.CastBF != nil {
		// Count$LastStateBattlefieldWithFallback: the
		// filter frozen at beginCast into the spell entry's trigMV
		if d.CastBF, err = cb.filter(c.CastBF, kop.ZoneBattlefield); err != nil {
			return err
		}
	}
	if c.ProtFrom != nil {
		if d.ProtFrom, err = cb.filter(c.ProtFrom, kop.ZoneBattlefield); err != nil {
			return err
		}
	}
	d.Infect, d.Toxic = c.Infect, uint8(c.Toxic)
	d.Cascade = uint8(c.Cascade)
	d.AllCreatureTypes = c.Changeling
	d.Evasion = c.Evasion
	d.Wither = c.Wither
	d.AloneAtk = c.AloneAtk
	d.AloneBlk = c.AloneBlk
	d.Engines = c.Engines
	d.AtkFace = c.AtkFace
	d.SagaN = uint8(c.SagaN)
	d.TapPowerCrew = int8(c.TapPowerCrew)
	d.TapPowerSaddle = int8(c.TapPowerSaddle)
	d.MinBlockers = uint8(c.MinBlockers)
	d.ExtraBlocks = uint8(c.ExtraBlocks)
	d.Lure = c.Lure
	d.MustBeBlocked = c.MustBeBlocked
	d.Daybound, d.Nightbound = c.Daybound, c.Nightbound
	d.AltAddOne = c.AltAddOne
	for _, st := range c.Subtypes {
		if st == "Cave" || st == "Desert" {
			d.TypedMana = true
		}
	}
	for _, r := range c.ProtColors {
		d.ProtColors |= kop.ColW << strings.IndexRune("WUBRG", r)
	}
	for _, r := range c.HexColors {
		d.HexColors |= kop.ColW << strings.IndexRune("WUBRG", r)
	}
	if c.Landwalk != nil {
		if d.Landwalk, err = cb.filter(c.Landwalk, kop.ZoneBattlefield); err != nil {
			return err
		}
	}
	for _, t := range c.Tokens {
		cb.out.tokens = append(cb.out.tokens, cb.pt.tokenIdx[t])
	}
	if len(c.Faces) > 0 {
		// pool indexes; the matchup remaps them to its CardIDs
		for i, n := range c.Faces {
			j, ok := cb.pt.tokenIdx[n]
			if i == 0 && !c.Token {
				j, ok = cb.pt.cardIdx[n]
			}
			if !ok {
				return fmt.Errorf("face %q is not in the pool", n)
			}
			d.Faces = append(d.Faces, kop.CardID(j))
		}
		d.Face, d.FaceMode = uint8(c.Face), c.FaceMode
	}
	return nil
}

// cost builds a kop cost.
func (cb *cardBuilder) cost(c lCost) (kop.Cost, error) {
	var out kop.Cost
	var err error
	if c.DrawN > 0 {
		return out, fmt.Errorf("draw cost outside a trigger's own cost window") // kop.PayDraw only
	}
	if c.BeholdN != 0 {
		return out, fmt.Errorf("behold cost outside a spell's behold fields") // CardDef.BeholdN/BeholdF
	}
	if c.EvidenceN > 0 || c.EvidenceX {
		// an activated ability's evidence is a table byte (kop.Ability.Evidence,
		// set by ability() from the lCost; cost is handed it zeroed); anywhere
		// else (a spell's additional cost, a flashback cost) it is refused
		return out, fmt.Errorf("evidence cost outside an activated ability")
	}
	if out.Mana, err = manaCostOf(c.Mana); err != nil {
		return out, err
	}
	counted := 0
	set := func(f kop.CostFlags, n int) {
		out.Flags |= f
		out.N = uint8(n)
		counted++
	}
	if c.Tap {
		out.Flags |= kop.CostTap
	}
	if c.Waterbend > 0 {
		// Cost.N shares the waterbend amount: kgen refuses it beside every
		// other counted part.
		set(kop.CostWaterbend, c.Waterbend)
	}
	if c.WaterbendX {
		set(kop.CostWaterbend, 0) // N 0: the announced X is the amount (Cost.Mana.X)
	}
	if c.SacSelf {
		out.Flags |= kop.CostSacSelf
	}
	if c.TapHost {
		out.Flags |= kop.CostTapHost
	}
	if c.SacHost {
		out.Host = kop.CostHostSac
		if c.HostExile {
			out.Host = kop.CostHostExile
		}
	}
	out.ExileBf = c.SacExile
	if c.XMin > 0 {
		out.XMin = uint8(c.XMin) // the announced X's floor (kernel cost_sacx.go)
	}
	if c.Sac != nil {
		set(kop.CostSac, c.SacN) // SacN 0 (SacX): the announced X (kernel cost_sacx.go)
		// You can only sacrifice what you control: say so in the filter, as
		// dirac/vanilla's Hungry Ghoul does.
		f := *c.Sac
		f.Clauses = append([]lClause(nil), f.Clauses...)
		for i := range f.Clauses {
			if f.Clauses[i].Ctrl == whoAny {
				f.Clauses[i].Ctrl = whoYou
			}
		}
		if out.Filter, err = cb.filter(&f, kop.ZoneBattlefield); err != nil {
			return out, err
		}
	}
	if c.DiscardN > 0 {
		set(kop.CostDiscard, c.DiscardN)
	}
	if c.DiscardHand {
		out.DiscardHand = true
	}
	if c.DiscardF != nil {
		// a filtered discard shares Cost.N (the card count) and Cost.Filter
		// (the candidates' spec, the hand zone): alone among the counted
		// parts
		out.Flags |= kop.CostDiscardFiltered
		if out.Filter, err = cb.filter(c.DiscardF, kop.ZoneHand); err != nil {
			return out, err
		}
	}
	if c.DiscardSelf {
		out.Flags |= kop.CostDiscardSelf
	}
	if c.Loyalty > 0 {
		set(kop.CostAddCtr, c.Loyalty)
		out.Ctr = kop.CtrLoyalty
	}
	if c.Loyalty < 0 {
		set(kop.CostRemoveCtr, -c.Loyalty)
		out.Ctr = kop.CtrLoyalty
	}
	if c.LoyaltyX {
		set(kop.CostLoyaltyX, 0)
		out.Ctr = kop.CtrLoyalty
	}
	if c.SubCtrX {
		// SubCounter<X/KIND>: the same announced-X cost flag, without the
		// loyalty marker, announcing up to the source's counters of this kind
		set(kop.CostLoyaltyX, 0)
		if out.Ctr, err = counterKind(c.SubCtrXKnd); err != nil {
			return out, err
		}
	}
	if c.Exert {
		out.Exert = true
	}
	if c.AddCtr > 0 {
		if c.Loyalty != 0 || c.SubCtr != 0 {
			return out, fmt.Errorf("counter-placing cost beside another counter part")
		}
		set(kop.CostAddCtr, c.AddCtr)
		if c.AddCtrKnd == "M0M1" {
			// -0/-1 counters are read only beside a counter-cost mana
			// ability (prep.ctrMana), so the kind is not in counterKinds: an
			// effect putting one stays refused (Essence Flare, Krovikan
			// Plague)
			out.Ctr = kop.CtrM0M1
		} else if out.Ctr, err = counterKind(c.AddCtrKnd); err != nil {
			return out, err
		}
	}
	if c.SubCtr > 0 {
		set(kop.CostRemoveCtr, c.SubCtr)
		if out.Ctr, err = counterKind(c.SubCtrKnd); err != nil {
			return out, err
		}
	}
	if c.SubAmongN > 0 {
		// the counters come off permanents the
		// payer controls, or the source itself when Filter is 0
		set(kop.CostSubAmong, c.SubAmongN)
		if c.SubAmongKnd == "Any" {
			out.Ctr = kop.CtrAll
		} else if out.Ctr, err = counterKind(c.SubAmongKnd); err != nil {
			return out, err
		}
		if c.SubAmongF != nil {
			f := *c.SubAmongF
			f.Clauses = append([]lClause(nil), f.Clauses...)
			for i := range f.Clauses {
				if f.Clauses[i].Ctrl == whoAny {
					f.Clauses[i].Ctrl = whoYou
				}
			}
			if out.Filter, err = cb.filter(&f, kop.ZoneBattlefield); err != nil {
				return out, err
			}
		}
	}
	if c.PayLife > 0 && c.LifeX {
		return out, refuse("life cost: PayLife<N> beside PayLife<X>")
	}
	if c.PayLife > 0 && c.ExileG != nil && c.ExileHand && c.PayLife == c.ExileGN {
		// PayLife<N> beside ExileFromHand<N/Spec> (an alternative cost,
		// lower_altexilehand.go): the two parts share Cost.N, one count
		out.Flags |= kop.CostPayLife
	} else if c.PayLife > 0 {
		set(kop.CostPayLife, c.PayLife)
	}
	if c.LifeX {
		// the announced form: CostPayLife with N 0 (kernel cost_paylifex.go)
		set(kop.CostPayLife, 0)
	}
	if c.TapN > 0 {
		set(kop.CostTapX, c.TapN)
		if out.Filter, err = cb.filter(c.TapF, kop.ZoneBattlefield); err != nil {
			return out, err
		}
	}
	if c.TapPowN > 0 {
		set(kop.CostTapPower, c.TapPowN)
		if c.TapPowCrew {
			out.Flags |= kop.CostCrew
		}
		if c.TapPowSaddle {
			out.Flags |= kop.CostSaddle
		}
		if out.Filter, err = cb.filter(c.TapPowF, kop.ZoneBattlefield); err != nil {
			return out, err
		}
	}
	if c.ExileG != nil {
		set(kop.CostExileGrave, c.ExileGN)
		if c.ExileGBf {
			out.Flags |= kop.CostExileCtrlOrGrave
		}
		zone := kop.ZoneGraveyard
		if c.ExileHand {
			out.ExileHand, zone = true, kop.ZoneHand
		}
		out.ExileGSelf = c.ExileGSelf
		if out.Filter, err = cb.filter(c.ExileG, zone); err != nil {
			return out, err
		}
	}
	if c.Energy > 0 {
		set(kop.CostPayEnergy, c.Energy)
	}
	if c.ReturnSelf {
		out.Flags |= kop.CostReturnSelf
	}
	if c.ExileSelf {
		out.Flags |= kop.CostExileSelf
	}
	if c.PutLibSelf {
		out.PutLibSelf = true
	}
	if c.Untap {
		out.Flags |= kop.CostUntapSelf
	}
	if c.Return != nil {
		set(kop.CostReturn, c.ReturnN)
		if out.Filter, err = cb.filter(c.Return, kop.ZoneBattlefield); err != nil {
			return out, err
		}
	}
	if c.Forage != nil {
		// Filter: the Food a forage may sacrifice; counted so that no
		// other part shares Cost.N / Cost.Filter
		out.Flags |= kop.CostForage
		counted++
		if out.Filter, err = cb.filter(c.Forage, kop.ZoneBattlefield); err != nil {
			return out, err
		}
	}
	if c.BlightN > 0 {
		// Blight<N> (CR 701.68): one creature you control, the source
		// included (gorge cost.parse: Spec Creature.YouCtrl)
		set(kop.CostBlight, c.BlightN)
		f := &lFilter{Clauses: []lClause{{Types: tCreature, Ctrl: whoYou}}}
		if out.Filter, err = cb.filter(f, kop.ZoneBattlefield); err != nil {
			return out, err
		}
	}
	if counted > 1 {
		return out, fmt.Errorf("cost with two counted parts")
	}
	return out, nil
}

// filter interns an LIR filter (objects only) as a chain of kop filters.
// filterMarkPermanent builds f for an entry counted in the battlefield
// destination whose object may have left: the clause records that its base
// was Permanent so the card matcher can reject it (gorge's Permanent base
// reads the live zone when the spec's zone is the battlefield).
func (cb *cardBuilder) filterMarkPermanent(f *lFilter) (kop.FilterID, error) {
	old := cb.markPerm
	cb.markPerm = true
	id, err := cb.filter(f, kop.ZoneNone)
	cb.markPerm = old
	return id, err
}

// filterDepartures builds a filter read over this turn's zone entries, where a
// battlefield departure's "you control" reads who it left under and "you own"
// its owner (State.matchDeparture), so the two stay apart.
func (cb *cardBuilder) filterDepartures(f *lFilter, zone kop.Zone) (kop.FilterID, error) {
	cb.keepOwner = true
	id, err := cb.filter(f, zone)
	cb.keepOwner = false
	return id, err
}

func (cb *cardBuilder) filter(f *lFilter, zone kop.Zone) (kop.FilterID, error) {
	if f == nil {
		return 0, nil
	}
	return cb.filterZones(f, slices.Repeat([]kop.Zone{zone}, len(f.Clauses)))
}

// filterZones is filter with a zone per clause (zones[i] for clause i; the
// TgtGraveExileCard chain).
func (cb *cardBuilder) filterZones(f *lFilter, zones []kop.Zone) (kop.FilterID, error) {
	if f == nil {
		return 0, nil
	}
	if len(f.Clauses) == 0 {
		return 0, fmt.Errorf("player-only filter where an object filter is needed")
	}
	var next kop.FilterID
	for i := len(f.Clauses) - 1; i >= 0; i-- {
		kf, err := cb.clause(f.Clauses[i], zones[i])
		if err != nil {
			return 0, err
		}
		kf.Or = next
		kf.Each = f.Each && i == 0
		if i == 0 && f.PickSumMax != nil {
			// the pick-set bound rides the head clause (kop.Filter.PickSumMax;
			// WithTotalCMC$, kernel State.pickFits)
			if kf.PickSumMax, err = cb.value(*f.PickSumMax); err != nil {
				return 0, err
			}
		}
		if i == 0 {
			kf.PickDistinctPower = f.PickDistinctPower
		}
		if kf.Flags2&kop.F2SameNameAsSource != 0 || kf.Pred == kop.PredNamed {
			cb.out.readsName = true
		}
		next = cb.pt.internFilter(kf)
	}
	return next, nil
}

func (pt *poolTables) internFilter(kf kop.Filter) kop.FilterID {
	key := fmt.Sprintf("%#v", kf)
	if id, ok := pt.filterKey[key]; ok {
		return id
	}
	pt.filters = append(pt.filters, kf)
	id := kop.FilterID(len(pt.filters) - 1)
	pt.filterKey[key] = id
	return id
}

var flag2 = map[uint64]kop.FilterFlags2{
	fKicked: kop.F2Kicked, fCastByYou: kop.F2CastByYou,
	fEnteredThisTurn: kop.F2EnteredThisTurn, fChosenType: kop.F2ChosenType, fChosenColor: kop.F2ChosenColor,
	fRemembered: kop.F2Remembered, fNotRemembered: kop.F2NotRemembered, fChosenCard: kop.F2ChosenCard,
	fNotChosenCard: kop.F2NotChosenCard, fAttachedToParentTarget: kop.F2AttachedToT0, fTargetedBy: kop.F2TargetedBy,
	fWithFlying: kop.F2WithFlying, fWithoutFlying: kop.F2WithoutFlying,
	fBasic: kop.F2Basic, fSharesColorWithOtherRemembered: kop.F2SharesColorRem, fNotTriggerObj: kop.F2NotTriggerObj, fWasDamaged: kop.F2WasDamaged,
	fLegendary: kop.F2Legendary, fSaddled: kop.F2Saddled, fSneaked: kop.F2Sneaked,
	fBlockedBySrc: kop.F2BlockedBySrc, fDamagedBy: kop.F2DamagedBy, fCrewedSrc: kop.F2CrewedSrc, fWarped: kop.F2Warped,
	fBargained: kop.F2Kicked, // Card.Self+bargained reads the permanent's kicked provenance
	fTeamwork:  kop.F2Kicked, // Card.Self+Teamwork reads the same provenance
}

var flag1 = map[uint64]kop.FilterFlags{
	fSelf: kop.FSelf, fOther: kop.FOther, fToken: kop.FToken, fNonToken: kop.FNontoken, fAttacking: kop.FAttacking,
	fBlocking: kop.FBlocking, fTapped: kop.FTapped, fUntapped: kop.FUntapped,
	fEquippedBy: kop.FAttachedBySrc, fEnchantedBy: kop.FAttachedBySrc,
	fExiledWith: kop.FExiledWith, fNotAttacking: kop.FNotAttacking, fNotAttachedBy: kop.FNotAttachedBySrc,
}

// clauseSubtype2 puts a clause's second subtype on kf: it takes the one Pred
// slot (kernel PredSubtype: Table.Subtypes index N, read on a permanent and on a
// card in any other zone), so it refuses beside any other predicate.
func (cb *cardBuilder) clauseSubtype2(c lClause, kf *kop.Filter) error {
	if c.Subtype2 == "" {
		return nil
	}
	i, ok := cb.pt.sub(c.Subtype2)
	if !ok {
		return fmt.Errorf("subtype %q", c.Subtype2)
	}
	if kf.Pred != kop.PredNone || c.Name != "" && c.Flags&fNamedAsSource == 0 {
		return fmt.Errorf("two subtypes beside a compare")
	}
	kf.Pred, kf.PredN = kop.PredSubtype, int16(i)
	return nil
}

func (cb *cardBuilder) clause(c lClause, zone kop.Zone) (kop.Filter, error) {
	var kf kop.Filter
	kf.Zone = zone
	kf.Special = c.Special
	if zone == kop.ZoneBattlefield {
		kf.Zone = kop.ZoneNone
	}
	// Legendary rides in Types (TLegendary; the parser admits it only
	// alone, the any-of mask then reading "is legendary").
	kf.Types = typeMask(c.Types &^ tBasic)
	if cb.markPerm && c.BasePermanent {
		kf.Flags2 |= kop.F2BasePermanent
	}
	if c.Types&tBasic != 0 {
		kf.Flags2 |= kop.F2Basic
	}
	if c.Types&tEmblem != 0 {
		kf.Types &^= kop.TypeMask(tEmblem & 0xff)
		kf.Flags2 |= kop.F2Emblem
	}
	kf.NotTypes = typeMask(c.NotTypes)
	kf.Colors, kf.NotColors = colorMask(c.Colors), colorMask(c.NotColors)
	switch c.Ctrl {
	case whoEachPl:
		// a hand, library or graveyard card is controlled by its owner, the
		// kernel card matcher's owner (matchCardOneAt)
		if zone != kop.ZoneBattlefield && zone != kop.ZoneHand && zone != kop.ZoneLibrary && zone != kop.ZoneGraveyard {
			return kf, refuse("RememberedPlayerCtrl off the battlefield")
		}
		kf.Ctrl = kop.CtrlEachPl
	case whoHitBySrc:
		// a Ctrl value, as whoEachPl (FilterFlags2 is full);
		// battlefield only
		if zone != kop.ZoneBattlefield {
			return kf, refuse("wasDealtCombatDamageThisTurnBySource off the battlefield")
		}
		kf.Ctrl = kop.CtrlHitBySrc
	case whoTgtPlayer, whoDefending, whoTgtOrCtrl:
		// controller roles the kernel reads through a Flags2 bit (or, for
		// CtrlTgt, a target slot); the card matchers never see them
		// (battlefield only)
		if zone != kop.ZoneBattlefield {
			if c.Ctrl == whoTgtPlayer {
				return kf, refuse("TargetedPlayerCtrl off the battlefield")
			}
			if c.Ctrl == whoTgtOrCtrl {
				return kf, refuse("ControlledBy TargetedOrController off the battlefield")
			}
			return kf, refuse("ControlledBy TriggeredDefendingPlayer outside an Attacks trigger's battlefield filter")
		}
		switch c.Ctrl {
		case whoTgtPlayer:
			kf.Flags2 |= kop.F2TgtPlayerCtrl
		case whoTgtOrCtrl:
			if c.CtrlTgt == 0 {
				return kf, refuse("ControlledBy TargetedOrController without a target slot")
			}
			kf.CtrlTgt = c.CtrlTgt
		default:
			kf.Flags2 |= kop.F2DefendingCtrl
		}
		kf.Ctrl = kop.CtrlAny
	default:
		kf.Ctrl = kop.CtrlSel(c.Ctrl)
	}
	switch {
	case c.Owner == whoEachPl:
		// a graveyard, hand or library card is controlled by its owner, the
		// kernel card matcher's owner read (matchCardOneAt's fl.Owner arm);
		// the battlefield perm matcher has no owner arm for the role
		if zone != kop.ZoneGraveyard && zone != kop.ZoneHand && zone != kop.ZoneLibrary {
			return kf, refuse("RememberedPlayerOwn off the battlefield")
		}
		kf.Owner = kop.CtrlEachPl
	default:
		kf.Owner = kop.CtrlSel(c.Owner)
	}
	if c.Owner == whoTgtPlayer || c.Owner == whoDefending {
		f2, err := ownerRoleF2(c, zone) // lower_filterreferents.go
		if err != nil {
			return kf, err
		}
		kf.Flags2 |= f2
		kf.Owner = kop.CtrlAny
	}
	if c.SpellBase {
		// the Spell base: an object on the stack (gorge compiled_predicate
		// base Spell); only the ReduceCost ValidTarget$ Filter2 reads it
		if zone != kop.ZoneStack {
			return kf, refuse("a Spell head outside a stack filter")
		}
		kf.Flags |= kop.FSpell
	}
	if zone != kop.ZoneBattlefield && zone != kop.ZoneStack && kf.Ctrl == kop.CtrlAny && !cb.keepOwner {
		// Off the battlefield the kernel reads Ctrl as the owner (matchCard).
		kf.Ctrl, kf.Owner = kf.Owner, kop.CtrlAny
	}
	if c.Subtype != "" {
		i, ok := cb.pt.sub(c.Subtype)
		if !ok {
			return kf, fmt.Errorf("subtype %q", c.Subtype)
		}
		kf.Sub = uint8(i + 1)
	}
	if c.NotSubtype != "" {
		i, ok := cb.pt.sub(c.NotSubtype)
		if !ok {
			return kf, fmt.Errorf("subtype %q", c.NotSubtype)
		}
		kf.NotSub = uint8(i + 1)
	}
	for j, w := range c.MoreNotSubtype {
		if w == "" {
			continue
		}
		i, ok := cb.pt.sub(w)
		if !ok {
			return kf, fmt.Errorf("subtype %q", w)
		}
		kf.NotSubs[j] = uint8(i + 1)
	}
	if c.Flags&fWarped != 0 && zone != kop.ZoneBattlefield {
		// the kernel's warp mark is the pending warp exile of a
		// battlefield permanent (kernel warpedPerm); a spell or card read
		// would fail closed
		return kf, refuse("CastSa Spell.Warp off the battlefield")
	}
	if c.Flags&fSneaked != 0 && !cb.selfCast && zone != kop.ZoneBattlefield {
		// a battlefield permanent reads the arena's mechSneaked mark
		// the stack reads are the SpellCast self trigger
		// and Count$ValidStack Card.Self+sneaked
		return kf, refuse("sneaked off the battlefield and the stack")
	}
	if c.Unblocked {
		kf.Flags2 |= kop.F2Unblocked // a battlefield read only (the card matchers fail closed)
	}
	for b := uint64(1); b != 0; b <<= 1 {
		if c.Flags&b == 0 {
			continue
		}
		if f, ok := flag1[b]; ok {
			kf.Flags |= f
		} else if f, ok := flag2[b]; ok {
			kf.Flags2 |= f
		} else {
			return kf, fmt.Errorf("filter flag %s has no kop form", flagNames[bitIndex(b)])
		}
	}
	if c.TypesAll != 0 {
		// the all-of type conjunction takes the clause's one Pred slot
		switch {
		case c.CmpStat == cmpNone:
			kf.Pred, kf.PredN = kop.PredTypesAll, int16(typeMask(c.TypesAll))
		case c.CmpStat == cmpCMC && c.CmpV == nil && (c.CmpOp == opLE || c.CmpOp == opLT):
			// a constant mana-value bound beside the conjunction: one
			// combined predicate (Resistance Is Futile's Artifact.Creature
			// with cmcLE3); LT lowers the integer bound by one
			bound := c.CmpN
			if c.CmpOp == opLT {
				bound--
			}
			kf.Pred, kf.PredN, kf.PredV = kop.PredTypesAllCMCLE, int16(typeMask(c.TypesAll)), kop.Const(bound)
			c.CmpStat = cmpNone
		default:
			return kf, fmt.Errorf("type conjunction %#x beside a %s predicate (one Pred slot)", c.TypesAll, cmpStatName(c.CmpStat))
		}
	}
	switch c.CmpStat {
	case cmpNone:
	case cmpPower:
		if c.CmpV != nil && c.CmpOp == opEQ && zone == kop.ZoneStack {
			// a spell's power == a value (Talion's powerEQChosen; kernel
			// filter_ptvalue.go): PredPowerGEV with PredN 1
			v, err := cb.value(*c.CmpV)
			if err != nil {
				return kf, err
			}
			kf.Pred, kf.PredN, kf.PredV = kop.PredPowerGEV, 1, v
			break
		}
		if c.CmpV != nil && (c.CmpOp == opLE || c.CmpOp == opLT) && zone == kop.ZoneGraveyard {
			// a graveyard card's printed power <= a value (Ruthless
			// Technomancer's powerLEX target; kernel cardPowerGEV with PredN 2)
			bound := *c.CmpV
			if c.CmpOp == opLT {
				if bound.Neg {
					return kf, fmt.Errorf("power compare < a negated value")
				}
				bound.Add-- // power is an integer: < X is <= X-1
			}
			v, err := cb.value(bound)
			if err != nil {
				return kf, err
			}
			kf.Pred, kf.PredN, kf.PredV = kop.PredPowerGEV, 2, v
			break
		}
		if done, err := cb.powerBattlefieldClause(c, zone, &kf); err != nil {
			return kf, err
		} else if done {
			break
		}
		op, n := intCmpGELE(c.CmpOp, c.CmpN) // powers are integers: LT n is LE n-1
		switch op {
		case opGE:
			kf.Pred = kop.PredPowerGE
		case opLE:
			kf.Pred = kop.PredPowerLE
		default:
			return kf, fmt.Errorf("power compare %d", c.CmpOp)
		}
		kf.PredN = int16(n)
	case cmpToughness:
		if c.CmpV != nil && c.CmpOp == opEQ && zone == kop.ZoneStack {
			// a spell's toughness == a value (Talion's toughnessEQChosen;
			// kernel filter_ptvalue.go): PredToughnessLEV with PredN 1
			v, err := cb.value(*c.CmpV)
			if err != nil {
				return kf, err
			}
			kf.Pred, kf.PredN, kf.PredV = kop.PredToughnessLEV, 1, v
			break
		}
		if c.CmpV != nil && (c.CmpOp == opLE || c.CmpOp == opLT) && zone == kop.ZoneBattlefield {
			// a battlefield toughness <= a value (Zero Point Ballad's
			// toughnessLEX; kernel predPermCold, the PredPowerGEV twin)
			bound := *c.CmpV
			if c.CmpOp == opLT {
				// toughness is an integer: < X is <= X-1
				if bound.Neg {
					return kf, fmt.Errorf("toughness compare < a negated value")
				}
				bound.Add--
			}
			v, err := cb.value(bound)
			if err != nil {
				return kf, err
			}
			kf.Pred, kf.PredV = kop.PredToughnessLEV, v
			break
		}
		if done, err := cb.toughnessGTClause(c, zone, &kf); err != nil {
			return kf, err
		} else if done {
			break
		}
		op, n := c.CmpOp, c.CmpN
		if c.CmpV == nil && c.CmpX == "" {
			op, n = intCmpGELE(op, n)
		}
		switch op {
		case opGE:
			kf.Pred = kop.PredToughnessGE
		case opLE:
			kf.Pred = kop.PredToughnessLE
		default:
			return kf, fmt.Errorf("toughness compare %d", c.CmpOp)
		}
		if c.CmpV != nil || c.CmpX != "" {
			return kf, fmt.Errorf("toughness compare against a value")
		}
		kf.PredN = int16(n)
	case cmpHasX:
		kf.Pred = kop.PredHasXCost
	case cmpPowerLTToughness:
		kf.Pred = kop.PredPowerLTToughness
	case cmpMultiColor:
		kf.Pred = kop.PredMultiColor
	case cmpHistoric:
		kf.Pred = kop.PredHistoric
		i, _ := cb.pt.sub("Saga")
		kf.PredN = int16(i + 1)
	case cmpEquipped:
		kf.Pred = kop.PredEquipped
	case cmpEnchanted:
		kf.Pred = kop.PredEnchanted
	case cmpAttached:
		kf.Pred = kop.PredAttached
	case cmpActiveCtrl:
		kf.Pred = kop.PredActiveCtrl
	case cmpTrigPlayerCtrl:
		kf.Pred = kop.PredTrigPlayerCtrl
	case cmpModified:
		kf.Pred = kop.PredModified
	case cmpHasCounters:
		kf.Pred = kop.PredHasCounters
	case cmpMonstrous:
		kf.Pred = kop.PredMonstrous
	case cmpNoCounters:
		kf.Pred = kop.PredNoCounters
	case cmpLacksKw:
		kf.Pred, kf.PredN = kop.PredLacksKeyword, int16(c.CmpN)
	case cmpHasManaAbility:
		kf.Pred, kf.PredN = kop.PredHasManaAbility, int16(c.CmpN)
	case cmpOutlaw:
		kf.Pred, kf.PredN = kop.PredOutlaw, int16(c.CmpN)
		for _, w := range [...]string{"Assassin", "Mercenary", "Pirate", "Rogue", "Warlock"} {
			cb.pt.sub(w) // the kernel finds the five by name (genPrep.outlawSub)
		}
	case cmpMonoColor:
		kf.Pred = kop.PredMonoColor
	case cmpSameNameT0:
		kf.Pred = kop.PredSameNameT0
	case cmpSameNameRem:
		kf.Pred = kop.PredSameNameRem
	case cmpAdventureCard:
		kf.Pred = kop.PredAdventureCard
	case cmpSolved:
		// a Case's solved designation (kernel predPermCold);
		// off the battlefield nothing is solved (gorge clears Solved on
		// the zone move)
		kf.Pred, kf.PredN = kop.PredSolved, int16(c.CmpN)
	case cmpSuspected:
		// the arena designation bit (kernel predPermCold); off
		// the battlefield nothing is suspected (predCardCold)
		kf.Pred, kf.PredN = kop.PredSuspected, int16(c.CmpN)
	case cmpPermMark:
		// an arena slot mark (kernel permMarkRead); off the
		// battlefield nothing is marked (predCardCold)
		kf.Pred, kf.PredN = kop.PredPermMark, int16(c.CmpN)
	case cmpExiledWith:
		// the exiled-with source link (kernel exwCard); PredN 1
		// is Defined$ ExiledWith, which also honours a resolving trigger's
		// fire ver (gorge exiledWithSet)
		kf.Pred, kf.PredN = kop.PredExiledWith, int16(c.CmpN)
	case cmpTriggerObj:
		// the resolving trigger's object (kernel isTriggerObj), wherever
		// it is now: the ChooseFromDefined$ TriggeredCards pool's
		// membership (Paranormal Analyst's return)
		kf.Pred, kf.PredN = kop.PredTriggerObj, int16(c.CmpN)
	case cmpFaceDown:
		kf.Pred = kop.PredFaceDown // a face-down battlefield permanent (kernel faceDownPred)
	case cmpPowerGTBase:
		kf.Pred = kop.PredPowerGTBase // current power > base power (kernel powerGTBase)
	case cmpWord7:
		if c.CmpN == cmpNoNameGrave {
			return kf, refuse("doesNotShareNameWith YourGraveyard without OtherYourBattlefield")
		}
		if c.CmpN == int(kop.W7NoAbilities) && zone == kop.ZoneBattlefield {
			// a permanent's granted or lost abilities change the answer
			return kf, refuse("NoAbilities on the battlefield")
		}
		if c.CmpN == int(kop.W7EnteredFromBF) && zone == kop.ZoneBattlefield {
			// the word reads a card's latest arrival; a permanent has none to read
			return kf, refuse("ThisTurnEnteredFrom_Battlefield on the battlefield")
		}
		kf.Pred, kf.PredN = kop.PredWord7, int16(c.CmpN) // (kernel wordPerm / wordCard)
	case cmpNotF2:
		// !targetedBy / !ThisTurnEntered / !wasDealtDamageThisTurn
		// (kernel predNotF2); CmpN is the negated Flags2 bit's index
		kf.Pred, kf.PredN = kop.PredNotF2, int16(c.CmpN)
	case cmpWasCast:
		// the entering permanent's own resolving spell
		// (kernel enterCast); a battlefield read only
		if zone != kop.ZoneBattlefield {
			return kf, refuse("wasCast off the battlefield")
		}
		kf.Pred, kf.PredN = kop.PredWasCastEnter, int16(c.CmpN)
	case cmpWasCastSpell:
		// a spell on the stack (kernel wasCastSpell reads
		// its entry); a permanent is never read here
		if zone == kop.ZoneBattlefield {
			return kf, refuse("spell cast provenance on the battlefield")
		}
		kf.Pred, kf.PredN = kop.PredWasCastSpell, int16(c.CmpN)
	case cmpTotalPT, cmpBlocked:
		// battlefield-only high predicates (kernel predPermCold)
		if zone != kop.ZoneBattlefield {
			return kf, refuse("totalPT / blocked off the battlefield")
		}
		if c.CmpStat == cmpBlocked {
			kf.Pred = kop.PredBlocked
			break
		}
		cmp := [...]kop.DmgCmp{opGE: kop.DmgGE, opLE: kop.DmgLE, opEQ: kop.DmgEQ, opGT: kop.DmgGT, opLT: kop.DmgLT, opNE: kop.DmgNE}[c.CmpOp]
		if c.CmpN < 0 || c.CmpN > 255 {
			return kf, fmt.Errorf("totalPT bound %d", c.CmpN)
		}
		kf.Pred, kf.PredN = kop.PredTotalPT, kop.DmgGate(cmp, c.CmpN)
	case cmpHasKw:
		if c.CmpN == bits.TrailingZeros32(uint32(kop.KwFlash)) && zone == kop.ZoneBattlefield {
			return kf, refuse("hasKeywordFlash on the battlefield") // the derived keyword set is not proved to carry KwFlash
		}
		kf.Pred, kf.PredN = kop.PredHasKeyword, int16(c.CmpN)
	case cmpCMC:
		if c.CmpV != nil {
			bound := *c.CmpV
			switch c.CmpOp {
			case opLE:
			case opEQ:
				kf.PredN = 1 // == the value (Repurposing Bay's cmcEQX)
			case opLT:
				// mana values are integers: < X is <= X-1 (cmcLTX)
				if bound.Neg {
					return kf, fmt.Errorf("mana value compare < a negated value")
				}
				bound.Add--
			default:
				return kf, fmt.Errorf("mana value compare %d against a value", c.CmpOp)
			}
			kf.Pred = kop.PredCMCLEV
			v, err := cb.value(bound)
			if err != nil {
				return kf, err
			}
			kf.PredV = v
			break
		}
		switch c.CmpOp {
		case opLE:
			kf.Pred, kf.PredN = kop.PredCMCLE, int16(c.CmpN)
		case opGE:
			kf.Pred, kf.PredN = kop.PredCMCGE, int16(c.CmpN)
		case opEQ:
			kf.Pred, kf.PredN = kop.PredCMCEQ, int16(c.CmpN)
		case opLT:
			// mana values are integers: < N is <= N-1 (cmcLT10)
			kf.Pred, kf.PredN = kop.PredCMCLE, int16(c.CmpN-1)
		case opGT:
			// ... and > N is >= N+1 (cmcGT10)
			kf.Pred, kf.PredN = kop.PredCMCGE, int16(c.CmpN+1)
		default:
			return kf, fmt.Errorf("mana value compare %d", c.CmpOp)
		}
	case cmpCounters:
		k, err := counterKind(c.CmpCounter)
		if err != nil {
			return kf, err
		}
		switch {
		case c.CmpOp == opGE:
			if c.CmpN > 127 {
				// the threshold is the int16 high byte: above 127 it would
				// wrap negative and the predicate would match everything
				return kf, fmt.Errorf("counter compare %d on %s", c.CmpOp, c.CmpCounter)
			}
			kf.Pred = kop.PredCountersGE
			kf.PredN = int16(c.CmpN<<8 | int(k))
		case c.CmpOp == opLT && c.CmpN >= 1 || c.CmpOp == opLE || c.CmpOp == opEQ && c.CmpN == 0:
			// fewer than n: LE n is LT n+1, EQ0 is LT 1
			n := c.CmpN
			if c.CmpOp != opLT {
				n++
			}
			if k == kop.CtrStash || n > 127 {
				return kf, fmt.Errorf("counter compare %d on %s", c.CmpOp, c.CmpCounter)
			}
			kf.Pred = kop.PredCountersLT
			kf.PredN = int16(n<<8 | int(k))
		default:
			return kf, fmt.Errorf("counter compare %d", c.CmpOp)
		}
	}
	if err := cb.clauseSubtype2(c, &kf); err != nil {
		return kf, err
	}
	if c.Name != "" && c.Flags&fNamedAsSource == 0 {
		// a named<X> clause, the source's own name included
		// (gorge wordNamed -> sharesName, CR 201.2), any zone: Pred = PredNamed, PredN = the
		// pool entry with that printed name, -1 when the pool has none
		// (no object has the name: sharesName matches nothing). The
		// matchers read the name card (nameCardAt: a renamed permanent,
		// CR 707.2 copies; face-down has none, CR 708.2).
		if kf.Pred != kop.PredNone {
			return kf, fmt.Errorf("named clause beside a predicate (Pred %d)", kf.Pred)
		}
		ix := cb.pt.named[c.Name]
		if len(ix) > 1 {
			return kf, fmt.Errorf("named clause %q names %d pool entries", c.Name, len(ix))
		}
		kf.Pred, kf.PredN = kop.PredNamed, -1
		if len(ix) == 1 {
			kf.PredN = int16(ix[0])
		}
	}
	return kf, nil
}

func bitIndex(b uint64) int {
	for i := 0; i < 64; i++ {
		if b == 1<<i {
			return i
		}
	}
	return -1
}

// tgtRefOf is the kop ref naming target slot n (0..4): slots 0-2 are
// contiguous, 3 and 4 are the high ordinals.
func tgtRefOf(n int) kop.Ref {
	switch n {
	case 3:
		return kop.RefTarget3
	case 4:
		return kop.RefTarget4
	}
	return kop.RefTarget0 + kop.Ref(n)
}

// powerBattlefieldClause lowers a clause's power compare against a value, or
// a battlefield power == a constant, into kf (done false: not one of these).
// A power >= a value is a permanent's (Gix's Command's greatest power; kernel
// predPermCold, or powerGEVFrom in the filter's source context), or an
// object's where it now is, a departed trigger object's last known power
// (kernel cardPowerGEV); > X lowers as >= X+1 (power is an integer; Ensign
// Harry Kim, Operations' powerGTY over his own power). A battlefield power ==
// a constant (Amalia Benavides Aguirre's powerEQ20) is PredPowerGEV with
// PredN 3.
func (cb *cardBuilder) powerBattlefieldClause(c lClause, zone kop.Zone, kf *kop.Filter) (done bool, err error) {
	if c.CmpV == nil && c.CmpX == "" && c.CmpOp == opEQ && zone == kop.ZoneBattlefield {
		kf.Pred, kf.PredN, kf.PredV = kop.PredPowerGEV, 3, kop.Const(c.CmpN)
		return true, nil
	}
	if c.CmpV == nil {
		return false, nil
	}
	bound := *c.CmpV
	if c.CmpOp == opGT && !bound.Neg {
		bound.Add++
	} else if c.CmpOp != opGE {
		return false, fmt.Errorf("power compare against a value")
	}
	if zone != kop.ZoneBattlefield && zone != kop.ZoneNone {
		return false, fmt.Errorf("power compare against a value")
	}
	v, err := cb.value(bound)
	if err != nil {
		return false, err
	}
	kf.Pred, kf.PredV = kop.PredPowerGEV, v
	return true, nil
}

// toughnessGTClause lowers a toughness > a value (Hulkling's toughnessGTZ;
// kernel predPermCold, or cardToughnessGEV for a departed trigger object's
// last known toughness) into kf (done false: not this shape). Toughness is an
// integer, so > X is >= X+1: PredToughnessLEV with PredN 3 (toughness >=
// PredV).
func (cb *cardBuilder) toughnessGTClause(c lClause, zone kop.Zone, kf *kop.Filter) (done bool, err error) {
	if c.CmpV == nil || c.CmpOp != opGT || c.CmpV.Neg || (zone != kop.ZoneBattlefield && zone != kop.ZoneNone) {
		return false, nil
	}
	bound := *c.CmpV
	bound.Add++
	v, err := cb.value(bound)
	if err != nil {
		return false, err
	}
	kf.Pred, kf.PredN, kf.PredV = kop.PredToughnessLEV, 3, v
	return true, nil
}

// tgtSlotOfRef is the target slot a kop target ref names (ok false: not one).
func tgtSlotOfRef(r kop.Ref) (int, bool) {
	switch r {
	case kop.RefTarget0, kop.RefTarget1, kop.RefTarget2:
		return int(r - kop.RefTarget0), true
	case kop.RefTarget3:
		return 3, true
	case kop.RefTarget4:
		return 4, true
	}
	return 0, false
}

// ref maps an LIR ref.
func (cb *cardBuilder) ref(r lRef) (kop.Ref, error) {
	switch r.Kind {
	case rNone:
		return kop.RefNone, nil
	case rTarget:
		if r.N > 4 {
			return 0, fmt.Errorf("target slot %d", r.N)
		}
		return tgtRefOf(int(r.N)), nil
	case rTargetController, rTargetOwner:
		if r.Kind == rTargetController && r.N > 0 && r.N <= 2 {
			return kop.RefTgtCtrl1 + kop.Ref(r.N-1), nil // a later slot's controller
		}
		if r.N != 0 {
			return 0, fmt.Errorf("controller/owner of target slot %d", r.N)
		}
		if r.Kind == rTargetController {
			return kop.RefTargetController, nil
		}
		return kop.RefTargetOwner, nil
	case rSelf:
		return kop.RefSelf, nil
	case rYou:
		return kop.RefController, nil
	case rEachOpp:
		return kop.RefEachOpponent, nil
	case rEachPlayer:
		return kop.RefEachPlayer, nil
	case rTriggered, rTriggeredNew:
		return kop.RefTriggerObj, nil
	case rTriggeredSource:
		return kop.RefTriggerSource, nil
	case rTriggeredPlayer:
		return kop.RefTriggerPlayer, nil
	case rTriggeredTgtCtrl:
		return kop.RefTriggerTgtCtrl, nil
	case rEquipped:
		return kop.RefAttached, nil
	case rRemembered:
		return kop.RefRemembered, nil
	case rDelayedRemembered:
		return kop.RefDelayedRemembered, nil
	case rReplaced:
		return kop.RefReplaced, nil
	case rChosen:
		return kop.RefChosen, nil
	case rEach:
		return kop.RefEach, nil
	case rSacrificed:
		return kop.RefSacrificed, nil
	case rHost:
		return kop.RefHost, nil
	case rDefending:
		return kop.RefDefendingPlayer, nil
	case rRememberedController:
		return kop.RefRememberedController, nil
	case rDiscarded:
		return kop.RefDiscarded, nil
	case rLibTop:
		return kop.RefLibTop, nil
	case rOppNonRemCtrl:
		return kop.RefOppNonRemCtrl, nil
	case rPromised:
		return kop.RefPromised, nil
	case rAttachedCtrl:
		return kop.RefAttachedCtrl, nil
	case rLastCreated:
		return kop.RefLastCreated, nil
	case rSrcOwner:
		return kop.RefSrcOwner, nil
	case rTriggerAttackers:
		return kop.RefTriggerAttackers, nil
	case rRememberedOwner:
		return kop.RefRememberedOwner, nil
	case rImprinted:
		return kop.RefImprinted, nil
	case rTargetUnion:
		switch r.N {
		case 3:
			return kop.RefTargets01, nil
		case 7:
			return kop.RefTargets012, nil
		case 6:
			return kop.RefTargets12, nil
		case 15:
			return kop.RefTargets0123, nil
		}
	}
	return 0, fmt.Errorf("ref %s", r)
}

// value maps an LIR value to a ValueRef (simple kinds direct, the rest
// through the Exprs table).
func (cb *cardBuilder) value(v lValue) (kop.ValueRef, error) {
	plain := v.Mul == 0 && v.Div == 0 && v.Add == 0 && !v.Neg
	switch {
	case v.Kind == vNone:
		return kop.ValueRef{}, nil
	case v.Kind == vConst && plain:
		return kop.Const(v.N), nil
	case v.Kind == vConst && v.Neg && v.Mul == 0 && v.Div == 0 && v.Add == 0:
		return kop.Const(-v.N), nil
	case v.Kind == vX && plain:
		return kop.ValueRef{K: kop.VX}, nil
	case v.Kind == vX && v.Neg && v.Mul == 0 && v.Div == 0 && v.Add == 0:
		return kop.ValueRef{K: kop.VNegX}, nil
	case v.Kind == vTriggerAmount && plain:
		return kop.ValueRef{K: kop.VTriggerAmount}, nil
	case v.Kind == vPowerOf && plain && v.Ref.Kind == rSelf:
		return kop.ValueRef{K: kop.VSelfPower}, nil
	case v.Kind == vPowerOf && plain && v.Ref.Kind == rTarget && v.Ref.N == 0:
		return kop.ValueRef{K: kop.VTargetPower}, nil
	case v.Kind == vToughnessOf && plain && v.Ref.Kind == rTarget && v.Ref.N == 0:
		return kop.ValueRef{K: kop.VTargetToughness}, nil
	case v.Kind == vCountersOn && plain && v.Ref.Kind == rSelf && v.S != "ALL": // ALL: the expression's CtrAll sum
		k, err := counterKind(v.S)
		if err != nil {
			return kop.ValueRef{}, err
		}
		return kop.ValueRef{K: kop.VCountersSelf, N: int16(k)}, nil
	case v.Kind == vLastDamage && plain:
		return kop.ValueRef{K: kop.VLastDamage}, nil
	case v.Kind == vLifeGainedThisTurn && plain:
		return kop.ValueRef{K: kop.VLifeGainedTurn}, nil
	case v.Kind == vCount && plain:
		id, err := cb.filter(v.F, kop.ZoneBattlefield)
		if err != nil {
			return kop.ValueRef{}, err
		}
		return kop.ValueRef{K: kop.VCountFilter, N: int16(id)}, nil
	}
	e, err := cb.expr(v)
	if err != nil {
		return kop.ValueRef{}, err
	}
	key := fmt.Sprintf("%#v", e)
	if i, ok := cb.pt.exprKey[key]; ok {
		return kop.ValueRef{K: kop.VExpr, N: i}, nil
	}
	cb.pt.exprs = append(cb.pt.exprs, e)
	i := int16(len(cb.pt.exprs) - 1)
	cb.pt.exprKey[key] = i
	return kop.ValueRef{K: kop.VExpr, N: i}, nil
}

var exprKinds = map[valKind]kop.ExprKind{
	vConst: kop.ExConst, vX: kop.ExX, vCount: kop.ExCount, vCountGraveyard: kop.ExCountGraveyard,
	vSumPower: kop.ExSumPower, vDistinctCMC: kop.ExDistinctCMC, vKicked: kop.ExKicked, vMorbid: kop.ExMorbid,
	vAttackedThisTurn: kop.ExAttackedThisTurn, vPowerOf: kop.ExPowerOf, vToughnessOf: kop.ExToughnessOf,
	vLifeOf: kop.ExLifeOf, vLibrarySizeOf: kop.ExLibrarySizeOf, vStartingLife: kop.ExStartingLife,
	vTriggerAmount: kop.ExTriggerAmount, vLifeGainedThisTurn: kop.ExLifeGainedThisTurn,
	vOppLifeLostThisTurn: kop.ExOppLifeLostThisTurn, vCountersOn: kop.ExCountersOn, vCreaturesDied: kop.ExCreaturesDied,
	vRememberedCount: kop.ExRememberedCount, vExcessDamage: kop.ExExcessDamage, vLastLifeLost: kop.ExLastLifeLost,
	vTokensCreated: kop.ExTokensCreated, vLifeLostThisTurn: kop.ExLifeLostThisTurn, vSum: kop.ExSum,
	vHandSizeOf: kop.ExHandSizeOf, vCountExile: kop.ExCountExile, vExileTypes: kop.ExCountExile, vOppsLostLife: kop.ExOppsLostLife, vMVOf: kop.ExMVOf, vNotOwnLibMV: kop.ExNotOwnLibMV,
	vCountAtLeast: kop.ExCountAtLeast,
	vPromisedGift: kop.ExPromisedGift, vResolvedThisTurn: kop.ExResolvedThisTurn,
	vThisTurnCast: kop.ExThisTurnCast,
	vRollResult:   kop.ExRollResult, vCastManaSpent: kop.ExCastManaSpent, vConverge: kop.ExConverge,
	vCompare: kop.ExCompare, vBranches: kop.ExBranches,
	vDevotion: kop.ExDevotion, vParty: kop.ExParty, vDrawnThisTurn: kop.ExDrawnThisTurn, vGraveTypes: kop.ExGraveTypes,
	vValidAgg: kop.ExValidAgg, vGraveAgg: kop.ExGraveAgg, vYourSpeed: kop.ExYourSpeed,
	vHighestOppPoison: kop.ExHighestOppPoison, vThisTurnEntered: kop.ExThisTurnEntered,
	vThisTurnCastSpec:   kop.ExThisTurnCastSpec,
	vCommittedCrime:     kop.ExCommittedCrime,
	vSelfPrepared:       kop.ExSelfPrepared,
	vThisTurnActGrave:   kop.ExThisTurnActGrave,
	vSneaked:            kop.ExSneaked,
	vTriggerSpellMVLKI:  kop.ExTriggerSpellMVLKI,
	vPoisonOf:           kop.ExPoisonOf,
	vProduct:            kop.ExProduct,
	vAttackersAll:       kop.ExAttackersAll,
	vRefCount:           kop.ExRefCount,
	vNumColorsOf:        kop.ExNumColorsOf,
	vCountHand:          kop.ExCountHand,
	vYourTurns:          kop.ExYourTurns,
	vLowestLife:         kop.ExLowestLife,
	vOppsAttacked:       kop.ExOppsAttacked,
	vRedNCDmg:           kop.ExRedNCDmg,
	vVoid:               kop.ExVoid,
	vRevolt:             kop.ExRevolt,
	vLeftZone:           kop.ExLeftZone,
	vCastFromGrave:      kop.ExCastFromGrave,
	vFirstCast:          kop.ExFirstCast,
	vIfCastOwnMain:      kop.ExIfCastOwnMain,
	vCtrAdded:           kop.ExCtrAdded,
	vSacrificedThisTurn: kop.ExSacrificedThisTurn,
	vDiscardedThisTurn:  kop.ExDiscardedThisTurn,
	vScriedThisTurn:     kop.ExScriedThisTurn,
	vDilemmasFaced:      kop.ExDilemmasFaced,
	vExiledWithN:        kop.ExExiledWithN,
	vRemLKIPT:           kop.ExRemLKIPT,
	vCastBattlefield:    kop.ExCastBattlefield,
	vMod:                kop.ExMod,
	vPow:                kop.ExPow,
	vAttackedTurn:       kop.ExAttackedTurn,
	vPhaseCount:         kop.ExPhaseCount,
	vAdamant:            kop.ExAdamant,
	vChromaOf:           kop.ExChromaOf,
	vYourExtraTurns:     kop.ExYourExtraTurns,
	vCounterKinds:       kop.ExCounterKinds,
	vTargetPlayers:      kop.ExTargetPlayers,
	vPWTypes:            kop.ExCount,
	vTapPower:           kop.ExTapPower,
	vStepIn:             kop.ExStepIn,
	vTgtSpent:           kop.ExTgtSpent,
	vOppExtreme:         kop.ExOppExtreme,
	vPlExtreme:          kop.ExPlExtreme,
	vThisTurnActExhaust: kop.ExThisTurnActExhaust,
	vActivations:        kop.ExActivations,
	vLifeLostLastTurn:   kop.ExLifeLostLastTurn,
	vDmgAmount:          kop.ExDmgAmountReceived,
	vMonarch:            kop.ExMonarch, // lower_monarch.go
}

func (cb *cardBuilder) expr(v lValue) (kop.Expr, error) {
	k, ok := exprKinds[v.Kind]
	if !ok {
		return kop.Expr{}, fmt.Errorf("value kind %s", valNames[v.Kind])
	}
	e := kop.Expr{Kind: k, N: int16(v.N), Mul: int8(v.Mul), Div: int8(v.Div), Add: int8(v.Add), Neg: v.Neg}
	if v.Kind == vCompare {
		e.Op = cmpOps[v.Op]
	}
	var err error
	if v.Kind == vCtrAdded && !cb.ctrGate {
		return e, refuse("Count$CountersAddedThisTurn outside a >= 1 condition") // the layout keeps no amounts (value_countersadded.go)
	}
	if v.Kind == vLeftZone {
		e.N = int16(v.Zone)
	}
	if v.Kind == vExileTypes {
		e.N = 1 // ExCountExile N 1: the distinct card types, not the count
	}
	if v.Kind == vPWTypes {
		e.N = 1 // ExCount N 1: the distinct planeswalker subtypes, not the count
	}
	if v.Kind == vThisTurnEntered {
		e.N = int16(v.Zone)
		e.From = v.From
		e.Prop = v.Prop
		zone := kop.ZoneNone
		if kop.Zone(v.Zone) == kop.ZoneBattlefield {
			zone = kop.ZoneBattlefield
		}
		if v.F != nil {
			if e.F, err = cb.filterDepartures(v.F, zone); err != nil {
				return e, err
			}
		}
		if v.F2 != nil {
			if e.F2, err = cb.filterDepartures(v.F2, kop.ZoneNone); err != nil {
				return e, err
			}
		}
		if v.F3 != nil {
			if e.F3, err = cb.filterDepartures(v.F3, kop.ZoneNone); err != nil {
				return e, err
			}
		}
		if v.F4 != nil {
			if e.F4, err = cb.filterMarkPermanent(v.F4); err != nil {
				return e, err
			}
		}
		return e, nil
	}
	if v.F != nil {
		zone := kop.ZoneBattlefield
		switch v.Kind {
		case vCountGraveyard:
			zone = kop.ZoneGraveyard
		case vCountHand:
			zone = kop.ZoneHand
			if v.N == 1 {
				zone = kop.ZoneLibrary // Count$ValidLibrary
			}
		case vCountExile, vExileTypes:
			zone = kop.ZoneExile
		case vThisTurnCastSpec, vRefCount, vPlExtreme:
			zone = kop.ZoneNone // the object wherever it now lives
		}
		if v.Kind == vLeftZone {
			e.F, err = cb.filterDepartures(v.F, kop.ZoneNone)
		} else {
			e.F, err = cb.filter(v.F, zone)
		}
		if err != nil {
			return e, err
		}
	}
	if v.Ref.Kind != rNone {
		if e.Ref, err = cb.ref(v.Ref); err != nil {
			return e, err
		}
	}
	if v.Kind == vCountersOn && v.S == "ALL" {
		e.Ctr = kop.CtrAll // Count$CardCounters.ALL: every kind summed (kernel refCounters)
	} else if v.S != "" && v.Kind != vRememberedCount { // vRememberedCount's S is the ctx marker (value.go), not a counter kind
		if e.Ctr, err = counterKind(v.S); err != nil {
			return e, err
		}
	}
	if len(v.Sub) > 2 {
		return e, fmt.Errorf("value with %d operands", len(v.Sub))
	}
	for i, s := range v.Sub {
		if e.Sub[i], err = cb.value(s); err != nil {
			return e, err
		}
	}
	return e, nil
}

var cmpOps = map[uint8]kop.CmpOp{opGE: kop.CmpGE, opLE: kop.CmpLE, opEQ: kop.CmpEQ, opGT: kop.CmpGT, opLT: kop.CmpLT, opNE: kop.CmpNE}

var condKinds = map[condKind]kop.CondKind{cPresent: kop.CondPresent, cValue: kop.CondValue, cKicked: kop.CondKicked,
	cThreshold: kop.CondThreshold, cYourTurn: kop.CondYourTurn, cRefMatches: kop.CondRefMatches,
	cPromisedGift: kop.CondPromised, cAnd: kop.CondAnd, cEchoOwed: kop.CondEchoOwed}

// cond interns a condition (0 for none).
func (cb *cardBuilder) cond(c lCond) (kop.CondID, error) {
	if c.Kind == cNone {
		return 0, nil
	}
	k, ok := condKinds[c.Kind]
	if !ok {
		return 0, fmt.Errorf("condition %s", condNames[c.Kind])
	}
	kc := kop.Cond{Kind: k, Op: cmpOps[c.Op], Neg: c.Neg}
	var err error
	if c.Kind == cAnd {
		if len(c.Sub) != 2 {
			return 0, fmt.Errorf("conjunction with %d operands", len(c.Sub))
		}
		for i, sub := range c.Sub {
			if kc.Sub[i], err = cb.cond(sub); err != nil {
				return 0, err
			}
		}
	} else {
		if c.F != nil {
			zone := kop.ZoneBattlefield
			if filterReadsSpellProv(c.F) {
				zone = kop.ZoneStack // the resolving spell itself
			}
			if kc.F, err = cb.filter(c.F, zone); err != nil {
				return 0, err
			}
		}
		if c.Ref.Kind != rNone {
			if kc.Ref, err = cb.ref(c.Ref); err != nil {
				return 0, err
			}
		}
		if c.A.Kind == vCtrAdded {
			if c.Op != opGE || !c.B.isConst() || c.B.N != 1 {
				return 0, refuse("Count$CountersAddedThisTurn compared other than >= 1")
			}
			if c.A.Mul != 0 || c.A.Div != 0 || c.A.Add != 0 || c.A.Neg {
				return 0, refuse("Count$CountersAddedThisTurn with a modifier (the layout is a 0/1 presence flag, not an amount)")
			}
			cb.ctrGate = true
		}
		kc.A, err = cb.value(c.A)
		cb.ctrGate = false
		if err != nil {
			return 0, err
		}
		if kc.B, err = cb.value(c.B); err != nil {
			return 0, err
		}
	}
	key := fmt.Sprintf("%#v", kc)
	if id, ok := cb.pt.condKey[key]; ok {
		return id, nil
	}
	cb.pt.conds = append(cb.pt.conds, kc)
	id := kop.CondID(len(cb.pt.conds) - 1)
	cb.pt.condKey[key] = id
	return id, nil
}

// override interns an override record and returns its index.
func (cb *cardBuilder) override(o *lOverride) (int, error) {
	var ko kop.Override
	var err error
	ko.HasPT, ko.SetP, ko.SetT = o.HasSetPT, int8(o.SetP), int8(o.SetT)
	if o.PVal.Kind != vNone || o.TVal.Kind != vNone {
		ko.ValPT, ko.LivePT = !o.LivePT, o.LivePT // a live record keeps PA/TA, not a resolution-time value
		if ko.PA, err = cb.value(o.PVal); err != nil {
			return 0, err
		}
		if ko.TA, err = cb.value(o.TVal); err != nil {
			return 0, err
		}
		cb.out.ovVals = append(cb.out.ovVals, ko.PA, ko.TA)
	}
	ko.HasTypes, ko.SetTypes = o.HasSetTypes, typeMask(o.SetTypes)
	// an Equipment subtype the override adds or sets is the kernel's
	// TEquipment too (The Irencrag, Domesticated Watercourse)
	ko.AddTypes = typeMask(o.AddTypes) | subtypeTypeBits(o.AddSubtypes)
	if o.HasSetSubtypes && o.HasSetTypes {
		ko.SetTypes |= subtypeTypeBits(o.SetSubtypes)
	} else if o.HasSetSubtypes {
		ko.AddTypes |= subtypeTypeBits(o.SetSubtypes)
	}
	ko.HasSub = o.HasSetSubtypes
	ko.AllCreatureTypes = o.AllCreatureTypes
	ko.RemoveCreatureSub = o.RemoveCreatureSub
	ko.HasColors, ko.SetColors = o.HasSetColors, colorMask(o.SetColors)
	ko.LoseAbilities = o.LoseAbilities
	ko.KeepP, ko.KeepT = o.KeepP, o.KeepT
	ko.WhileHost, ko.CantAttack = o.WhileHost, o.CantAttack
	ko.WhileTapped = o.WhileTapped
	ko.NoCost = o.NoCost
	ko.RemoveLegendary = o.RemoveLegendary
	ko.GrantStatics = o.GrantStatics
	ko.PTDelta = o.PTDelta
	ko.AddColors = colorMask(o.AddColors)
	if ko.AddKw, err = kwMask(o.AddKw); err != nil {
		return 0, err
	}
	if ko.RemoveKw, err = kwMask(o.RemoveKw); err != nil {
		return 0, err
	}
	if len(o.GrantAbility) > 2 {
		return 0, fmt.Errorf("override grants %d abilities", len(o.GrantAbility))
	}
	for i, g := range o.GrantAbility {
		j, ok := cb.abMap[g]
		if !ok {
			return 0, fmt.Errorf("override grants ability %d which was not emitted", g)
		}
		ko.Grant[i] = uint8(j + 1)
	}
	if o.Name != "" {
		ko.NameCard = uint16(cb.pt.tokenIdx[nameCardKey+o.Name] + 1)
	}
	set, err := cb.subIndexes(o.SetSubtypes)
	if err != nil {
		return 0, err
	}
	add, err := cb.subIndexes(o.AddSubtypes)
	if err != nil {
		return 0, err
	}
	var keep []uint8
	for i, w := range cb.pt.subtypes {
		if o.Kept.Has(w) {
			keep = append(keep, uint8(i))
		}
	}
	if o.Kept.Has("Aura") {
		ko.KeepTypes |= kop.TAura
	}
	if o.Kept.Has("Equipment") {
		ko.KeepTypes |= kop.TEquipment
	}
	if o.Kept.Has("Vehicle") {
		ko.KeepTypes |= kop.TVehicle
	}
	// A dedup override (Earthbend's fixed 0/0 Creature + haste) is identical
	// for every carrier, so share one pool record instead of one per card.
	if o.Dedup {
		for i := range cb.pt.overrides {
			if cb.pt.overrides[i] == ko &&
				slices.Equal(cb.pt.ovSubs[i].set, set) &&
				slices.Equal(cb.pt.ovSubs[i].add, add) &&
				slices.Equal(cb.pt.ovSubs[i].keep, keep) {
				return i, nil
			}
		}
	}
	cb.pt.overrides = append(cb.pt.overrides, ko)
	cb.pt.ovSubs = append(cb.pt.ovSubs, ovSub{set: set, add: add, keep: keep})
	return len(cb.pt.overrides) - 1, nil
}

// chosenColorOverridesOK asserts the contract: the
// five records from first are consecutive, identical apart from SetColors,
// and W, U, B, R, G in that order, so the kernel's record pick
// C + res.chosen - 1 names the chosen colour's record.
func (cb *cardBuilder) chosenColorOverridesOK(first int) error {
	if first+5 > len(cb.pt.overrides) {
		return fmt.Errorf("chosen-colour override: %d records from %d", len(cb.pt.overrides)-first, first)
	}
	base := cb.pt.overrides[first]
	base.SetColors = 0
	want := [5]kop.ColorMask{kop.ColW, kop.ColU, kop.ColB, kop.ColR, kop.ColG}
	for i, col := range want {
		o := cb.pt.overrides[first+i]
		cmp := o
		cmp.SetColors = 0
		if cmp != base {
			return fmt.Errorf("chosen-colour override %d differs beyond SetColors", first+i)
		}
		if o.SetColors != col {
			return fmt.Errorf("chosen-colour override %d has SetColors %#x, want %#x", first+i, o.SetColors, col)
		}
	}
	return nil
}

func init() {
	_ = strings.TrimSpace
}

// changelingSubs adds to a Changeling card's printed subtypes every pool
// subtype that is a creature type (gorge's effects.CreatureTypeWordList,
// CR 702.73a). The matchup keeps the rules-relevant ones, so the card
// matches every creature type a filter, grant or choice in its matchup
// names. Not covered: a chosen creature type outside the matchup's subtypes
// (TypeVocab 255), which only an all-creature-types override matches.
func changelingSubs(pt *poolTables, subs []uint8) []uint8 {
	have := map[uint8]bool{}
	for _, s := range subs {
		have[s] = true
	}
	for _, w := range effects.CreatureTypeWordList() {
		if i, ok := pt.subIdx[w]; ok && !have[uint8(i)] {
			have[uint8(i)] = true
			subs = append(subs, uint8(i))
		}
	}
	sort.Slice(subs, func(a, b int) bool { return subs[a] < subs[b] })
	return subs
}

// putsTimeCounters reports whether d's programs put time counters
// (kop.CtrTime) on something other than the card itself (buildCard's
// Impending guard). The kernel's impending dormancy reads a time counter on a
// permanent whose card is printed with Impending, so a card without Impending
// putting them on itself (a time-counter clock: Cursed Recording) can never
// make anything dormant, while a counter placed on another permanent could
// land on an Impending card cast for its full cost and wrongly silence it.
func putsTimeCounters(d *kop.CardDef) bool {
	for i := range d.Abilities {
		for _, in := range d.Abilities[i].Prog {
			switch in.Op {
			case kop.OpPutCounter, kop.OpPutCounterAll, kop.OpEnterWithCounters, kop.OpDoubleCounters, kop.OpDigCounters:
				if kop.CounterKind(in.C) == kop.CtrTime && !(in.Op == kop.OpPutCounter && in.Obj == kop.RefSelf) {
					return true
				}
			case kop.OpMoveCounter, kop.OpReturnEx:
				if in.B.K == kop.VConst && kop.CounterKind(in.B.N) == kop.CtrTime {
					return true
				}
			case kop.OpExt:
				if in.Ext() == kop.ExtSearchCounters && kop.CounterKind(in.C) == kop.CtrTime {
					return true
				}
			}
		}
	}
	return false
}
