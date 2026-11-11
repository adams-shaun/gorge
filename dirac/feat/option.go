package feat

import (
	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Option is one encoded option (policynet.Option's network inputs): Slots
// and Hashed in emission order, the OptionDenseWidth dense scalars, and the
// entity refs (1 + the State.Cards index; 0 none).
type Option struct {
	Slots   [64]Feature
	NSlots  uint8
	Hashed  [6]Feature
	NHashed uint8
	Dense   [OptionDenseWidth]float32
	EntA    int32
	EntB    int32
}

// The option-kind and decision-kind vocabularies (policynet option.go).
var optionKinds = [...]string{
	"ability", "activate", "attacker", "block", "bottom", "cast", "concede",
	"exile", "keep", "mode", "mulligan", "name", "no", "number", "pass",
	"permanent", "play_land", "player", "sacrifice", "trigger", "type", "x",
	"yes",
}

// decisionKinds is decision.Kinds' order.
var decisionKinds = [...]string{"priority", "target", "attackers", "blockers", "mulligan", "modes",
	"trigger_order", "trigger_optional", "commander_zone", "choose", "replacement", "arrange", "starting_player"}

var modeValues = [...]string{"", "kicked", "surged", "flashback", "miracle"}

// Slot layout offsets.
const (
	optKindOffset = 0
	decKindOffset = 24
	typeOffset    = 37
	colourOffset  = 49
	mvOffset      = 55
	modeOffset    = 64
	altCostOffset = 70
	kwOffset      = 71
	zoneOffset    = 103
)

// Option kinds the kernel's actions name (an index into kindNames).
const (
	okPass uint8 = iota
	okPlayLand
	okCast
	okAbility
	okAttacker
	okBlock
	okDone
	okPermanent
	okPlayer
	okSpell
	okCard
	okMode
	okKeep
	okMulligan
	okBottom
	okTrigger
	okYes
	okNo
	okX
	okSacrifice
	okDiscard
	okTapCost
	okDivision
	okGraveyard
	// gorge's KChoose / KModes kinds of the generated tables' asks
	okSearch
	okHiddenPick
	okHandMove
	okDig
	okTwoPiles
	okPileA
	okPileB
	okPrimary
	okTriggerCostPay
	okTriggerCostDecline
	okColor
	okType
	okOpeningYes
	okOpeningNo
	okNumber
	okDigBottom
	okAltAddCost
	okReplacement // gorge's KReplacement order ask (kernel ChGenReplOrder)
	okPayW        // gorge's hybrid pip announcement pay_W..pay_G (kernel ChGenPip)
	okPayU
	okPayB
	okPayR
	okPayG
	okEnchant     // gorge's CR 303.4f "enchant" ask (kernel ChGenEnchant)
	okSubCounter  // gorge's "subcounter" cost ask (kernel ChCostSubCtr, Quilled Greatwurm)
	okDivide      // gorge's CR 601.2d "divide" ask (kernel ChCastDivide)
	okReveal      // gorge's reveal_pick "reveal" (kernel ChGenReveal)
	okTap         // gorge's taporuntap "tap" (kernel ChGenTapOrUntap)
	okUntap       // gorge's taporuntap "untap"
	okTop         // gorge's explore "top" (kernel ChGenExplore)
	okCounterPick // gorge's bolster "counter_pick" (kernel ChGenBolster)
	okProliferate // gorge's "proliferate" pick (kernel ChGenProlif)
	okPayGeneric  // gorge's twobrid generic face pay_generic (kernel ChGenPip mode 6)
	okCommandZone // gorge's KCommanderZone "command_zone" (kernel ChCommanderZone mode 0)
	okLeave       // gorge's KCommanderZone "leave" (kernel ChCommanderZone mode 1)
	numOK
)

var kindNames = [numOK]string{"pass", "play_land", "cast", "ability", "attacker", "block", "done", "permanent",
	"player", "spell", "card", "mode", "keep", "mulligan", "bottom", "trigger", "yes", "no", "x", "sacrifice",
	"discard", "tapcost", "division", "graveyard",
	"search", "hidden_pick", "hand_move", "dig", "twopiles", "pile-a", "pile-b", "primary",
	"trigger_cost_pay", "trigger_cost_decline", "color", "type", "opening_yes", "opening_no", "number", "dig_bottom", "altaddcost", "replacement",
	"pay_W", "pay_U", "pay_B", "pay_R", "pay_G", "enchant", "subcounter", "divide", "reveal", "tap", "untap", "top", "counter_pick", "proliferate", "pay_generic",
	"command_zone", "leave"}

// decision kinds (indices into decisionKinds), -1 = other.
const (
	dkPriority        = 0
	dkTarget          = 1
	dkAttackers       = 2
	dkBlockers        = 3
	dkMulligan        = 4
	dkModes           = 5
	dkTriggerOrder    = 6
	dkTriggerOptional = 7
	dkCommanderZone   = 8
	dkChoose          = 9
	dkReplacement     = 10
	dkArrange         = 11
)

// kindInfo is one option kind's precomputed encoding.
type kindInfo struct {
	slot int    // optionKinds index, or len(optionKinds) for other
	row  uint16 // "optkind|<kind>"
}

var kinds = func() (k [numOK]kindInfo) {
	for i, n := range kindNames {
		k[i].slot = len(optionKinds)
		for j, o := range optionKinds {
			if o == n {
				k[i].slot = j
			}
		}
		k[i].row = row(add(off, "optkind|"+n))
	}
	return
}()

var (
	rowCardPlayer     = row(add(off, "card|player"))
	rowCardPlayerNone = row(add(off, "card|player\x1fnone"))
	rowDeckindOther   = row(add(off, "mz|deckind|other"))
)

// optIn is the gorge decision.Option a kernel action stands for: the
// fields policynet's option encoder reads.
type optIn struct {
	kind   uint8
	dk     int8 // decisionKinds index
	obj    uint8
	player bool // names a player
	// named is the seat (0 or 1) of the player the option names, -1 none:
	// a player target, or an attack's defending player (policynet's
	// odSeat0.. relative-seat one-hot)
	named    int8
	attacker uint8
	battle   uint8
	mode     uint8 // modeValues index; len(modeValues) = other
	altCost  bool
	amount   int32
	required bool
	group    bool
}

// optionOf maps a kernel action of decision d onto the gorge option it
// stands for (dirac/doc.go's decision grammar).
func (e *Encoder) optionOf(d *dirac.Decision, a dirac.Action) optIn {
	o := optIn{obj: noObj, attacker: noObj, battle: noObj, named: -1}
	src, tgt, mode := a.Source(), a.Target(), a.Mode()
	if a.Kind() == dirac.AkDone {
		o.kind = okDone
	}
	switch d.Kind {
	case dirac.DkPriority:
		o.dk = dkPriority
		switch a.Kind() {
		case dirac.AkPass:
			o.kind = okPass
		case dirac.AkPlayLand:
			o.kind, o.obj = okPlayLand, src
		case dirac.AkCast, dirac.AkCastCasualty, dirac.AkCastConspire:
			o.kind, o.obj = okCast, src
			switch {
			case a.Kind() == dirac.AkCastCasualty || a.Kind() == dirac.AkCastConspire:
				o.mode = uint8(len(modeValues)) // gorge's "casualty" (the mode-other slot)
			case mode&1 != 0 && e.st0.FeatOffspringCast(dirac.Obj(src)):
				o.mode = uint8(len(modeValues)) // gorge's "offspring" (the mode-other slot)
			case mode&1 != 0:
				o.mode = 1
			case mode&2 != 0:
				o.mode = 3
			case mode&dirac.FeatCastMayPlay != 0:
				// gorge's "mayplay" mode (the mode-other slot), except on
				// the synthetic cast its planner lists for an untyped
				// permission, which names no mode (not a scripted one: a
				// lowered potential play, pair 7489's Hawk)
				st := e.st0
				if st.FeatMayPlayUntyped(d.Player, dirac.Obj(src)) && st.FeatPlannedCast(d.Player, dirac.Obj(src), mode) &&
					!st.FeatPoolPays(d.Player, dirac.Obj(src), mode) && !st.FeatScripted(d.Player, a) {
					// a cost-waiving permission (CR 118.9) turns the printed
					// arm into gorge's alternative-cost arm
					o.altCost = mode&dirac.FeatCastFree != 0
				} else {
					o.mode = uint8(len(modeValues))
				}
			}
		case dirac.AkActivate:
			o.kind, o.obj = okAbility, src
		}
	case dirac.DkAttackers:
		o.dk = dkAttackers
		if a.Kind() == dirac.AkAttack {
			o.kind, o.obj = okAttacker, src
			// gorge's attacker option names the defending player (Option.
			// Player): with two seats, always the attacker's opponent --
			// a planeswalker's controller or a battle's protector included
			o.named = int8(1 - d.Player&1)
			// gorge marks every pair of a creature that attacks each
			// combat if able Required (rules mustAttackRequired)
			o.required = e.st0.MustAttack(dirac.Obj(src))
			if dirac.IsObjTarget(tgt) {
				o.battle = tgt
			}
		}
	case dirac.DkBlockers:
		o.dk = dkBlockers
		if a.Kind() == dirac.AkBlock {
			o.kind, o.obj, o.attacker, o.group = okBlock, src, tgt, true
		}
	case dirac.DkTarget:
		o.dk = dkTarget
		o.group = e.st0.FeatTargetGroup()
		switch {
		case a.Kind() == dirac.AkDone || tgt == dirac.NoTarget:
			o.kind = okDone // closing a multi-object slot / declining "up to one"
		case tgt == dirac.PlayerTarget0 || tgt == dirac.PlayerTarget1:
			o.kind, o.player = okPlayer, true
			o.named = int8(tgt - dirac.PlayerTarget0)
		default:
			// gorge names every object target "permanent" except one on
			// the stack ("spell").
			o.obj = tgt
			o.kind = okPermanent
			if int(tgt) < int(e.kp.NObj) && e.po(tgt).Zone == kop.ZoneStack {
				o.kind = okSpell
			}
		}
	case dirac.DkModes:
		o.dk, o.obj = dkModes, e.source
		if a.Kind() != dirac.AkDone { // a multi-pick charm's Done keeps okDone
			o.kind = okMode
		}
	case dirac.DkTriggerOrder:
		// AkOrder packs source = the trigger's source object, target = pending index.
		o.dk, o.kind, o.obj = dkTriggerOrder, okTrigger, src
	case dirac.DkMulligan:
		o.dk = dkMulligan
		if a.Kind() == dirac.AkKeep {
			o.kind = okKeep
		} else if a.Kind() == dirac.AkMulligan {
			o.kind = okMulligan
		}
	case dirac.DkChoose:
		o.dk = dkChoose
		if a.Kind() == dirac.AkDone {
			if d.Sub == dirac.ChArrange {
				o.dk = dkArrange
			}
			if dk, ok := genDoneDK(d.Sub); ok {
				o.dk = dk
			}
			break
		}
		o.obj = src
		switch d.Sub {
		case dirac.ChX, dirac.ChTrigX, dirac.ChMultikick:
			o.kind, o.obj, o.amount = okX, noObj, int32(mode)
		case dirac.ChCostDiscard, dirac.ChCleanupDiscard:
			o.kind = okDiscard
		case dirac.ChCostSacrifice, dirac.ChSacrifice:
			o.kind = okSacrifice
		case dirac.ChCostTap:
			o.kind = okTapCost
		case dirac.ChCostSubCtr:
			o.kind = okSubCounter
		case dirac.ChBottom:
			o.dk, o.kind = dkMulligan, okBottom
		case dirac.ChDiscard:
			o.dk, o.kind = dkModes, okDiscard
		case dirac.ChArrange:
			o.dk, o.kind = dkArrange, okBottom
			if e.st0.FeatArrangeGraveyard() {
				o.kind = okGraveyard
			} else if e.st0.FeatArrangeDig() {
				o.kind = okDigBottom
			}
		case dirac.ChWardPay:
			// gorge poses a ward cost as modes [pay, decline] (unless_pay /
			// unless_decline: the mode-other slot)
			o.dk, o.kind, o.obj, o.mode = dkModes, okMode, noObj, uint8(len(modeValues))
		case dirac.ChOptional:
			o.dk, o.obj = dkTriggerOptional, e.source
			o.kind = okNo
			if mode == 1 {
				o.kind = okYes
			}
		case dirac.ChOptDiscMay:
			// gorge's may-election: KChoose kinds "yes"/"no", no object
			o.kind, o.obj = okNo, noObj
			if mode == 1 {
				o.kind = okYes
			}
		case dirac.ChLegend:
			o.kind = okKeep
		case dirac.ChCommanderZone:
			// CR 903.9a/b: gorge's KCommanderZone options command_zone / leave
			o.dk, o.kind = dkCommanderZone, okCommandZone
			if mode == 1 {
				o.kind = okLeave
			}
		case dirac.ChDivide:
			o.kind = okDivision
			o.amount = e.firstShare(src, int(mode))
		case dirac.ChAltAdd:
			// gorge's "altaddcost" option: Amount is the part's index in
			// the card's either-or list (0 the spell's own, 1 the
			// alternative); it names no object
			o.kind, o.obj, o.amount = okAltAddCost, noObj, int32(tgt)
		default:
			e.genOption(d, a, &o)
		}
	}
	return o
}

// firstShare is a damage split's first blocker share (gorge's division
// option Amount): split idx of the attacker's power among its blockers.
func (e *Encoder) firstShare(atk uint8, idx int) int32 {
	if int(atk) < int(e.kp.NObj) && e.po(atk).Flags&dirac.ProjBlocking != 0 {
		return int32(idx) // a two-attacker blocker's division: the first share is the option index
	}
	n := 0
	for i := 0; i < int(e.kp.NObj); i++ {
		if e.po(uint8(i)).Blocks == dirac.Obj(atk) {
			n++
		}
	}
	if n == 0 || int(atk) >= int(e.kp.NObj) {
		return 0
	}
	var out [dirac.MaxPerm]uint8 // one share per blocker
	if n > len(out) {
		return 0
	}
	p := int(e.po(atk).Power)
	if p < 0 {
		p = 0
	}
	dirac.SplitAt(p, n, idx, out[:n])
	return int32(out[0])
}

// cref is a resolved option card (policynet cardRef).
type cref struct {
	c    *vcard
	zone int // zone class 0..3, -1 for the stack
	mine bool
}

type optScratch struct {
	keys  [dirac.MaxOptions]uint32
	pos   [dirac.MaxOptions]int16
	n     int
	refs  []cref
	ent   [256]int32
	entOK bool
	gen   uint32
}

// lookup resolves obj in the card index's fixed order: my hand,
// battlefield, graveyard, exile; the opponent's battlefield, graveyard,
// exile; the stack's cards. First match wins.
func (e *Encoder) lookup(o uint8) (cref, bool) {
	if o == noObj {
		return cref{}, false
	}
	v := &e.v
	me, opp := &v.players[v.seat&1], &v.players[1-v.seat&1]
	for zi, l := range [4][]int32{me.hand, me.bf, me.grave, me.exile} {
		zone := [4]int{zHand, zBattlefield, zGraveyard, zExile}[zi]
		for _, ci := range l {
			if v.cards[ci].obj == o {
				return cref{&v.cards[ci], zone, true}, true
			}
		}
	}
	for zi, l := range [3][]int32{opp.bf, opp.grave, opp.exile} {
		zone := [3]int{zBattlefield, zGraveyard, zExile}[zi]
		for _, ci := range l {
			if v.cards[ci].obj == o {
				return cref{&v.cards[ci], zone, false}, true
			}
		}
	}
	for i := range v.stack {
		if ci := v.stack[i].cv; ci >= 0 && v.cards[ci].obj == o {
			c := &v.cards[ci]
			return cref{c, -1, c.ctrl == v.seat}, true
		}
	}
	return cref{}, false
}

// entRef is 1 + o's index in the entity list (0 = none).
func (e *Encoder) entRef(o uint8) int32 {
	if o == noObj {
		return 0
	}
	for i := range e.cards {
		_ = i
	}
	v := &e.v
	n := int32(0)
	var hit int32
	e.gen++
	if e.gen == 0 {
		e.seen = [256]uint32{}
		e.gen = 1
	}
	visit := func(ci int32) bool {
		c := &v.cards[ci]
		if e.seen[c.obj] == e.gen {
			return false
		}
		e.seen[c.obj] = e.gen
		n++
		if c.obj == o {
			hit = n
			return true
		}
		return false
	}
	me, opp := &v.players[v.seat&1], &v.players[1-v.seat&1]
	for _, l := range [3][]int32{me.bf, opp.bf, me.hand} {
		for _, ci := range l {
			if visit(ci) {
				return hit
			}
		}
	}
	for i := range v.stack {
		if ci := v.stack[i].cv; ci >= 0 && visit(ci) {
			return hit
		}
	}
	return 0
}

// EncodeOptions encodes every option of d (the decision EncodeState was
// last called for) into dst, in d's option order, and returns it. Each
// option's index/count feature is its place in gorge's net list
// (listpos.go), with no bot answer named.
func (e *Encoder) EncodeOptions(d *dirac.Decision, dst []Option) []Option {
	return e.EncodeOptionsBot(d, dst, -1)
}

// EncodeOptionsBot is EncodeOptions with bot, the index in d of the bot's
// answer (-1 none): at a priority decision a planner-paid cast with no
// legacy option is listed first among gorge's appended synthetic casts
// when it is the bot's (azmcts botFirst).
func (e *Encoder) EncodeOptionsBot(d *dirac.Decision, dst []Option, bot int) []Option {
	e.source = uint8(e.st0.FeatSource())
	n := int(d.N)
	if cap(dst) < n {
		dst = make([]Option, n)
	}
	dst = dst[:n]
	gn := e.listShape(d, bot)
	e.opt.n = gn
	for i := 0; i < n; i++ {
		o := e.optionOf(d, d.Options[i])
		if d.Kind == dirac.DkPriority && e.opt.keys[i] != noPos && e.opt.keys[i]>>24 >= secSynthBot {
			// azmcts payCastOption: the plain cast of the object, no mode
			o.mode = 0
		}
		e.encodeOption(&dst[i], o, int(e.opt.pos[i]), gn)
	}
	return dst
}

// ListPos is option i's position in gorge's net list as the last
// EncodeOptions placed it.
func (e *Encoder) ListPos(i int) int { return int(e.opt.pos[i]) }

// ListLen is the length of gorge's net list at the last EncodeOptions.
func (e *Encoder) ListLen() int { return e.opt.n }

func (e *Encoder) encodeOption(out *Option, o optIn, idx, n int) {
	*out = Option{}
	set := func(r int) {
		out.Slots[out.NSlots] = Feature{Row: uint16(r), Value: 1}
		out.NSlots++
	}
	hash := func(r uint16) {
		out.Hashed[out.NHashed] = Feature{Row: r, Value: 1}
		out.NHashed++
	}
	set(optKindOffset + kinds[o.kind].slot)
	decOther := o.dk < 0
	if decOther {
		set(decKindOffset + len(decisionKinds))
	} else {
		set(decKindOffset + int(o.dk))
	}
	ref, haveCard := e.lookup(o.obj)
	isPlayer := o.player
	var c *card
	if haveCard {
		c = ref.c.c
	}
	any := false
	if haveCard {
		for i := range cardTypeBits {
			if ref.c.fold()&(1<<i) != 0 {
				set(typeOffset + i)
				any = true
			}
		}
		set(typeOffset + 10) // CardView.Token is set on every card
		any = true
	}
	if !any {
		set(typeOffset + 11)
	}
	mv := int32(0)
	if haveCard {
		for k := 0; k < 6; k++ {
			if c.colours[k] {
				set(colourOffset + k)
			}
		}
		mv = c.mv
	}
	b := mv
	if b > 8 {
		b = 8
	}
	set(mvOffset + int(b))
	set(modeOffset + int(o.mode))
	if o.altCost {
		set(altCostOffset)
	}
	if haveCard {
		other := false
		e.eachKw(ref.c, func(k *kwInfo) {
			if k.bit >= 0 {
				set(kwOffset + int(k.bit))
			} else {
				other = true
			}
		})
		if other {
			set(kwOffset + 31)
		}
		if ref.zone >= 0 && ref.zone < 4 {
			m := 0
			if ref.mine {
				m = 1
			}
			set(zoneOffset + m*4 + ref.zone)
		}
	}
	d := &out.Dense
	if haveCard {
		rc := ref.c
		d[0] = float32(rc.power)
		d[1] = float32(rc.tough)
		d[2] = float32(rc.tough - rc.damage)
		d[3] = float32(rc.damage)
		d[4] = b2f(rc.tapped)
		d[5] = b2f(rc.sick)
		d[6] = b2f(rc.attacking)
		d[19] = float32(c.mv)
		d[20] = float32(rc.acts)
		for _, nn := range rc.ctr {
			d[21] += float32(nn)
		}
		d[22] = b2f(rc.attached != noObj)
	}
	d[7] = b2f(isPlayer)
	if o.named >= 0 {
		// the relative seat: 0 me, 1 the opponent (policynet odSeat0..)
		d[24+int((o.named-int8(e.v.seat&1))&1)] = 1
	}
	var aref cref
	haveAtk := false
	if o.attacker != noObj {
		aref, haveAtk = e.lookup(o.attacker)
		if haveAtk {
			d[8] = float32(aref.c.power)
			d[9] = float32(aref.c.tough)
			d[10] = float32(aref.c.damage)
			if haveCard {
				d[11] = float32(ref.c.tough - aref.c.power)
			}
		}
	}
	if mv > 0 {
		avail := float32(e.v.players[e.v.seat&1].avail)
		d[12] = float32(mv)
		d[13] = float32(mv) - avail
		if float32(mv) > avail {
			d[14] = 1
		}
	}
	d[15] = float32(o.amount)
	d[16] = b2f(o.altCost)
	d[17] = b2f(o.required)
	if n > 1 {
		d[18] = float32(idx) / float32(n-1)
	}
	d[23] = b2f(o.group)
	// Hashed ids.
	if isPlayer {
		hash(rowCardPlayer)
		hash(rowCardPlayerNone)
	} else if haveCard {
		hash(c.optCard)
		if o.attacker != noObj {
			h := add(addB(add(off, "card|"), c.name), "\x1f")
			if haveAtk {
				h = addB(add(h, "atk|"), aref.c.c.name)
			} else {
				h = add(h, "atk|unknown")
			}
			hash(row(h))
		} else {
			hash(c.optNone)
		}
	}
	hash(kinds[o.kind].row)
	if decOther {
		hash(rowDeckindOther)
	}
	if haveCard && len(c.api) > 0 {
		hash(c.optAPI)
	}
	out.EntA = e.entRef(o.obj)
	rel := o.attacker
	if rel == noObj {
		rel = o.battle
	}
	out.EntB = e.entRef(rel)
}

func b2f(b bool) float32 {
	if b {
		return 1
	}
	return 0
}
