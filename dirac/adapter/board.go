package adapter

import (
	"cmp"
	"slices"
	"strings"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/canon"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/rules/scriptfacts"
	"github.com/adams-shaun/gorge/engine/state"
)

// board is the game a client's view is projected from: a read-only image of the
// kernel's position, rebuilt after each answer. No game runs on it and nothing
// replays it, so it is written here directly and in no other file.
type board struct {
	g       *state.Game
	game    *Game
	facts   []objFacts // by id-1
	pending []state.PendingTrigger
	// blockers[o] is who has blocked kernel object o this combat.
	blockers [dirac.MaxObj][]state.ObjID
	zone     [2][7][]state.ObjID
	prio     state.PlayerID
	step     kop.Step
	seen     bool
	nstack   uint8
	// resolving: the last decision was asked inside a resolution.
	resolving bool
	// divided: the last decision divided combat damage, so that damage has been dealt.
	divided bool
	// landOpen is each seat's land drop as of now: gorge counts a seat's lands until its
	// next turn (Game.landsPlayed).
	landOpen [2]bool
}

// objFacts are the derived characteristics the view asks for, by object.
type objFacts struct {
	power, tough int32
	name         string
	effective    string
	keywords     []string
	types        []string
	retyped      bool
	optional     bool
	decider      state.PlayerID
}

var stepOf = [...]state.Step{
	kop.StepUntap: state.StepUntap, kop.StepUpkeep: state.StepUpkeep, kop.StepDraw: state.StepDraw, kop.StepMain1: state.StepMain1,
	kop.StepBeginCombat: state.StepBeginCombat, kop.StepAttackers: state.StepDeclareAttackers, kop.StepBlockers: state.StepDeclareBlockers,
	kop.StepFirstStrikeDamage: state.StepCombatDamage, kop.StepDamage: state.StepCombatDamage, kop.StepEndCombat: state.StepEndCombat,
	kop.StepMain2: state.StepMain2, kop.StepEnd: state.StepEnd, kop.StepCleanup: state.StepCleanup,
}

