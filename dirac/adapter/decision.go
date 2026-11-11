package adapter

import (
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/canon"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/rules/cost"
	"github.com/adams-shaun/gorge/engine/state"
)

// pose builds the native decision for the kernel's current step: its options, in
// the order gorge enumerates them, folded to one whole answer.
func (g *Game) pose() {
	g.pend = &decision.Decision{Seq: uint64(len(g.log.Events)), Player: state.PlayerID(g.d.Player), Min: 1, Max: 1}
	g.acts, g.canons, g.paid = g.acts[:0], g.canons[:0], g.paid[:0]
	g.multi = false
	if g.d.Kind == dirac.DkPriority {
		g.posePriority()
		return
	}
	g.poseAsk()
}

// entry is one priority play before it is sorted into gorge's order.
type entry struct {
	act dirac.Action
	c   canon.Canon
	key uint64
	opt decision.Option
	// plan: gorge would pay this cast through its planner rather than the pool.
	plan bool
}

func (g *Game) posePriority() {
	g.targets = targetSlot{} // a later ability from one source asks from its first target again
	p := g.d.Player
	d := g.pend
	d.Kind = decision.KPriority
	var es []entry
	var pass dirac.Action
	hasPass := false
	for i := 0; i < int(g.d.N); i++ {
		x := g.d.Options[i]
		if x.Kind() == dirac.AkPass {
			pass, hasPass = x, true
			continue
		}
		es = append(es, g.priorityEntry(p, x))
	}
	for i := 1; i < len(es); i++ {
		for j := i; j > 0 && es[j].key < es[j-1].key; j-- {
			es[j], es[j-1] = es[j-1], es[j]
		}
	}
	for i := range es {
		e := &es[i]
		if e.plan && g.addPaid(e) {
			continue
		}
		e.opt.Index = len(d.Options)
		d.Options = append(d.Options, e.opt)
		g.acts = append(g.acts, e.act)
		g.canons = append(g.canons, e.c)
	}
	if hasPass {
		d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "pass", Label: "Pass"})
		g.acts = append(g.acts, pass)
		g.canons = append(g.canons, g.tr.Action(pass))
	}
	d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "concede", Label: "Concede"})
	g.acts = append(g.acts, 0)
	g.canons = append(g.canons, canon.Pack(canon.CanonKind(0xFF), canon.RefNone, canon.RefNone, 0))
	d.PaymentActionsBuilt = true
	d.PotentialCastsKnown = true
}

// priorityEntry reads one kernel play as a native option.
func (g *Game) priorityEntry(p uint8, x dirac.Action) entry {
	c := g.tr.Action(x)
	src, mode := x.Source(), x.Mode()
	po := &g.kp.Objs[src]
	zidx := uint64(po.ZoneIdx)
	e := entry{act: x, c: c, opt: decision.Option{Obj: g.ids.obj(src)}}
	e.opt.Label = g.cardName(src)
	switch c.Kind() {
	case canon.ActPlayLand:
		e.opt.Kind = "play_land"
		if c.Mode() == 1 {
			e.opt.Mode = "modal_land"
		}
		e.opt.LandWontStay = g.landWontStay(p, g.face(po.Card))
		e.key = zidx << 8
	case canon.ActActivate:
		e.opt.Kind, e.opt.Ability = "ability", int(c.Mode())
		if x.Mode()&0x80 != 0 { // a granted ability has no index of its own (rules' Option.SVar)
			e.opt.Ability = 0
			if name, sa := g.grantedSA(g.kp.Objs[x.Target()].Card, x.Mode()&^0x80); sa != nil {
				e.opt.SVar, e.opt.GrantSource = name, g.ids.obj(x.Target())
				e.opt.Cost = offerCost(sa)
			}
		} else if sa := g.faceSA(po.Card, c.Mode()); sa != nil {
			e.opt.Cost = offerCost(sa)
			e.opt.Loyalty = loyaltyOf(sa, e.opt.Cost)
		}
		e.key = 3<<40 | abilityZoneKey(po)<<36 | abilitySeat(po)<<32 | zidx<<8 | uint64(mode)
	case canon.ActSpecial:
		e.opt.Kind = canon.SpecialKinds[c.Mode()&15]
		e.opt.Mode = specialMode(e.opt.Kind, c.Mode()>>4)
		e.key = 4<<40 | zidx<<8 | uint64(mode)
	default:
		e.opt.Kind, e.opt.Mode = "cast", canon.CastModeOf(c)
		e.opt.AltCostIndex = canon.CastAlt(c)
		e.key, e.plan = g.castKey(p, x, po, zidx)
	}
	return e
}

// castKey is a cast's place in gorge's enumeration, and whether it goes through the
// payment extension: every cast the floating pool cannot pay and that has a cost.
func (g *Game) castKey(p uint8, x dirac.Action, po *dirac.ProjObj, zidx uint64) (uint64, bool) {
	src, mode := x.Source(), x.Mode()
	o := dirac.Obj(src)
	fromHand := po.Zone == kop.ZoneHand
	if g.s.FeatPoolPays(p, o, mode) || g.free(po, mode) {
		if mode&2 != 0 {
			return 1<<40 | zidx<<8, false
		}
		sub := uint64(0)
		if !fromHand {
			sub = 2
		}
		return sub<<32 | zidx<<8 | 1 + uint64(mode&1), false
	}
	sub := uint64(0)
	if mode&2 != 0 {
		sub = 3
	} else if !fromHand {
		sub = 2
	}
	return 7<<40 | sub<<32 | zidx<<8 | uint64(mode), true
}

