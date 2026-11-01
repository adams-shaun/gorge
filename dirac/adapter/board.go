package adapter

import (
	"strings"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/canon"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// board is the game a client's view is projected from: a read-only image of the
// kernel's position, rebuilt after each answer. No game runs on it and nothing
// replays it, so it is written here directly and in no other file.
type board struct {
	g     *state.Game
	game  *Game
	facts []objFacts // by id-1
	zone  [2][7][]state.ObjID
	prio  state.PlayerID
	step  kop.Step
	seen  bool
	// landOpen is the active seat's land drop as of now, and the other seat's as of
	// the end of its own turn, as gorge's count stands until its next untap.
	landOpen [2]bool
}

// objFacts are the derived characteristics the view asks for, by object.
type objFacts struct {
	power, tough int32
	keywords     []string
	types        []string
	retyped      bool
}

var stepOf = [...]state.Step{
	kop.StepUntap: state.StepUntap, kop.StepUpkeep: state.StepUpkeep, kop.StepDraw: state.StepDraw, kop.StepMain1: state.StepMain1,
	kop.StepBeginCombat: state.StepBeginCombat, kop.StepAttackers: state.StepDeclareAttackers, kop.StepBlockers: state.StepDeclareBlockers,
	kop.StepFirstStrikeDamage: state.StepCombatDamage, kop.StepDamage: state.StepCombatDamage, kop.StepEndCombat: state.StepEndCombat,
	kop.StepMain2: state.StepMain2, kop.StepEnd: state.StepEnd, kop.StepCleanup: state.StepCleanup,
}

var counterNames = func() [kop.NumCounterKinds2]string {
	var n [kop.NumCounterKinds2]string
	n[kop.CtrP1P1], n[kop.CtrM1M1], n[kop.CtrLoyalty], n[kop.CtrStun] = "P1P1", "M1M1", "LOYALTY", "STUN"
	n[kop.CtrShield], n[kop.CtrFlying], n[kop.CtrOther] = "SHIELD", "Flying", "OTHER"
	n[kop.CtrIncubation], n[kop.CtrBait], n[kop.CtrSoul] = "INCUBATION", "BAIT", "SOUL"
	n[kop.CtrFellowship], n[kop.CtrRevival], n[kop.CtrStash] = "FELLOWSHIP", "REVIVAL", "STASH"
	return n
}()

// materialise rebuilds the board from the position settle read.
func (g *Game) materialise() {
	b := &g.board
	if b.g == nil {
		names := g.cfg.Names
		if len(names) == 0 {
			names = []string{"p0", "p1"}
		}
		b.g = state.NewGameLife(names, int32(g.kp.Life[0]))
		b.landOpen = [2]bool{true, true}
		for i := range b.g.Players {
			if i < len(g.cfg.PlayerNames) {
				b.g.Players[i].PlayerName = g.cfg.PlayerNames[i]
			}
		}
	}
	b.game = g
	gm := b.g
	kp := &g.kp
	g.s.ProjectChars(kp)
	for int(g.ids.next)-1 > len(gm.Objs) {
		gm.AddObject(nil, 0).Zone = state.ZCeased
	}
	gm.NextID = g.ids.next
	if n := int(g.ids.next) - 1; cap(b.facts) < n {
		b.facts = make([]objFacts, n, n+32)
	} else {
		b.facts = b.facts[:n]
	}
	g.boardGame()
	for z := range b.zone[0] {
		b.zone[0][z], b.zone[1][z] = b.zone[0][z][:0], b.zone[1][z][:0]
	}
	for i := 0; i < int(kp.NObj); i++ {
		g.boardObject(uint8(i))
	}
	g.boardZones()
	g.boardStack()
	g.boardCombat()
}

func (g *Game) boardGame() {
	b := &g.board
	gm, kp := b.g, &g.kp
	gm.Turn = int32(kp.Turn)
	gm.Step = stepOf[kp.Step]
	gm.Active = state.PlayerID(kp.Active)
	// gorge moves priority to the active player as a step begins, and leaves it with
	// the last player who held it through any other ask.
	switch {
	case g.d.Kind == dirac.DkPriority:
		b.prio = state.PlayerID(g.d.Player)
	case kp.Step != b.step || !b.seen:
		b.prio = state.PlayerID(kp.Active)
	}
	b.step, b.seen = kp.Step, true
	gm.Priority = b.prio
	gm.Over, gm.Winner, gm.Draw = kp.Over, 0, false
	if kp.Over {
		if kp.Winner >= 0 {
			gm.Winner = state.PlayerID(kp.Winner)
		} else {
			gm.Draw = true
		}
	}
	if g.conceded {
		gm.Over, gm.Winner, gm.Draw = true, state.PlayerID(1-g.loser), false
	}
	gm.DayNight = state.DayNight(g.s.DayNight())
	for p := range gm.Players {
		pl := &gm.Players[p]
		pl.Life = int32(kp.Life[p])
		if uint8(p) == kp.Active && kp.Turn > 0 {
			b.landOpen[p] = g.s.FeatLandDropOpen(uint8(p))
		}
		pl.Lost = gm.Over && !gm.Draw && gm.Winner != state.PlayerID(p)
		pl.LandsPlayed = 0
		if uint8(p) == kp.Active {
			pl.LandsPlayed = int32(kp.Lands[p])
		}
		for k := 0; k < 6; k++ {
			pl.Pool[k] = int32(kp.Pool[p][k])
		}
		pl.Counters = pl.Counters[:0]
		if n := g.s.Poison(uint8(p)); n > 0 {
			pl.Counters = append(pl.Counters, state.Counter{Kind: "POISON", N: int32(n)})
		}
	}
}

// boardObject writes kernel object o into the board.
func (g *Game) boardObject(i uint8) {
	b := &g.board
	po := &g.kp.Objs[i]
	id := g.ids.obj(i)
	o := &b.g.Objs[id-1]
	printed := po.Card
	if po.Flags&dirac.ProjBase != 0 {
		printed = po.Base
	}
	card := g.card(printed)
	*o = state.Object{ID: id, Card: card, Owner: state.PlayerID(po.Owner), Controller: state.PlayerID(po.Ctrl),
		Zone: zoneOf(po.Zone), Tapped: po.Flags&dirac.ProjTapped != 0, SummonSick: po.Flags&dirac.ProjSick != 0,
		FaceDown: po.Flags&dirac.ProjFaceDown != 0, PhasedOut: po.Zone >= kop.ZonePhased, Damage: int32(po.Damage),
		IsToken: po.Token, IsCopy: po.Flags&dirac.ProjCopy != 0, IsAttacking: po.Flags&dirac.ProjAttacking != 0,
		Monstrous: po.Flags&dirac.ProjMonstrous != 0, Suspected: po.Flags2&dirac.ProjSuspected != 0,
		ActivatedThisTurn: int32(g.s.FeatActivations(dirac.Obj(i))), FaceIdx: g.tbl.Cards[printed].Face}
	if card != nil && printed != po.Card {
		o.CopyFace = g.face(po.Card)
	}
	o.SyncFace()
	if po.Attached != dirac.NoObj {
		o.AttachedTo = g.ids.obj(uint8(po.Attached))
	}
	if o.IsAttacking && po.Attacks >= dirac.PlayerTarget0 && po.Attacks != dirac.NoTarget {
		o.Attacking = state.PlayerID(po.Attacks - dirac.PlayerTarget0)
	} else if o.IsAttacking && dirac.IsObjTarget(po.Attacks) {
		o.Attacking = state.PlayerID(g.kp.Objs[po.Attacks].Ctrl)
	}
	if o.Zone != state.ZBattlefield && g.s.FeatSickOff(dirac.Obj(i)) {
		o.SummonSick = true
	}
	for k, n := range po.Counters {
		if n != 0 && counterNames[k] != "" {
			o.AddCounter(counterNames[k], int32(n))
		}
	}
	for x, n := range po.CountersX {
		if n != 0 {
			o.AddCounter(kop.CtrXNames[x], int32(n))
		}
	}
	g.objFacts(i, id)
	if o.Zone != state.ZCeased && o.Zone != state.ZStack {
		seat := po.Owner
		if o.Zone == state.ZBattlefield {
			seat = po.Ctrl
		}
		b.place(o.Zone, seat, i, id)
	}
}

func zoneOf(z kop.Zone) state.Zone {
	switch z {
	case kop.ZoneLibrary:
		return state.ZLibrary
	case kop.ZoneHand:
		return state.ZHand
	case kop.ZoneBattlefield, kop.ZonePhased, kop.ZonePhasedInd:
		return state.ZBattlefield
	case kop.ZoneGraveyard:
		return state.ZGraveyard
	case kop.ZoneExile:
		return state.ZExile
	case kop.ZoneStack:
		return state.ZStack
	}
	return state.ZCeased
}

// place records id at its position in its zone; boardZones orders them.
func (b *board) place(z state.Zone, seat, i uint8, id state.ObjID) {
	list := b.zone[seat&1][z]
	b.zone[seat&1][z] = append(list, id)
}

// boardZones hands each zone over in the kernel's order.
func (g *Game) boardZones() {
	b := &g.board
	for seat := range b.zone {
		for _, z := range []state.Zone{state.ZLibrary, state.ZHand, state.ZBattlefield, state.ZGraveyard, state.ZExile} {
			ids := b.zone[seat][z]
			for i := 1; i < len(ids); i++ {
				for j := i; j > 0 && g.zoneIdx(ids[j]) < g.zoneIdx(ids[j-1]); j-- {
					ids[j], ids[j-1] = ids[j-1], ids[j]
				}
			}
			b.g.SetZone(z, state.PlayerID(seat), ids)
		}
	}
}

// zoneIdx is an object's position in its zone.
func (g *Game) zoneIdx(id state.ObjID) uint8 {
	for i := 0; i < int(g.kp.NObj); i++ {
		if g.ids.objs[i] == id {
			return g.kp.Objs[i].ZoneIdx
		}
	}
	return 0
}

// boardStack writes the stack, bottom to top, and each ability's own object.
func (g *Game) boardStack() {
	b := &g.board
	gm, kp := b.g, &g.kp
	gm.Stack = gm.Stack[:0]
	_, _, building := g.s.BuildingCast()
	for i := 0; i < int(kp.NStack); i++ {
		e := &kp.Stack[i]
		var o *state.Object
		if e.Kind == 0 {
			id := g.ids.obj(uint8(e.Source))
			o = &gm.Objs[id-1]
			gm.Stack = append(gm.Stack, id)
			if sa := g.stackSA(e); sa != latentSA {
				o.ChosenModes = g.chosenModes(e, sa)
			}
		} else {
			id := g.ids.stack[i]
			o = &gm.Objs[id-1]
			*o = state.Object{ID: id, Zone: state.ZStack, Controller: state.PlayerID(e.Ctrl), Owner: state.PlayerID(e.Ctrl),
				Source: g.ids.obj(uint8(e.Source)), Ability: g.stackSA(e), StackKindKnown: true, StackKind: state.StackKindActivated}
			if e.Kind == 2 {
				o.StackKind = state.StackKindTriggered
			}
			o.ChosenModes = g.chosenModes(e, o.Ability)
			gm.Stack = append(gm.Stack, id)
		}
		o.Controller = state.PlayerID(e.Ctrl)
		if g.s.FeatStackKicked(i) {
			o.CastFlags |= state.FlagKicked
		}
		// gorge writes X as the cast commits, not as it is announced
		if !building || i < int(kp.NStack)-1 {
			o.X = int32(e.X)
		}
		o.Targets = o.Targets[:0]
		for si, t := range e.Targets {
			if t != dirac.NoTarget {
				o.Targets = append(o.Targets, g.stateTarget(t))
			}
			for j := 0; j < int(e.NExtra); j++ {
				if int(e.ExtraSlot[j]) == si {
					o.Targets = append(o.Targets, g.stateTarget(e.Extra[j]))
				}
			}
		}
	}
}

// latentSA stands for an ability no face lists, such as a reflexive trigger's: it is
// on the stack, and has no printed text to show.
var latentSA = &cards.SA{}

// stackSA is the spell ability a stack entry is resolving.
func (g *Game) stackSA(e *dirac.ProjStack) *cards.SA {
	srcCard := e.Card
	if int(e.Source) < int(g.kp.NObj) {
		srcCard = g.kp.Objs[e.Source].Card
	}
	if gi := g.tr.StackAbility(srcCard, e.Card, e.Ability); gi != 255 {
		if sa := g.faceSA(e.Card, gi); sa != nil {
			return sa
		}
	}
	return latentSA
}

// chosenModes names a modal entry's chosen modes, in execution order.
func (g *Game) chosenModes(e *dirac.ProjStack, sa *cards.SA) []string {
	if sa == nil || sa.API != "Charm" && sa.API != "GenericChoice" || int(e.Card) >= len(g.tbl.Cards) || int(e.Ability) >= len(g.tbl.Cards[e.Card].Abilities) {
		return nil
	}
	bits := e.Mode & uint8(1<<g.tbl.Cards[e.Card].Abilities[e.Ability].NModes-1)
	var out []string
	for i, name := range strings.Split(sa.Params["Choices"], ",") {
		if i < 8 && bits&(1<<i) != 0 {
			out = append(out, strings.TrimSpace(name))
		}
	}
	return out
}

func (g *Game) stateTarget(t uint8) state.Target {
	switch {
	case t == dirac.StackTargetGone:
		return state.Target{}
	case t >= canon.RefPlayer:
		return state.Target{Player: state.PlayerID(t - canon.RefPlayer), IsPlayer: true}
	}
	return state.Target{Obj: g.ids.ref(t, &g.kp)}
}

// boardCombat lists each attacker's blockers.
func (g *Game) boardCombat() {
	gm, kp := g.board.g, &g.kp
	for i := 0; i < int(kp.NObj); i++ {
		po := &kp.Objs[i]
		if po.Zone == kop.ZoneBattlefield && po.Flags&dirac.ProjBlocking != 0 && po.Blocks != dirac.NoObj {
			a := &gm.Objs[g.ids.obj(uint8(po.Blocks))-1]
			a.BlockedBy = append(a.BlockedBy, g.ids.obj(uint8(i)))
		}
	}
}