// materialise rebuilds the board from the position settle read.
func (g *Game) materialise() {
	b := &g.board
	if b.g == nil {
		names := g.cfg.Names
		if len(names) == 0 {
			names = []string{"p0", "p1"}
		}
		b.g = state.NewGameLife(names, int32(g.kp.Life[0]))
		b.g.Tokens = g.eng.reg.Tokens
		for i := range b.g.Players {
			if i < len(g.cfg.PlayerNames) {
				b.g.Players[i].PlayerName = g.cfg.PlayerNames[i]
			}
		}
	}
	if !b.seen {
		b.landOpen = [2]bool{true, true}
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
	g.boardRenames()
	g.boardZones()
	g.boardStack()
	g.boardCombat()
	g.boardPending()
}

// boardPending lists the triggers waiting to go on the stack: those an answered
// order has settled, in that order, then the rest.
func (g *Game) boardPending() {
	b := &g.board
	b.pending = b.pending[:0]
	ordered := g.s.PendingOrdered()
	for _, i := range ordered {
		g.addPending(int(i))
	}
	for i := 0; i < g.s.NumPending(); i++ {
		if !slices.Contains(ordered, uint8(i)) {
			g.addPending(i)
		}
	}
}

func (g *Game) addPending(i int) {
	src, card, ab, ctrl, _ := g.s.PendingTrigger(i)
	pt := state.PendingTrigger{Source: g.ids.obj(uint8(src)), Controller: state.PlayerID(ctrl), Label: g.cardName(uint8(src))}
	if gi, ok := g.tr.Origin(card, ab); ok {
		if g.triggerOptional(card, gi) {
			pt.Optional, pt.Decider = true, state.PlayerID(ctrl)
		}
		// a delayed trigger has no line of its own: gorge labels it with the card's first
		if gi == 255 {
			gi = 0x80
		}
		if d := g.triggerText(card, gi); d != "" {
			pt.Label += ": " + d
		}
	}
	g.board.pending = append(g.board.pending, pt)
}

// triggerOptional reads a card's trigger for the "you may" a resolution asks. The
// kernel lowers some of them to a payment, so its own flag does not say.
func (g *Game) triggerOptional(c dirac.CardID, gi uint8) bool {
	f := g.face(c)
	if f == nil || gi&0x80 == 0 || int(gi&^0x80) >= len(f.Triggers) {
		return false
	}
	return scriptfacts.TriggerOptionalSpec(f.Triggers[gi&^0x80]) != ""
}

// triggerText is a card's trigger's own description.
func (g *Game) triggerText(c dirac.CardID, gi uint8) string {
	f := g.face(c)
	if f == nil || gi&0x80 == 0 || int(gi&^0x80) >= len(f.Triggers) {
		return ""
	}
	return f.Triggers[gi&^0x80].ParamStr(cards.PKTriggerDescription)
}

// noteTally counts this turn's casts and tokens: the kernel keeps them only for a card
// that reads them, and the view shows them for every seat.
func (g *Game) noteTally(before *[dirac.MaxObj]kop.Zone, turn int32) {
	if turn != g.tallyTurn {
		g.tally, g.tallyTurn = [2]turnTally{}, turn
	}
	for o := 0; o < int(g.kp.NObj); o++ {
		po := &g.kp.Objs[o]
		switch {
		case po.Zone == kop.ZoneStack && before[o] != kop.ZoneStack:
			if !g.s.FeatStackCopy(dirac.Obj(o)) { // not cast (CR 707.10)
				g.tally[po.Ctrl&1].spells++
			}
		case g.building == dirac.Obj(o) && before[o] == kop.ZoneStack && po.Zone != kop.ZoneStack && g.tally[po.Ctrl&1].spells > 0:
			g.tally[po.Ctrl&1].spells-- // the proposal was reversed
		case po.Token && before[o] == kop.ZoneNone && po.Zone != kop.ZoneNone:
			g.tally[po.Ctrl&1].tokens++
		}
	}
	g.building = dirac.NoObj
	if _, _, _, ok := g.s.Building(); ok {
		g.building = g.s.FeatSource()
	}
}

// boardRenames gives a permanent the name an Aura or Equipment attached to it sets.
func (g *Game) boardRenames() {
	b := &g.board
	for i := 0; i < int(g.kp.NObj); i++ {
		po := &g.kp.Objs[i]
		if po.Zone != kop.ZoneBattlefield || po.Attached == dirac.NoObj || g.kp.Objs[po.Attached].Zone != kop.ZoneBattlefield {
			continue // an Aura whose host has left waits on state-based actions, and sets it nothing
		}
		f := g.face(po.Card)
		if f == nil {
			continue
		}
		for j := range f.Statics {
			st := &f.Statics[j]
			host := &b.facts[g.ids.obj(uint8(po.Attached))-1]
			if name := st.ParamStr(cards.PKSetName); name != "" {
				host.name = name
			}
			if added := st.ParamStr(cards.PKAddType); added != "" && host.retyped {
				orderAdded(host.types, strings.Split(added, " & "))
			}
		}
	}
}

// orderAdded moves the words an effect added to the end of a type line, in the effect's
// own order: gorge's line reads what stayed, then what was added.
func orderAdded(types, added []string) {
	kept := types[:0:0]
	for _, w := range types {
		if !slices.Contains(added, w) {
			kept = append(kept, w)
		}
	}
	for _, w := range added {
		if slices.Contains(types, w) {
			kept = append(kept, w)
		}
	}
	copy(types, kept)
}

// noteExiled follows what the entry that was resolving exiled: the kernel keeps this
// link only for a card that reads it, and the view shows it for every card. A card
// leaves every list as it leaves exile, and a source entering the battlefield other
// than from the stack starts over.
func (g *Game) noteExiled(before *[dirac.MaxObj]kop.Zone, top *dirac.ProjStack) {
	if g.exiled == nil {
		g.exiled = make([][]state.ObjID, dirac.MaxObj)
	}
	src, targetedOnly := -1, false
	if top != nil {
		if exiles, targeted := g.resolvedExile(top); exiles {
			src, targetedOnly = int(top.Source), targeted
		}
	}
	from := 0
	if src >= 0 {
		from = len(g.exiled[src])
	}
	for o := 0; o < int(g.kp.NObj); o++ {
		po := &g.kp.Objs[o]
		switch {
		case po.Zone == kop.ZoneExile && before[o] != kop.ZoneExile:
			if src >= 0 && !po.Token && (!targetedOnly || targets(top, uint8(o))) {
				g.exiled[src] = append(g.exiled[src], g.ids.obj(uint8(o)))
			}
		case po.Zone != kop.ZoneExile && before[o] == kop.ZoneExile:
			id := g.ids.obj(uint8(o))
			for s := range g.exiled {
				g.exiled[s] = slices.DeleteFunc(g.exiled[s], func(x state.ObjID) bool { return x == id })
			}
		}
		ver := g.s.ObjVer(dirac.Obj(o))
		// a permanent that left and came back within the answer has moved all the same
		returned := before[o] == kop.ZoneBattlefield && g.vers[o] != ver
		if po.Zone == kop.ZoneBattlefield && (returned || before[o] != kop.ZoneBattlefield && before[o] != kop.ZoneStack) {
			g.exiled[o] = g.exiled[o][:0]
		}
		g.vers[o] = ver
	}
	if src >= 0 && from < len(g.exiled[src]) {
		slices.SortFunc(g.exiled[src][from:], func(a, b state.ObjID) int { return int(g.zoneIdx(a)) - int(g.zoneIdx(b)) })
	}
}

// targets reports whether a stack entry targets object o.
func targets(e *dirac.ProjStack, o uint8) bool {
	return slices.Contains(e.Targets[:], dirac.TCode(o)) || slices.Contains(e.Extra[:e.NExtra], dirac.TCode(o))
}

// resolvedExile reports whether a stack entry's ability moves objects to exile through
// ChangeZone, and whether only what it targets: anything else that reached exile in
// that resolution went by another rule, such as a replacement on a death it caused.
func (g *Game) resolvedExile(e *dirac.ProjStack) (exiles, targeted bool) {
	sa := g.stackSA(e)
	if sa == latentSA {
		return false, false
	}
	f := g.face(e.Card)
	targeted = true
	var walk func(x *cards.SA)
	walk = func(x *cards.SA) {
		for ; x != nil; x = x.Sub {
			if x.APIKind() == cards.APIChangeZone {
				if cz := effects.ChangeZoneOf(x); cz.DestinationKnown && cz.Destination == state.ZExile {
					exiles = true
					targeted = targeted && x.Params["ValidTgts"] != ""
				}
			}
			if x.APIKind() == cards.APIChangeZoneAll && effects.ChangeZoneAllOf(x).DestinationLower == "exile" {
				exiles, targeted = true, false
			}
			if (x.API == "Charm" || x.API == "GenericChoice") && f != nil {
				for _, name := range strings.Split(x.Params["Choices"], ",") {
					walk(cards.ResolveSVar(f.SVars, strings.TrimSpace(name)))
				}
			}
		}
	}
	walk(sa)
	return exiles, targeted
}

func (g *Game) boardGame() {
	b := &g.board
	gm, kp := b.g, &g.kp
	gm.Turn = int32(kp.Turn)
	gm.Step = stepOf[kp.Step]
	gm.Active = state.PlayerID(kp.Active)
	// gorge moves priority to the active player once all have passed, and leaves it
	// with the last player who held it through any other ask. It starts with the first
	// seat, whoever will take the first turn.
	switch {
	case g.d.Kind == dirac.DkPriority:
		b.prio = state.PlayerID(g.d.Player)
	case kp.Turn == 0:
		b.prio = 0
	case g.d.Sub == dirac.ChDivide && (!g.passed || stepOf[kp.Step] == stepOf[b.step]):
		// a combat damage division comes before the step gives anyone priority, and
		// the second one follows the first-strike round without it moving
	case kp.Step != b.step || !b.seen || b.divided || (g.passed || b.resolving) && !g.s.FeatResolving():
		b.prio = state.PlayerID(kp.Active)
	}
	b.step, b.seen, b.resolving, b.divided = kp.Step, true, g.s.FeatResolving(), g.d.Sub == dirac.ChDivide
	gm.Priority = b.prio
	// a kernel stopped at a limit reads as over, but the game has no result
	over := kp.Over && g.stalled == 0
	gm.Over, gm.Winner, gm.Draw = over, 0, false
	if over {
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
		if kp.Turn > 0 {
			if uint8(p) == kp.Active {
				g.landsPlayed[p] = kp.Lands[p]
			}
			b.landOpen[p] = g.landsPlayed[p] < g.s.FeatLandLimit(uint8(p))
		}
		pl.Lost = gm.Over && !gm.Draw && gm.Winner != state.PlayerID(p)
		pl.Commanders, pl.CmdCasts, pl.CmdDamage = pl.Commanders[:0], pl.CmdCasts[:0], pl.CmdDamage[:0]
		// the projection carries a seat's totals, saturating at 255; a partner
		// (slot 1, CR 702.124) takes its own count out of them, the first
		// commander the rest, or its exact count once a total has saturated
		part := func(total uint8, exact func(j uint8) uint8, j uint8) int32 {
			if j == 1 || total == 255 {
				return int32(exact(j))
			}
			return int32(total) - int32(exact(1))
		}
		for j := uint8(0); j < 2; j++ {
			if c := g.s.FeatCommanderN(uint8(p), j); c != dirac.NoObj {
				pl.Commanders = append(pl.Commanders, g.ids.obj(uint8(c)))
				pl.CmdCasts = append(pl.CmdCasts, part(kp.CmdCast[p], func(j uint8) uint8 { return g.s.FeatCommanderCastN(uint8(p), j) }, j))
			}
		}
		// CmdDamage is dense over the match's commanders, seat order then each
		// seat's slot order: the damage this seat took from each one (CR 903.10a)
		for q := range gm.Players {
			for j := uint8(0); j < 2; j++ {
				if g.s.FeatCommanderN(uint8(q), j) != dirac.NoObj {
					pl.CmdDamage = append(pl.CmdDamage, part(kp.CmdDmg[p][q], func(j uint8) uint8 { return g.s.FeatCommanderDamageN(uint8(p), uint8(q), j) }, j))
				}
			}
		}
		pl.LandsPlayed = 0
		if uint8(p) == kp.Active {
			pl.LandsPlayed = int32(kp.Lands[p])
		}
		for k := 0; k < 6; k++ {
			pl.Pool[k] = int32(kp.Pool[p][k])
		}
		pl.LifeGainedThisTurn, pl.LifeLostThisTurn = g.s.FeatLifeThisTurn(uint8(p))
		pl.SpellsCastThisTurn, pl.TokensCreatedThisTurn = g.tally[p].spells, g.tally[p].tokens
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
		FaceDown: po.Flags&dirac.ProjFaceDown != 0, PhasedOut: po.Zone.Phased(), Damage: int32(po.Damage),
		IsToken: po.Token, IsCopy: po.Flags&dirac.ProjCopy != 0, IsAttacking: po.Flags&dirac.ProjAttacking != 0,
		Monstrous: po.Flags&dirac.ProjMonstrous != 0, Suspected: po.Flags2&dirac.ProjSuspected != 0,
		ActivatedThisTurn: int32(g.s.FeatActivations(dirac.Obj(i))), FaceIdx: g.tbl.Cards[printed].Face}
	if card != nil && printed != po.Card {
		o.CopyFace = g.face(po.Card)
	}
	o.SyncFace()
	if int(i) < len(g.exiled) && len(g.exiled[i]) > 0 {
		o.ExiledCards = g.exiled[i]
	}
	if po.Attached != dirac.NoObj {
		o.AttachedTo = g.ids.obj(uint8(po.Attached))
	}
	if o.IsAttacking && dirac.TCode(po.Attacks) >= dirac.PlayerTarget0 && dirac.TCode(po.Attacks) != dirac.NoTarget {
		o.Attacking = state.PlayerID(dirac.TCode(po.Attacks) - dirac.PlayerTarget0)
	} else if o.IsAttacking && dirac.IsObjTarget(dirac.TCode(po.Attacks)) {
		o.Attacking = state.PlayerID(g.kp.Objs[po.Attacks].Ctrl)
	}
	if o.Zone != state.ZBattlefield && g.s.FeatSickOff(dirac.Obj(i)) {
		o.SummonSick = true
	}
	for k, n := range po.Counters {
		if n != 0 && canon.CounterNames[k] != "" {
			o.AddCounter(canon.CounterNames[k], int32(n))
		}
	}
	// a kind removed to none stays listed at zero while the permanent stays
	listed := g.s.FeatCountersListed(dirac.Obj(i), &po.Counters)
	for k := range po.Counters {
		if listed&(1<<k) != 0 && po.Counters[k] == 0 && canon.CounterNames[k] != "" {
			o.Counters = append(o.Counters, state.Counter{Kind: canon.CounterNames[k]})
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
	case kop.ZoneCommand:
		return state.ZCommand
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
		for _, z := range []state.Zone{state.ZLibrary, state.ZHand, state.ZBattlefield, state.ZGraveyard, state.ZExile, state.ZCommand} {
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
		f := &b.facts[o.ID-1]
		f.optional, f.decider = false, 0
		if gi := g.tr.StackAbility(e.Card, e.Card, e.Ability); e.Kind == 2 && g.triggerOptional(e.Card, gi) {
			f.optional, f.decider = true, state.PlayerID(e.Ctrl)
		}
		if g.s.FeatStackKicked(i) {
			o.CastFlags |= state.FlagKicked
		}
		// gorge writes X as the cast commits, not as it is announced
		if !building || i < int(kp.NStack)-1 {
			o.X = int32(e.X)
		}
		o.Targets = o.Targets[:0]
		for si, t := range e.Targets {
			if dirac.TCode(t) != dirac.NoTarget {
				o.Targets = append(o.Targets, g.stateTarget(uint8(t)))
			}
			for j := 0; j < int(e.NExtra); j++ {
				if int(e.ExtraSlot[j]) == si {
					o.Targets = append(o.Targets, g.stateTarget(uint8(e.Extra[j])))
				}
			}
		}
	}
}

// latentSA stands for an ability no face lists and no card text describes: it is on
// the stack, and has no printed text to show.
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
	if sa := g.reflexiveSA(e.Card); sa != nil {
		return sa
	}
	if e.Kind == 1 {
		if _, sa := g.grantedSA(e.Card, e.Ability); sa != nil {
			return sa
		}
	}
	return latentSA
}

// reflexiveSA is what a "when you do" on card c runs with its target. Only a targeted
// one is read, for what the target was allowed to be.
func (g *Game) reflexiveSA(c dirac.CardID) *cards.SA {
	f := g.face(c)
	if f == nil {
		return nil
	}
	find := func(sa *cards.SA) *cards.SA {
		for x := sa; x != nil; x = x.Sub {
			if x.API != "ImmediateTrigger" {
				continue
			}
			run := cards.ResolveSVar(f.SVars, x.Params["Execute"])
			if run != nil && run.Params["ValidTgts"] != "" {
				return run
			}
		}
		return nil
	}
	for _, sa := range f.Abilities {
		if run := find(sa); run != nil {
			return run
		}
	}
	for i := range f.Triggers {
		if run := find(cards.ResolveSVar(f.SVars, f.Triggers[i].Params["Execute"])); run != nil {
			return run
		}
	}
	return nil
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
	case dirac.TCode(t) == dirac.StackTargetGone:
		return state.Target{}
	case t >= canon.RefPlayer:
		return state.Target{Player: state.PlayerID(t - canon.RefPlayer), IsPlayer: true}
	}
	return state.Target{Obj: g.ids.ref(t, &g.kp)}
}

// blockOrder is where id stands in its attacker's block, one that has left first.
func (g *Game) blockOrder(id state.ObjID) int {
	if id == 0 {
		return -1
	}
	return int(g.s.FeatBlockOrder(dirac.Obj(g.RefOf(id))))
}

// boardCombat lists each attacker's blockers in block order, a blocker that has since
// left as a zero in its place, as gorge's log folds it: a blocked creature stays
// blocked (CR 509.1h) and the object that left is a new one (CR 400.7).
func (g *Game) boardCombat() {
	b := &g.board
	kp := &g.kp
	for i := 0; i < int(kp.NObj); i++ {
		if !g.s.FeatBlocked(dirac.Obj(i)) {
			b.blockers[i] = b.blockers[i][:0]
		}
		for j, id := range b.blockers[i] {
			if id != 0 && !g.blocks(id, dirac.Obj(i)) {
				b.blockers[i][j] = 0
			}
		}
	}
	for i := 0; i < int(kp.NObj); i++ {
		po := &kp.Objs[i]
		if po.Zone == kop.ZoneBattlefield && po.Flags&dirac.ProjBlocking != 0 && po.Blocks != dirac.NoObj {
			if id := g.ids.obj(uint8(i)); !slices.Contains(b.blockers[po.Blocks], id) {
				b.blockers[po.Blocks] = append(b.blockers[po.Blocks], id)
				slices.SortStableFunc(b.blockers[po.Blocks], func(x, y state.ObjID) int { return cmp.Compare(g.blockOrder(x), g.blockOrder(y)) })
			}
		}
	}
	for i := 0; i < int(kp.NObj); i++ {
		if len(b.blockers[i]) > 0 {
			b.g.Objs[g.ids.obj(uint8(i))-1].BlockedBy = append(b.g.Objs[g.ids.obj(uint8(i))-1].BlockedBy, b.blockers[i]...)
		}
	}
}

// blocks reports whether object id is on the battlefield blocking attacker.
func (g *Game) blocks(id state.ObjID, attacker dirac.Obj) bool {
	r := g.RefOf(id)
	if int(r) >= int(g.kp.NObj) {
		return false
	}
	po := &g.kp.Objs[r]
	return po.Zone == kop.ZoneBattlefield && po.Flags&dirac.ProjBlocking != 0 && po.Blocks == attacker
}