// free reports a cast whose mana cost is nothing: there is nothing to plan.
func (g *Game) free(po *dirac.ProjObj, mode uint8) bool {
	if int(po.Card) >= len(g.tbl.Cards) || mode&1 != 0 {
		return false
	}
	cd := &g.tbl.Cards[po.Card]
	return cd.Cost.MV() == 0 && cd.Cost.X == 0 && cd.Types&kop.TLand == 0 && mode&2 == 0
}

// addPaid offers a cast as a payment action with one plan. A second cast of the
// same object and origin, such as its kicked cast, has no identity of its own there,
// and stays an ordinary option; it reports false.
func (g *Game) addPaid(e *entry) bool {
	d := g.pend
	cast := decision.PlannedCast{Object: e.opt.Obj, Origin: castOrigin(g.kp.Objs[e.act.Source()].Zone, e.opt.Mode)}
	for i := range d.PaymentActions {
		if d.PaymentActions[i].Cast == cast {
			return false
		}
	}
	plan := decision.PaymentPlan{Version: decision.PaymentPlanV1, Activations: g.tapsOf(e.act)}
	id, err := decision.PaymentPlanID(d.Seq, d.Player, cast, plan)
	if err != nil {
		return false
	}
	plan.ID = id
	aid, err := decision.PaymentActionID(decision.PaymentPlanV1, d.Seq, d.Player, cast)
	if err != nil {
		return false
	}
	d.PaymentActions = append(d.PaymentActions, decision.PaymentAction{ID: aid, Cast: cast, Label: e.opt.Label, Plans: []decision.PaymentPlan{plan}})
	g.paid = append(g.paid, e.act)
	return true
}

func castOrigin(z kop.Zone, mode string) string {
	switch {
	case mode == "flashback":
		return decision.OriginFlashback
	case z == kop.ZoneExile:
		return "exile"
	case z == kop.ZoneGraveyard:
		return "graveyard"
	}
	return "hand"
}

// tapsOf is what the kernel taps as it applies a cast, as a plan's activations. A
// cast with targets or modes still to choose has tapped nothing yet.
func (g *Game) tapsOf(a dirac.Action) []decision.PaymentActivation {
	g.scratch()
	g.ws.Apply(a)
	g.ws.Project(&g.wkp)
	out := []decision.PaymentActivation{}
	for o := 0; o < int(g.kp.NObj) && len(out) < decision.MaxPaymentActivations; o++ {
		if g.wkp.Objs[o].Flags&^g.kp.Objs[o].Flags&dirac.ProjTapped != 0 {
			out = append(out, decision.PaymentActivation{Source: g.ids.obj(uint8(o))})
		}
	}
	return out
}

func abilityZoneKey(po *dirac.ProjObj) uint64 {
	switch po.Zone {
	case kop.ZoneBattlefield:
		return 0
	case kop.ZoneGraveyard:
		return 2
	case kop.ZoneExile:
		return 3
	case kop.ZoneHand:
		return 4
	}
	return 5
}

func abilitySeat(po *dirac.ProjObj) uint64 {
	if po.Zone == kop.ZoneBattlefield {
		return uint64(po.Ctrl)
	}
	return uint64(po.Owner)
}

// specialMode spells a special action's sub-mode as gorge's Option.Mode.
func specialMode(kind string, sub uint8) string {
	switch kind {
	case "unlock":
		if sub > 0 {
			return strconv.Itoa(int(sub) - 1)
		}
	case "turn_face_up":
		return [...]string{"", "manifest", "cloak", "morph", "megamorph", "disguise", "", ""}[sub&7]
	}
	return ""
}

