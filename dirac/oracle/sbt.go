package oracle

import (
	"context"
	"strconv"
	"strings"

	"github.com/adams-shaun/gorge/bots/spellbench/builtins"
	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

// SBT is sb-tactical playing the kernel: gorge's own decision core
// (bots/spellbench/builtins.NewTactical with the hosted weights, the seat
// gorge's "sbtactical" policy builds) answering a gorge view.View and
// decision.Decision that the agent builds from the kernel position
// (sbt_view.go, below). Nothing of sb-tactical is re-implemented: the
// adapter's job is to show the seat what gorge would show it.
//
//   - Priority: the kernel's options are its auto-pay vocabulary, each a
//     payable play. They are presented in gorge's three candidate sources
//     (an offered option when the floating pool pays it or it costs no
//     mana, a planner-paid cast as a PaymentAction, any other play -- a
//     kicked cast, an {X} cast, an ability gorge prices only as a potential
//     play -- as a PotentialAction the stand-in planner prices), in gorge's
//     enumeration order, and scored by builtins.Seat.TacticalPriority; the
//     best candidate's key names the kernel option. The kernel's autopay is
//     the planner: a play it offers is payable, so every potential play is
//     priced and nothing is "insufficient". No mana is lowered.
//   - Every other kind: the kernel decision is canonicalised (the oracle's
//     realSide translation, canon.go grammar) and posed as the gorge
//     decision that grammar stands for; Seat.Decide answers it.
//     Declarations (attackers, blockers) and trigger orders are posed whole
//     at their first single pick and the answer is queued as the kernel's
//     picks then Done.
//
// One SBT plays both seats of one game at a time (a seat each, seeded per
// game by NewGame). It allocates (gorge's seat and the view are
// string-and-map shaped), so it is an agent, not a hot-loop policy.
type SBT struct {
	c     *SBTCards
	seats [2]*builtins.Seat
	w     builtins.TacticalWeights

	rs   realSide // canonicaliser + projection scratch
	v    view.View
	zone []uint8

	// queued picks of a declaration answered whole: served while the
	// kernel's next decision continues it (the same kind and player, the
	// next decision number, the last served pick no longer offered)
	q       []dirac.Action
	qKind   dirac.DecisionKind
	qPlayer uint8
	qSeq    uint32
	qLast   dirac.Action

	// the target slot being asked within one build (card, ability, source)
	tgtKey  uint32
	tgtSlot int

	planner sbtPlanner
	seq     uint64
	// last is the gorge decision the last Answer posed (diagnostics)
	last *decision.Decision

	Stats SBTStats
}

// SBTStats counts the agent's fallbacks.
type SBTStats struct {
	Decisions int // decisions answered (queued picks included)
	Queued    int // picks served from a queued declaration
	Fallbacks int // answers the kernel did not offer (option 0 or Done taken instead)
}

// NewSBT returns an agent over c's table with the hosted weights.
func NewSBT(c *SBTCards) *SBT {
	a := &SBT{c: c, w: builtins.DefaultTacticalWeights()}
	a.rs.tbl = c.tbl
	a.rs.origin = c.gorgeAbility
	a.rs.generated = true
	a.NewGame([2]uint64{1, 2})
	return a
}

// NewGame starts a game: one fresh sb-tactical seat per player, seeded as
// gorge self-play seeds its seats (the per-seat seed).
func (a *SBT) NewGame(seeds [2]uint64) {
	for p := 0; p < 2; p++ {
		a.seats[p] = builtins.NewTactical(builtins.AutoPay, seeds[p], a.c.lookup, a.w)
	}
	a.q = a.q[:0]
	a.tgtKey, a.tgtSlot = 0, 0
}

// Seat is player p's gorge seat (tests).
func (a *SBT) Seat(p uint8) *builtins.Seat { return a.seats[p&1] }

// Pick answers d (an index into d.Options[:d.N]); it satisfies the kernel
// search's Bot interface.
func (a *SBT) Pick(s *dirac.State, d *dirac.Decision) int {
	a.Stats.Decisions++
	if i := a.fromQueue(s, d); i >= 0 {
		a.Stats.Queued++
		return i
	}
	acts := a.Answer(s, d)
	if len(acts) == 0 {
		a.Stats.Fallbacks++
		return 0
	}
	i := optIndex(d, acts[0])
	if i < 0 {
		a.Stats.Fallbacks++
		return 0
	}
	a.q = a.q[:0]
	if len(acts) > 1 {
		a.q = append(a.q, acts[1:]...)
		a.qKind, a.qPlayer, a.qSeq, a.qLast = d.Kind, d.Player, s.SeqNo(), acts[0]
	}
	return i
}

// Cards is the table knowledge the agent plays with.
func (a *SBT) Cards() *SBTCards { return a.c }

func optIndex(d *dirac.Decision, a dirac.Action) int {
	for i := 0; i < int(d.N); i++ {
		if d.Options[i] == a {
			return i
		}
	}
	return -1
}

// fromQueue serves the next queued pick of the declaration in progress.
func (a *SBT) fromQueue(s *dirac.State, d *dirac.Decision) int {
	if len(a.q) == 0 {
		return -1
	}
	if d.Kind != a.qKind || d.Player != a.qPlayer || s.SeqNo() != a.qSeq+1 || optIndex(d, a.qLast) >= 0 {
		// not the continuation of the queued declaration (a search world
		// moved on, or another answer was applied)
		a.q = a.q[:0]
		return -1
	}
	for len(a.q) > 0 {
		x := a.q[0]
		a.q = a.q[1:]
		if i := optIndex(d, x); i >= 0 {
			a.qSeq, a.qLast = s.SeqNo(), x
			return i
		}
		if x.Kind() == dirac.AkDone {
			break
		}
	}
	a.q = a.q[:0]
	// the declaration cannot go on as planned: Done when it is offered
	for i := 0; i < int(d.N); i++ {
		if d.Options[i].Kind() == dirac.AkDone {
			a.Stats.Fallbacks++
			return i
		}
	}
	return -1
}

// Answer is the seat's whole answer to d as kernel actions in pick order
// (one action for a single pick; a declaration's picks then Done). It
// mutates the seat's state exactly as one gorge Decide would.
func (a *SBT) Answer(s *dirac.State, d *dirac.Decision) []dirac.Action {
	a.rs.s.CopyFrom(s)
	a.rs.d = *d
	a.rs.s.Project(&a.rs.kp)
	a.seq++
	seat := a.seats[d.Player&1]
	if d.Kind == dirac.DkPriority {
		return a.priority(s, d, seat)
	}
	a.trackTarget(s, d)
	gd, acts := a.gorgeDecision(s, d)
	a.last = gd
	v := a.buildView(s, d.Player, nil)
	in, err := seat.Decide(context.Background(), *v, *gd)
	if err != nil {
		return nil
	}
	var out []dirac.Action
	for _, ch := range in.Choices {
		if ch >= 0 && ch < len(acts) {
			out = append(out, acts[ch])
		}
	}
	switch gd.Kind {
	case decision.KAttackers, decision.KBlockers:
		out = append(out, dirac.MakeAction(dirac.AkDone, 0, 0, 0))
	default:
		if len(out) == 0 {
			// an empty answer (an up-to target declined, nothing kept):
			// the kernel's NoTarget / Done
			for i := 0; i < int(d.N); i++ {
				if x := d.Options[i]; x.Kind() == dirac.AkDone || x.Kind() == dirac.AkTarget && x.Target() == dirac.NoTarget {
					out = append(out, x)
					break
				}
			}
		}
	}
	return out
}

// sbtPlanner is the priority planner stand-in: every potential play the
// kernel offers is priced with an (empty) witness.
type sbtPlanner struct{ plans []rules.PotentialPlan }

func (p *sbtPlanner) PotentialPaymentPlans(state.PlayerID) []rules.PotentialPlan { return p.plans }

var emptyPlan = &decision.PaymentPlan{}

// prioEntry is one priority candidate's kernel action and gorge identity.
type sbtPrio struct {
	act  dirac.Action
	key  uint64 // gorge enumeration order
	src  uint8  // builtins.KeyOption / KeyPlan / KeyPotential
	kind string
	obj  state.ObjID
	ab   int
	mode string
}

func (a *SBT) priority(s *dirac.State, d *dirac.Decision, seat *builtins.Seat) []dirac.Action {
	p := d.Player
	kp := &a.rs.kp
	var es []sbtPrio
	var pass dirac.Action
	hasPass := false
	zidx := func(o uint8) uint64 {
		po := &kp.Objs[o]
		return uint64(po.ZoneIdx)
	}
	for i := 0; i < int(d.N); i++ {
		x := d.Options[i]
		src, mode := x.Source(), x.Mode()
		switch x.Kind() {
		case dirac.AkPass:
			pass, hasPass = x, true
		case dirac.AkPlayLand:
			es = append(es, sbtPrio{act: x, key: 0<<40 | zidx(uint8(src))<<8, src: builtins.KeyOption, kind: "play_land", obj: sbtObjID(uint8(src))})
		case dirac.AkCast, dirac.AkCastCasualty, dirac.AkCastConspire:
			cm := ""
			switch {
			case x.Kind() == dirac.AkCastConspire:
				cm = "conspired" // CR 702.78a: gorge's "conspired" cast option
			case x.Kind() == dirac.AkCastCasualty && s.FeatTeamworkCard(dirac.Obj(src)):
				cm = "teamworked" // CR 702.194: gorge's "teamworked" cast option
			case x.Kind() == dirac.AkCastCasualty && s.FeatBargainCard(dirac.Obj(src)):
				cm = "bargained" // CR 702.166: gorge's "bargained" cast option
			case x.Kind() == dirac.AkCastCasualty:
				cm = "casualty"
			case mode&1 != 0 && s.FeatOptCostCard(dirac.Obj(src)):
				cm = "optionalcost" // CR 601.2b: gorge's "optionalcost" cast option
			case mode&1 != 0 && s.FeatOffspringCast(dirac.Obj(src)):
				cm = "offspring"
			case mode&1 != 0 && s.FeatBuybackCast(dirac.Obj(src)):
				cm = "buyback" // CR 702.27a: gorge's "buyback" cast option
			case mode&1 != 0 && s.FeatEntwineCast(dirac.Obj(src)):
				cm = "entwined" // CR 702.42a: gorge's "entwined" cast option
			case mode&1 != 0:
				cm = "kicked"
			case mode&2 != 0 && x.Target() == dirac.TCode(dirac.FeatEscapeTgt):
				cm = "escape" // CR 702.42a: the escape cast a StatEscapeGrant static gives (kw_escape.go)
			case mode&2 != 0 && s.FeatMayhemCard(dirac.Obj(src)):
				cm = "mayhem"
			case mode&2 != 0 && x.Target() == dirac.TCode(dirac.FeatCastEscapeTgt):
				cm = "escape" // CR 702.138a: gorge's "escape" cast option, marked by the target byte (kw_escape.go)
			case mode&2 != 0 && s.FeatHarmonizeCast(dirac.Obj(src)):
				cm = "harmonize"
			case mode&2 != 0 && x.Target() == dirac.TCode(dirac.FeatRetraceTgt) && s.FeatRetraceCard(dirac.Obj(src)):
				cm = "retrace" // CR 702.81a: the printed cost plus the discard-a-land cost (kw_retrace.go)
				if s.FeatJumpStartCard(dirac.Obj(src)) {
					cm = "jumpstart" // CR 702.84a: the same cast with any card discarded (kw_jumpstart.go)
				}
			case mode&2 != 0 && x.Target() != 0:
				cm = "beam_me_up"
			case mode&2 != 0:
				cm = "flashback"
			case mode&dirac.FeatCastMayPlay != 0:
				cm = "mayplay"
			}
			o := dirac.Obj(src)
			e := sbtPrio{act: x, kind: "cast", obj: sbtObjID(uint8(src)), mode: cm}
			fromHand := kp.Objs[src].Zone == kop.ZoneHand
			switch {
			case s.FeatPoolPays(p, o, mode) || a.free(uint8(src), mode):
				e.src = builtins.KeyOption
				if mode&2 != 0 {
					e.key = 1<<40 | zidx(uint8(src))<<8
				} else {
					sub := uint64(0)
					if !fromHand {
						sub = 2
					}
					e.key = 0<<40 | sub<<32 | zidx(uint8(src))<<8 | 1 + uint64(mode&1)
				}
			case mode&1 == 0 && s.FeatPlannedCast(p, o, mode) && (mode&dirac.FeatCastMayPlay == 0 || s.FeatMayPlayUntyped(p, o)):
				e.src = builtins.KeyPlan
				sub := uint64(0)
				if mode&2 != 0 {
					sub = 3
				} else if !fromHand {
					sub = 2
				}
				e.key = 7<<40 | sub<<32 | zidx(uint8(src))<<8
			default:
				e.src = builtins.KeyPotential
				e.key = 8<<40 | zidx(uint8(src))<<8 | uint64(mode)
			}
			es = append(es, e)
		case dirac.AkActivate:
			gi, ok := a.c.gorgeAbility(kp.Objs[src].Card, mode)
			if !ok {
				gi = 255
			}
			e := sbtPrio{act: x, kind: "ability", obj: sbtObjID(uint8(src)), ab: int(gi)}
			po := &kp.Objs[src]
			zr := [8]uint64{}
			_ = zr
			zone := uint64(5)
			switch po.Zone {
			case kop.ZoneBattlefield:
				zone = 0
			case kop.ZoneGraveyard:
				zone = 2
			case kop.ZoneHand:
				zone = 4
			case kop.ZoneExile:
				zone = 3
			}
			seatOf := uint64(po.Owner)
			if po.Zone == kop.ZoneBattlefield {
				seatOf = uint64(po.Ctrl)
			}
			if s.FeatPlannedActivation(p, dirac.Obj(src), mode) || s.FeatAbilityPoolPays(p, dirac.Obj(src), mode, dirac.Obj(x.Target())) {
				e.src = builtins.KeyOption
				e.key = 3<<40 | zone<<36 | seatOf<<32 | zidx(uint8(src))<<8 | uint64(mode)
			} else {
				e.src = builtins.KeyPotential
				e.key = 8<<40 | 1<<36 | zidx(uint8(src))<<8 | uint64(mode)
			}
			es = append(es, e)
		}
	}
	if len(es) == 0 {
		if hasPass {
			return []dirac.Action{pass}
		}
		return nil
	}
	for i := 1; i < len(es); i++ {
		for j := i; j > 0 && es[j].key < es[j-1].key; j-- {
			es[j], es[j-1] = es[j-1], es[j]
		}
	}
	gd := decision.Decision{Seq: a.seq, Player: state.PlayerID(p), Kind: decision.KPriority, Min: 1, Max: 1}
	var optAct []dirac.Action
	var pots []decision.PotentialAction
	a.planner.plans = a.planner.plans[:0]
	for _, e := range es {
		if e.src != builtins.KeyOption {
			continue
		}
		gd.Options = append(gd.Options, decision.Option{Index: len(gd.Options), Kind: e.kind, Obj: e.obj, Ability: e.ab, Mode: e.mode})
		optAct = append(optAct, e.act)
	}
	if hasPass {
		gd.Options = append(gd.Options, decision.Option{Index: len(gd.Options), Kind: "pass"})
		optAct = append(optAct, pass)
	}
	for _, e := range es {
		switch e.src {
		case builtins.KeyPlan:
			pc := decision.PlannedCast{Object: e.obj}
			if e.mode == "flashback" {
				pc.Origin = decision.OriginFlashback
			}
			gd.PaymentActions = append(gd.PaymentActions, decision.PaymentAction{ID: "k" + strconv.Itoa(len(gd.PaymentActions)),
				Cast: pc, Plans: []decision.PaymentPlan{{}}})
		case builtins.KeyPotential:
			pa := decision.PotentialAction{Kind: e.kind, Obj: e.obj, Ability: e.ab, Mode: e.mode}
			pots = append(pots, pa)
			a.planner.plans = append(a.planner.plans, rules.PotentialPlan{Action: pa, Plan: emptyPlan})
		}
	}
	gd.PaymentActionsBuilt = true
	a.last = &gd
	seat.SetPlanner(&a.planner)
	v := a.buildView(s, p, pots)
	cands, best, ok := seat.TacticalPriority(*v, gd)
	if !ok {
		if hasPass {
			return []dirac.Action{pass}
		}
		return []dirac.Action{es[0].act}
	}
	k := cands[best].Key
	switch k.Src {
	case builtins.KeyOption:
		if k.Opt >= 0 && k.Opt < len(optAct) {
			return []dirac.Action{optAct[k.Opt]}
		}
	case builtins.KeyPlan:
		for _, e := range es {
			if e.src == builtins.KeyPlan && e.obj == k.Obj {
				return []dirac.Action{e.act}
			}
		}
	case builtins.KeyPotential:
		for _, e := range es {
			if e.src == builtins.KeyPotential && e.obj == k.Obj && e.kind == k.Kind && e.ab == k.Ability && e.mode == k.Mode {
				return []dirac.Action{e.act}
			}
		}
	}
	if hasPass {
		return []dirac.Action{pass}
	}
	return []dirac.Action{es[0].act}
}

// free reports a cast of o whose mana cost is nothing (gorge offers it as
// an ordinary option: there is nothing to plan).
func (a *SBT) free(o uint8, mode uint8) bool {
	cid := a.rs.kp.Objs[o].Card
	if int(cid) >= len(a.c.tbl.Cards) || mode&1 != 0 {
		return false
	}
	cd := &a.c.tbl.Cards[cid]
	return cd.Cost.MV() == 0 && cd.Cost.X == 0 && cd.Types&kop.TLand == 0 && mode&2 == 0
}

// trackTarget follows the target slot a DkTarget asks within one build.
func (a *SBT) trackTarget(s *dirac.State, d *dirac.Decision) {
	if d.Kind != dirac.DkTarget {
		return
	}
	card, ab, _, ok := s.Building()
	src := s.FeatSource()
	key := uint32(card)<<16 | uint32(ab)<<8 | uint32(src) | 1<<31
	if !ok {
		key = uint32(src) | 1<<30
	}
	if key == a.tgtKey {
		a.tgtSlot++
	} else {
		a.tgtKey, a.tgtSlot = key, 0
	}
}

// gorgeDecision poses kernel decision d (not priority) as gorge would:
// the canonical translation of each option (realSide.canon) read back as
// the gorge option it stands for. acts[i] is gorge option i's kernel action.
func (a *SBT) gorgeDecision(s *dirac.State, d *dirac.Decision) (*decision.Decision, []dirac.Action) {
	r := &a.rs
	me := d.Player
	dk := r.dkind()
	gd := &decision.Decision{Seq: a.seq, Player: state.PlayerID(me), Min: 1, Max: 1}
	var acts []dirac.Action
	ref := func(x Ref) state.ObjID {
		switch {
		case x == RefNone:
			return 0
		case x >= RefPlayer:
			return 0
		case x >= RefStack:
			i := int(x - RefStack)
			if i < int(r.kp.NStack) && r.kp.Stack[i].Kind == 0 {
				return sbtObjID(uint8(r.kp.Stack[i].Source))
			}
			return sbtStackID(i)
		}
		return sbtObjID(uint8(x))
	}
	type opt struct {
		o   decision.Option
		act dirac.Action
		key uint32
	}
	var os []opt
	hasDone, unlessPay := false, false
	for i := 0; i < int(d.N); i++ {
		x := d.Options[i]
		c := r.canon(x)
		k := c.Kind()
		o := decision.Option{Obj: ref(c.Src())}
		zkey := func(obj Ref) uint32 {
			if obj >= RefStack || int(obj) >= int(r.kp.NObj) {
				return uint32(obj)
			}
			po := &r.kp.Objs[obj]
			return uint32(po.Zone)<<16 | uint32(po.Ctrl)<<8 | uint32(po.ZoneIdx)
		}
		key := uint32(i)
		switch {
		case k == ActDone:
			hasDone = true
			continue
		case k == ActAttack:
			o.Kind = "attacker"
			if c.Tgt() >= RefPlayer {
				o.Player = state.PlayerID(c.Tgt() - RefPlayer)
				key = uint32(o.Player)<<24 | zkey(c.Src())
			} else {
				o.Battle = ref(c.Tgt())
				if int(c.Tgt()) < int(r.kp.NObj) {
					o.Player = state.PlayerID(r.kp.Objs[c.Tgt()].Ctrl)
				}
				key = uint32(o.Player)<<24 | zkey(c.Src())<<4 | 1
			}
			o.Required = s.MustAttack(dirac.Obj(c.Src()))
		case k == ActBlock:
			o.Kind = "block"
			o.Attacker = ref(c.Tgt())
			o.Player = state.PlayerID(me)
			o.Group = "blocker:" + strconv.FormatUint(uint64(o.Obj), 10)
			if s.HasKeyword(dirac.Obj(c.Tgt()), kop.KwMenace) {
				o.MinBlockers = 2
			}
			key = zkey(c.Src())<<8 | zkey(c.Tgt())&0xFF
		case k == ActTarget:
			t := c.Tgt()
			switch {
			case t >= RefPlayer:
				o.Kind, o.Player, o.Obj = "player", state.PlayerID(t-RefPlayer), 0
				key = uint32(t - RefPlayer)
			case t >= RefStack:
				o.Kind, o.Obj = "spell", ref(t)
				key = 1<<24 | uint32(t-RefStack)
			default:
				o.Obj = ref(t)
				o.Kind = "card"
				if int(t) < int(r.kp.NObj) {
					po := &r.kp.Objs[t]
					o.Controller = state.PlayerID(po.Owner)
					if po.Zone == kop.ZoneBattlefield {
						o.Kind, o.Controller = "permanent", state.PlayerID(po.Ctrl)
					}
					o.Player = o.Controller // gorge's target option names the object's controller
				}
				key = 1<<24 | 1<<20 | zkey(t)
			}
		case k == ActMode:
			o.Kind = "mode"
			key = uint32(c.Mode())
			if d.Sub == dirac.ChWardPay || d.Sub == dirac.ChGenPayMode {
				// gorge's unless-pay ask (ward included): pay / decline
				// marked by Mode, ResumeKind "unless_pay"
				o.Player = state.PlayerID(me)
				o.Obj = 0
				if x.Mode() == 1 {
					o.Mode, o.Label = decision.ModeUnlessPay, "Pay the cost"
				} else {
					o.Mode, o.Label = decision.ModeUnlessDecline, "Don't pay"
				}
				unlessPay = true
			}
		case k == ActOrder:
			o.Kind = "trigger"
			key = uint32(x.Target())
		case k == ActYes || k == ActNo:
			o.Kind = "yes"
			key = 0
			if k == ActNo {
				o.Kind, key = "no", 1
			}
		case k == ActArrange:
			o.Kind = "arrange"
		case k == ActReplacement:
			o.Kind = "replacement"
		case k == ActBottom:
			o.Kind = "bottom"
		case k == ActKeep:
			o.Kind = "keep"
		case k == ActCommanderZone:
			// CR 903.9a: gorge's command_zone (mode 0) / leave (mode 1)
			o.Kind = "command_zone"
			key = uint32(c.Mode())
			if c.Mode() == 1 {
				o.Kind = "leave"
			}
		case k == ActMulligan:
			o.Kind = "mulligan"
		case k >= ActChoose:
			ci := int(k - ActChoose)
			o.Kind = "other"
			if ci < len(chooseKinds) {
				o.Kind = chooseKinds[ci]
			}
			if o.Obj == 0 && c.Tgt() >= RefPlayer && c.Tgt() != RefNone {
				o.Player = state.PlayerID(c.Tgt() - RefPlayer)
			}
			switch o.Kind {
			case "x", "number":
				o.Amount = int(c.Mode())
			case "mana", "color":
				if int(c.Mode()) < 6 {
					o.ManaSymbol = sbtManaSyms[c.Mode()]
				}
			case "pay":
				o.Kind = "pay_" + [8]string{"W", "U", "B", "R", "G", "C", "generic", "life"}[c.Mode()&7]
			}
			key = a.chooseKey(s, d, x, zkey)
		}
		os = append(os, opt{o: o, act: x, key: key})
	}
	for i := 1; i < len(os); i++ {
		for j := i; j > 0 && os[j].key < os[j-1].key; j-- {
			os[j], os[j-1] = os[j-1], os[j]
		}
	}
	for i := range os {
		os[i].o.Index = i
		gd.Options = append(gd.Options, os[i].o)
		acts = append(acts, os[i].act)
	}
	n := len(gd.Options)
	switch dk {
	case DAttackers:
		gd.Kind, gd.Min, gd.Max = decision.KAttackers, 0, n
		gd.Prompt = "declare attackers"
	case DBlockers:
		gd.Kind, gd.Min, gd.Max = decision.KBlockers, 0, n
		gd.Prompt = "declare blockers"
	case DTarget:
		gd.Kind = decision.KTarget
		if hasDone {
			gd.Min = 0
		}
		a.targetEffect(s, gd)
	case DModes:
		gd.Kind = decision.KModes
		if unlessPay {
			gd.ResumeKind, gd.ResumeSA, gd.Prompt = "unless_pay", sbtUnlessSA, "Pay the cost, or decline"
		}
		if n > 0 && gd.Options[0].Kind == "discard" {
			gd.Prompt = "Choose a card to discard"
		}
	case DTriggerOrder:
		gd.Kind, gd.Min, gd.Max = decision.KTriggerOrder, n, n
	case DTriggerOptional:
		gd.Kind = decision.KTriggerOptional
	case DArrange:
		gd.Kind = decision.KArrange
	case DReplacement:
		gd.Kind = decision.KReplacement
	case DCommanderZone:
		gd.Kind, gd.Prompt = decision.KCommanderZone, "Put this commander into the command zone?"
	case DMulligan:
		gd.Kind = decision.KMulligan
	default:
		gd.Kind = decision.KChoose
		if n > 0 {
			switch gd.Options[0].Kind {
			case "discard":
				gd.Prompt = "Discard a card"
			case "sacrifice":
				// sb-tactical answers a cost's sacrifice ("Sacrifice a
				// permanent to ..."), gorge's default policy an effect's
				gd.Prompt = "Choose 1 permanent(s) to sacrifice"
				if d.Sub == dirac.ChCostSacrifice {
					gd.Prompt = "Sacrifice a permanent to pay the cost"
				}
			}
		}
		if hasDone {
			gd.Min = 0
		}
	}
	if hasDone {
		gd.Min = 0
	}
	if gd.Min > n {
		gd.Min = n
	}
	return gd, acts
}

// chooseKey is a DkChoose option's place in gorge's option order (the
// dirac/feat listpos rule).
func (a *SBT) chooseKey(s *dirac.State, d *dirac.Decision, x dirac.Action, zkey func(Ref) uint32) uint32 {
	src, mode := x.Source(), x.Mode()
	switch d.Sub {
	case dirac.ChOptional, dirac.ChWardPay, dirac.ChOptDiscMay, dirac.ChGenMayShuffle, dirac.ChGenPay, dirac.ChGenPayMode,
		dirac.ChGenTopBottom, dirac.ChGenLeyline, dirac.ChGenDiscUnless:
		return 1 - uint32(mode&1) // yes / pay / top / unless first
	case dirac.ChGenDivide, dirac.ChCastDivide:
		return uint32(s.FeatDivideIndex(uint8(src)))
	case dirac.ChX, dirac.ChTrigX, dirac.ChMultikick, dirac.ChDivide, dirac.ChGenBranch, dirac.ChGenPileChoose, dirac.ChGenColor, dirac.ChGenManaChoice, dirac.ChAltAdd, dirac.ChGenPip, dirac.ChGenReplOrder, dirac.ChGenGift, dirac.ChCostForage, dirac.ChGenKeepTapped, dirac.ChGenDestAlt:
		return uint32(mode)
	case dirac.ChGenType:
		return s.FeatTypeKey(d.Player, int(x.Target())<<8|int(mode))
	case dirac.ChGenChooseCard, dirac.ChGenFaceDownPick, dirac.ChGenCastFree:
		return uint32(src)
	}
	if src == dirac.TCode(uint8(dirac.NoObj)) || src >= dirac.PlayerTarget0 {
		return uint32(mode)
	}
	return zkey(Ref(src))
}

// targetEffect fills a KTarget decision's Source and TargetEffect: the
// object being cast or activated (or the resolving entry's) and the gorge
// SA of the target slot asked (the k-th targeted SA of the ability's chain).
func (a *SBT) targetEffect(s *dirac.State, gd *decision.Decision) {
	src := s.FeatSource()
	if src != dirac.NoObj {
		gd.Source = sbtObjID(uint8(src))
	}
	card, ab, _, ok := s.Building()
	if !ok {
		return
	}
	var sa *cards.SA
	abs := a.c.tbl.Cards
	activated := int(card) < len(abs) && int(ab) < len(abs[card].Abilities) &&
		(abs[card].Abilities[ab].Kind == kop.AbActivated || abs[card].Abilities[ab].Kind == kop.AbMana)
	defer func() {
		// gorge's cast-time ask names no source for an activated ability
		// (the ability object is not minted yet; rules/cast_targets.go),
		// an Attach (equip) excepted
		if activated && (gd.TargetEffect == nil || gd.TargetEffect.API != "Attach") {
			gd.Source = 0
		}
	}()
	if int(card) < len(abs) && int(ab) < len(abs[card].Abilities) && abs[card].Abilities[ab].Kind == kop.AbSpell {
		sa = a.c.gorgeSA(card, 255)
	} else if gi, ok := a.c.gorgeAbility(card, ab); ok {
		sa = a.c.gorgeSA(card, gi)
	} else if ab == 255 {
		sa = a.c.gorgeSA(card, 255)
	}
	// a modal spell's chain is its chosen modes' (in mode order)
	var chain []*cards.SA
	sc := a.c.card(card)
	for x := sa; x != nil; x = x.Sub {
		if (x.API == "Charm" || x.API == "GenericChoice") && sc != nil && sc.gc != nil {
			f := sc.gc.Faces[0]
			modes := s.BuildModes()
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
	}
	k := 0
	for i, x := range chain {
		if _, ok := x.Params["ValidTgts"]; !ok {
			if _, ok := x.Params["TgtPrompt"]; !ok {
				continue
			}
		}
		if k >= a.tgtSlot && sbtSlotFits(x, gd) || i == len(chain)-1 {
			gd.TargetEffect = sbtTargetEffect(x)
			return
		}
		k++
	}
	if sa != nil {
		gd.TargetEffect = sbtTargetEffect(sa)
	}
}

// sbtUnlessSA stands for the resolving ability of an unless-pay ask (gorge's
// default policy reads only that one is there).
var sbtUnlessSA = &cards.SA{}

// sbtSlotFits reports whether a targeted SA's ValidTgts$ admits the offered
// targets' controllers (the asked slot's: the kernel elides a single-option
// slot, so the agent cannot count slots alone).
func sbtSlotFits(x *cards.SA, gd *decision.Decision) bool {
	valid := x.Params["ValidTgts"]
	if valid == "" || len(gd.Options) == 0 {
		return true
	}
	for _, o := range gd.Options {
		ok := false
		for _, alt := range strings.Split(valid, ",") {
			alt = strings.TrimSpace(alt)
			typ, quals, _ := strings.Cut(alt, ".")
			if o.Kind == "player" {
				if typ != "Player" && typ != "Opponent" && typ != "Any" {
					continue
				}
			} else if typ == "Player" || typ == "Opponent" {
				continue
			}
			mine := o.Player == gd.Player
			fits := true
			for _, q := range strings.Split(quals, "+") {
				switch q {
				case "YouCtrl", "YouOwn", "You":
					fits = fits && mine
				case "OppCtrl", "OppOwn", "Opponent":
					fits = fits && !mine
				}
			}
			if typ == "Opponent" {
				fits = fits && !mine
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

// sbtTargetEffect is gorge's describeTargetEffect for sa without the
// engine: the API, the removal shape (rules targetRemoval) and a literal
// damage amount (a dynamic one is left unknown).
func sbtTargetEffect(sa *cards.SA) *decision.TargetEffect {
	te := &decision.TargetEffect{API: sa.API}
	switch sa.APIKind() {
	case cards.APIDestroy, cards.APIDestroyAll:
		te.Removal = &decision.RemovalEffect{Kind: "destroy"}
	case cards.APISacrifice, cards.APISacrificeAll:
		te.Removal = &decision.RemovalEffect{Kind: "sacrifice"}
	case cards.APIChangeZone:
		if cz := effects.ChangeZoneOf(sa); cz.DestinationKnown {
			if k := sbtRemovalKind(strings.ToLower(cz.Destination.String())); k != "" {
				te.Removal = &decision.RemovalEffect{Kind: k, Destination: cz.Destination.String()}
			}
		}
	case cards.APIChangeZoneAll:
		dest := effects.ChangeZoneAllOf(sa).DestinationLower
		if k := sbtRemovalKind(dest); k != "" {
			te.Removal = &decision.RemovalEffect{Kind: k, Destination: dest}
		}
	case cards.APIDealDamage, cards.APIDamageAll:
		te.Damage = &decision.DamageEffect{}
		if amt := effects.DamageAmount(sa); amt.Present {
			if n, err := strconv.Atoi(strings.TrimSpace(amt.Text)); err == nil && n >= 0 {
				te.Damage.Amount = &n
			}
		}
	}
	return te
}

func sbtRemovalKind(dest string) string {
	switch dest {
	case "exile":
		return "exile"
	case "hand":
		return "bounce"
	case "graveyard":
		return "graveyard"
	case "library":
		return "library"
	case "command":
		return "command"
	}
	return ""
}