// poseAsk builds every decision but priority from the canonical form of each kernel
// option, read back as the gorge option it stands for.
func (g *Game) poseAsk() {
	d := g.pend
	dk := g.tr.DKind()
	type opt struct {
		o   decision.Option
		act dirac.Action
		c   canon.Canon
		key uint32
	}
	var os []opt
	hasDone, unlessPay := false, false
	for i := 0; i < int(g.d.N); i++ {
		x := g.d.Options[i]
		c := g.tr.Action(x)
		if c.Kind() == canon.ActDone {
			hasDone = true
			continue
		}
		o, key, unless := g.askOption(x, c, uint32(i))
		unlessPay = unlessPay || unless
		os = append(os, opt{o: o, act: x, c: c, key: key})
	}
	for i := 1; i < len(os); i++ {
		for j := i; j > 0 && os[j].key < os[j-1].key; j-- {
			os[j], os[j-1] = os[j-1], os[j]
		}
	}
	for i := range os {
		os[i].o.Index = i
		d.Options = append(d.Options, os[i].o)
		g.acts = append(g.acts, os[i].act)
		g.canons = append(g.canons, os[i].c)
	}
	n := len(d.Options)
	switch dk {
	case canon.DAttackers:
		d.Kind, d.Min, d.Max, d.Prompt = decision.KAttackers, 0, n, "declare attackers"
	case canon.DBlockers:
		d.Kind, d.Min, d.Max, d.Prompt = decision.KBlockers, 0, n, "declare blockers"
	case canon.DTarget:
		d.Kind, d.Prompt = decision.KTarget, "Choose a target"
		g.fillTargetEffect(d)
	case canon.DModes:
		d.Kind, d.Prompt = decision.KModes, "Choose"
		g.labelModes(d)
		if unlessPay {
			d.ResumeKind, d.ResumeSA, d.Prompt = "unless_pay", unlessSA, "Pay the cost, or decline"
		}
		if n > 0 && d.Options[0].Kind == "discard" {
			d.Prompt = "Choose a card to discard"
		}
	case canon.DTriggerOrder:
		d.Kind, d.Min, d.Max, d.Prompt = decision.KTriggerOrder, n, n, "Order your triggers"
	case canon.DTriggerOptional:
		d.Kind, d.Prompt = decision.KTriggerOptional, "Use this trigger?"
	case canon.DArrange:
		d.Kind, d.Prompt = decision.KArrange, "Arrange"
		d.Min, d.Max = 0, n
		kind := g.arrangeKind()
		if kind == "dig_bottom" {
			d.Min = n
		} else {
			d.Restable = true // a scry or surveil's other pile has an order of its own
		}
		for i := range d.Options {
			d.Options[i].Kind = kind
		}
	case canon.DReplacement:
		d.Kind, d.Prompt = decision.KReplacement, "Choose a replacement to apply"
	case canon.DCommanderZone:
		d.Kind, d.Prompt = decision.KCommanderZone, "Put this commander into the command zone?"
	case canon.DMulligan:
		d.Kind, d.Prompt = decision.KMulligan, "Mulligan?"
	default:
		d.Kind, d.Prompt = decision.KChoose, "Choose"
		if n > 0 {
			switch d.Options[0].Kind {
			case "discard":
				d.Prompt = "Discard a card"
			case "sacrifice":
				d.Prompt = "Choose 1 permanent(s) to sacrifice"
				if g.d.Sub == dirac.ChCostSacrifice {
					d.Prompt = "Sacrifice a permanent to pay the cost"
				}
			}
		}
	}
	switch {
	case d.Kind == decision.KChoose && g.d.Sub == dirac.ChCastDivide:
		d.Min = g.s.FeatCastDivideLeft()
		d.Max, d.Repeatable = d.Min, true
	case d.Kind == decision.KChoose && g.d.Sub == dirac.ChGenDivide:
		d.Min = g.s.PicksLeft()
		d.Max, d.Repeatable = d.Min, true
	case d.Kind == decision.KChoose && g.d.Sub == dirac.ChGenPip:
		// each symbol is its own ask
	case d.Kind == decision.KChoose, d.Kind == decision.KModes, d.Kind == decision.KTarget:
		d.Min, d.Max = g.span()
	}
	if lo, hi, ok := g.s.FeatTargetBounds(); ok && d.Kind == decision.KTarget {
		d.Min, d.Max = lo, hi
	}
	if hasDone {
		d.Min = 0
	}
	if d.Min > n && !d.Repeatable {
		d.Min = n
	}
	if d.Kind != decision.KTarget && g.d.Sub != dirac.ChWardPay {
		d.Source = g.askSource(d)
	}
	if d.Kind == decision.KChoose && n > 0 && d.Options[0].Kind == "dig" {
		d.ResumeSA = g.resolvingSA("Dig")
	}
	g.multi = d.Min != 1 || d.Max != 1
}

// askSource is the object a decision other than a target's belongs to.
func (g *Game) askSource(d *decision.Decision) state.ObjID {
	if d.Kind == decision.KCommanderZone && len(d.Options) > 0 {
		return d.Options[0].Obj // the commander its owner is asked about (CR 903.9a)
	}
	if g.d.Sub == dirac.ChDivide && len(d.Options) > 0 {
		return d.Options[0].Obj
	}
	src := g.s.FeatEnterSource()
	if src == dirac.NoObj {
		src = g.s.FeatSource()
	}
	if src == dirac.NoObj {
		return 0
	}
	return g.ids.obj(uint8(src))
}

// arrangeKind names where a scry or surveil's unchosen cards go, which every option
// of the ask shares.
func (g *Game) arrangeKind() string {
	switch {
	case g.s.FeatArrangeGraveyard():
		return "graveyard"
	case g.s.FeatArrangeDig():
		return "dig_bottom"
	}
	return "bottom"
}

// span walks this ask's repeated picks on the scratch position, always taking the
// first offered: how many picks it needs at least and takes at most. The kernel asks
// one pick at a time where gorge asks for a list.
func (g *Game) span() (min, max int) {
	g.scratch()
	min = -1
	for {
		pick, done := dirac.Action(0), false
		found := false
		for i := 0; i < int(g.wd.N); i++ {
			if isDecline(g.wd.Options[i]) {
				done = true
			} else if !found {
				pick, found = g.wd.Options[i], true
			}
		}
		if done && min < 0 {
			min = max
		}
		if !found {
			break
		}
		g.ws.Apply(pick)
		max++
		prev := g.wd
		g.ws.StepInto(&g.wd)
		if !g.sameAsk() || !continues(&prev, &g.wd, pick) {
			break
		}
	}
	if min < 0 {
		min = max
	}
	return min, max
}

// continues reports whether next is the same ask as prev with pick gone from it.
func continues(prev, next *dirac.Decision, pick dirac.Action) bool {
	if next.Kind != prev.Kind || next.Sub != prev.Sub || next.Player != prev.Player || next.N == 0 || next.N >= prev.N {
		return false
	}
	for i := 0; i < int(next.N); i++ {
		a := next.Options[i]
		if a == pick {
			return false
		}
		found := false
		for j := 0; j < int(prev.N); j++ {
			found = found || prev.Options[j] == a
		}
		if !found && !isDecline(a) {
			return false
		}
	}
	return true
}

// askOption reads one kernel option of an ask as the gorge option it stands for, with
// its place in gorge's order.
func (g *Game) askOption(x dirac.Action, c canon.Canon, i uint32) (o decision.Option, key uint32, unless bool) {
	me := g.d.Player
	k := c.Kind()
	o.Obj = g.ids.ref(c.Src(), &g.kp)
	o.Player = state.PlayerID(me)
	key = i
	switch {
	case k == canon.ActAttack:
		o.Kind = "attacker"
		if c.Tgt() >= canon.RefPlayer {
			o.Player = state.PlayerID(c.Tgt() - canon.RefPlayer)
			key = g.zoneKey(c.Src()) << 8
		} else {
			o.Battle = g.ids.ref(c.Tgt(), &g.kp)
			if int(c.Tgt()) < int(g.kp.NObj) {
				o.Player = state.PlayerID(g.kp.Objs[c.Tgt()].Ctrl)
			}
			key = g.zoneKey(c.Src())<<8 | 1 + g.zoneKey(c.Tgt())&0x7F
		}
		o.Required = g.s.MustAttack(dirac.Obj(c.Src()))
		o.Label = g.cardName(c.Src())
	case k == canon.ActBlock:
		o.Kind, o.Attacker, o.Player = "block", g.ids.ref(c.Tgt(), &g.kp), state.PlayerID(me)
		o.Group = "blocker:" + strconv.FormatUint(uint64(o.Obj), 10)
		if g.s.HasKeyword(dirac.Obj(c.Tgt()), kop.KwMenace) {
			o.MinBlockers = 2
		}
		key = g.zoneKey(c.Src())<<8 | g.zoneKey(c.Tgt())&0xFF
		o.Label = g.cardName(c.Src()) + " blocks " + g.cardName(c.Tgt())
	case k == canon.ActTarget:
		t := c.Tgt()
		switch {
		case t >= canon.RefPlayer:
			o.Kind, o.Player, o.Obj = "player", state.PlayerID(t-canon.RefPlayer), 0
			o.Controller = o.Player
			key = uint32(t - canon.RefPlayer)
			o.Label = "Player " + strconv.Itoa(int(o.Player)+1)
		case t >= canon.RefStack:
			o.Kind, o.Obj = "spell", g.ids.ref(t, &g.kp)
			key = 1<<24 | uint32(t-canon.RefStack)
			o.Label = g.stackName(int(t - canon.RefStack))
		default:
			o.Obj = g.ids.ref(t, &g.kp)
			o.Kind = "permanent"
			o.Label = g.cardName(t)
			if int(t) < int(g.kp.NObj) {
				po := &g.kp.Objs[t]
				o.Controller = state.PlayerID(po.Owner)
				switch po.Zone {
				case kop.ZoneBattlefield:
					o.Controller = state.PlayerID(po.Ctrl)
				case kop.ZoneStack:
					o.Kind = "spell"
				}
				o.Player = o.Controller
			}
			key = 1<<24 | 1<<20 | g.zoneKey(t)
			if o.Kind == "spell" {
				key = 1<<24 | uint32(g.stackPos(t))
			}
		}
	case k == canon.ActMode:
		o.Kind = "mode"
		o.Label = "Mode " + strconv.Itoa(int(c.Mode())+1)
		o.Player = state.PlayerID(me)
		if src := g.s.FeatSource(); src != dirac.NoObj && g.d.Sub != dirac.ChWardPay {
			o.Obj = g.ids.obj(uint8(src))
		}
		if g.d.Sub == dirac.ChGenCastFree {
			o.Obj = g.ids.obj(x.Source())
			o.Label = "Play " + g.cardName(x.Source())
		}
		key = uint32(c.Mode())
		if g.d.Sub == dirac.ChWardPay || g.d.Sub == dirac.ChGenPayMode {
			unless = true
			pay, decline := g.unlessWords()
			if x.Mode() == 1 {
				o.Mode, o.Label = decision.ModeUnlessPay, pay
			} else {
				o.Mode, o.Label = decision.ModeUnlessDecline, decline
			}
		}
	case k == canon.ActOrder:
		o.Kind, o.Label = "trigger", g.cardName(c.Src())
		key = uint32(x.Target())
	case k == canon.ActYes:
		o.Kind, o.Label, key = "yes", "Yes", 0
	case k == canon.ActNo:
		o.Kind, o.Label, key = "no", "No", 1
	case k == canon.ActArrange:
		o.Kind, o.Label = "arrange", g.cardName(c.Src())
		key = g.zoneKey(c.Src())
	case k == canon.ActReplacement:
		o.Kind, o.Label = "replacement", g.cardName(c.Src())
	case k == canon.ActBottom:
		o.Kind, o.Label = "bottom", g.cardName(c.Src())
		key = g.zoneKey(c.Src())
	case k == canon.ActKeep:
		o.Kind, o.Label = "keep", "Keep"
	case k == canon.ActCommanderZone:
		// CR 903.9a: mode 0 puts it into the command zone, mode 1 leaves it
		o.Kind, o.Label = "command_zone", "Put it into the command zone"
		if c.Mode() == 1 {
			o.Kind, o.Label = "leave", "Leave it"
		}
	case k == canon.ActMulligan:
		o.Kind, o.Label = "mulligan", "Mulligan"
	case k >= canon.ActChoose:
		o, key = g.chooseOption(x, c)
	}
	return o, key, unless
}

// chooseOption reads a KChoose pick.
func (g *Game) chooseOption(x dirac.Action, c canon.Canon) (decision.Option, uint32) {
	o := decision.Option{Obj: g.ids.ref(c.Src(), &g.kp), Kind: "other", Player: state.PlayerID(g.d.Player)}
	if i := int(c.Kind() - canon.ActChoose); i < len(canon.ChooseKinds) {
		o.Kind = canon.ChooseKinds[i]
	}
	if o.Obj == 0 && c.Tgt() >= canon.RefPlayer && c.Tgt() != canon.RefNone {
		o.Player = state.PlayerID(c.Tgt() - canon.RefPlayer)
	}
	switch o.Kind {
	case "x":
		o.Amount = int(c.Mode())
		o.Label = "X = " + strconv.Itoa(o.Amount)
	case "multikick":
		o.Amount = int(c.Mode())
		o.Label = "Pay the multikicker cost " + strconv.Itoa(o.Amount) + " time(s)"
	case "number":
		o.Amount = int(c.Mode())
		o.Label = strconv.Itoa(o.Amount)
	case "altaddcost":
		o.Amount = int(x.Target()) // the part; the canonical mode is its position, payable parts first
		o.Label = g.altAddLabel(o.Amount)
	case "division":
		g.labelDivision(&o, c.Mode())
	case "primary":
		o.Label, o.Player = "top", 0
	case "bottom":
		o.Player = 0
	case "mana":
		if int(c.Mode()) < len(canon.ManaSyms) {
			o.ManaSymbol = canon.ManaSyms[c.Mode() : c.Mode()+1]
			o.Label = o.ManaSymbol
		}
	case "color":
		if int(c.Mode()) < len(colorNames) {
			o.Label = colorNames[c.Mode()]
		}
	case "pay":
		o.Kind = "pay_" + [8]string{"W", "U", "B", "R", "G", "C", "generic", "life"}[c.Mode()&7]
		o.Amount = 1
	case "gift_promise":
		// gorge giftAsk: the promised opponent, the mode-th of the
		// decider's opponents in AliveFrom(decider) order
		n := 0
		for _, p := range g.board.g.AliveFrom(state.PlayerID(g.d.Player)) {
			if p == state.PlayerID(g.d.Player) {
				continue
			}
			if n == int(c.Mode()) {
				o.Player = p
				o.Label = "Promise " + g.board.g.SeatFacingName(p) + " the gift"
				break
			}
			n++
		}
	case "vote_card":
		// gorge effCardVote names the candidate's controller (CR 400.2)
		if t := c.Src(); int(t) < int(g.kp.NObj) {
			o.Player = state.PlayerID(g.kp.Objs[t].Ctrl)
		}
	}
	if o.Label == "" {
		if o.Obj != 0 {
			o.Label = g.cardName(c.Src())
		} else {
			o.Label = strings.ReplaceAll(o.Kind, "_", " ")
		}
	}
	return o, g.chooseKey(x)
}

// altAddLabel words part i of the either-or additional cost of the spell being built.
func (g *Game) altAddLabel(i int) string {
	card, _, _, _ := g.s.Building()
	if param, ok := g.face(card).KeywordParam("AlternateAdditionalCost"); ok {
		if parts := cost.AltAddParts(param); i < len(parts) {
			return cost.CapitaliseFirst(cost.CostPhrase(cost.ParseCost(parts[i])))
		}
	}
	return strconv.Itoa(i)
}

// unlessWords words the two options of a cost asked inside a resolution: ward's, read
// off the permanent it protects, or the resolving ability's UnlessCost$.
func (g *Game) unlessWords() (pay, decline string) {
	src := g.s.FeatSource()
	var f *cards.Face
	if src != dirac.NoObj {
		f = g.face(g.kp.Objs[src].Card)
	}
	if g.d.Sub == dirac.ChWardPay && f != nil {
		if cost, ok := f.KeywordParam("Ward"); ok {
			return effects.UnlessPayPhrase(cost), "Don't pay"
		}
	}
	if top := int(g.kp.NStack) - 1; top >= 0 {
		for sa := g.stackSA(&g.kp.Stack[top]); sa != nil; sa = sa.Sub {
			if cost := effects.ActivationOf(sa).UnlessCost; cost != "" {
				name := "The permanent"
				if f != nil {
					name = f.Name
				}
				_, pay, decline = effects.UnlessWording(sa, cost, name)
				return pay, decline
			}
		}
	}
	return "Pay the cost", "Don't pay"
}

// colorNames words a colour choice, in canon.ManaSyms' order.
var colorNames = [...]string{"White", "Blue", "Black", "Red", "Green"}

// landWontStay reports whether land f, played by p, would not stay on the battlefield:
// its enters trigger asks for a land to return and p controls no other that qualifies.
func (g *Game) landWontStay(p uint8, f *cards.Face) bool {
	if f == nil {
		return false
	}
	filter := effects.EntryLandReturn(f)
	if filter == "" {
		return false
	}
	for o := 0; o < int(g.kp.NObj); o++ {
		po := &g.kp.Objs[o]
		if po.Zone == kop.ZoneBattlefield && po.Ctrl == p && returnFits(filter, g.face(po.Card), po.Flags&dirac.ProjTapped != 0) {
			return false
		}
	}
	return true
}

// returnFits reads a return cost's filter against a permanent the seat controls:
// Type.mod+mod, of what these costs say. A modifier it does not know is taken to fit.
func returnFits(filter string, f *cards.Face, tapped bool) bool {
	if f == nil {
		return false
	}
	base, mods, _ := strings.Cut(filter, ".")
	if !slices.Contains(f.Types, base) {
		return false
	}
	for _, m := range strings.Split(mods, "+") {
		if not, ok := strings.CutPrefix(m, "non"); ok && slices.Contains(f.Types, not) || m == "untapped" && tapped {
			return false
		}
	}
	return true
}

// labelDivision says how split number i divides the attacker's damage, in the words a
// v2 mapping rebuilds. Amount is the first blocker's share, as on gorge.
func (g *Game) labelDivision(o *decision.Option, i uint8) {
	var blockers [dirac.MaxPerm]dirac.Obj
	var split [dirac.MaxPerm]uint8
	n, damage := g.s.FeatDivision(&blockers)
	dirac.SplitAt(damage, n, int(i), split[:])
	o.Amount = int(split[0])
	for j := range n {
		if j > 0 {
			o.Label += ","
		}
		o.Label += strconv.Itoa(int(split[j])) + " to " + g.face(g.kp.Objs[blockers[j]].Card).Name
	}
}

// stackPos is where spell object o sits on the stack.
func (g *Game) stackPos(o canon.Ref) int {
	for i := 0; i < int(g.kp.NStack); i++ {
		if e := &g.kp.Stack[i]; e.Kind == 0 && canon.Ref(e.Source) == o {
			return i
		}
	}
	return 0
}

// zoneKey orders objects as gorge's zone walks meet them.
func (g *Game) zoneKey(obj canon.Ref) uint32 {
	if obj >= canon.RefStack || int(obj) >= int(g.kp.NObj) {
		return uint32(obj)
	}
	po := &g.kp.Objs[obj]
	return uint32(po.Zone)<<16 | uint32(po.Ctrl)<<8 | uint32(po.ZoneIdx)
}

// chooseKey is a DkChoose option's place in gorge's option order.
func (g *Game) chooseKey(x dirac.Action) uint32 {
	src, mode := x.Source(), x.Mode()
	switch g.d.Sub {
	case dirac.ChOptional, dirac.ChWardPay, dirac.ChOptDiscMay, dirac.ChGenMayShuffle, dirac.ChGenPay, dirac.ChGenPayMode,
		dirac.ChGenTopBottom, dirac.ChGenLeyline, dirac.ChGenDiscUnless:
		return 1 - uint32(mode&1) // yes, pay, top and unless come first
	case dirac.ChGenDivide, dirac.ChCastDivide:
		return uint32(g.s.FeatDivideIndex(src))
	case dirac.ChX, dirac.ChTrigX, dirac.ChMultikick, dirac.ChDivide, dirac.ChGenBranch, dirac.ChGenPileChoose, dirac.ChGenColor, dirac.ChGenManaChoice,
		dirac.ChAltAdd, dirac.ChGenPip, dirac.ChGenReplOrder, dirac.ChGenGift, dirac.ChCostForage, dirac.ChGenKeepTapped:
		return uint32(mode)
	case dirac.ChGenType:
		return g.s.FeatTypeKey(g.d.Player, int(x.Target())<<8|int(mode))
	case dirac.ChGenChooseCard, dirac.ChGenChooseEach, dirac.ChGenCastFree:
		return uint32(src)
	}
	if src == uint8(dirac.NoObj) || src >= dirac.PlayerTarget0 {
		return uint32(mode)
	}
	return g.zoneKey(src)
}

// unlessSA stands for the resolving ability of an unless-pay ask; a client reads only
// that one is there.
var unlessSA = &cards.SA{}

// targetSlot follows which target of one build the kernel is asking for.
type targetSlot struct {
	key  uint32
	slot int
}

// fillTargetEffect names a target ask's source and what the chosen target will
// suffer: the k-th targeted ability of the chain being cast or activated.
func (g *Game) fillTargetEffect(d *decision.Decision) {
	card, ab, _, building := g.s.Building()
	src := g.s.FeatSource()
	key := uint32(src) | 1<<30
	if building {
		key = uint32(card)<<16 | uint32(ab)<<8 | uint32(src) | 1<<31
	}
	if key == g.targets.key {
		g.targets.slot++
	} else {
		g.targets = targetSlot{key: key}
	}
	if src != dirac.NoObj {
		d.Source = g.ids.obj(uint8(src))
	}
	if !building {
		if top := int(g.kp.NStack) - 1; top >= 0 {
			targetSet(d, g.stackSA(&g.kp.Stack[top]))
		}
		return
	}
	if top := int(g.kp.NStack) - 1; top >= 0 && g.kp.Stack[top].Kind == 2 {
		d.Source = g.ids.stack[top]
	}
	abs := g.tbl.Cards
	valid := int(card) < len(abs) && int(ab) < len(abs[card].Abilities)
	activated := valid && (abs[card].Abilities[ab].Kind == kop.AbActivated || abs[card].Abilities[ab].Kind == kop.AbMana)
	sa := g.buildingSA(card, ab)
	g.chooseTargetEffect(d, card, sa)
	// gorge's TargetEffect.Host: the object casting or activating, which the
	// bot never adds past Min to an up-to removal of its own
	if d.TargetEffect != nil && src != dirac.NoObj {
		d.TargetEffect.Host = g.ids.obj(uint8(src))
	}
	// gorge's cast-time ask names no source for an activated ability, whose object
	// is not yet minted, an Attach excepted
	if activated && (d.TargetEffect == nil || d.TargetEffect.API != "Attach") {
		d.Source = 0
	}
}

// resolvingSA is the part of the resolving ability that runs api, or nil.
func (g *Game) resolvingSA(api string) *cards.SA {
	top := int(g.kp.NStack) - 1
	if top < 0 {
		return nil
	}
	sa := g.stackSA(&g.kp.Stack[top])
	for sa != nil && sa.API != api {
		sa = sa.Sub
	}
	return sa
}

// buildingSA is the spell ability a cast or activation under way runs.
func (g *Game) buildingSA(card dirac.CardID, ab uint8) *cards.SA {
	abs := g.tbl.Cards
	if int(card) < len(abs) && int(ab) < len(abs[card].Abilities) && abs[card].Abilities[ab].Kind == kop.AbSpell || ab == 255 {
		return g.faceSA(card, 255)
	}
	if g.tr.Origin != nil {
		if gi, ok := g.tr.Origin(card, ab); ok {
			return g.faceSA(card, gi)
		}
	}
	return nil
}

// labelModes words a modal ability's options from its Choices$, as gorge does.
func (g *Game) labelModes(d *decision.Decision) {
	var sa *cards.SA
	card, ab, _, building := g.s.Building()
	if building {
		sa = g.buildingSA(card, ab)
	} else if top := int(g.kp.NStack) - 1; top >= 0 {
		card, sa = g.kp.Stack[top].Card, g.stackSA(&g.kp.Stack[top])
	}
	f := g.face(card)
	if sa == nil || f == nil || sa.API != "Charm" {
		return
	}
	choices := strings.Split(sa.Params["Choices"], ",")
	for i := range d.Options {
		o := &d.Options[i]
		if m := int(g.canons[i].Mode()); o.Kind == "mode" && m < len(choices) {
			name := strings.TrimSpace(choices[m])
			o.Label = effects.CharmModeLabel(cards.ResolveSVar(f.SVars, name), name)
		}
	}
}

// targetSet writes what a target ask's whole answer must satisfy: Decision.Validate
// holds an answer to one option of a Group.
func targetSet(d *decision.Decision, sa *cards.SA) {
	if sa == nil {
		return
	}
	t := effects.TargetsOf(sa)
	d.TargetsWithSameController = t.Has(effects.TgtSameController)
	if t.Flags&(effects.TgtForEachPlayer|effects.TgtDifferentControllers) == 0 {
		return
	}
	for i := range d.Options {
		d.Options[i].Group = "target-controller-" + strconv.Itoa(int(d.Options[i].Controller))
	}
}

// costDesc matches a cost part with a display name after its count and type, and
// costHybrid a hybrid symbol as a script spells it.
var (
	costDesc   = regexp.MustCompile(`<([^/>]+/[^/>]+)/[^>]*>`)
	costHybrid = regexp.MustCompile(`\b([WUBRG2])([WUBRG])\b`)
)

// offerCost is an ability's cost as the wire writes it.
func offerCost(sa *cards.SA) string {
	return costHybrid.ReplaceAllString(costDesc.ReplaceAllString(sa.ParamStr(cards.PKCost), "<$1>"), "$1/$2")
}

// loyaltyRe reads a loyalty cost's counters from an ability's cost text.
var loyaltyRe = regexp.MustCompile(`(?i)(Add|Sub)Counter<(\w+)/LOYALTY>`)

// loyaltyOf is the facts a bot ranks a loyalty ability by, nil for any other ability.
func loyaltyOf(sa *cards.SA, cost string) *decision.Loyalty {
	m := loyaltyRe.FindStringSubmatch(cost)
	if m == nil {
		return nil
	}
	l := &decision.Loyalty{Ultimate: strings.EqualFold(strings.TrimSpace(sa.ParamStr(cards.PKUltimate)), "True")}
	n, err := strconv.Atoi(m[2])
	switch {
	case err != nil:
		l.X = true
	case strings.EqualFold(m[1], "Add"):
		l.Delta = int32(n)
	default:
		l.Delta = -int32(n)
	}
	return l
}

func (g *Game) chooseTargetEffect(d *decision.Decision, card dirac.CardID, sa *cards.SA) {
	var chain []*cards.SA
	f := g.face(card)
	for x := sa; x != nil; x = x.Sub {
		if (x.API == "Charm" || x.API == "GenericChoice") && f != nil {
			modes := g.s.BuildModes()
			for i, name := range strings.Split(x.Params["Choices"], ",") {
				if i < 8 && modes&(1<<i) != 0 {
					for m := cards.ResolveSVar(f.SVars, strings.TrimSpace(name)); m != nil; m = m.Sub {
						chain = append(chain, m)
					}
				}
			}
			continue
		}
		chain = append(chain, x)
		if x.API == "ImmediateTrigger" && f != nil {
			for m := cards.ResolveSVar(f.SVars, x.Params["Execute"]); m != nil; m = m.Sub {
				chain = append(chain, m)
			}
		}
	}
	k := 0
	for i, x := range chain {
		if _, ok := x.Params["ValidTgts"]; !ok {
			if _, ok := x.Params["TgtPrompt"]; !ok {
				continue
			}
		}
		if k >= g.targets.slot && slotFits(x, d) || i == len(chain)-1 {
			d.TargetEffect = g.targetEffect(x, f, chain[:i])
			targetSet(d, x)
			return
		}
		k++
	}
	if sa != nil {
		d.TargetEffect = g.targetEffect(sa, f, nil)
		targetSet(d, sa)
	}
}

// slotFits reports whether a targeted ability's ValidTgts$ admits the offered targets'
// controllers: the kernel skips a slot with one option, so counting slots is not enough.
func slotFits(x *cards.SA, d *decision.Decision) bool {
	valid := x.Params["ValidTgts"]
	if valid == "" || len(d.Options) == 0 {
		return true
	}
	for i := range d.Options {
		o := &d.Options[i]
		ok := false
		for _, alt := range strings.Split(valid, ",") {
			typ, quals, _ := strings.Cut(strings.TrimSpace(alt), ".")
			if o.Kind == "player" {
				if typ != "Player" && typ != "Opponent" && typ != "Any" {
					continue
				}
			} else if typ == "Player" || typ == "Opponent" {
				continue
			}
			mine := o.Player == d.Player
			fits := typ != "Opponent" || !mine
			for _, q := range strings.Split(quals, "+") {
				switch q {
				case "YouCtrl", "YouOwn", "You":
					fits = fits && mine
				case "OppCtrl", "OppOwn", "Opponent":
					fits = fits && !mine
				}
			}
			if fits {
				ok = true
				break
			}
		}
		if !ok {
			return false
		}
	}
	return true
}

// targetEffect describes sa for a seat choosing its target: the API, the removal shape,
// the damage it deals as things stand, and what an Effect grants. before is the chain
// ahead of sa, which says which of the ability's damage instructions is sa's.
func (g *Game) targetEffect(sa *cards.SA, f *cards.Face, before []*cards.SA) *decision.TargetEffect {
	te := &decision.TargetEffect{API: sa.API}
	switch sa.APIKind() {
	case cards.APIDestroy, cards.APIDestroyAll:
		te.Removal = &decision.RemovalEffect{Kind: "destroy"}
	case cards.APISacrifice, cards.APISacrificeAll:
		te.Removal = &decision.RemovalEffect{Kind: "sacrifice"}
	case cards.APIChangeZone:
		if cz := effects.ChangeZoneOf(sa); cz.DestinationKnown {
			if k := removalKind(strings.ToLower(cz.Destination.String())); k != "" {
				te.Removal = &decision.RemovalEffect{Kind: k, Destination: cz.Destination.String()}
			}
		}
	case cards.APIChangeZoneAll:
		dest := effects.ChangeZoneAllOf(sa).DestinationLower
		if k := removalKind(dest); k != "" {
			te.Removal = &decision.RemovalEffect{Kind: k, Destination: dest}
		}
	case cards.APIDealDamage, cards.APIDamageAll:
		te.Damage = &decision.DamageEffect{}
		if n, ok := g.damageAmount(sa, f, before); ok && n >= 0 {
			te.Damage.Amount = &n
		}
	}
	if sa.API == "Effect" && f != nil {
		te.Statics = effects.GrantedStaticModes(sa, f.SVars)
	}
	return te
}

// damageAmount is what sa deals: a literal, or the kernel's reading of its damage
// instruction. An amount that reads the target being chosen, or what a cost will name,
// has no value yet.
func (g *Game) damageAmount(sa *cards.SA, f *cards.Face, before []*cards.SA) (int, bool) {
	amt := effects.DamageAmount(sa)
	if !amt.Present {
		return 0, false
	}
	raw := strings.TrimSpace(amt.Text)
	if n, err := strconv.Atoi(raw); err == nil {
		return n, true
	}
	if f != nil {
		if body := f.SVars[strings.TrimLeft(raw, "+-")]; strings.Contains(body, "Targeted$") || effects.AmountAwaitsCost(body) {
			return 0, false
		}
	}
	n := 0
	for _, x := range before {
		if k := x.APIKind(); k == cards.APIDealDamage || k == cards.APIDamageAll {
			n++
		}
	}
	return g.s.FeatDamageOp(n)
}

func removalKind(dest string) string {
	switch dest {
	case "exile", "graveyard", "library", "command":
		return dest
	case "hand":
		return "bounce"
	}
	return ""
}
